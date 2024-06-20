package http

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/worker"
	"github.com/jmoiron/sqlx"
	"io"
	"log/slog"
	"net/http"

	"git.tdpain.net/codemicro/readingList/models"
	"github.com/go-playground/validator"
)

func Listen(db *sqlx.DB, conf *config.Config, newArticleChan chan *models.NewArticle) error {
	slog.Info("starting HTTP server", "address", conf.HTTPAddress)

	e := &endpoints{
		DB:                db,
		Config:            conf,
		NewArticleChannel: newArticleChan,
	}

	mux := http.NewServeMux()
	mux.Handle("POST /ingest", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := e.ingest(rw, req); err != nil {
			slog.Error("error in ingest HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))
	mux.Handle("POST /generate", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := e.generate(rw, req); err != nil {
			slog.Error("error in generate HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	return http.ListenAndServe(conf.HTTPAddress, mux)
}

type endpoints struct {
	DB                *sqlx.DB
	Config            *config.Config
	NewArticleChannel chan *models.NewArticle
}

func (e endpoints) ingest(rw http.ResponseWriter, req *http.Request) error {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if subtle.ConstantTimeCompare([]byte("Bearer "+e.Config.Token), []byte(req.Header.Get("Authorization"))) == 0 {
		rw.WriteHeader(http.StatusUnauthorized)
		return nil
	}

	rawBodyData, err := io.ReadAll(req.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}

	requestData := new(models.NewArticle)
	if err := json.Unmarshal(rawBodyData, requestData); err != nil {
		_, _ = rw.Write([]byte(err.Error()))
		rw.WriteHeader(http.StatusBadRequest)
		return nil
	}

	if err := validator.New().Struct(requestData); err != nil {
		_, _ = rw.Write([]byte(err.Error()))
		rw.WriteHeader(http.StatusBadRequest)
		return nil
	}

	e.NewArticleChannel <- requestData

	rw.WriteHeader(http.StatusNoContent)
	return nil
}

func (e *endpoints) generate(rw http.ResponseWriter, _ *http.Request) error {
	if err := worker.GenerateSiteAndUpload(e.DB, e.Config); err != nil {
		return err
	}
	rw.WriteHeader(204)
	return nil
}
