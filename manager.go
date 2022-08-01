package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/codemicro/readingList/transport"
	"github.com/jszwec/csvutil"
)

func AddRowToCSV() error {
	data := new(transport.Inputs)
	if err := loadInputs(data); err != nil {
		return err
	}

	if err := data.Validate(); err != nil {
		return err
	}

	hnURL, err := queryHackerNews(data.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to search Hacker News for URL %s\n", data.URL)
	}

	// if CSV file does not exist, create it with a header
	csvFilePath := readingListFile
	{
		_, err := os.Stat(csvFilePath)
		if os.IsNotExist(err) {

			b, err := csvutil.Marshal([]*readingListEntry{})
			if err != nil {
				return err
			}

			err = ioutil.WriteFile(csvFilePath, b, 0644)
			if err != nil {
				return err
			}

		}
	}

	// make changes to CSV file

	f, err := os.OpenFile(csvFilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	// This call to Marshal will generate something like this:
	//     url,title,description,date
	//     Hello,Title,Description,2021-07-12T18:04:20.981291487+01:00
	// We're only interested in appending the LAST line, so we split the output by newlines and only take the item with
	// index 1.
	b, err := csvutil.Marshal([]*readingListEntry{{
		URL:           data.URL,
		Title:         data.Title,
		Description:   strings.ReplaceAll(data.Description, "\n", " "),
		Image:         data.Image,
		Date:          time.Now(),
		HackerNewsURL: hnURL,
	}})
	if err != nil {
		return err
	}

	splitOutput := bytes.Split(b, []byte("\n"))

	if _, err = f.Write(append(splitOutput[1], byte('\n'))); err != nil {
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	return nil
}

func loadInputs(data any) error {
	return json.Unmarshal([]byte(os.Getenv("RL_INPUT_JSON")), data)
}

// HACKER NEWS STUFF

var hnHTTPClient = new(http.Client)

type hackerNewsEntry struct {
	ObjectID string `json:"objectID"`
	Points   int    `json:"points"`
}

var hackerNewsSubmissionURL = "https://news.ycombinator.com/item?id=%s"

// queryHackerNews searches the Hacker News index to find a submission with a matching URL to that provided.
// If a submission is found, its URL is returned. If no submission is found, an empty string is returned. If multiple submissions are found, the URL of the one with the most points is returned.
func queryHackerNews(url string) (string, error) {

	req, err := http.NewRequest("GET", "https://hn.algolia.com/api/v1/search", nil)
	if err != nil {
		return "", err
	}

	// why does this fel so hacky
	queryParams := req.URL.Query()
	queryParams.Add("restrictSearchableAttributes", "url")
	queryParams.Add("hitsPerPage", "1000")
	queryParams.Add("query", url)
	req.URL.RawQuery = queryParams.Encode()

	resp, err := hnHTTPClient.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HN Search returned a non-200 status code: %d", resp.StatusCode)
	}

	responseBody, _ := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

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

	return fmt.Sprintf(hackerNewsSubmissionURL, targetSubmission.ObjectID), nil
}
