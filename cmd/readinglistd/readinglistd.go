package main

import (
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/database"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/http"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/worker"
	"git.tdpain.net/codemicro/readingList/models"
	"log/slog"
)

func main() {
	if err := run(); err != nil {
		slog.Error("unhandled error", "error", err)
	}
}

func run() error {
	conf, err := config.Get()
	if err != nil {
		return err
	}

	db, err := database.NewDB(conf.DatabaseFilename)
	if err != nil {
		return err
	}

	newArticleChan := make(chan *models.NewArticle, 5)
	worker.RunWorker(db, newArticleChan, conf)
	return http.Listen(db, conf, newArticleChan)
}
