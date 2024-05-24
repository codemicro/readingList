package main

import (
	"cmp"
	"errors"
	"log/slog"
	"os"

	"git.tdpain.netcodemicro/readingList/models"
)

func main() {
	if err := run(); err != nil {
		slog.Error("unhandled error", "error", err)
	}
}

func run() error {
	var conf = struct {
		Token                  string
		HTTPAddress            string
		DatabaseFilename       string
		PalmatumAuthentication string
		SiteName               string
	}{
		Token:                  os.Getenv("READINGLISTD_INGEST_TOKEN"),
		HTTPAddress:            cmp.Or(os.Getenv("READINGLISTD_HTTP_ADDR"), ":9231"),
		DatabaseFilename:       cmp.Or(os.Getenv("READINGLISTD_DATABASE_FILENAME"), "readinglist.sqlite3.db"),
		PalmatumAuthentication: os.Getenv("READINGLISTD_PALMATUM_AUTH"),
		SiteName:               os.Getenv("READINGLISTD_SITE_NAME"),
	}

	if conf.Token == "" {
		return errors.New("READINGLISTD_INGEST_TOKEN not set")
	}

	if conf.PalmatumAuthentication == "" {
		return errors.New("READINGLISTD_PALMATUM_AUTH not set")
	}

	if conf.SiteName == "" {
		return errors.New("READINGLISTD_SITE_NAME not set")
	}

	db, err := NewDB(conf.DatabaseFilename)
	if err != nil {
		return err
	}

	newArticleChan := make(chan *models.NewArticle, 5)
	RunWorker(db, newArticleChan, conf.PalmatumAuthentication, conf.SiteName)
	return HTTPListen(conf.HTTPAddress, conf.Token, newArticleChan)
}
