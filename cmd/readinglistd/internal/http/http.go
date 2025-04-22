package http

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/database"
	"git.tdpain.net/codemicro/readingList/models"
	"github.com/go-playground/validator"
	g "github.com/maragudk/gomponents"
	. "github.com/maragudk/gomponents/html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func Listen(conf *config.Config, db *database.DB) error {
	slog.Info("starting HTTP server", "address", conf.HTTPAddress)

	e := &endpoints{DB: db, Config: conf}

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

	mux.Handle("GET /list", http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if err := e.list(rw, req); err != nil {
			slog.Error("error in list HTTP handler", "error", err, "request", req)
			rw.WriteHeader(http.StatusInternalServerError)
		}
	}))

	return http.ListenAndServe(conf.HTTPAddress, mux)
}

type endpoints struct {
	DB     *database.DB
	Config *config.Config
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

	if err := processNewArticle(e.DB, requestData); err != nil {
		_, _ = rw.Write([]byte(err.Error()))
		rw.WriteHeader(http.StatusBadRequest)
	} else {
		rw.WriteHeader(http.StatusNoContent)
	}

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

	var page g.Node
	if err := processNewArticle(e.DB, &data.NewArticle); err != nil {
		page = basePageWithBackgroundColour("Addition failed", "#fadbd8", P(
			StyleAttr("font-weight: bold;"),
			g.Text("Error: "+err.Error()),
		))
	} else {
		page = basePageWithBackgroundColour("Success!", "#d4efdf", P(
			StyleAttr("font-weight: bold;"),
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
		)
	}

	return page.Render(rw)
}

func (e endpoints) list(rw http.ResponseWriter, req *http.Request) error {
	year, _ := strconv.Atoi(req.URL.Query().Get("year"))
	month, _ := strconv.Atoi(req.URL.Query().Get("month"))

	var articles []*models.Article
	var err error

	if year != 0 && month != 0 {
		articles, err = e.DB.GetArticlesForMonth(year, month)
		if err != nil {
			return fmt.Errorf("get articles for month %d-%d: %w", year, month, err)
		}
	} else {
		articles, err = e.DB.GetAllArticles()
		if err != nil {
			return fmt.Errorf("get all articles: %w", err)
		}
	}

	res, err := json.Marshal(articles)
	if err != nil {
		return fmt.Errorf("marshal all articles: %w", err)
	}

	rw.Header().Set("Content-Type", "application/json")
	_, _ = rw.Write(res)
	return nil
}

func basePageWithBackgroundColour(title, colour string, content ...g.Node) g.Node {
	styles := `body {
		font-family: sans-serif;
		font-size: 1.1rem;
		padding: 1em;
	`
	if colour != "" {
		styles += "background-color: " + colour + ";\n"
	}
	styles += "}"

	return HTML(
		Head(
			Meta(g.Attr("name", "viewport"), g.Attr("content", "width=device-width, initial-scale=1")),
			TitleEl(g.Text(title)),
			StyleEl(g.Text(styles)),
		),
		Body(content...),
	)
}

func basePage(title string, content ...g.Node) g.Node {
	return basePageWithBackgroundColour(title, "", content...)
}

func unorderedList(x []string) g.Node {
	return Ul(g.Map(x, func(s string) g.Node {
		return Li(g.Text(s))
	})...)
}

func processNewArticle(db *database.DB, newArticle *models.NewArticle) error {
	article := &models.Article{
		NewArticle: *newArticle,
	}

	{ // remove fragment
		parsed, err := url.Parse(article.URL)
		if err != nil {
			return errors.New("invalid URL")
		}
		parsed.Fragment = ""
		article.URL = parsed.String()
	}

	hnURL, err := queryHackerNews(article.URL)
	if err != nil {
		slog.Warn("unable to query hacker news", "error", err, "article", article)
	}
	article.HackerNewsURL = hnURL

	if len(article.Description) > 500 {
		article.Description = article.Description[:500] + " [trimmed]"
	}

	if err := db.InsertArticle(article); err != nil {
		slog.Error("unable to insert article", "error", err, "article", newArticle)
		return errors.New("fatal database error")
	}

	return nil
}

//func (e endpoints) generate(rw http.ResponseWriter, _ *http.Request) error {
//	if err := worker.GenerateSiteAndUpload(e.DB, e.Config); err != nil {
//		return err
//	}
//	rw.WriteHeader(204)
//	return nil
//}

// queryHackerNews searches the Hacker News index to find a submission with a matching URL to that provided.
// If a submission is found, its URL is returned. If no submission is found, an empty string is returned. If multiple submissions are found, the URL of the one with the most points is returned.
func queryHackerNews(queryURL string) (string, error) {
	queryParams := make(url.Values)
	queryParams.Add("restrictSearchableAttributes", "url")
	queryParams.Add("hitsPerPage", "1000")
	queryParams.Add("query", queryURL)

	req, err := http.NewRequest("GET", "https://hn.algolia.com/api/v1/search?"+queryParams.Encode(), nil)
	if err != nil {
		return "", err
	}

	resp, err := new(http.Client).Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HN Search returned a non-200 status code: %d", resp.StatusCode)
	}

	responseBody, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	type hackerNewsEntry struct {
		ObjectID string `json:"objectID"`
		Points   int    `json:"points"`
	}

	var x struct {
		Hits []*hackerNewsEntry `json:"hits"`
	}

	err = json.Unmarshal(responseBody, &x)
	if err != nil {
		return "", err
	}

	var targetSubmission *hackerNewsEntry
	if len(x.Hits) == 0 {
		return "", nil
	} else if len(x.Hits) == 1 {
		targetSubmission = x.Hits[0]
	} else {
		// must be more than one hit
		var topRatedSubmission *hackerNewsEntry
		for _, entry := range x.Hits {
			if topRatedSubmission == nil || entry.Points > topRatedSubmission.Points {
				topRatedSubmission = entry
			}
		}
		targetSubmission = topRatedSubmission
	}

	return fmt.Sprintf("https://news.ycombinator.com/item?id=%s", targetSubmission.ObjectID), nil
}
