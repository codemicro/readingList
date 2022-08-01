package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const readingListFile = "readingList.csv"

type readingListEntry struct {
	URL           string    `csv:"url,omitempty"`
	Title         string    `csv:"title,omitempty"`
	Description   string    `csv:"description,omitempty"`
	Image         string    `csv:"image,omitempty"`
	Date          time.Time `csv:"date,omitempty"`
	HackerNewsURL string    `csv:"hnurl,omitempty"`
}

func run() error {
	if len(os.Args) == 1 || !(os.Args[1] == "add" || os.Args[1] == "generateSite") {
		return fmt.Errorf("usage: %s [add|generateSite]", os.Args[0])
	}

	var f func() error

	switch os.Args[1] {
	case "add":
		f = AddRowToCSV
	case "generateSite":
		f = GenerateSite
	default:
		return errors.New("unreachable")
	}

	return f()
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
