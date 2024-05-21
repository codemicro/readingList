package main

import (
	"cmp"
	"errors"
	"log/slog"
	"os"

	"github.com/codemicro/readingList/models"
)

func main() {
	if err := run(); err != nil {
		slog.Error("unhandled error", "error", err)
	}
}

func run() error {
	var conf = struct {
		Token       string
		HTTPAddress string
		DatabaseFilename string
	}{
		Token:       os.Getenv("READINGLISTD_INGEST_TOKEN"),
		HTTPAddress: cmp.Or(os.Getenv("READINGLISTD_HTTP_ADDR"), ":9231"),
		DatabaseFilename: cmp.Or(os.Getenv("READINGLISTD_DATABASE_FILENAME"), "readinglist.sqlite3.db"),
	}

	if conf.Token == "" {
		return errors.New("READINGLISTD_INGEST_TOKENS not set")
	}

	db, err := NewDB(conf.DatabaseFilename)
	if err != nil {
		return err
	}

	newArticleChan := make(chan *models.NewArticle, 5)
	RunWorker(db, newArticleChan)
	return HTTPListen(conf.HTTPAddress, conf.Token, newArticleChan)
}
