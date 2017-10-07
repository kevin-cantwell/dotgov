package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"

	"github.com/asciimoo/colly"
)

func main() {
	site, err := url.Parse("https://www.whitehouse.gov")
	if err != nil {
		panic(lineNo(err))
	}
	fmt.Println(site)
	if err := saveURL(site); err != nil {
		panic(lineNo(err))
	}
	visited[site.String()] = true

	// Find and visit all links
	c := colly.NewCollector()
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

		if visited[link] {
			return
		}
		defer func() { visited[link] = true }()

		u, err := url.Parse(link)
		if err != nil {
			return
		}

		// Don't save other sites
		if u.Host != site.Host {
			return
		}

		fmt.Println(link)
		if err := saveURL(u); err != nil {
			fmt.Println("\t", err)
			return
		}

		c.Visit(link)
	})
	c.Visit(site.String())

	fileServer := http.FileServer(http.Dir(site.Host))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		repath := func(path string) string {
			base := filepath.Base(path)
			if base == "." {
				base = ""
			}
			return filepath.Dir(path) + "/_" + base
		}
		r.URL.Path = repath(r.URL.Path)
		r.URL.RawPath = repath(r.URL.RawPath)
		fileServer.ServeHTTP(w, r)
	})
	fmt.Println("listening at http://localhost:7000")
	http.ListenAndServe(":7000", nil)
}

var (
	visited = map[string]bool{}
)

func saveURL(u *url.URL) error {
	// Fetch url contents
	resp, err := http.Get(u.String())
	if err != nil {
		return lineNo(err)
	}
	defer resp.Body.Close()

	// Relativize host strings
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return lineNo(err)
	}
	// body = bytes.Replace(body, []byte(fmt.Sprintf("%s://%s", u.Scheme, u.Host)), []byte("/"), -1)

	// Write file
	filename := filepath.Base(u.EscapedPath())
	if filename == "." {
		filename = ""
	}
	filename = "_" + filename // This is to differentiate requests for files from directories of the same name
	dirpath := filepath.Join(u.Hostname(), filepath.Dir(u.EscapedPath()))
	if err := os.MkdirAll(dirpath, 0755); err != nil {
		return lineNo(err)
	}

	writepath := filepath.Join(dirpath, filename)
	if err := ioutil.WriteFile(writepath, body, 0644); err != nil {
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
