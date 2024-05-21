package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/codemicro/readingList/models"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

func RunWorker(db *sqlx.DB, c chan *models.NewArticle) {
	go worker(db, c)
}

func worker(db *sqlx.DB, c chan *models.NewArticle) {
	var newArticle *models.NewArticle
	for {
		newArticle = <-c
	loop:
		for {
			article := &models.Article{
				NewArticle: *newArticle,
				ID:         uuid.New(),
			}

			hnURL, err := queryHackerNews(article.URL)
			if err != nil {
				slog.Warn("unable to query hacker news", "error", err, "article", article)
			}

			article.HackerNewsURL = hnURL
			if err := InsertArticle(db, article); err != nil {
				slog.Error("unable to insert article", "error", err, "article", article)
				break
			}

			// The purpose of this is to delay rebuilding the site if another article appears in the next 20 seconds
			ticker := time.NewTicker(time.Second * 20)
			select {
			case <-ticker.C:
				ticker.Stop()
				break loop
			case newArticle = <-c:
				ticker.Stop()
				continue
			}
		}

		allArticles, err := GetAllArticles(db)
		if err != nil {
			slog.Error("unable to fetch all articles", "error", err)
			continue
		}

		sitePath, err := GenerateSite(allArticles)
		if err != nil {
			slog.Error("unable to generate site", "error", err)
			continue
		}

		siteZipFile, err := packageSite(sitePath)
		if err != nil {
			slog.Error("unable to package site", "error", err)
			continue
		}

		_ = os.RemoveAll(sitePath)

		// TODO: Upload to Palmatum
		_ = os.WriteFile("f.zip", siteZipFile, 0777)
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

	responseBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

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

func packageSite(sitePath string) ([]byte, error) {
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

	return buffer.Bytes(), nil
}
