package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	_ "net/http/pprof"
	"os"
	"sort"
	"strings"

	"git.arvados.org/arvados.git/sdk/go/arvados"
	"github.com/kshedden/gonpy"
	log "github.com/sirupsen/logrus"
)

type exportNumpy struct {
	filter filter
}

func (cmd *exportNumpy) RunCommand(prog string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
	inputFilename := flags.String("i", "-", "input `file`")
	outputFilename := flags.String("o", "-", "output `file`")
	annotationsFilename := flags.String("output-annotations", "", "output `file` for tile variant annotations tsv")
	librefsFilename := flags.String("output-onehot2tilevar", "", "when using -one-hot, create tsv `file` mapping column# to tag# and variant#")
	onehot := flags.Bool("one-hot", false, "recode tile variants as one-hot")
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
		if *outputFilename != "-" {
			err = errors.New("cannot specify output file in container mode: not implemented")
			return 1
		}
		runner := arvadosContainerRunner{
			Name:        "lightning export-numpy",
			Client:      arvados.NewClientFromEnv(),
			ProjectUUID: *projectUUID,
			RAM:         128000000000,
			VCPUs:       32,
			Priority:    *priority,
		}
		err = runner.TranslatePaths(inputFilename)
		if err != nil {
			return 1
		}
		runner.Args = []string{"export-numpy", "-local=true",
			fmt.Sprintf("-one-hot=%v", *onehot),
			"-i", *inputFilename,
			"-o", "/mnt/output/matrix.npy",
			"-output-annotations", "/mnt/output/annotations.tsv",
			"-output-onehot2tilevar", "/mnt/output/onehot2tilevar.tsv",
			"-max-variants", fmt.Sprintf("%d", cmd.filter.MaxVariants),
			"-min-coverage", fmt.Sprintf("%f", cmd.filter.MinCoverage),
			"-max-tag", fmt.Sprintf("%d", cmd.filter.MaxTag),
		}
		var output string
		output, err = runner.Run()
		if err != nil {
			return 1
		}
		fmt.Fprintln(stdout, output+"/matrix.npy")
		return 0
	}

	var input io.ReadCloser
	if *inputFilename == "-" {
		input = ioutil.NopCloser(stdin)
	} else {
		input, err = os.Open(*inputFilename)
		if err != nil {
			return 1
		}
		defer input.Close()
	}
	tilelib := &tileLibrary{
		retainNoCalls:       true,
		retainTileSequences: true,
		compactGenomes:      map[string][]tileVariantID{},
	}
	err = tilelib.LoadGob(context.Background(), input, strings.HasSuffix(*inputFilename, ".gz"), nil)
	if err != nil {
		return 1
	}
	err = input.Close()
	if err != nil {
		return 1
	}

	log.Info("filtering")
	cmd.filter.Apply(tilelib)
	log.Info("tidying")
	tilelib.Tidy()

	if *annotationsFilename != "" {
		log.Infof("writing annotations")
		var annow io.WriteCloser
		annow, err = os.OpenFile(*annotationsFilename, os.O_CREATE|os.O_WRONLY, 0666)
		if err != nil {
			return 1
		}
		defer annow.Close()
		err = (&annotatecmd{maxTileSize: 5000}).exportTileDiffs(annow, tilelib)
		if err != nil {
			return 1
		}
		err = annow.Close()
		if err != nil {
			return 1
		}
	}

	log.Info("building numpy array")
	out, rows, cols := cgs2array(tilelib.compactGenomes)
	var output io.WriteCloser
	if *outputFilename == "-" {
		output = nopCloser{stdout}
	} else {
		output, err = os.OpenFile(*outputFilename, os.O_CREATE|os.O_WRONLY, 0777)
		if err != nil {
			return 1
		}
		defer output.Close()
	}
	bufw := bufio.NewWriter(output)
	npw, err := gonpy.NewWriter(nopCloser{bufw})
	if err != nil {
		return 1
	}
	if *onehot {
		log.Info("recoding to onehot")
		recoded, librefs, recodedcols := recodeOnehot(out, cols)
		out, cols = recoded, recodedcols
		if *librefsFilename != "" {
			log.Infof("writing onehot column mapping")
			err = cmd.writeLibRefs(*librefsFilename, tilelib, librefs)
			if err != nil {
				return 1
			}
		}
	}
	log.Info("writing numpy")
	npw.Shape = []int{rows, cols}
	npw.WriteUint16(out)
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

func (*exportNumpy) writeLibRefs(fnm string, tilelib *tileLibrary, librefs []tileLibRef) error {
	f, err := os.OpenFile(fnm, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	defer f.Close()
	for i, libref := range librefs {
		_, err = fmt.Fprintf(f, "%d\t%d\t%d\n", i, libref.Tag, libref.Variant)
		if err != nil {
			return err
		}
	}
	return f.Close()
}

func cgs2array(cgs map[string][]tileVariantID) (data []uint16, rows, cols int) {
	var cgnames []string
	for name := range cgs {
		cgnames = append(cgnames, name)
	}
	sort.Strings(cgnames)

	rows = len(cgs)
	for _, cg := range cgs {
		if cols < len(cg) {
			cols = len(cg)
		}
	}
	data = make([]uint16, rows*cols)
	for row, name := range cgnames {
		for i, v := range cgs[name] {
			data[row*cols+i] = uint16(v)
		}
	}
	return
}

func recodeOnehot(in []uint16, incols int) (out []uint16, librefs []tileLibRef, outcols int) {
	rows := len(in) / incols
	maxvalue := make([]uint16, incols)
	for row := 0; row < rows; row++ {
		for col := 0; col < incols; col++ {
			if v := in[row*incols+col]; maxvalue[col] < v {
				maxvalue[col] = v
			}
		}
	}
	outcol := make([]int, incols)
	dropped := 0
	for incol, maxv := range maxvalue {
		outcol[incol] = outcols
		if maxv == 0 {
			dropped++
		}
		for v := 1; v <= int(maxv); v++ {
			librefs = append(librefs, tileLibRef{Tag: tagID(incol), Variant: tileVariantID(v)})
			outcols++
		}
	}
	log.Printf("recodeOnehot: dropped %d input cols with zero maxvalue", dropped)

	out = make([]uint16, rows*outcols)
	for inidx, row := 0, 0; row < rows; row++ {
		outrow := out[row*outcols:]
		for col := 0; col < incols; col++ {
			if v := in[inidx]; v > 0 {
				outrow[outcol[col]+int(v)-1] = 1
			}
			inidx++
		}
	}
	return
}

type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }
