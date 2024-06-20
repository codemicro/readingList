package main

import (
	"cmp"
	"errors"
	"log/slog"
	"os"
)

func main() {
	if err := run(); err != nil {
		slog.Error("unhandled error", "error", err)
	}
}

func run() error {
	var conf = struct {
		Token         string
		HTTPAddress   string
		DaemonAddress string
		DaemonToken   string
	}{
		Token:         os.Getenv("JMAPINGEST_JMAP_TOKEN"),
		DaemonAddress: cmp.Or(os.Getenv("JMAPINGEST_READINGLISTD_ADDR"), "localhost:9231"),
		DaemonToken:   os.Getenv("JMAPINGEST_READINGLISTD_TOKEN"),
	}

	if conf.Token == "" {
		return errors.New("JMAPINGEST_JMAP_TOKEN not set")
	}

	if conf.DaemonToken == "" {
		return errors.New("HTTPINGEST_READINGLISTD_TOKEN not set")
	}

	return nil
}
