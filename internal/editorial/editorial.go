// Package editorial finds a Codeforces problem's editorial and splits it into the part about that
// problem. The link is in the "Contest materials" box of the problem page; the text is the blog
// post behind it (blogEntry.view).
package editorial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/PuerkitoBio/goquery"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

type Kind string

const (
	Tutorial     Kind = "tutorial"
	Announcement Kind = "announcement"
	Video        Kind = "video"
	Stream       Kind = "stream"
	Other        Kind = "other"
)

// Material is one entry of the "Contest materials" box.
type Material struct {
	Kind   Kind
	Label  string // as shown on the page, e.g. "Tutorial #1"
	URL    string
	BlogID int    // 0 when the link is not a Codeforces blog post
	Locale string // "en", "ru" or "" when the page does not say
}

var blogPath = regexp.MustCompile(`^/blog/entry/(\d+)/?$`)

// clean drops control characters: titles and labels come from the network and end up in a terminal.
func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, s)
}

// linkOf turns a scraped href into a safe link: relative ones resolve against Codeforces, only
// http(s) is kept, and a Codeforces blog post gets its URL rebuilt from the id alone.
func linkOf(href string) (link string, blogID int, ok bool) {
	base, err := url.Parse(cf.BaseURL)
	if err != nil {
		return "", 0, false
	}
	u, err := base.Parse(strings.TrimSpace(href))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", 0, false
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	if host == strings.TrimPrefix(base.Hostname(), "www.") || host == "codeforces.com" {
		if g := blogPath.FindStringSubmatch(u.Path); g != nil {
			id, _ := strconv.Atoi(g[1])
			return cf.BaseURL + "/blog/entry/" + g[1], id, true
		}
	}
	return clean(u.String()), 0, true
}

func kindOf(label string) Kind {
	l := strings.ToLower(label)
	switch {
	case strings.Contains(l, "video"):
		return Video
	case strings.Contains(l, "stream") || strings.Contains(l, "discussion"):
		return Stream
	case strings.Contains(l, "tutorial") || strings.Contains(l, "editorial") || strings.Contains(l, "разбор"):
		return Tutorial
	case strings.Contains(l, "announcement") || strings.Contains(l, "анонс"):
		return Announcement
	}
	return Other
}

// ParseMaterials reads the "Contest materials" box of a problem or contest page; a page without
// the box (old contests) yields none.
func ParseMaterials(page []byte) ([]Material, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	var out []Material
	doc.Find(".roundbox").Each(func(_ int, box *goquery.Selection) {
		if !strings.Contains(box.Find(".caption").First().Text(), "Contest materials") {
			return
		}
		box.Find("ul li a[href]").Each(func(_ int, a *goquery.Selection) {
			href, _ := a.Attr("href")
			link, id, ok := linkOf(href)
			if !ok {
				return
			}
			loc := strings.Trim(strings.TrimSpace(a.Find(".resource-locale").Text()), "()")
			a.Find(".resource-locale").Remove()
			label := clean(strings.TrimSpace(a.Text()))
			out = append(out, Material{Kind: kindOf(label), Label: label, URL: link, BlogID: id, Locale: clean(loc)})
		})
	})
	return out, nil
}

// PickTutorial chooses the editorial post: a Tutorial that is a blog post, English first, the
// locale-less ones next; an explicitly Russian one is never picked.
func PickTutorial(ms []Material) (Material, bool) {
	best, rank := Material{}, 0
	for _, m := range ms {
		if m.Kind != Tutorial || m.BlogID == 0 || m.Locale == "ru" {
			continue
		}
		r := 1
		if m.Locale == "en" {
			r = 2
		}
		if r > rank {
			best, rank = m, r
		}
	}
	return best, rank > 0
}

// Block is a piece of the editorial: plain text, or a spoiler (hint, solution, code) that stays
// closed until asked for.
type Block struct {
	Title   string // spoiler title; empty for plain text
	HTML    string
	Spoiler bool
}

var problemHref = regexp.MustCompile(`/(?:contest|problemset/problem)/(\d+)/(?:problem/)?([A-Za-z]\d?)/?$`)

// Sections returns the blocks about one problem: from the paragraph that starts with its link
// (`1900A - Name`) up to the next problem's. When the post has no such paragraph for it, whole is
// true and the blocks are the entire post.
func Sections(content string, contest int, index string) (blocks []Block, whole bool, err error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(content))
	if err != nil {
		return nil, false, err
	}
	root := doc.Find(".ttypography").First()
	if root.Length() == 0 {
		root = doc.Find("body")
	}
	kids := root.Children()
	marks := map[int]string{}
	kids.Each(func(i int, el *goquery.Selection) {
		a := el.Find("a[href]").First()
		href, _ := a.Attr("href")
		g := problemHref.FindStringSubmatch(href)
		if g == nil || !strings.HasPrefix(strings.TrimSpace(el.Text()), strings.TrimSpace(a.Text())) {
			return
		}
		if n, _ := strconv.Atoi(g[1]); n == contest {
			marks[i] = strings.ToUpper(g[2])
		}
	})
	from, to := 0, kids.Length()
	whole = true
	for i := 0; i < kids.Length(); i++ {
		if marks[i] == strings.ToUpper(index) {
			from, whole = i, false
			for j := i + 1; j < kids.Length(); j++ {
				if _, ok := marks[j]; ok {
					to = j
					break
				}
			}
			break
		}
	}
	var plain strings.Builder
	flush := func() {
		if strings.TrimSpace(plain.String()) != "" {
			blocks = append(blocks, Block{HTML: plain.String()})
		}
		plain.Reset()
	}
	kids.Slice(from, to).Each(func(_ int, el *goquery.Selection) {
		if el.Is("div.spoiler") {
			flush()
			title := clean(strings.TrimSpace(el.Find("b.spoiler-title").First().Text()))
			body, _ := el.Find(".spoiler-content").First().Html()
			blocks = append(blocks, Block{Title: title, HTML: body, Spoiler: true})
			return
		}
		// a spoiler nested in plain text must not leak: leave a marker instead
		c := el.Clone()
		c.Find("div.spoiler").Each(func(_ int, s *goquery.Selection) {
			s.ReplaceWithHtml("<p><i>[hidden: " + strings.TrimSpace(s.Find("b.spoiler-title").First().Text()) + "]</i></p>")
		})
		h, _ := goquery.OuterHtml(c)
		plain.WriteString(h)
	})
	flush()
	for i := range blocks { // links in a post are site-relative: make them usable in a terminal
		blocks[i].HTML = strings.ReplaceAll(blocks[i].HTML, `href="/`, `href="`+cf.BaseURL+`/`)
	}
	return blocks, whole, nil
}

// Status says what Load found.
type Status string

const (
	OK          Status = "ok"
	None        Status = "none"        // no editorial is published
	NotEnglish  Status = "not-english" // only a Russian one
	Unsupported Status = "unsupported" // not a Codeforces problem
)

// Result is a problem's editorial ready to show.
type Result struct {
	Status Status
	Title  string
	URL    string
	Blocks []Block
	Whole  bool       // the post could not be split: Blocks is the entire post
	Links  []Material // announcement, video, discussion and further tutorials: shown as links only
}

// Spoilers counts the blocks that start closed.
func (r *Result) Spoilers() int {
	n := 0
	for _, b := range r.Blocks {
		if b.Spoiler {
			n++
		}
	}
	return n
}

// materialsTTL is how long a contest without an editorial is trusted: one may be published later.
const materialsTTL = 6 * time.Hour

func titleText(html string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return clean(html)
	}
	return clean(strings.TrimSpace(doc.Text()))
}

// Load returns a problem's editorial, cache-first: the materials box comes from the problem
// page (fetched once per contest), the post from blogEntry.view (fetched once per post).
func Load(ctx context.Context, s *store.Store, c *cf.Client, p cf.Problem, now time.Time) (*Result, error) {
	if p.Source() != cf.SourceCF {
		return &Result{Status: Unsupported}, nil
	}
	ms, err := materials(ctx, s, c, p, now)
	if err != nil {
		return nil, err
	}
	res := &Result{}
	pick, ok := PickTutorial(ms)
	for _, m := range ms {
		if m != pick || !ok {
			res.Links = append(res.Links, m)
		}
	}
	if !ok {
		res.Status = None
		for _, m := range ms {
			if m.Kind == Tutorial && m.Locale == "ru" {
				res.Status = NotEnglish
			}
		}
		return res, nil
	}
	b, err := s.Blog(pick.BlogID)
	if err != nil {
		return nil, err
	}
	if b == nil {
		if b, err = c.BlogEntry(ctx, pick.BlogID); err != nil {
			return nil, err
		}
		b.ID = pick.BlogID
		if err := s.SaveBlog(b, now); err != nil {
			return nil, err
		}
	}
	if b.Locale == "ru" {
		res.Status = NotEnglish
		return res, nil
	}
	res.Status, res.URL, res.Title = OK, pick.URL, titleText(b.Title)
	res.Blocks, res.Whole, err = Sections(b.Content, p.ContestID, p.Index)
	return res, err
}

func materials(ctx context.Context, s *store.Store, c *cf.Client, p cf.Problem, now time.Time) ([]Material, error) {
	data, at, ok, err := s.Materials(p.ContestID)
	if err != nil {
		return nil, err
	}
	var ms []Material
	if ok {
		if err := json.Unmarshal([]byte(data), &ms); err != nil {
			return nil, fmt.Errorf("cached materials: %w", err)
		}
		if _, has := PickTutorial(ms); has || now.Sub(at) < materialsTTL {
			return ms, nil
		}
	}
	page, err := c.Page(ctx, p.ContestID, p.Index)
	if err != nil {
		if ok {
			return ms, nil // offline: the stale list is better than nothing
		}
		return nil, err
	}
	if ms, err = ParseMaterials(page); err != nil {
		return nil, err
	}
	b, _ := json.Marshal(ms)
	return ms, s.SaveMaterials(p.ContestID, string(b), now)
}
