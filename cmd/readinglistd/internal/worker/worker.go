package worker

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/config"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/database"
	"git.tdpain.net/codemicro/readingList/cmd/readinglistd/internal/generator"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
	"errors"
	"git.tdpain.net/codemicro/readingList/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func RunSiteWorker(mctx *config.ModuleContext) {
	go siteWorker(mctx)
}

func siteWorker(mctx *config.ModuleContext) {
	var job *config.ArticleChannelWrapper
rootLoop:
	for {
		job = <-mctx.NewArticleChannel
	loop:
		for {			
			article := &models.Article{
				NewArticle: *job.Article,
				ID:         uuid.New(),
			}
			
			{ // remove fragment
				parsed, err := url.Parse(article.URL)
				if err != nil {
					job.Finish(errors.New("invalid URL"))
					slog.Error("invalud URL supplied to worker", "url", article.URL)
					continue rootLoop
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

			if err := database.InsertArticle(mctx.DB, article); err != nil {
				job.Finish(errors.New("fatal database error"))
				slog.Error("unable to insert article", "error", err, "article", article)
				continue rootLoop
			}

			job.Finish(nil)
			
			// The purpose of this is to delay rebuilding the site if another article appears in the next 20 seconds
			ticker := time.NewTicker(time.Second * 20)
			select {
			case <-ticker.C:
				ticker.Stop()
				break loop
			case job = <-mctx.NewArticleChannel:
				ticker.Stop()
				continue
			}
		}

		if err := GenerateSiteAndUpload(mctx.DB, mctx.Config); err != nil {
			slog.Error("error while executing site generation", "error", err)
			continue
		}
	}
}

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

var siteGenerationLock sync.Mutex

func GenerateSiteAndUpload(db *sqlx.DB, conf *config.Config) error {
	siteGenerationLock.Lock()
	defer siteGenerationLock.Unlock()

	allArticles, err := database.GetAllArticles(db)
	if err != nil {
		return fmt.Errorf("unable to fetch all articles: %w", err)
	}

	sitePath, err := generator.GenerateSite(allArticles)
	if err != nil {
		return fmt.Errorf("unable to generate site: %w", err)
	}

	siteZipFile, err := packageSite(sitePath)
	if err != nil {
		return fmt.Errorf("unable to package site: %w", err)
	}

	_ = os.RemoveAll(sitePath)

	if err := uploadSite(conf, siteZipFile); err != nil {
		return fmt.Errorf("unable to upload site to palmatum: %w", err)
	}

	return nil
}

func packageSite(sitePath string) (*bytes.Buffer, error) {
	dfs := os.DirFS(sitePath)
	buffer := new(bytes.Buffer)

	writer := zip.NewWriter(buffer)
	if err := writer.AddFS(dfs); err != nil {
		return nil, fmt.Errorf("add fs to zip file: %w", err)
	}
	if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("flush zip writer: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close zip writer: %w", err)
	}

	return buffer, nil
}

func uploadSite(conf *config.Config, reader io.Reader) error {
	bodyBuffer := new(bytes.Buffer)
	mpWriter := multipart.NewWriter(bodyBuffer)

	if err := mpWriter.WriteField("slug", conf.PalmatumSiteName); err != nil {
		return fmt.Errorf("write field to multipart: %w", err)
	}

	fieldWriter, err := mpWriter.CreateFormFile("bundle", "site.zip")
	if err != nil {
		return fmt.Errorf("create multipart field: %w", err)
	}

	if _, err := io.Copy(fieldWriter, reader); err != nil {
		return fmt.Errorf("copy site file to multipart writer: %w", err)
	}

	if err := mpWriter.Close(); err != nil {
		return fmt.Errorf("failed to close multipart writer: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, "https://management.pages.tdpain.net/api/site/bundle", bodyBuffer)
	if err != nil {
		return fmt.Errorf("make http request: %w", err)
	}

	req.Header.Set("Content-Type", mpWriter.FormDataContentType())
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(conf.PalmatumAuthentication)))

	resp, err := (&http.Client{
		Timeout: time.Second * 10,
	}).Do(req)
	if err != nil {
		return fmt.Errorf("do http request: %w", err)
	}

	bodyCont, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()

	if category := resp.StatusCode / 100; !(category == 2 || category == 3) {
		slog.Info("upload error encountered", "body", string(bodyCont))
		return fmt.Errorf("got %d status code returned from Palmatum", resp.StatusCode)
	}

	return nil
}
