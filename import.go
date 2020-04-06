package main

import (
	"bufio"
	"compress/gzip"
	"encoding/gob"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"git.arvados.org/arvados.git/sdk/go/arvados"
	log "github.com/sirupsen/logrus"
)

type importer struct {
	tagLibraryFile string
	refFile        string
	outputFile     string
	projectUUID    string
	runLocal       bool
	skipOOO        bool
	encoder        *gob.Encoder
}

func (cmd *importer) RunCommand(prog string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	var err error
	defer func() {
		if err != nil {
			fmt.Fprintf(stderr, "%s\n", err)
		}
	}()
	flags := flag.NewFlagSet("", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&cmd.tagLibraryFile, "tag-library", "", "tag library fasta `file`")
	flags.StringVar(&cmd.refFile, "ref", "", "reference fasta `file`")
	flags.StringVar(&cmd.outputFile, "o", "-", "output `file`")
	flags.StringVar(&cmd.projectUUID, "project", "", "project `UUID` for output data")
	flags.BoolVar(&cmd.runLocal, "local", false, "run on local host (default: run in an arvados container)")
	flags.BoolVar(&cmd.skipOOO, "skip-ooo", false, "skip out-of-order tags")
	priority := flags.Int("priority", 500, "container request priority")
	pprof := flags.String("pprof", "", "serve Go profile data at http://`[addr]:port`")
	err = flags.Parse(args)
	if err == flag.ErrHelp {
		err = nil
		return 0
	} else if err != nil {
		return 2
	} else if cmd.tagLibraryFile == "" {
		fmt.Fprintln(os.Stderr, "cannot import without -tag-library argument")
		return 2
	} else if flags.NArg() == 0 {
		flags.Usage()
		return 2
	}

	if *pprof != "" {
		go func() {
			log.Println(http.ListenAndServe(*pprof, nil))
		}()
	}

	if !cmd.runLocal {
		runner := arvadosContainerRunner{
			Name:        "lightning import",
			Client:      arvados.NewClientFromEnv(),
			ProjectUUID: cmd.projectUUID,
			RAM:         30000000000,
			VCPUs:       16,
			Priority:    *priority,
		}
		err = runner.TranslatePaths(&cmd.tagLibraryFile, &cmd.refFile, &cmd.outputFile)
		if err != nil {
			return 1
		}
		inputs := flags.Args()
		for i := range inputs {
			err = runner.TranslatePaths(&inputs[i])
			if err != nil {
				return 1
			}
		}
		if cmd.outputFile == "-" {
			cmd.outputFile = "/mnt/output/library.gob"
		} else {
			// Not yet implemented, but this should write
			// the collection to an existing collection,
			// possibly even an in-place update.
			err = errors.New("cannot specify output file in container mode: not implemented")
			return 1
		}
		runner.Args = append([]string{"import", "-local=true", "-tag-library", cmd.tagLibraryFile, "-ref", cmd.refFile, "-o", cmd.outputFile}, inputs...)
		var output string
		output, err = runner.Run()
		if err != nil {
			return 1
		}
		fmt.Fprintln(stdout, output+"/library.gob")
		return 0
	}

	infiles, err := listInputFiles(flags.Args())
	if err != nil {
		return 1
	}

	tilelib, err := cmd.loadTileLibrary()
	if err != nil {
		return 1
	}
	go func() {
		for range time.Tick(10 * time.Minute) {
			log.Printf("tilelib.Len() == %d", tilelib.Len())
		}
	}()

	var output io.WriteCloser
	if cmd.outputFile == "-" {
		output = nopCloser{stdout}
	} else {
		output, err = os.OpenFile(cmd.outputFile, os.O_CREATE|os.O_WRONLY, 0777)
		if err != nil {
			return 1
		}
		defer output.Close()
	}
	bufw := bufio.NewWriter(output)
	cmd.encoder = gob.NewEncoder(bufw)

	err = cmd.tileInputs(tilelib, infiles)
	if err != nil {
		return 1
	}
	err = bufw.Flush()
	if err != nil {
		return 1
	}
	err = output.Close()
	if err != nil {
		return 1
	}
	return 0
}

func (cmd *importer) tileFasta(tilelib *tileLibrary, infile string) (tileSeq, error) {
	var input io.ReadCloser
	input, err := os.Open(infile)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	if strings.HasSuffix(infile, ".gz") {
		input, err = gzip.NewReader(input)
		if err != nil {
			return nil, err
		}
		defer input.Close()
	}
	return tilelib.TileFasta(infile, input)
}

func (cmd *importer) loadTileLibrary() (*tileLibrary, error) {
	log.Printf("tag library %s load starting", cmd.tagLibraryFile)
	f, err := os.Open(cmd.tagLibraryFile)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rdr io.ReadCloser = f
	if strings.HasSuffix(cmd.tagLibraryFile, ".gz") {
		rdr, err = gzip.NewReader(f)
		if err != nil {
			return nil, fmt.Errorf("%s: gzip: %s", cmd.tagLibraryFile, err)
		}
		defer rdr.Close()
	}
	var taglib tagLibrary
	err = taglib.Load(rdr)
	if err != nil {
		return nil, err
	}
	if taglib.Len() < 1 {
		return nil, fmt.Errorf("cannot tile: tag library is empty")
	}
	log.Printf("tag library %s load done", cmd.tagLibraryFile)
	return &tileLibrary{taglib: &taglib, skipOOO: cmd.skipOOO}, nil
}

func listInputFiles(paths []string) (files []string, err error) {
	for _, path := range paths {
		if fi, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("%s: stat failed: %s", path, err)
		} else if !fi.IsDir() {
			if !strings.HasSuffix(path, ".2.fasta") || strings.HasSuffix(path, ".2.fasta.gz") {
				files = append(files, path)
			}
			continue
		}
		d, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("%s: open failed: %s", path, err)
		}
		defer d.Close()
		names, err := d.Readdirnames(0)
		if err != nil {
			return nil, fmt.Errorf("%s: readdir failed: %s", path, err)
		}
		sort.Strings(names)
		for _, name := range names {
			if strings.HasSuffix(name, ".vcf") || strings.HasSuffix(name, ".vcf.gz") {
				files = append(files, filepath.Join(path, name))
			} else if strings.HasSuffix(name, ".1.fasta") || strings.HasSuffix(name, ".1.fasta.gz") {
				files = append(files, filepath.Join(path, name))
			}
		}
		d.Close()
	}
	for _, file := range files {
		if strings.HasSuffix(file, ".1.fasta") || strings.HasSuffix(file, ".1.fasta.gz") {
			continue
		} else if _, err := os.Stat(file + ".csi"); err == nil {
			continue
		} else if _, err = os.Stat(file + ".tbi"); err == nil {
			continue
		} else {
			return nil, fmt.Errorf("%s: cannot read without .tbi or .csi index file", file)
		}
	}
	return
}

func (cmd *importer) tileInputs(tilelib *tileLibrary, infiles []string) error {
	starttime := time.Now()
	errs := make(chan error, 1)
	todo := make(chan func() error, len(infiles)*2)
	var encodeJobs sync.WaitGroup
	for _, infile := range infiles {
		infile := infile
		var phases sync.WaitGroup
		phases.Add(2)
		variants := make([][]tileVariantID, 2)
		if strings.HasSuffix(infile, ".1.fasta") || strings.HasSuffix(infile, ".1.fasta.gz") {
			todo <- func() error {
				defer phases.Done()
				log.Printf("%s starting", infile)
				defer log.Printf("%s done", infile)
				tseqs, err := cmd.tileFasta(tilelib, infile)
				variants[0] = tseqs.Variants()
				return err
			}
			infile2 := regexp.MustCompile(`\.1\.fasta(\.gz)?$`).ReplaceAllString(infile, `.2.fasta$1`)
			todo <- func() error {
				defer phases.Done()
				log.Printf("%s starting", infile2)
				defer log.Printf("%s done", infile2)
				tseqs, err := cmd.tileFasta(tilelib, infile2)
				variants[1] = tseqs.Variants()
				return err
			}
		} else {
			for phase := 0; phase < 2; phase++ {
				phase := phase
				todo <- func() error {
					defer phases.Done()
					log.Printf("%s phase %d starting", infile, phase+1)
					defer log.Printf("%s phase %d done", infile, phase+1)
					tseqs, err := cmd.tileGVCF(tilelib, infile, phase)
					variants[phase] = tseqs.Variants()
					return err
				}
			}
		}
		encodeJobs.Add(1)
		go func() {
			defer encodeJobs.Done()
			phases.Wait()
			if len(errs) > 0 {
				return
			}
			ntags := len(variants[0])
			if ntags < len(variants[1]) {
				ntags = len(variants[1])
			}
			flat := make([]tileVariantID, ntags*2)
			for i := 0; i < ntags; i++ {
				for hap := 0; hap < 2; hap++ {
					if i < len(variants[hap]) {
						flat[i*2+hap] = variants[hap][i]
					}
				}
			}
			err := cmd.encoder.Encode(LibraryEntry{
				CompactGenomes: []CompactGenome{{Name: infile, Variants: flat}},
			})
			if err != nil {
				select {
				case errs <- err:
				default:
				}
			}
		}()
	}
	go close(todo)
	var tileJobs sync.WaitGroup
	var running int64
	for i := 0; i < runtime.NumCPU()*9/8+1; i++ {
		tileJobs.Add(1)
		atomic.AddInt64(&running, 1)
		go func() {
			defer tileJobs.Done()
			defer atomic.AddInt64(&running, -1)
			for fn := range todo {
				if len(errs) > 0 {
					return
				}
				err := fn()
				if err != nil {
					select {
					case errs <- err:
					default:
					}
				}
				remain := len(todo) + int(atomic.LoadInt64(&running)) - 1
				ttl := time.Now().Sub(starttime) * time.Duration(remain) / time.Duration(cap(todo)-remain)
				eta := time.Now().Add(ttl)
				log.Printf("progress %d/%d, eta %v (%v)", cap(todo)-remain, cap(todo), eta, ttl)
			}
		}()
	}
	tileJobs.Wait()
	encodeJobs.Wait()
	go close(errs)
	return <-errs
}

func (cmd *importer) tileGVCF(tilelib *tileLibrary, infile string, phase int) (tileseq tileSeq, err error) {
	if cmd.refFile == "" {
		err = errors.New("cannot import vcf: reference data (-ref) not specified")
		return
	}
	args := []string{"bcftools", "consensus", "--fasta-ref", cmd.refFile, "-H", fmt.Sprint(phase + 1), infile}
	indexsuffix := ".tbi"
	if _, err := os.Stat(infile + ".csi"); err == nil {
		indexsuffix = ".csi"
	}
	if out, err := exec.Command("docker", "image", "ls", "-q", "lightning-runtime").Output(); err == nil && len(out) > 0 {
		args = append([]string{
			"docker", "run", "--rm",
			"--log-driver=none",
			"--volume=" + infile + ":" + infile + ":ro",
			"--volume=" + infile + indexsuffix + ":" + infile + indexsuffix + ":ro",
			"--volume=" + cmd.refFile + ":" + cmd.refFile + ":ro",
			"lightning-runtime",
		}, args...)
	}
	consensus := exec.Command(args[0], args[1:]...)
	consensus.Stderr = os.Stderr
	stdout, err := consensus.StdoutPipe()
	defer stdout.Close()
	if err != nil {
		return
	}
	err = consensus.Start()
	if err != nil {
		return
	}
	defer consensus.Wait()
	tileseq, err = tilelib.TileFasta(fmt.Sprintf("%s phase %d", infile, phase+1), stdout)
	if err != nil {
		return
	}
	err = stdout.Close()
	if err != nil {
		return
	}
	err = consensus.Wait()
	if err != nil {
		err = fmt.Errorf("%s phase %d: bcftools: %s", infile, phase, err)
		return
	}
	return
}
