// +build mage

package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"sort"
	"text/template"
	"time"

	"github.com/jszwec/csvutil"
	"github.com/schollz/progressbar/v3"
	"github.com/stevelacy/daz"
)

const dateFormat = "2006-01-02"

type readingListEntry struct {
	URL         string    `csv:"url,omitempty"`
	Title       string    `csv:"title,omitempty"`
	Description string    `csv:"description,omitempty"`
	Image       string    `csv:"image,omitempty"`
	Date        time.Time `csv:"date,omitempty"`
}

// renderAnchor renders a HTML anchor tag
func renderAnchor(text, url string, newTab bool) daz.HTML {
	attrs := daz.Attr{
		"href": url,
		"rel":  "noopener",
	}
	if newTab {
		attrs["target"] = "_blank"
	}
	return daz.H("a", attrs, text)
}

func renderUnsafeAnchor(text, url string, newTab bool) daz.HTML {
	attrs := daz.Attr{
		"href": url,
		"rel":  "noopener",
	}
	if newTab {
		attrs["target"] = "_blank"
	}
	return daz.H("a", attrs, daz.UnsafeContent(text))
}

//go:embed page.template.html
var htmlPageTemplate []byte

// renderHTMLPage renders a complete HTML page
func renderHTMLPage(title, titleBar, pageContent, extraHeadeContent string) ([]byte, error) {

	tpl, err := template.New("page").Parse(string(htmlPageTemplate))
	if err != nil {
		return nil, err
	}
	outputBuf := new(bytes.Buffer)

	tpl.Execute(outputBuf, struct {
		Title            string
		Content          string
		PageTitleBar     string
		ExtraHeadContent string
	}{Content: pageContent, PageTitleBar: titleBar, Title: title, ExtraHeadContent: extraHeadeContent})

	return outputBuf.Bytes(), nil
}

type entryGroup struct {
	Date    time.Time
	Entries entrySlice
}

type entryGroupSlice []*entryGroup

func (e entryGroupSlice) Len() int {
	return len(e)
}

func (e entryGroupSlice) Less(i, j int) bool {
	return e[i].Date.After(e[j].Date)
}

func (e entryGroupSlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

type entrySlice []*readingListEntry

func (e entrySlice) Len() int {
	return len(e)
}

func (e entrySlice) Less(i, j int) bool {
	return e[i].Date.After(e[j].Date)
}

func (e entrySlice) Swap(i, j int) {
	e[i], e[j] = e[j], e[i]
}

func groupEntriesByMonth(entries []*readingListEntry) entryGroupSlice {
	groupMap := make(map[time.Time]*entryGroup)

	for _, entry := range entries {
		newTime := time.Date(entry.Date.Year(), entry.Date.Month(), 1, 0, 0, 0, 0, time.UTC)
		if groupMap[newTime] == nil {
			groupMap[newTime] = &entryGroup{
				Date: newTime,
			}
		}
		groupMap[newTime].Entries = append(groupMap[newTime].Entries, entry)
	}

	var o entryGroupSlice
	for _, group := range groupMap {
		sort.Sort(group.Entries)
		o = append(o, group)
	}
	sort.Sort(o)

	return o
}

var hnHTTPClient = new(http.Client)

type hackerNewsEntry struct {
	ObjectID string `json:"objectID"`
	Points   int    `json:"points"`
}

var hackerNewsSubmissionURL = "https://news.ycombinator.com/item?id=%s"

var totalRequestedHNQueries int

// queryHackerNews searches the Hacker News index to find a submission with a matching URL to that provided.
// If a submission is found, its URL is returned. If no submission is found, an empty string is returned. If multiple submissions are found, the URL of the one with the most points is returned.
func queryHackerNews(url string) (string, error) {

	if totalRequestedHNQueries > 500 { // there's a ratelimit of 10000 search requests per hour - stopping at 500 per run this means that we can add a maximum of 20 pages per hour
		return "", nil
	}

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
	totalRequestedHNQueries += 1
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

// makeTILHTML generates HTML from a []*entryGroup to make a list of articles
func makeListHTML(groups []*entryGroup) string {

	const headerLevel = "h3"

	numGroups := len(groups)

	var parts []interface{}
	for groupNumber, group := range groups {

		dateString := fmt.Sprintf("%s %d", group.Date.Month().String(), group.Date.Year())

		header := daz.H(headerLevel, dateString)

		pb := progressbar.NewOptions(len(group.Entries),
			progressbar.OptionSetDescription(fmt.Sprintf("[%d/%d] %s", groupNumber+1, numGroups, dateString)),
		)

		var entries []daz.HTML
		for _, article := range group.Entries {

			hnURL, err := queryHackerNews(article.URL)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Warning: unable to complete Hacker News query for article %s", article.URL)
			}

			var titleLineItems = []interface{}{
				renderAnchor(article.Title, article.URL, false),
				" - " + article.Date.Format(dateFormat),
			}

			if hnURL != "" {
				titleLineItems = append(titleLineItems, " ")
				titleLineItems = append(titleLineItems, daz.H("a", daz.Attr{"href": hnURL, "rel": "noopener"}, daz.H("img", daz.Attr{"src": "https://news.ycombinator.com/y18.gif", "height": "14em", "title": "View on Hacker News", "alt": "Hacker News logo"})))
			}

			titleLine := daz.H("summary", titleLineItems...)

			detailedInfo := []interface{}{}

			{
				var descriptionContent string
				if article.Description != "" {
					descriptionContent = article.Description
				} else {
					descriptionContent = "<none>"
				}
				detailedInfo = append(detailedInfo, daz.H("div", "Description:", daz.H("i", descriptionContent)))
			}

			{
				if article.Image != "" {
					detailedInfo = append(detailedInfo, daz.H("div", "Image:", daz.H("br"), daz.H("img", daz.Attr{"src": article.Image, "loading": "lazy", "style": "max-width: 256px;"})))
				}
			}

			detailedInfo = append(detailedInfo, daz.Attr{"class": "description"})
			entries = append(entries, daz.H("li", daz.H("details", titleLine, daz.H("div", detailedInfo...))))

			pb.Add(1)
		}

		parts = append(parts, []daz.HTML{header, daz.H("ul", entries)})
	}

	fmt.Println() // the progress bars do weird newline things

	return daz.H("div", parts...)()
}

func GenerateSite() error {

	const outputDir = ".site"
	const readingListFile = "readingList.csv"

	// read CSV file
	var entries []*readingListEntry

	fcont, err := ioutil.ReadFile(readingListFile)
	if err != nil {
		return err
	}

	err = csvutil.Unmarshal(fcont, &entries)
	if err != nil {
		return err
	}

	numArticles := len(entries)
	groupedEntries := groupEntriesByMonth(entries)

	const pageTitle = "akp's reading list"

	head := daz.H(
		"div",
		daz.H("h1", pageTitle),
		daz.H(
			"p",
			daz.UnsafeContent(
				fmt.Sprintf(
					"A mostly complete list of articles I've read<br>There are currently %d entries in the list<br>Last modified %s<br>Repo: %s",
					numArticles,
					time.Now().Format(dateFormat),
					renderUnsafeAnchor("<code>codemicro/readingList</code>", "https://github.com/codemicro/readingList", false)(),
				),
			),
		),
	)

	listingHTML := makeListHTML(groupedEntries)

	outputContent, err := renderHTMLPage(pageTitle, head(), listingHTML, "")
	if err != nil {
		return err
	}

	_ = os.Mkdir(".site", 0777)

	return ioutil.WriteFile(".site/index.html", outputContent, 0644)
}
