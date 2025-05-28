// mimir-whisper-converter reads Whisper files on disk and generates Mimir
// data Blocks.
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof" //nolint
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/mimir-graphite/v2/pkg/graphite/convert/whisperconverter"
)

const (
	FILELIST  = "filelist"
	DATERANGE = "daterange"
	PASS1     = "pass1"
	PASS2     = "pass2"
)

// This value will be overridden during the build process using -ldflags.
var version = "development"

var (
	namePrefix = flag.String(
		"name-prefix",
		"",
		"Prefix to prepend before every metric name, should include the '.' if necessary.",
	)
	whisperDirectory = flag.String(
		"whisper-directory",
		"/opt/graphite/storage/whisper",
		"The directory that contains the Whisper file structure. The portion of the file paths specified in this flag will be stripped from the metric names.",
	)
	intermediateDirectory = flag.String(
		"intermediate-directory",
		"/tmp/intermediate/",
		"The directory for the intermediate output data, necessary for the conversion process.",
	)
	blocksDirectory = flag.String(
		"blocks-directory",
		"/tmp/blocks/data/",
		"The directory to write finished Mimir blocks.",
	)
	fileFilterPattern = flag.String(
		"file-filter",
		".+\\.wsp$",
		"A regex pattern to be applied to all filenames. Only filenames matching the pattern will be imported.  Does not filter based on path name.",
	)
	threads = flag.Int(
		"threads",
		10, //nolint:gomnd
		"Number of workers threads to process and convert whisper files. In general, the conversion process is RAM-limited, and the amount of RAM used will be the number of threads multiplied by the size of the files being converted.  Therefore with smaller input files, more threads can be used.",
	)
	workerCount = flag.Int(
		"workers",
		1,
		"The number of workers tackling this batch job. Workers are different executions of this binary, usually on different machines.",
	)
	workerID = flag.Int(
		"workerID",
		0,
		"Zero-based index of which worker this instantiation corresponds to.  Indexes should be manually assigned to each worker as part of the system deploying the multi-worker conversion.",
	)
	resumeIntermediate = flag.Bool(
		"resume-intermediate",
		true,
		"If true, existing intermediate files (if any) will be used to resume progress. If false, existing intermediate files will be overwritten.",
	)
	resumeBlocks = flag.Bool(
		"resume-blocks",
		true,
		"If true, dates are skipped if there exists a finished output block for that date. If false, new blocks will always be written, possibly creating duplicate data if blocks already exist at the destination.",
	)
	targetWhisperFiles = flag.String(
		"target-whisper-files",
		"",
		"Path to a file that (will) contain a newline-delimited list of target whisper files for import. The file-filter pattern will be applied to this list. If blank, files will be walked at runtime.",
	)
	customLabels = flag.String(
		"custom-labels",
		"",
		"An optional comma-separated list of extra label name to label value to be applied to all metrics during conversion. This can be useful if you want to mark all metrics as coming from a specific archive, for example. This is applied during the second pass and has no effect on the first pass conversion.",
	)

	versionFlag = flag.Bool("version", false, "Display the version of the binary")
	verboseFlag = flag.Bool("verbose", false, "If true, outputs info logging")
	debugFlag   = flag.Bool("debug", false, "If true, outputs debug logging")
	quietFlag   = flag.Bool("quiet", false, "If true, suppresses logging")

	startDateFlag = flag.String("start-date", "", "The earliest date to process in YYYY-MM-DD format")
	endDateFlag   = flag.String("end-date", "", "The last date to process in YYYY-MM-DD format")
)

// Will be simplifying main() as we go.
//
//nolint:gocyclo
func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Usage of mimir-whisper-converter:

mimir-whisper-converter [arguments] <command>

mimir-whisper-converter is a utility for converting Graphite Whisper archives to
Mimir blocks.

Because archives can be very large, it does this conversion in multiple steps,
designed to run separately to reduce memory consumption.

The commands are:

	filelist	Pre-generate a simple newline-separated list of files to process.
			This is useful when the number of files is extremely large, to avoid
			out-of-memory errors as the file list is built.

			Required flags: --whisper-directory, --target-whisper-files

	daterange	Print the minimum and maximum timestamps for the dataset in a format
			suitable for use as arguments to pass1 and pass2.

			Required flags: --whisper-directory

	pass1		Perform the first pass conversion of Whisper input files to
			intermediate files. The first pass of the conversion reads all Whisper
			files and generates an intermediate file format containing all of the
			metric data on a per-date basis.

			Required flags: , --start-date, --end-date, --whisper-directory, --intermediate-directory

	pass2		Perform the second pass conversion from intermediate files to Mimir
			blocks. The second pass fo the conversion generates Mimir blocks from all
			of the intermediate files that were generated in pass1.

			Required flags: , --start-date, --end-date, --intermediate-directory, --blocks-directory

Flags:

`)
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, `

Example Usage:

	rangeOpts=$(mimir-whisper-converter --whisper-directory /opt/graphite/storage/whisper --quiet daterange)
	mimir-whisper-converter --whisper-directory /opt/graphite/storage/whisper $rangeOpts --intermediate-directory /tmp/intermediate pass1
	mimir-whisper-converter --intermiedate-directory /tmp/intermediate --blocks-directory /opt/mimir/blocks $rangeOpts pass2
`)

	}
	flag.Parse()

	if *versionFlag {
		_, _ = fmt.Fprintf(os.Stdout, "%s\n", version)
		os.Exit(0)
	}

	if len(flag.Args()) != 1 {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: Need exactly one command (%s)\n", flag.Args())
		flag.Usage()
		os.Exit(1)
	}
	if *workerID >= *workerCount {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: Worker ID must be <= the number of workers (%d and %d)\n", *workerID, *workerCount)
		flag.Usage()
		os.Exit(1)
	}
	command := flag.Args()[0]
	if *whisperDirectory == "" {
		_, _ = fmt.Fprintf(os.Stderr, "ERROR: Need to specify --whisper-directory\n")
		flag.Usage()
		os.Exit(1)
	}

	var dates []time.Time
	if command != DATERANGE && command != FILELIST {
		if *startDateFlag == "" {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: Need to specify --start-date\n")
			flag.Usage()
			os.Exit(1)
		}
		startDate, err := time.Parse("2006-01-02", *startDateFlag)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: error parsing --start-date: %v\n", err)
			flag.Usage()
			os.Exit(1)
		}
		endDate, err := time.Parse("2006-01-02", *endDateFlag)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: error parsing --end-date: %v\n", err)
			flag.Usage()
			os.Exit(1)
		}
		if startDate.After(endDate) {
			_, _ = fmt.Fprintf(os.Stderr, "ERROR: end date must be same or after start date\n")
			flag.Usage()
			os.Exit(1)
		}
		for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
			dates = append(dates, d)
		}
	}

	logger := log.NewLogfmtLogger(os.Stderr)
	levelOption := level.AllowWarn()
	if *quietFlag {
		levelOption = level.AllowNone()
	} else if *verboseFlag {
		levelOption = level.AllowInfo()
	} else if *debugFlag {
		levelOption = level.AllowDebug()
	}
	logger = level.NewFilter(logger, levelOption)
	logger = log.WithPrefix(logger, "ts", log.DefaultTimestampUTC)

	converter := whisperconverter.NewWhisperConverter(
		*namePrefix,
		*whisperDirectory,
		regexp.MustCompile(*fileFilterPattern),
		*threads,
		*workerCount,
		*workerID,
		ParseCustomLabels(*customLabels),
		dates,
		logger,
	)

	go func() {
		err := http.ListenAndServe("localhost:8081", nil)
		if err != nil {
			_ = level.Error(logger).Log("msg", "could not start http server for pprof", "err", err)
		} else {
			_ = level.Info(logger).Log("msg", "pprof at: http://localhost:8081/debug/pprof")
		}
	}()

	switch command {
	case DATERANGE:
		converter.CommandDateRange(*targetWhisperFiles)
	case FILELIST:
		err := converter.CommandFileList(*targetWhisperFiles)
		if err != nil {
			level.Error(logger).Log("msg", "Error generating file list", "err", err)
			os.Exit(1)
		}
	case PASS1:
		err := converter.CommandPass1(*targetWhisperFiles, *intermediateDirectory, *resumeIntermediate)
		if err != nil {
			level.Error(logger).Log("msg", "Error running pass1", "err", err)
			os.Exit(1)
		}
	case PASS2:
		err := converter.CommandPass2(*intermediateDirectory, *blocksDirectory, *resumeBlocks)
		if err != nil {
			level.Error(logger).Log("msg", "Error running pass2", "err", err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "ERROR: Unknown command: %s\n", command)
		flag.Usage()
		os.Exit(1)
	}

	processed := converter.GetProcessedCount()
	skipped := converter.GetSkippedCount()
	level.Info(logger).Log("msg", fmt.Sprintf("All done. Processed %d files, %d skipped", processed, skipped))
}

// ParseCustomLabels converts a csv string to a labels.Labels slice. panics on
// error.
func ParseCustomLabels(arg string) labels.Labels {
	r := csv.NewReader(strings.NewReader(arg))
	var labelStrings []string
	for {
		token, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		labelStrings = append(labelStrings, token...)
	}
	return labels.FromStrings(labelStrings...)
}
