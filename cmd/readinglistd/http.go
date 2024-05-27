package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"github.com/jmoiron/sqlx"
	"io"
	"log/slog"
	"net/http"

	"git.tdpain.net/codemicro/readingList/models"
	"github.com/go-playground/validator"
)

func HTTPListen(db *sqlx.DB, conf *config, newArticleChan chan *models.NewArticle) error {
	slog.Info("starting HTTP server", "address", conf.HTTPAddress)

	mux := http.NewServeMux()
	mux.Handle("POST /ingest", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := ingestHandler(rw, req, conf, newArticleChan); err != nil {
			slog.Error("error in ingest HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))
	mux.Handle("POST /generate", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := generateHandler(rw, req, conf, db); err != nil {
			slog.Error("error in generate HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	return http.ListenAndServe(conf.HTTPAddress, mux)
}

func ingestHandler(rw http.ResponseWriter, req *http.Request, conf *config, newArticleChan chan *models.NewArticle) error {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if subtle.ConstantTimeCompare([]byte("Bearer "+conf.Token), []byte(req.Header.Get("Authorization"))) == 0 {
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

	newArticleChan <- requestData

	rw.WriteHeader(http.StatusNoContent)
	return nil
}

func generateHandler(rw http.ResponseWriter, req *http.Request, conf *config, db *sqlx.DB) error {
	if err := doSiteGeneration(db, conf); err != nil {
		return err
	}
	rw.WriteHeader(204)
	return nil
}
