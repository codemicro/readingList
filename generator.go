package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jszwec/csvutil"
	g "github.com/maragudk/gomponents"
	c "github.com/maragudk/gomponents/components"
	. "github.com/maragudk/gomponents/html"
	"github.com/schollz/progressbar/v3"
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

// renderHTMLPage renders a complete HTML page
func renderHTMLPage(title string, body []g.Node) ([]byte, error) {
	b := new(bytes.Buffer)
	err := c.HTML5(c.HTML5Props{
		Title:    title,
		Language: "en-GB",
		Head:     []g.Node{Link(g.Attr("rel", "stylesheet"), g.Attr("href", "https://www.tdpain.net/assets/css/ghpages.css"), g.Attr("type", "text/css"))},
		Body:     []g.Node{Div(g.Attr("class", "container"), g.Group(body))},
	}).Render(b)
	if err != nil {
		return nil, err
	}
	return b.Bytes(), nil
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
func makeListHTML(groups []*entryGroup) g.Node {

	headerLevel := H3

	numGroups := len(groups)

	var subsections []g.Node
	for i := numGroups - 1; i >= 0; i -= 1 {
		group := groups[i]
		subsections = append(subsections, A(g.Attr("href", "#"+group.ID), g.Textf("%s %d", group.Date.Month().String()[:3], group.Date.Year())))
	}

	parts := []g.Node{
		Br(),
		Span(g.Text("Jump to :: "), g.Group(g.Map(len(subsections), func(i int) g.Node {
			n := subsections[i]
			if i != len(subsections)-1 {
				n = g.Group([]g.Node{n, g.Text(" :: ")})
			}
			return n
		}))),
	}

	for groupNumber, group := range groups {

		dateString := group.Title

		header := headerLevel(g.Attr("id", group.ID), g.Text(dateString))

		pb := progressbar.NewOptions(len(group.Entries),
			progressbar.OptionSetDescription(fmt.Sprintf("[%d/%d] %s", groupNumber+1, numGroups, dateString)),
		)

		var entries []g.Node
		for _, article := range group.Entries {

			entries = append(entries, articleLinkComponent(
				article.URL,
				article.Title,
				article.Description,
				article.Date.Format(dateFormat),
				article.HackerNewsURL),
			)

			pb.Add(1)
		}

		parts = append(parts, header, Ul(entries...))
	}

	fmt.Println() // the progress bars do weird newline things

	return Div(parts...)
}

func articleLinkComponent(url, title, description, date, hnURL string) g.Node {
	return Li(
		A(g.Attr("href", url), g.Text(title)),
		g.Text(" - "+date),
		g.If(hnURL != "", g.Group([]g.Node{
			g.Text(" - "),
			A(
				g.Attr("href", hnURL),
				g.Attr("rel", "noopener"),
				Img(
					g.Attr("src", "https://news.ycombinator.com/y18.gif"),
					g.Attr("height", "14em"),
					g.Attr("title", "View on Hacker News"),
					g.Attr("alt", "Hacker News logo"),
				)),
		})),
		g.If(description != "", Span(g.Attr("class", "secondary"), g.Text(" - "+description))),
	)
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

	head := Div(
		H1(g.Text(pageTitle)),
		P(g.Raw(
			fmt.Sprintf(
				"A mostly complete list of articles I've read on the internet<br>There are currently %d entries in the list<br>Last modified %s<br>Repo: %s",
				numArticles,
				time.Now().Format(dateFormat),
				"<a href=\"https://github.com/codemicro/readingList\" rel=\"noopener\"><code>codemicro/readingList</code></a>",
			),
		)),
	)

	listing := makeListHTML(groupedEntries)

	outputContent, err := renderHTMLPage(pageTitle, []g.Node{head, Hr(), listing})
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
