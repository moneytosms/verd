// Package cses reads the public CSES Problem Set: the task list and each task's statement.
// There is no API, so this parses the HTML pages. Submitting needs a CSES login and is left to
// the browser.
package cses

import (
	"bytes"
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
)

const BaseURL = "https://cses.fi"

const userAgent = "verd (+https://github.com/moneytosms/verd)"

// Client fetches CSES pages, at most one request per Interval.
type Client struct {
	Base     string
	HTTP     *http.Client
	Interval time.Duration

	mu   sync.Mutex
	last time.Time
}

func New() *Client {
	return &Client{Base: BaseURL, HTTP: &http.Client{Timeout: 30 * time.Second}, Interval: time.Second}
}

func (c *Client) get(ctx context.Context, path string) ([]byte, error) {
	c.mu.Lock()
	if d := c.Interval - time.Since(c.last); !c.last.IsZero() && d > 0 {
		time.Sleep(d)
	}
	c.last = time.Now()
	c.mu.Unlock()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", cf.ErrNetwork, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: cses http %d", cf.ErrNetwork, res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

// Problems fetches the task list. A task's section is its only tag; the solver count is its
// popularity (CSES has no ratings).
func (c *Client) Problems(ctx context.Context) ([]cf.Problem, error) {
	page, err := c.get(ctx, "/problemset/")
	if err != nil {
		return nil, err
	}
	return ParseList(page)
}

// Page fetches and parses one task.
func (c *Client) Page(ctx context.Context, id int) (*scrape.Detail, error) {
	page, err := c.get(ctx, fmt.Sprintf("/problemset/task/%d", id))
	if err != nil {
		return nil, err
	}
	return ParseTask(page)
}

var taskHref = regexp.MustCompile(`^/problemset/task/(\d+)$`)

// ParseList extracts the tasks from the problem set page.
func ParseList(page []byte) ([]cf.Problem, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var out []cf.Problem
	doc.Find("h2").Each(func(_ int, h *goquery.Selection) {
		section := strings.TrimSpace(h.Text())
		h.NextFiltered("ul.task-list").Find("li.task").Each(func(_ int, li *goquery.Selection) {
			a := li.Find("a").First()
			href, _ := a.Attr("href")
			m := taskHref.FindStringSubmatch(href)
			if m == nil {
				return
			}
			id, _ := strconv.Atoi(m[1])
			p := cf.Problem{ContestID: id, Index: cf.SourceCSES, Name: strings.TrimSpace(a.Text()), Tags: []string{section}}
			// "solved / attempted"
			if f := strings.Fields(strings.ReplaceAll(li.Find(".detail").Text(), "/", " ")); len(f) > 0 {
				p.SolvedCount, _ = strconv.Atoi(f[0])
			}
			out = append(out, p)
		})
	})
	if len(out) == 0 {
		return nil, fmt.Errorf("no tasks found on the CSES problem set page")
	}
	return out, nil
}

var limitNum = regexp.MustCompile(`[\d.]+`)

// ParseTask turns a task page into a Detail. The statement is stored as a .problem-statement
// block (the shape scrape.Render expects) with math as $$$...$$$ and the example moved out
// into Samples.
func ParseTask(page []byte) (*scrape.Detail, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	md := doc.Find("div.md").First()
	if md.Length() == 0 {
		return nil, scrape.ErrNoStatement
	}
	d := &scrape.Detail{}
	doc.Find(".task-constraints li").Each(func(_ int, li *goquery.Selection) {
		n, _ := strconv.ParseFloat(limitNum.FindString(strings.SplitN(li.Text(), ":", 2)[1]), 64)
		switch {
		case strings.HasPrefix(li.Text(), "Time limit"):
			d.TimeLimitMS = int(n * 1000)
		case strings.HasPrefix(li.Text(), "Memory limit"):
			d.MemoryLimitMB = int(n)
		}
	})
	md.Find("span.math").Each(func(_ int, s *goquery.Selection) {
		s.ReplaceWithHtml("$$$" + html.EscapeString(s.Text()) + "$$$")
	})
	// "Example", "Example 1", ...: each is "Input:" pre "Output:" pre. Lift them into Samples.
	md.Find("h1").FilterFunction(func(_ int, h *goquery.Selection) bool {
		id, _ := h.Attr("id")
		return strings.HasPrefix(id, "example")
	}).Each(func(_ int, ex *goquery.Selection) {
		var in string
		haveIn := false
	walk:
		for n := ex.Next(); n.Length() > 0; n = ex.Next() {
			switch goquery.NodeName(n) {
			case "p":
				if t := strings.TrimSpace(n.Text()); t == "Input:" || t == "Output:" {
					n.Remove()
					continue
				}
			case "pre":
				if !haveIn {
					in, haveIn = n.Text(), true
				} else {
					d.Samples = append(d.Samples, scrape.Sample{Input: in, Output: n.Text()})
					haveIn = false
				}
				n.Remove()
				continue
			}
			break walk
		}
		ex.Remove()
	})
	md.Find("h1").Each(func(_ int, h *goquery.Selection) { h.ReplaceWithHtml("<h3>" + html.EscapeString(h.Text()) + "</h3>") })
	inner, err := md.Html()
	if err != nil {
		return nil, err
	}
	d.Statement = `<div class="problem-statement">` + inner + `</div>`
	if strings.Contains(strings.ToLower(md.Text()), "absolute or relative error") {
		d.Hint = "float"
	}
	return d, nil
}
