package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"io"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/blake2b"
)

type tileVariantID uint16 // 1-based

type tileLibRef struct {
	Tag     tagID
	Variant tileVariantID
}

type tileSeq map[string][]tileLibRef

func (tseq tileSeq) Variants() ([]tileVariantID, int, int) {
	maxtag := 0
	for _, refs := range tseq {
		for _, ref := range refs {
			if maxtag < int(ref.Tag) {
				maxtag = int(ref.Tag)
			}
		}
	}
	vars := make([]tileVariantID, maxtag+1)
	var kept, dropped int
	for _, refs := range tseq {
		for _, ref := range refs {
			if vars[int(ref.Tag)] != 0 {
				dropped++
			} else {
				kept++
			}
			vars[int(ref.Tag)] = ref.Variant
		}
	}
	return vars, kept, dropped
}

type tileLibrary struct {
	includeNoCalls bool
	skipOOO        bool
	taglib         *tagLibrary
	variant        [][][blake2b.Size256]byte
	refseqs        map[string]map[string][]tileLibRef
	compactGenomes map[string][]tileVariantID
	// count [][]int
	// seq map[[blake2b.Size]byte][]byte
	variants int
	// if non-nil, write out any tile variants added while tiling
	encoder *gob.Encoder

	mtx sync.Mutex
}

func (tilelib *tileLibrary) loadTagSet(newtagset [][]byte) error {
	// Loading a tagset means either passing it through to the
	// output (if it's the first one we've seen), or just ensuring
	// it doesn't disagree with what we already have.
	if len(newtagset) == 0 {
		return nil
	}
	tilelib.mtx.Lock()
	defer tilelib.mtx.Unlock()
	if tilelib.taglib == nil || tilelib.taglib.Len() == 0 {
		tilelib.taglib = &tagLibrary{}
		err := tilelib.taglib.setTags(newtagset)
		if err != nil {
			return err
		}
		if tilelib.encoder != nil {
			err = tilelib.encoder.Encode(LibraryEntry{
				TagSet: newtagset,
			})
			if err != nil {
				return err
			}
		}
	} else if tilelib.taglib.Len() != len(newtagset) {
		return fmt.Errorf("cannot merge libraries with differing tagsets")
	} else {
		current := tilelib.taglib.Tags()
		for i := range newtagset {
			if !bytes.Equal(newtagset[i], current[i]) {
				return fmt.Errorf("cannot merge libraries with differing tagsets")
			}
		}
	}
	return nil
}

func (tilelib *tileLibrary) loadTileVariants(tvs []TileVariant, variantmap map[tileLibRef]tileVariantID) error {
	for _, tv := range tvs {
		// Assign a new variant ID (unique across all inputs)
		// for each input variant.
		variantmap[tileLibRef{Tag: tv.Tag, Variant: tv.Variant}] = tilelib.getRef(tv.Tag, tv.Sequence).Variant
	}
	return nil
}

func (tilelib *tileLibrary) loadCompactGenomes(cgs []CompactGenome, variantmap map[tileLibRef]tileVariantID, onLoadGenome func(CompactGenome)) error {
	log.Debugf("loadCompactGenomes: %d", len(cgs))
	var wg sync.WaitGroup
	errs := make(chan error, 1)
	for _, cg := range cgs {
		wg.Add(1)
		cg := cg
		go func() {
			defer wg.Done()
			for i, variant := range cg.Variants {
				if len(errs) > 0 {
					return
				}
				if variant == 0 {
					continue
				}
				tag := tagID(i / 2)
				newvariant, ok := variantmap[tileLibRef{Tag: tag, Variant: variant}]
				if !ok {
					err := fmt.Errorf("oops: genome %q has variant %d for tag %d, but that variant was not in its library", cg.Name, variant, tag)
					select {
					case errs <- err:
					default:
					}
					return
				}
				log.Tracef("loadCompactGenomes: cg %s tag %d variant %d => %d", cg.Name, tag, variant, newvariant)
				cg.Variants[i] = newvariant
			}
			if onLoadGenome != nil {
				onLoadGenome(cg)
			}
			if tilelib.encoder != nil {
				err := tilelib.encoder.Encode(LibraryEntry{
					CompactGenomes: []CompactGenome{cg},
				})
				if err != nil {
					select {
					case errs <- err:
					default:
					}
					return
				}
			}
			if tilelib.compactGenomes != nil {
				tilelib.mtx.Lock()
				defer tilelib.mtx.Unlock()
				tilelib.compactGenomes[cg.Name] = cg.Variants
			}
		}()
	}
	wg.Wait()
	go close(errs)
	return <-errs
}

func (tilelib *tileLibrary) loadCompactSequences(cseqs []CompactSequence, variantmap map[tileLibRef]tileVariantID) error {
	log.Debugf("loadCompactSequences: %d", len(cseqs))
	for _, cseq := range cseqs {
		for _, tseq := range cseq.TileSequences {
			for i, libref := range tseq {
				if libref.Variant == 0 {
					// No variant (e.g., import
					// dropped tiles with
					// no-calls) = no translation.
					continue
				}
				v, ok := variantmap[libref]
				if !ok {
					return fmt.Errorf("oops: CompactSequence %q has variant %d for tag %d, but that variant was not in its library", cseq.Name, libref.Variant, libref.Tag)
				}
				tseq[i].Variant = v
			}
		}
		if tilelib.encoder != nil {
			if err := tilelib.encoder.Encode(LibraryEntry{
				CompactSequences: []CompactSequence{cseq},
			}); err != nil {
				return err
			}
		}
	}
	tilelib.mtx.Lock()
	defer tilelib.mtx.Unlock()
	if tilelib.refseqs == nil {
		tilelib.refseqs = map[string]map[string][]tileLibRef{}
	}
	for _, cseq := range cseqs {
		tilelib.refseqs[cseq.Name] = cseq.TileSequences
	}
	return nil
}

// Load library data from rdr. Tile variants might be renumbered in
// the process; in that case, genomes variants will be renumbered to
// match.
//
// If onLoadGenome is non-nil, call it on each CompactGenome entry.
func (tilelib *tileLibrary) LoadGob(ctx context.Context, rdr io.Reader, onLoadGenome func(CompactGenome)) error {
	cgs := []CompactGenome{}
	cseqs := []CompactSequence{}
	variantmap := map[tileLibRef]tileVariantID{}
	err := DecodeLibrary(rdr, func(ent *LibraryEntry) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := tilelib.loadTagSet(ent.TagSet); err != nil {
			return err
		}
		if err := tilelib.loadTileVariants(ent.TileVariants, variantmap); err != nil {
			return err
		}
		cgs = append(cgs, ent.CompactGenomes...)
		cseqs = append(cseqs, ent.CompactSequences...)
		return nil
	})
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	err = tilelib.loadCompactGenomes(cgs, variantmap, onLoadGenome)
	if err != nil {
		return err
	}
	err = tilelib.loadCompactSequences(cseqs, variantmap)
	if err != nil {
		return err
	}
	return nil
}

type importStats struct {
	InputFile              string
	InputLabel             string
	InputLength            int
	InputCoverage          int
	TileCoverage           int
	PathLength             int
	DroppedOutOfOrderTiles int
}

func (tilelib *tileLibrary) TileFasta(filelabel string, rdr io.Reader) (tileSeq, []importStats, error) {
	ret := tileSeq{}
	type jobT struct {
		label string
		fasta []byte
	}
	todo := make(chan jobT)
	scanner := bufio.NewScanner(rdr)
	go func() {
		defer close(todo)
		var fasta []byte
		var seqlabel string
		for scanner.Scan() {
			buf := scanner.Bytes()
			if len(buf) > 0 && buf[0] == '>' {
				todo <- jobT{seqlabel, fasta}
				seqlabel, fasta = strings.SplitN(string(buf[1:]), " ", 2)[0], nil
				log.Debugf("%s %s reading fasta", filelabel, seqlabel)
			} else {
				fasta = append(fasta, bytes.ToLower(buf)...)
			}
		}
		todo <- jobT{seqlabel, fasta}
	}()
	type foundtag struct {
		pos    int
		tagid  tagID
		taglen int
	}
	found := make([]foundtag, 2000000)
	path := make([]tileLibRef, 2000000)
	totalFoundTags := 0
	totalPathLen := 0
	skippedSequences := 0
	stats := make([]importStats, 0, len(todo))
	for job := range todo {
		if len(job.fasta) == 0 {
			continue
		} else if strings.Contains(job.label, "_") {
			skippedSequences++
			continue
		}
		log.Debugf("%s %s tiling", filelabel, job.label)

		found = found[:0]
		tilelib.taglib.FindAll(job.fasta, func(tagid tagID, pos, taglen int) {
			found = append(found, foundtag{pos: pos, tagid: tagid, taglen: taglen})
		})
		totalFoundTags += len(found)

		basesOut := 0
		skipped := 0
		path = path[:0]
		last := foundtag{tagid: -1}
		if tilelib.skipOOO {
			keep := longestIncreasingSubsequence(len(found), func(i int) int { return int(found[i].tagid) })
			for i, x := range keep {
				found[i] = found[x]
			}
			skipped = len(found) - len(keep)
			found = found[:len(keep)]
		}
		for i, f := range found {
			log.Tracef("%s %s found[%d] == %#v", filelabel, job.label, i, f)
			if last.tagid < 0 {
				// first tag in sequence
				last = foundtag{tagid: f.tagid}
				continue
			}
			libref := tilelib.getRef(last.tagid, job.fasta[last.pos:f.pos+f.taglen])
			path = append(path, libref)
			if libref.Variant > 0 {
				// Count output coverage from
				// the end of the previous tag
				// (if any) to the end of the
				// current tag, IOW don't
				// double-count coverage for
				// the previous tag.
				basesOut += countBases(job.fasta[last.pos+last.taglen : f.pos+f.taglen])
			} else {
				// If we dropped this tile
				// (because !includeNoCalls),
				// set taglen=0 so the
				// overlapping tag is counted
				// toward coverage on the
				// following tile.
				f.taglen = 0
			}
			last = f
		}
		if last.tagid < 0 {
			log.Warnf("%s %s no tags found", filelabel, job.label)
		} else {
			libref := tilelib.getRef(last.tagid, job.fasta[last.pos:])
			path = append(path, libref)
			if libref.Variant > 0 {
				basesOut += countBases(job.fasta[last.pos+last.taglen:])
			}
		}

		pathcopy := make([]tileLibRef, len(path))
		copy(pathcopy, path)
		ret[job.label] = pathcopy

		basesIn := countBases(job.fasta)
		log.Infof("%s %s fasta in %d coverage in %d coverage out %d path len %d skipped %d", filelabel, job.label, len(job.fasta), basesIn, basesOut, len(path), skipped)
		stats = append(stats, importStats{
			InputFile:              filelabel,
			InputLabel:             job.label,
			InputLength:            len(job.fasta),
			InputCoverage:          basesIn,
			TileCoverage:           basesOut,
			PathLength:             len(path),
			DroppedOutOfOrderTiles: skipped,
		})

		totalPathLen += len(path)
	}
	log.Printf("%s tiled with total path len %d in %d sequences (skipped %d sequences with '_' in name, skipped %d out-of-order tags)", filelabel, totalPathLen, len(ret), skippedSequences, totalFoundTags-totalPathLen)
	return ret, stats, scanner.Err()
}

func (tilelib *tileLibrary) Len() int {
	tilelib.mtx.Lock()
	defer tilelib.mtx.Unlock()
	return tilelib.variants
}

// Return a tileLibRef for a tile with the given tag and sequence,
// adding the sequence to the library if needed.
func (tilelib *tileLibrary) getRef(tag tagID, seq []byte) tileLibRef {
	if !tilelib.includeNoCalls {
		for _, b := range seq {
			if b != 'a' && b != 'c' && b != 'g' && b != 't' {
				// return "tile not found" if seq has any
				// no-calls
				return tileLibRef{Tag: tag}
			}
		}
	}
	tilelib.mtx.Lock()
	// if tilelib.seq == nil {
	// 	tilelib.seq = map[[blake2b.Size]byte][]byte{}
	// }
	if tilelib.variant == nil && tilelib.taglib != nil {
		tilelib.variant = make([][][blake2b.Size256]byte, tilelib.taglib.Len())
	}
	if int(tag) >= len(tilelib.variant) {
		// If we haven't seen the tag library yet (as in a
		// merge), tilelib.taglib.Len() is zero. We can still
		// behave correctly, we just need to expand the
		// tilelib.variant slice as needed.
		if int(tag) >= cap(tilelib.variant) {
			// Allocate 2x capacity.
			newslice := make([][][blake2b.Size256]byte, int(tag)+1, (int(tag)+1)*2)
			copy(newslice, tilelib.variant)
			tilelib.variant = newslice[:int(tag)+1]
		} else {
			// Use previously allocated capacity, avoiding
			// copy.
			tilelib.variant = tilelib.variant[:int(tag)+1]
		}
	}
	seqhash := blake2b.Sum256(seq)
	for i, varhash := range tilelib.variant[tag] {
		if varhash == seqhash {
			tilelib.mtx.Unlock()
			return tileLibRef{Tag: tag, Variant: tileVariantID(i + 1)}
		}
	}
	tilelib.variants++
	tilelib.variant[tag] = append(tilelib.variant[tag], seqhash)
	// tilelib.seq[seqhash] = append([]byte(nil), seq...)
	variant := tileVariantID(len(tilelib.variant[tag]))
	tilelib.mtx.Unlock()

	if tilelib.encoder != nil {
		tilelib.encoder.Encode(LibraryEntry{
			TileVariants: []TileVariant{{
				Tag:      tag,
				Variant:  variant,
				Blake2b:  seqhash,
				Sequence: seq,
			}},
		})
	}
	return tileLibRef{Tag: tag, Variant: variant}
}

func countBases(seq []byte) int {
	n := 0
	for _, c := range seq {
		if isbase[c] {
			n++
		}
	}
	return n
}
