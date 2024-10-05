package http

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/models"
	"github.com/go-playground/validator"
	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

func Listen(mctx *config.ModuleContext) error {
	slog.Info("starting HTTP server", "address", mctx.Config.HTTPAddress)

	e := &endpoints{mctx}

	mux := http.NewServeMux()

	mux.Handle("POST /ingest/direct", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := e.directIngest(rw, req); err != nil {
			slog.Error("error in directIngest HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	mux.Handle("GET /ingest/browser", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := e.browserIngest(rw, req); err != nil {
			slog.Error("error in browserIngest HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	//mux.Handle("POST /generate", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
	//	if err := e.generate(rw, req); err != nil {
	//		slog.Error("error in generate HTTP handler", "error", err, "request", req)
	//		rw.WriteHeader(http.StatusInternalServerError)
	//	}
	//}))

	return http.ListenAndServe(mctx.Config.HTTPAddress, mux)
}

type endpoints struct {
	*config.ModuleContext
}

// directIngest is an ingest endpoint that accepts JSON-encoded bodies.
func (e endpoints) directIngest(rw http.ResponseWriter, req *http.Request) error {
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

// browserIngest is an ingest endpoint that accepts URL-encoded parameters, designed to be exposed directly to the outside world.
func (e endpoints) browserIngest(rw http.ResponseWriter, req *http.Request) error {
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

	if subtle.ConstantTimeCompare([]byte(data.Token), []byte(e.Config.Token)) == 0 {
		rw.WriteHeader(401)
		n := basePage("Invalid token", g.Text("Unauthorised - invalid token"))
		return n.Render(rw)
	}

	e.NewArticleChannel <- &data.NewArticle
	
	return basePage("Success!", P(
		StyleAttr("color: darkgreen; font-weight: bold;"),
		g.Text("Success!"),
	),
		P(
			g.Textf("Title: %s", data.NewArticle.Title), Br(),
			g.Textf("URL: %s", data.NewArticle.URL), Br(),
			g.Text("Description: "),
			g.If(data.NewArticle.Description == "", I(g.Text("none"))),
			g.If(data.NewArticle.Description != "", g.Text(data.NewArticle.Description)),
		),
		Script(g.Raw(`setTimeout(function(){history.back();}, 750);`)),
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

//func (e endpoints) generate(rw http.ResponseWriter, _ *http.Request) error {
//	if err := worker.GenerateSiteAndUpload(e.DB, e.Config); err != nil {
//		return err
//	}
//	rw.WriteHeader(204)
//	return nil
//}
