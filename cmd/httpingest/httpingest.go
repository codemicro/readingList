package main

import (
	"bytes"
	"cmp"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"git.tdpain.net/codemicro/readingList/models"
	"github.com/go-playground/validator"
	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
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
		Token:         os.Getenv("HTTPINGEST_TOKEN"),
		HTTPAddress:   cmp.Or(os.Getenv("HTTPINGEST_HTTP_ADDR"), ":9232"),
		DaemonAddress: cmp.Or(os.Getenv("HTTPINGEST_READINGLISTD_ADDR"), "localhost:9231"),
		DaemonToken:   os.Getenv("HTTPINGEST_READINGLISTD_TOKEN"),
	}

	if conf.Token == "" {
		return errors.New("HTTPINGEST_TOKEN not set")
	}

	if conf.DaemonToken == "" {
		return errors.New("HTTPINGEST_READINGLISTD_TOKEN not set")
	}

	return HTTPListen(conf.HTTPAddress, conf.Token, conf.DaemonAddress, conf.DaemonToken)
}

func HTTPListen(addr string, token string, daemonAddress string, daemonToken string) error {
	slog.Info("starting HTTP server", "address", addr)

	mux := http.NewServeMux()
	mux.Handle("GET /", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := handler(rw, req, token, daemonAddress, daemonToken); err != nil {
			slog.Error("error in HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	return http.ListenAndServe(addr, mux)
}

func handler(rw http.ResponseWriter, req *http.Request, token string, daemonAddress string, daemonToken string) error {
	rw.Header().Set("Access-Control-Allow-Origin", "*")
	rw.Header().Set("Content-Type", "text/html")

	data := &struct {
		models.NewArticle
		NextURL string `validate:"required,url"`
		Token   string `validate:"required"`
	}{
		NewArticle: models.NewArticle{
			URL:         req.URL.Query().Get("url"),
			Title:       req.URL.Query().Get("title"),
			Description: req.URL.Query().Get("description"),
			ImageURL:    req.URL.Query().Get("image"),
			Date:        time.Now().In(time.UTC),
		},
		NextURL: req.URL.Query().Get("nexturl"),
		Token:   req.URL.Query().Get("token"),
	}

	{
		validate := validator.New()
		err := validate.Struct(data)
		if err != nil {
			rw.WriteHeader(400)
			n := basePage("Bad request", g.Text("Bad request"), Br(), unorderedList(strings.Split(err.Error(), "\n")))
			return n.Render(rw)
		}
	}

	if subtle.ConstantTimeCompare([]byte(data.Token), []byte(token)) == 0 {
		rw.WriteHeader(401)
		n := basePage("Invalid token", g.Text("Unauthorised - invalid token"))
		return n.Render(rw)
	}

	serialisedInputs, err := json.Marshal(data.NewArticle)
	if err != nil {
		return err
	}

	rctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	newReq, err := http.NewRequestWithContext(rctx, "POST", (&url.URL{
		Scheme: "http",
		Host:   daemonAddress,
		Path:   "/ingest",
	}).String(), bytes.NewBuffer(serialisedInputs))
	if err != nil {
		return err
	}

	newReq.Header.Set("Authorization", "Bearer "+daemonToken)

	resp, err := http.DefaultClient.Do(newReq)
	if err != nil {
		rw.WriteHeader(500)
		n := basePage("Internal Server Error", g.Text("Failed to ingest\n"+err.Error()))
		return n.Render(rw)
	}

	bodyCont, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if category := resp.StatusCode / 100; category != 2 {
		rw.WriteHeader(500)
		n := basePage("Internal Server Error", g.Text("Failed to ingest\nBad status code"))
		slog.Error("ingest error encountered", "body", string(bodyCont), "statusCode", resp.StatusCode)
		return n.Render(rw)
	}

	return basePage("Success!", Span(
		StyleAttr("color: darkgreen;"),
		g.Text("Success!"),
	),
		Script(g.Rawf(`setTimeout(function(){window.location.replace(%#v);}, 500);`, data.NextURL)),
	).Render(rw)
}

func basePage(title string, content ...g.Node) g.Node {
	return HTML(
		Head(
			Meta(g.Attr("name", "viewport"), g.Attr("content", "width=device-width, initial-scale=1")),
			TitleEl(g.Text(title)),
			StyleEl(g.Text(`
body {
	font-family: sans-serif;
	font-size: 1.1rem;
	padding: 1em;
}
`)),
		),
		Body(content...),
	)
}

func unorderedList(x []string) g.Node {
	return Ul(g.Map(x, func(s string) g.Node {
		return Li(g.Text(s))
	})...)
}
