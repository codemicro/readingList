package main

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/codemicro/readingList/models"
	"github.com/go-playground/validator"
)

func HTTPListen(addr string, token string, newArticleChan chan *models.NewArticle) error {
	slog.Info("starting HTTP server", "address", addr)

	mux := http.NewServeMux()
	mux.Handle("POST /ingest", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := httpHandler(rw, req, token, newArticleChan); err != nil {
			slog.Error("error in HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	return http.ListenAndServe(addr, mux)
}

func httpHandler(rw http.ResponseWriter, req *http.Request, token string, newArticleChan chan *models.NewArticle) error {
	if req.Method != http.MethodPost {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if subtle.ConstantTimeCompare([]byte("Bearer "+token), []byte(req.Header.Get("Authorization"))) == 0 {
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
	return nil
}
