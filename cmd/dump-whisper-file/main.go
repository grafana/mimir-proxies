package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/kisielk/whisper-go/whisper"
)

func main() {
	headerOnly := flag.Bool("header", true, "Print header")
	archive := flag.Int("archive", 0, "Archive number to dump")
	formatTimestamp := flag.Bool("timestamp", true, "Format timestamp")
	flag.Parse()

	for _, fn := range flag.Args() {
		err := dumpWhisperFile(fn, *headerOnly, *archive, *formatTimestamp)
		if err != nil {
			log.Println("error while dumping", fn, ":", err)
		}
	}
}

func dumpWhisperFile(fn string, header bool, archive int, formatTs bool) error {
	fd, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer fd.Close()

	w, err := whisper.OpenWhisper(fd)
	if err != nil {
		return err
	}

	if header {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		err = enc.Encode(w.Header)
		return err
	}

	pts, err := w.DumpArchive(archive)
	if err != nil {
		return err
	}

	for _, p := range pts {
		if p.Timestamp == 0 {
			continue
		}

		if formatTs {
			fmt.Printf("%g\t%d\t%s\n", p.Value, p.Timestamp, time.Unix(int64(p.Timestamp), 0).UTC().Format(time.RFC3339Nano))
		} else {
			fmt.Printf("%g\t%d\n", p.Value, p.Timestamp)
		}
	}

	return nil
}
