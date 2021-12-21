// Copyright (C) The Lightning Authors. All rights reserved.
//
// SPDX-License-Identifier: AGPL-3.0

package lightning

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"git.arvados.org/arvados.git/sdk/go/arvados"
	"github.com/arvados/lightning/hgvs"
	"github.com/kshedden/gonpy"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2b"
)

type sliceNumpy struct {
	filter  filter
	threads int
}

func (cmd *sliceNumpy) RunCommand(prog string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var err error
	defer func() {
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", err)
		}
	}()
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.SetOutput(stderr)
	pprof := flags.String("pprof", "", "serve Go profile data at http://`[addr]:port`")
	runlocal := flags.Bool("local", false, "run on local host (default: run in an arvados container)")
	projectUUID := flags.String("project", "", "project `UUID` for output data")
	priority := flags.Int("priority", 500, "container request priority")
	inputDir := flags.String("input-dir", "./in", "input `directory`")
	outputDir := flags.String("output-dir", "./out", "output `directory`")
	ref := flags.String("ref", "", "reference name (if blank, choose last one that appears in input)")
	regionsFilename := flags.String("regions", "", "only output columns/annotations that intersect regions in specified bed `file`")
	expandRegions := flags.Int("expand-regions", 0, "expand specified regions by `N` base pairs on each side`")
	mergeOutput := flags.Bool("merge-output", false, "merge output into one matrix.npy and one matrix.annotations.csv")
	flags.IntVar(&cmd.threads, "threads", 16, "number of memory-hungry assembly threads")
	cmd.filter.Flags(flags)
	err = flags.Parse(args)
	if err == flag.ErrHelp {
		err = nil
		return 0
	} else if err != nil {
		return 2
	}

	if *pprof != "" {
		go func() {
			log.Println(http.ListenAndServe(*pprof, nil))
		}()
	}

	if !*runlocal {
		runner := arvadosContainerRunner{
			Name:        "lightning slice-numpy",
			Client:      arvados.NewClientFromEnv(),
			ProjectUUID: *projectUUID,
			RAM:         750000000000,
			VCPUs:       96,
			Priority:    *priority,
			KeepCache:   2,
			APIAccess:   true,
		}
		err = runner.TranslatePaths(inputDir, regionsFilename)
		if err != nil {
			return 1
		}
		runner.Args = []string{"slice-numpy", "-local=true",
			"-pprof=:6060",
			"-input-dir=" + *inputDir,
			"-output-dir=/mnt/output",
			"-threads=" + fmt.Sprintf("%d", cmd.threads),
			"-regions=" + *regionsFilename,
			"-expand-regions=" + fmt.Sprintf("%d", *expandRegions),
			"-merge-output=" + fmt.Sprintf("%v", *mergeOutput),
		}
		runner.Args = append(runner.Args, cmd.filter.Args()...)
		var output string
		output, err = runner.Run()
		if err != nil {
			return 1
		}
		fmt.Fprintln(stdout, output)
		return 0
	}

	infiles, err := allGobFiles(*inputDir)
	if err != nil {
		return 1
	}
	if len(infiles) == 0 {
		err = fmt.Errorf("no input files found in %s", *inputDir)
		return 1
	}
	sort.Strings(infiles)

	var cgnames []string
	var refseq map[string][]tileLibRef
	var reftiledata = make(map[tileLibRef][]byte, 11000000)
	in0, err := open(infiles[0])
	if err != nil {
		return 1
	}

	matchGenome, err := regexp.Compile(cmd.filter.MatchGenome)
	if err != nil {
		err = fmt.Errorf("-match-genome: invalid regexp: %q", cmd.filter.MatchGenome)
		return 1
	}

	taglen := -1
	DecodeLibrary(in0, strings.HasSuffix(infiles[0], ".gz"), func(ent *LibraryEntry) error {
		if len(ent.TagSet) > 0 {
			taglen = len(ent.TagSet[0])
		}
		for _, cseq := range ent.CompactSequences {
			if cseq.Name == *ref || *ref == "" {
				refseq = cseq.TileSequences
			}
		}
		for _, cg := range ent.CompactGenomes {
			if matchGenome.MatchString(cg.Name) {
				cgnames = append(cgnames, cg.Name)
			}
		}
		for _, tv := range ent.TileVariants {
			if tv.Ref {
				reftiledata[tileLibRef{tv.Tag, tv.Variant}] = tv.Sequence
			}
		}
		return nil
	})
	if err != nil {
		return 1
	}
	in0.Close()
	if refseq == nil {
		err = fmt.Errorf("%s: reference sequence not found", infiles[0])
		return 1
	}
	if taglen < 0 {
		err = fmt.Errorf("tagset not found")
		return 1
	}
	if len(cgnames) == 0 {
		err = fmt.Errorf("no genomes found matching regexp %q", cmd.filter.MatchGenome)
		return 1
	}
	sort.Strings(cgnames)

	{
		labelsFilename := *outputDir + "/labels.csv"
		log.Infof("writing labels to %s", labelsFilename)
		var f *os.File
		f, err = os.Create(labelsFilename)
		if err != nil {
			return 1
		}
		defer f.Close()
		for i, name := range cgnames {
			_, err = fmt.Fprintf(f, "%d,%q\n", i, trimFilenameForLabel(name))
			if err != nil {
				err = fmt.Errorf("write %s: %w", labelsFilename, err)
				return 1
			}
		}
		err = f.Close()
		if err != nil {
			err = fmt.Errorf("close %s: %w", labelsFilename, err)
			return 1
		}
	}

	log.Info("indexing reference tiles")
	type reftileinfo struct {
		variant  tileVariantID
		seqname  string // chr1
		pos      int    // distance from start of chromosome to starttag
		tiledata []byte // acgtggcaa...
	}
	isdup := map[tagID]bool{}
	reftile := map[tagID]*reftileinfo{}
	for seqname, cseq := range refseq {
		pos := 0
		for _, libref := range cseq {
			tiledata := reftiledata[libref]
			if len(tiledata) == 0 {
				err = fmt.Errorf("missing tiledata for tag %d variant %d in %s in ref", libref.Tag, libref.Variant, seqname)
				return 1
			}
			if isdup[libref.Tag] {
				log.Printf("dropping reference tile %+v from %s @ %d, tag not unique", libref, seqname, pos)
			} else if reftile[libref.Tag] != nil {
				log.Printf("dropping reference tile %+v from %s @ %d, tag not unique", tileLibRef{Tag: libref.Tag, Variant: reftile[libref.Tag].variant}, reftile[libref.Tag].seqname, reftile[libref.Tag].pos)
				delete(reftile, libref.Tag)
				log.Printf("dropping reference tile %+v from %s @ %d, tag not unique", libref, seqname, pos)
				isdup[libref.Tag] = true
			} else {
				reftile[libref.Tag] = &reftileinfo{
					seqname:  seqname,
					variant:  libref.Variant,
					tiledata: tiledata,
					pos:      pos,
				}
			}
			pos += len(tiledata) - taglen
		}
		log.Printf("... %s done, len %d", seqname, pos+taglen)
	}

	var mask *mask
	if *regionsFilename != "" {
		log.Printf("loading regions from %s", *regionsFilename)
		mask, err = makeMask(*regionsFilename, *expandRegions)
		if err != nil {
			return 1
		}
		log.Printf("before applying mask, len(reftile) == %d", len(reftile))
		log.Printf("deleting reftile entries for regions outside %d intervals", mask.Len())
		for tag, rt := range reftile {
			if !mask.Check(strings.TrimPrefix(rt.seqname, "chr"), rt.pos, rt.pos+len(rt.tiledata)) {
				delete(reftile, tag)
			}
		}
		log.Printf("after applying mask, len(reftile) == %d", len(reftile))
	}

	var toMerge [][]int16
	if *mergeOutput {
		toMerge = make([][]int16, len(infiles))
	}

	throttleMem := throttle{Max: cmd.threads} // TODO: estimate using mem and data size
	throttleNumpyMem := throttle{Max: cmd.threads/2 + 1}
	log.Info("generating annotations and numpy matrix for each slice")
	var done int64
	for infileIdx, infile := range infiles {
		infileIdx, infile := infileIdx, infile
		throttleMem.Go(func() error {
			seq := make(map[tagID][]TileVariant, 50000)
			cgs := make(map[string]CompactGenome, len(cgnames))
			f, err := open(infile)
			if err != nil {
				return err
			}
			defer f.Close()
			log.Infof("%04d: reading %s", infileIdx, infile)
			err = DecodeLibrary(f, strings.HasSuffix(infile, ".gz"), func(ent *LibraryEntry) error {
				for _, tv := range ent.TileVariants {
					if tv.Ref {
						continue
					}
					if mask != nil && reftile[tv.Tag] == nil {
						// Don't waste
						// time/memory on
						// masked-out tiles.
						continue
					}
					variants := seq[tv.Tag]
					if len(variants) == 0 {
						variants = make([]TileVariant, 100)
					}
					for len(variants) <= int(tv.Variant) {
						variants = append(variants, TileVariant{})
					}
					variants[int(tv.Variant)] = tv
					seq[tv.Tag] = variants
				}
				for _, cg := range ent.CompactGenomes {
					if matchGenome.MatchString(cg.Name) {
						cgs[cg.Name] = cg
					}
				}
				return nil
			})
			if err != nil {
				return err
			}
			tagstart := cgs[cgnames[0]].StartTag
			tagend := cgs[cgnames[0]].EndTag

			// TODO: filters

			log.Infof("%04d: renumber/dedup variants for tags %d-%d", infileIdx, tagstart, tagend)
			variantRemap := make([][]tileVariantID, tagend-tagstart)
			throttleCPU := throttle{Max: runtime.GOMAXPROCS(0)}
			for tag, variants := range seq {
				tag, variants := tag, variants
				throttleCPU.Acquire()
				go func() {
					defer throttleCPU.Release()
					count := make(map[[blake2b.Size256]byte]int, len(variants))

					rt := reftile[tag]
					if rt != nil {
						count[blake2b.Sum256(rt.tiledata)] = 0
					}

					for _, cg := range cgs {
						idx := int(tag-tagstart) * 2
						if idx < len(cg.Variants) {
							for allele := 0; allele < 2; allele++ {
								v := cg.Variants[idx+allele]
								if v > 0 && len(variants[v].Sequence) > 0 {
									count[variants[v].Blake2b]++
								}
							}
						}
					}
					// hash[i] will be the hash of
					// the variant(s) that should
					// be at rank i (0-based).
					hash := make([][blake2b.Size256]byte, 0, len(count))
					for b := range count {
						hash = append(hash, b)
					}
					sort.Slice(hash, func(i, j int) bool {
						bi, bj := &hash[i], &hash[j]
						if ci, cj := count[*bi], count[*bj]; ci != cj {
							return ci > cj
						} else {
							return bytes.Compare((*bi)[:], (*bj)[:]) < 0
						}
					})
					// rank[b] will be the 1-based
					// new variant number for
					// variants whose hash is b.
					rank := make(map[[blake2b.Size256]byte]tileVariantID, len(hash))
					for i, h := range hash {
						rank[h] = tileVariantID(i + 1)
					}
					// remap[v] will be the new
					// variant number for original
					// variant number v.
					remap := make([]tileVariantID, len(variants))
					for i, tv := range variants {
						remap[i] = rank[tv.Blake2b]
					}
					variantRemap[tag-tagstart] = remap
					if rt != nil {
						rt.variant = rank[blake2b.Sum256(rt.tiledata)]
					}
				}()
			}
			throttleCPU.Wait()

			annotationsFilename := fmt.Sprintf("%s/matrix.%04d.annotations.csv", *outputDir, infileIdx)
			log.Infof("%04d: writing %s", infileIdx, annotationsFilename)
			annof, err := os.Create(annotationsFilename)
			if err != nil {
				return err
			}
			annow := bufio.NewWriterSize(annof, 1<<20)
			outcol := 0
			for tag := tagstart; tag < tagend; tag++ {
				rt, ok := reftile[tag]
				if !ok {
					if mask == nil {
						outcol++
					}
					// Excluded by specified
					// regions, or reference does
					// not use any variant of this
					// tile. (TODO: log this?
					// mention it in annotations?)
					continue
				}
				fmt.Fprintf(annow, "%d,%d,%d,=,%s,%d,,,\n", tag, outcol, rt.variant, rt.seqname, rt.pos)
				variants := seq[tag]
				reftilestr := strings.ToUpper(string(rt.tiledata))
				remap := variantRemap[tag-tagstart]
				done := make([]bool, len(variants))
				for v, tv := range variants {
					v := remap[v]
					if v == rt.variant || done[v] {
						continue
					} else {
						done[v] = true
					}
					if len(tv.Sequence) < taglen || !bytes.HasSuffix(rt.tiledata, tv.Sequence[len(tv.Sequence)-taglen:]) {
						fmt.Fprintf(annow, "%d,%d,%d,,%s,%d,,,\n", tag, outcol, v, rt.seqname, rt.pos)
						continue
					}
					if lendiff := len(rt.tiledata) - len(tv.Sequence); lendiff < -1000 || lendiff > 1000 {
						fmt.Fprintf(annow, "%d,%d,%d,,%s,%d,,,\n", tag, outcol, v, rt.seqname, rt.pos)
						continue
					}
					diffs, _ := hgvs.Diff(reftilestr, strings.ToUpper(string(tv.Sequence)), 0)
					for _, diff := range diffs {
						diff.Position += rt.pos
						fmt.Fprintf(annow, "%d,%d,%d,%s:g.%s,%s,%d,%s,%s,%s\n", tag, outcol, v, rt.seqname, diff.String(), rt.seqname, diff.Position, diff.Ref, diff.New, diff.Left)
					}
				}
				outcol++
			}
			err = annow.Flush()
			if err != nil {
				return err
			}
			err = annof.Close()
			if err != nil {
				return err
			}

			log.Infof("%04d: preparing numpy", infileIdx)
			throttleNumpyMem.Acquire()
			rows := len(cgnames)
			cols := 2 * outcol
			out := make([]int16, rows*cols)
			for row, name := range cgnames {
				out := out[row*cols:]
				outcol := 0
				for col, v := range cgs[name].Variants {
					tag := tagstart + tagID(col/2)
					if mask != nil && reftile[tag] == nil {
						continue
					}
					if variants, ok := seq[tag]; ok && len(variants) > int(v) && len(variants[v].Sequence) > 0 {
						out[outcol] = int16(variantRemap[tag-tagstart][v])
					} else {
						out[outcol] = -1
					}
					outcol++
				}
			}
			seq = nil
			throttleNumpyMem.Release()

			if *mergeOutput {
				log.Infof("%04d: matrix fragment %d rows x %d cols", infileIdx, rows, cols)
				toMerge[infileIdx] = out
			} else {
				fnm := fmt.Sprintf("%s/matrix.%04d.npy", *outputDir, infileIdx)
				err = writeNumpyInt16(fnm, out, rows, cols)
				if err != nil {
					return err
				}
			}
			log.Infof("%s: done (%d/%d)", infile, int(atomic.AddInt64(&done, 1)), len(infiles))
			return nil
		})
	}
	if err = throttleMem.Wait(); err != nil {
		return 1
	}
	if *mergeOutput {
		log.Info("merging output matrix and annotations")

		annoFilename := fmt.Sprintf("%s/matrix.annotations.csv", *outputDir)
		annof, err := os.Create(annoFilename)
		if err != nil {
			return 1
		}
		annow := bufio.NewWriterSize(annof, 1<<20)

		rows := len(cgnames)
		cols := 0
		for _, chunk := range toMerge {
			cols += len(chunk) / rows
		}
		out := make([]int16, rows*cols)
		hgvsCols := map[string][2][]int16{} // hgvs -> [[g0,g1,g2,...], [g0,g1,g2,...]] (slice of genomes for each phase)
		startcol := 0
		for outIdx, chunk := range toMerge {
			chunkcols := len(chunk) / rows
			for row := 0; row < rows; row++ {
				copy(out[row*cols+startcol:], chunk[row*chunkcols:(row+1)*chunkcols])
			}
			toMerge[outIdx] = nil

			annotationsFilename := fmt.Sprintf("%s/matrix.%04d.annotations.csv", *outputDir, outIdx)
			log.Infof("reading %s", annotationsFilename)
			buf, err := os.ReadFile(annotationsFilename)
			if err != nil {
				return 1
			}
			err = os.Remove(annotationsFilename)
			if err != nil {
				return 1
			}
			for _, line := range bytes.Split(buf, []byte{'\n'}) {
				if len(line) == 0 {
					continue
				}
				fields := bytes.SplitN(line, []byte{','}, 9)
				tag, _ := strconv.Atoi(string(fields[0]))
				incol, _ := strconv.Atoi(string(fields[1]))
				tileVariant, _ := strconv.Atoi(string(fields[2]))
				hgvsID := string(fields[3])
				seqname := string(fields[4])
				pos, _ := strconv.Atoi(string(fields[5]))
				refseq := fields[6]
				if hgvsID == "" {
					// Null entry for un-diffable
					// tile variant
					continue
				}
				if hgvsID == "=" {
					// Null entry for ref tile
					continue
				}
				if mask != nil && !mask.Check(strings.TrimPrefix(seqname, "chr"), pos, pos+len(refseq)) {
					// The tile intersects one of
					// the selected regions, but
					// this particular HGVS
					// variant does not.
					continue
				}
				hgvsColPair := hgvsCols[hgvsID]
				if hgvsColPair[0] == nil {
					// values in new columns start
					// out as -1 ("no data yet")
					// or 0 ("=ref") here, may
					// change to 1 ("hgvs variant
					// present") below, either on
					// this line or a future line.
					hgvsColPair = [2][]int16{make([]int16, len(cgnames)), make([]int16, len(cgnames))}
					rt, ok := reftile[tagID(tag)]
					if !ok {
						err = fmt.Errorf("bug: seeing annotations for tag %d, but it has no reftile entry", tag)
						return 1
					}
					for ph := 0; ph < 2; ph++ {
						for row := 0; row < rows; row++ {
							v := chunk[row*chunkcols+incol*2+ph]
							if tileVariantID(v) == rt.variant {
								hgvsColPair[ph][row] = 0
							} else {
								hgvsColPair[ph][row] = -1
							}
						}
					}
					hgvsCols[hgvsID] = hgvsColPair
					hgvsref := hgvs.Variant{
						Position: pos,
						Ref:      string(refseq),
						New:      string(refseq),
					}
					fmt.Fprintf(annow, "%d,%d,%d,%s:g.%s,%s,%d,%s,%s,%s\n", tag, incol+startcol/2, rt.variant, seqname, hgvsref.String(), seqname, pos, refseq, refseq, fields[8])
				}
				fmt.Fprintf(annow, "%d,%d,%d,%s,%s,%d,%s,%s,%s\n", tag, incol+startcol/2, tileVariant, hgvsID, seqname, pos, refseq, fields[7], fields[8])
				for ph := 0; ph < 2; ph++ {
					for row := 0; row < rows; row++ {
						v := chunk[row*chunkcols+incol*2+ph]
						if int(v) == tileVariant {
							hgvsColPair[ph][row] = 1
						}
					}
				}
			}

			startcol += chunkcols
		}
		err = annow.Flush()
		if err != nil {
			return 1
		}
		err = annof.Close()
		if err != nil {
			return 1
		}
		err = writeNumpyInt16(fmt.Sprintf("%s/matrix.npy", *outputDir), out, rows, cols)
		if err != nil {
			return 1
		}
		out = nil

		cols = len(hgvsCols) * 2
		log.Printf("building hgvs-based matrix: %d rows x %d cols", rows, cols)
		out = make([]int16, rows*cols)
		hgvsIDs := make([]string, 0, len(hgvsCols))
		for hgvsID := range hgvsCols {
			hgvsIDs = append(hgvsIDs, hgvsID)
		}
		sort.Strings(hgvsIDs)
		var hgvsLabels bytes.Buffer
		for idx, hgvsID := range hgvsIDs {
			fmt.Fprintf(&hgvsLabels, "%d,%s\n", idx, hgvsID)
			for ph := 0; ph < 2; ph++ {
				hgvscol := hgvsCols[hgvsID][ph]
				for row, val := range hgvscol {
					out[row*cols+idx*2+ph] = val
				}
			}
		}
		err = writeNumpyInt16(fmt.Sprintf("%s/hgvs.npy", *outputDir), out, rows, cols)
		if err != nil {
			return 1
		}

		fnm := fmt.Sprintf("%s/hgvs.annotations.csv", *outputDir)
		log.Printf("writing hgvs labels: %s", fnm)
		err = ioutil.WriteFile(fnm, hgvsLabels.Bytes(), 0777)
		if err != nil {
			return 1
		}
	}
	return 0
}

func writeNumpyInt16(fnm string, out []int16, rows, cols int) error {
	output, err := os.Create(fnm)
	if err != nil {
		return err
	}
	defer output.Close()
	bufw := bufio.NewWriterSize(output, 1<<26)
	npw, err := gonpy.NewWriter(nopCloser{bufw})
	if err != nil {
		return err
	}
	log.WithFields(log.Fields{
		"filename": fnm,
		"rows":     rows,
		"cols":     cols,
	}).Infof("writing numpy: %s", fnm)
	npw.Shape = []int{rows, cols}
	npw.WriteInt16(out)
	err = bufw.Flush()
	if err != nil {
		return err
	}
	return output.Close()
}
