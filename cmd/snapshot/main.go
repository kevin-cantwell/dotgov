package main

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/asciimoo/colly"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Println("Usage:")
		fmt.Println("\tsnapshot https://www.whitehouse.gov")
		os.Exit(1)
	}

	site, err := url.Parse(os.Args[1])
	if err != nil {
		panic(lineNo(err))
	}
	fmt.Println(site)
	if err := saveHTML(site); err != nil {
		panic(lineNo(err))
	}
	visited[site.EscapedPath()] = true

	// Find and visit all links
	c := colly.NewCollector()
	c.Limit(&colly.LimitRule{
		Parallelism: 20,
	})
	c.OnHTML("a", func(e *colly.HTMLElement) {
		var link string
		switch e.Name {
		case "a", "link":
			link = e.Request.AbsoluteURL(e.Attr("href"))
		case "img", "script":
			link = e.Request.AbsoluteURL(e.Attr("src"))
		}

		if link == "" {
			return
		}

		u, err := url.Parse(link)
		if err != nil {
			return
		}

		// Don't save other sites
		if u.Host != site.Host {
			return
		}

		mu.Lock()
		if visited[u.EscapedPath()] {
			mu.Unlock()
			return
		}
		visited[u.EscapedPath()] = true
		mu.Unlock()

		if err := saveHTML(u); err != nil {
			fmt.Printf("%v\n\t%v\n", u, err)
			return
		}
		fmt.Println(u.String())

		go e.Request.Visit(link)
	})
	c.Visit(site.String())
	c.Wait()
}

var (
	mu      sync.Mutex
	visited = map[string]bool{}
)

func saveHTML(u *url.URL) error {
	// Fetch url contents
	resp, err := http.Get(u.String())
	if err != nil {
		return lineNo(err)
	}
	defer resp.Body.Close()

	buf := bufio.NewReader(resp.Body)
	peeked, err := buf.Peek(512)
	if err != nil {
		return lineNo(err)
	}

	contentType := http.DetectContentType(peeked)
	if !strings.Contains(contentType, "text/html") {
		// Skip non-html pages
		return lineNo(errors.New("wrong content-type: " + contentType))
	}

	dirpath := filepath.Join(u.Hostname(), u.EscapedPath())
	if err := os.MkdirAll(dirpath, 0755); err != nil {
		return lineNo(err)
	}

	writepath := filepath.Join(dirpath, "index.html")
	file, err := os.Create(writepath)
	if err != nil {
		return lineNo(err)
	}
	if _, err := buf.WriteTo(file); err != nil {
		return lineNo(err)
	}

	return nil
}

func lineNo(err error) error {
	if err == nil {
		return err
	}

	_, file, line, ok := runtime.Caller(1)
	if !ok {
		return err
	}
	return fmt.Errorf("%s:%d %s", filepath.Base(file), line, err.Error())
}
