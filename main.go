package main

import (
	"flag"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type category struct {
	name string
	url  string
}

func fetchTitles(client *http.Client, endpoint string) ([]string, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "avq/0.1")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request failed: %s", resp.Status)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		return nil, fmt.Errorf("unexpected content type: %s", ct)
	}
	return parseTitles(resp.Body)
}

func parseTitles(r io.Reader) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var titles []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			tds := tdElements(n)
			if len(tds) >= 2 {
				text := strings.Join(strings.Fields(textContent(tds[1])), " ")
				text = stdhtml.UnescapeString(text)
				if text != "" {
					titles = append(titles, text)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return titles, nil
}

func tdElements(row *html.Node) []*html.Node {
	var tds []*html.Node
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			tds = append(tds, c)
		}
	}
	return tds
}

func textContent(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return b.String()
}

func main() {
	movies := flag.Bool("m", false, "include movies")
	shows := flag.Bool("t", false, "include TV shows")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <query>\n\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Query the Audiovault for movies and TV shows.")
		fmt.Fprintln(flag.CommandLine.Output(), "Flags:")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	includeMovies := *movies
	includeShows := *shows
	if !*movies && !*shows {
		includeMovies = true
		includeShows = true
	}
	query := strings.Join(flag.Args(), " ")
	var categories []category
	if includeMovies {
		categories = append(categories, category{
			name: "movies",
			url:  "https://audiovault.net/movies",
		})
	}
	if includeShows {
		categories = append(categories, category{
			name: "shows",
			url:  "https://audiovault.net/shows",
		})
	}
	client := &http.Client{Timeout: 15 * time.Second}
	var results []string
	for _, cat := range categories {
		endpoint := fmt.Sprintf("%s?search=%s", cat.url, url.QueryEscape(query))
		titles, err := fetchTitles(client, endpoint)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to query %s: %v\n", cat.name, err)
			os.Exit(1)
		}
		results = append(results, titles...)
	}
	if len(results) == 0 {
		fmt.Println("No results.")
		return
	}
	for _, item := range results {
		fmt.Println(item)
	}
}
