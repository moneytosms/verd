package editorial

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func blogContent(t *testing.T, id string) string {
	var env struct{ Result cf.BlogEntry }
	if err := json.Unmarshal(fixture(t, "blog_"+id+".json"), &env); err != nil {
		t.Fatal(err)
	}
	return env.Result.Content
}

func TestParseMaterialsVariants(t *testing.T) {
	t.Run("announcement and tutorial", func(t *testing.T) { checkAnnouncementTutorial(t) })
	t.Run("locale-less tutorial", func(t *testing.T) { checkLocaleLessTutorial(t) })
	t.Run("no box", func(t *testing.T) {
		if ms, err := ParseMaterials([]byte(`<html><body><div class="problem-statement"></div></body></html>`)); err != nil || len(ms) != 0 {
			t.Fatalf("a page without the box has none: %+v %v", ms, err)
		}
	})
	t.Run("pick tutorial", func(t *testing.T) { checkPickTutorial(t) })
}

func checkAnnouncementTutorial(t *testing.T) {
	ms, err := ParseMaterials(fixture(t, "materials_announcement_tutorial.html"))
	if err != nil || len(ms) != 2 || ms[0].Kind != Announcement || ms[1].Kind != Tutorial || ms[1].BlogID != 122677 || ms[1].Locale != "en" {
		t.Fatalf("%+v %v", ms, err)
	}
	if ms[1].URL != "https://codeforces.com/blog/entry/122677" {
		t.Fatalf("url %q", ms[1].URL)
	}
}

// "Tutorial #1" has no locale label; a video and a discussion stream are links only
func checkLocaleLessTutorial(t *testing.T) {
	ms, _ := ParseMaterials(fixture(t, "materials_video_only.html"))
	kinds := map[Kind]int{}
	for _, m := range ms {
		kinds[m.Kind]++
	}
	if kinds[Tutorial] != 1 || kinds[Video] != 1 || kinds[Stream] != 1 || kinds[Announcement] != 1 {
		t.Fatalf("%+v", ms)
	}
	pick, ok := PickTutorial(ms)
	if !ok || pick.BlogID != 132689 || pick.Locale != "" {
		t.Fatalf("a locale-less Tutorial is still the editorial: %+v %v", pick, ok)
	}
}

func checkPickTutorial(t *testing.T) {
	if _, ok := PickTutorial([]Material{{Kind: Video, BlogID: 1}, {Kind: Announcement, BlogID: 2}}); ok {
		t.Fatal("videos and announcements are not editorials")
	}
	if _, ok := PickTutorial([]Material{{Kind: Tutorial, BlogID: 3, Locale: "ru"}}); ok {
		t.Fatal("a Russian-only Tutorial is not picked")
	}
	if m, ok := PickTutorial([]Material{{Kind: Tutorial, BlogID: 4}, {Kind: Tutorial, BlogID: 5, Locale: "en"}}); !ok || m.BlogID != 5 {
		t.Fatalf("english wins: %+v %v", m, ok)
	}
}

func TestSectionsSplitByProblemLink(t *testing.T) {
	c := blogContent(t, "118466")
	t.Run("own section", func(t *testing.T) { checkOwnSection(t, c) })
	t.Run("last problem", func(t *testing.T) { checkLastProblem(t, c) })
	t.Run("no section", func(t *testing.T) { checkNoSection(t, c) })
	t.Run("one-line intros", func(t *testing.T) { checkOneLineIntros(t) })
}

func checkOwnSection(t *testing.T, c string) {
	blocks, whole, err := Sections(c, 1850, "B")
	if err != nil || whole || len(blocks) < 3 {
		t.Fatalf("%v whole=%v n=%d", err, whole, len(blocks))
	}
	if blocks[0].Spoiler || !strings.Contains(blocks[0].HTML, "1850B - Ten Words of Wisdom") {
		t.Fatalf("starts with the problem's own line: %+v", blocks[0])
	}
	spoilers := 0
	for _, b := range blocks {
		if !b.Spoiler && (strings.Contains(b.HTML, "1850A - ") || strings.Contains(b.HTML, "1850C - ")) {
			t.Fatalf("neighbouring problems leaked in:\n%s", b.HTML)
		}
		if b.Spoiler {
			spoilers++
			if b.Title == "" || strings.Contains(b.HTML, `class="spoiler"`) {
				t.Fatalf("spoiler: %+v", b)
			}
		}
	}
	if spoilers < 2 {
		t.Fatalf("hints and a solution are spoilers: %d", spoilers)
	}
}

func checkLastProblem(t *testing.T, c string) {
	last, _, _ := Sections(c, 1850, "H")
	if len(last) == 0 || strings.Contains(last[0].HTML, "1850G - ") {
		t.Fatal("the last problem runs to the end of the post")
	}
}

func checkNoSection(t *testing.T, c string) {
	blocks, whole, _ := Sections(c, 1850, "Z")
	if !whole || len(blocks) == 0 {
		t.Fatalf("no section for the problem: the whole post, whole=%v", whole)
	}
	if _, whole, _ := Sections(c, 9999, "B"); !whole {
		t.Fatal("another contest's links are not sections")
	}
}

func checkOneLineIntros(t *testing.T) {
	b, whole, _ := Sections(blogContent(t, "88591"), 1501, "A")
	if whole || len(b) != 1 || !strings.Contains(b[0].HTML, "Alexey and Train") || strings.Contains(b[0].HTML, "Napoleon") {
		t.Fatalf("a post of one-line intros: whole=%v %+v", whole, b)
	}
}

func TestNestedSpoilerDoesNotLeak(t *testing.T) {
	c := `<div class="ttypography"><p><a href="/contest/5/problem/A">5A - x</a></p><blockquote>idea <div class="spoiler"><b class="spoiler-title">Solution</b><div class="spoiler-content" style="display:none"><p>SECRET</p></div></div></blockquote></div>`
	blocks, _, _ := Sections(c, 5, "A")
	for _, b := range blocks {
		if strings.Contains(b.HTML, "SECRET") {
			t.Fatalf("a nested spoiler leaked: %+v", b)
		}
	}
	if len(blocks) != 1 || !strings.Contains(blocks[0].HTML, "[hidden: Solution]") {
		t.Fatalf("%+v", blocks)
	}
}

type fakeCF struct {
	pages, blogs atomic.Int32
	materials    string
	blogID       string
	locale       string
}

func (f *fakeCF) client(t *testing.T) *cf.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/problemset/problem/"):
			f.pages.Add(1)
			w.Write(fixture(t, f.materials))
		case r.URL.Path == "/api/blogEntry.view":
			f.blogs.Add(1)
			var env struct {
				Status string
				Result map[string]any
			}
			json.Unmarshal(fixture(t, "blog_"+f.blogID+".json"), &env)
			if f.locale != "" {
				env.Result["locale"] = f.locale
			}
			json.NewEncoder(w).Encode(env)
		default:
			t.Errorf("unexpected %s", r.URL)
		}
	}))
	t.Cleanup(srv.Close)
	c := cf.New(srv.URL)
	c.Interval = 0
	return c
}

func openStore(t *testing.T) *store.Store {
	s, err := store.Open(filepath.Join(t.TempDir(), "v.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLoadCachesPageAndPost(t *testing.T) {
	f := &fakeCF{materials: "materials_announcement_tutorial.html", blogID: "118466"}
	c, s := f.client(t), openStore(t)
	now := time.Unix(1_000_000, 0)
	p := cf.Problem{ContestID: 1850, Index: "B"}
	res, err := Load(context.Background(), s, c, p, now)
	if err != nil || res.Status != OK || res.Whole || res.Spoilers() < 2 {
		t.Fatalf("%+v %v", res, err)
	}
	if res.URL != "https://codeforces.com/blog/entry/122677" || !strings.Contains(res.Title, "Editorial") {
		t.Fatalf("%+v", res)
	}
	if len(res.Links) != 1 || res.Links[0].Kind != Announcement {
		t.Fatalf("the announcement is a link: %+v", res.Links)
	}
	// another problem of the same contest: no new request at all
	if _, err := Load(context.Background(), s, c, cf.Problem{ContestID: 1850, Index: "C"}, now); err != nil {
		t.Fatal(err)
	}
	if f.pages.Load() != 1 || f.blogs.Load() != 1 {
		t.Fatalf("page=%d blog=%d", f.pages.Load(), f.blogs.Load())
	}
}

func TestLoadNoEditorialAndRussianOnly(t *testing.T) {
	// links only: announcement but no tutorial
	f := &fakeCF{materials: "materials_video_only.html", blogID: "118466"}
	c, s := f.client(t), openStore(t)
	// strip the Tutorial out of the cached list to model a video-only contest
	data := `[{"Kind":"video","Label":"Video Tutorial","URL":"https://codeforces.com/blog/entry/1","BlogID":1,"Locale":"en"},{"Kind":"stream","Label":"Discussion stream","URL":"https://codeforces.com/blog/entry/2","BlogID":2,"Locale":"en"}]`
	now := time.Unix(1_000_000, 0)
	s.SaveMaterials(7, data, now)
	res, err := Load(context.Background(), s, c, cf.Problem{ContestID: 7, Index: "A"}, now.Add(time.Hour))
	if err != nil || res.Status != None || len(res.Links) != 2 || f.blogs.Load() != 0 || f.pages.Load() != 0 {
		t.Fatalf("%+v %v pages=%d", res, err, f.pages.Load())
	}
	// a contest without an editorial is looked up again after a few hours
	if _, err := Load(context.Background(), s, c, cf.Problem{ContestID: 7, Index: "A"}, now.Add(7*time.Hour)); err != nil || f.pages.Load() != 1 {
		t.Fatalf("stale list should refetch the page: pages=%d %v", f.pages.Load(), err)
	}

	// the post exists but is Russian only
	f2 := &fakeCF{materials: "materials_announcement_tutorial.html", blogID: "118466", locale: "ru"}
	res, err = Load(context.Background(), openStore(t), f2.client(t), cf.Problem{ContestID: 1850, Index: "A"}, now)
	if err != nil || res.Status != NotEnglish || len(res.Blocks) != 0 {
		t.Fatalf("%+v %v", res, err)
	}

	res, err = Load(context.Background(), openStore(t), f.client(t), cf.Problem{ContestID: 1068, Index: cf.SourceCSES}, now)
	if err != nil || res.Status != Unsupported {
		t.Fatalf("CSES has no editorials: %+v %v", res, err)
	}
}

func TestHostileMaterialsAreSanitized(t *testing.T) {
	page := "<html><body><div class=\"roundbox\"><div class=\"caption\">Contest materials</div><ul>" +
		"<li><a href=\"/blog/entry/5\">Tutorial\x1b[31mred</a></li>" +
		"<li><a href=\"https://evil.example/blog/entry/6\">Tutorial</a></li>" +
		"<li><a href=\"javascript:alert(1)\">Tutorial</a></li>" +
		"<li><a href=\"/blog/entry/7?x=1#y\">Tutorial #2</a></li>" +
		"<li><a href=\"//codeforces.com/blog/entry/8/\">Tutorial #3</a></li>" +
		"<li><a href=\"https://youtu.be/abc\">Video Tutorial</a></li>" +
		"</ul></div></body></html>"
	ms, err := ParseMaterials([]byte(page))
	if err != nil {
		t.Fatal(err)
	}
	byURL := map[string]Material{}
	for _, m := range ms {
		byURL[m.URL] = m
		if strings.ContainsAny(m.Label+m.URL, "\x1b\x07") {
			t.Fatalf("control characters survived: %+v", m)
		}
	}
	t.Run("relative and foreign links", func(t *testing.T) { checkHostileLinks(t, byURL) })
	t.Run("rebuilt urls", func(t *testing.T) { checkRebuiltURLs(t, byURL) })
	if pick, ok := PickTutorial(ms); !ok || pick.BlogID == 0 {
		t.Fatalf("%+v", pick)
	}
}

func checkHostileLinks(t *testing.T, byURL map[string]Material) {
	if m := byURL["https://codeforces.com/blog/entry/5"]; m.BlogID != 5 || m.Label != "Tutorial[31mred" {
		t.Fatalf("relative blog link: %+v", m)
	}
	if m, ok := byURL["https://evil.example/blog/entry/6"]; !ok || m.BlogID != 0 {
		t.Fatalf("a foreign host is a plain link, never a blog id to fetch: %+v", m)
	}
	for u := range byURL {
		if strings.HasPrefix(u, "javascript:") {
			t.Fatal("javascript: links must be dropped")
		}
	}
}

func checkRebuiltURLs(t *testing.T, byURL map[string]Material) {
	if byURL["https://codeforces.com/blog/entry/7"].BlogID != 7 {
		t.Fatal("the URL is rebuilt from the id alone: the query and fragment are dropped")
	}
	if byURL["https://codeforces.com/blog/entry/8"].BlogID != 8 {
		t.Fatalf("protocol-relative codeforces link with a trailing slash: %+v", byURL)
	}
	if m := byURL["https://youtu.be/abc"]; m.Kind != Video || m.BlogID != 0 {
		t.Fatalf("%+v", m)
	}
}

func TestSpoilerTitleIsCleaned(t *testing.T) {
	c := "<div class=\"ttypography\"><p><a href=\"/contest/5/problem/A\">5A - x</a></p><div class=\"spoiler\"><b class=\"spoiler-title\">Hint\x1b[2J</b><div class=\"spoiler-content\"><p>t</p></div></div></div>"
	blocks, _, _ := Sections(c, 5, "A")
	for _, b := range blocks {
		if strings.Contains(b.Title, "\x1b") {
			t.Fatalf("%q", b.Title)
		}
	}
}
