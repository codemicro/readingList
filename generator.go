package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/jszwec/csvutil"
	"github.com/schollz/progressbar/v3"
	"github.com/stevelacy/daz"
)

const dateFormat = "2006-01-02"

type readingListEntry struct {
	URL           string    `csv:"url,omitempty"`
	Title         string    `csv:"title,omitempty"`
	Description   string    `csv:"description,omitempty"`
	Image         string    `csv:"image,omitempty"`
	Date          time.Time `csv:"date,omitempty"`
	HackerNewsURL string    `csv:"hnurl,omitempty"`
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
	Title   string
	Date    time.Time
	ID      string
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
				Date:  newTime,
				Title: fmt.Sprintf("%s %d", newTime.Month().String(), newTime.Year()),
				ID:    strings.ToLower(fmt.Sprintf("%s-%d", newTime.Month().String(), newTime.Year())),
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

// makeTILHTML generates HTML from a []*entryGroup to make a list of articles
func makeListHTML(groups []*entryGroup) string {

	const headerLevel = "h3"

	numGroups := len(groups)

	subsections := []interface{}{"Jump to "}
	for i := numGroups - 1; i >= 0; i -= 1 {
		group := groups[i]
		subsections = append(subsections, " :: ", renderAnchor(fmt.Sprintf("%s %d", group.Date.Month().String()[:3], group.Date.Year()), "#" + group.ID, false))
	}
	
	parts := []interface{}{daz.H("br"), daz.H("span", subsections...)}

	for groupNumber, group := range groups {

		dateString := group.Title

		header := daz.H(headerLevel, dateString, daz.Attr{"id": group.ID})

		pb := progressbar.NewOptions(len(group.Entries),
			progressbar.OptionSetDescription(fmt.Sprintf("[%d/%d] %s", groupNumber+1, numGroups, dateString)),
		)

		var entries []daz.HTML
		for _, article := range group.Entries {

			var titleLineItems = []interface{}{
				renderAnchor(article.Title, article.URL, false),
				" - " + article.Date.Format(dateFormat),
			}

			if article.HackerNewsURL != "" {
				titleLineItems = append(titleLineItems, " - ")
				titleLineItems = append(titleLineItems, daz.H("a", daz.Attr{"href": article.HackerNewsURL, "rel": "noopener"}, daz.H("img", daz.Attr{"src": "https://news.ycombinator.com/y18.gif", "height": "14em", "title": "View on Hacker News", "alt": "Hacker News logo"})))
			}

			if article.Description != "" {
				titleLineItems = append(titleLineItems, daz.H("span", daz.Attr{"class": "secondary"}, " - "+article.Description))
			}

			entries = append(entries, daz.H("li", titleLineItems...))

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
					"A mostly complete list of articles I've read on the internet<br>There are currently %d entries in the list<br>Last modified %s<br>Repo: %s",
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
	// _ = os.Mkdir(".site/assets", 0777)

	// if err := CopyDirectory("./assets", "./.site/assets"); err != nil {
	// 	return err
	// }

	return ioutil.WriteFile(".site/index.html", outputContent, 0644)
}

func main() {
	if err := GenerateSite(); err != nil {
		fmt.Fprintf(os.Stderr, "Unhandled error: %v\n", err)
		os.Exit(1)
	}
}
