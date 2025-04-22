package main

import (
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/database"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/http"
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

	db, err := database.NewDB(conf)
	if err != nil {
		return err
	}
	
	return http.Listen(conf, db)
}
