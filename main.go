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
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	baseURL   = "https://audiovault.net"
	userAgent = "avq/0.1"
)

type result struct {
	titles []string
	err    error
}

func main() {
	showMovies := flag.Bool("m", false, "Search movies")
	showTV := flag.Bool("t", false, "Search TV shows")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags] <query>\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.Parse()
	query := strings.TrimSpace(strings.Join(flag.Args(), " "))
	if query == "" {
		flag.Usage()
		os.Exit(2)
	}
	if !*showMovies && !*showTV {
		*showMovies, *showTV = true, true
	}
	endpoints := make(map[string]string)
	if *showMovies {
		endpoints["Movies"] = baseURL + "/movies"
	}
	if *showTV {
		endpoints["TV Shows"] = baseURL + "/shows"
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resChan := make(chan result, len(endpoints))
	var wg sync.WaitGroup
	for name, path := range endpoints {
		wg.Add(1)
		go func(n, p string) {
			defer wg.Done()
			titles, err := fetch(client, p, query)
			if err != nil {
				resChan <- result{err: fmt.Errorf("[%s] %w", n, err)}
				return
			}
			resChan <- result{titles: titles}
		}(name, path)
	}
	go func() {
		wg.Wait()
		close(resChan)
	}()
	found := false
	for res := range resChan {
		if res.err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", res.err)
			continue
		}
		for _, t := range res.titles {
			fmt.Println(t)
			found = true
		}
	}
	if !found {
		fmt.Println("No results found.")
	}
}

func fetch(client *http.Client, target, query string) ([]string, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("search", query)
	u.RawQuery = q.Encode()
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return parse(resp.Body)
}

func parse(r io.Reader) ([]string, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	var titles []string
	var f func(*html.Node)
	f = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "tr" {
			if title := extractTitleFromRow(n); title != "" {
				titles = append(titles, title)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(doc)
	return titles, nil
}

func extractTitleFromRow(row *html.Node) string {
	tdCount := 0
	for c := row.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && c.Data == "td" {
			tdCount++
			if tdCount == 2 {
				rawText := flattenText(c)
				return stdhtml.UnescapeString(strings.Join(strings.Fields(rawText), " "))
			}
		}
	}
	return ""
}

func flattenText(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var b strings.Builder
	var f func(*html.Node)
	f = func(node *html.Node) {
		if node.Type == html.TextNode {
			b.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			f(c)
		}
	}
	f(n)
	return b.String()
}
