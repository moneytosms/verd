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
	ms, err := ParseMaterials(fixture(t, "materials_announcement_tutorial.html"))
	if err != nil || len(ms) != 2 || ms[0].Kind != Announcement || ms[1].Kind != Tutorial || ms[1].BlogID != 122677 || ms[1].Locale != "en" {
		t.Fatalf("%+v %v", ms, err)
	}
	if ms[1].URL != "https://codeforces.com/blog/entry/122677" {
		t.Fatalf("url %q", ms[1].URL)
	}

	// "Tutorial #1" has no locale label; a video and a discussion stream are links only
	ms, _ = ParseMaterials(fixture(t, "materials_video_only.html"))
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

	if ms, err := ParseMaterials([]byte(`<html><body><div class="problem-statement"></div></body></html>`)); err != nil || len(ms) != 0 {
		t.Fatalf("a page without the box has none: %+v %v", ms, err)
	}
	if _, ok := PickTutorial([]Material{{Kind: Video, BlogID: 1}, {Kind: Announcement, BlogID: 2}}); ok {
		t.Fatal("videos and announcements are not editorials")
	}
	if _, ok := PickTutorial([]Material{{Kind: Tutorial, BlogID: 3, Locale: "ru"}}); ok {
		t.Fatal("a Russian-only Tutorial is not picked")
	}
	if m, _ := PickTutorial([]Material{{Kind: Tutorial, BlogID: 4}, {Kind: Tutorial, BlogID: 5, Locale: "en"}}); m.BlogID != 5 {
		t.Fatalf("english wins: %+v", m)
	}
}

func TestSectionsSplitByProblemLink(t *testing.T) {
	c := blogContent(t, "118466")
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
	last, _, _ := Sections(c, 1850, "H")
	if len(last) == 0 || strings.Contains(last[0].HTML, "1850G - ") {
		t.Fatal("the last problem runs to the end of the post")
	}

	blocks, whole, _ = Sections(c, 1850, "Z")
	if !whole || len(blocks) == 0 {
		t.Fatalf("no section for the problem: the whole post, whole=%v", whole)
	}
	if _, whole, _ := Sections(c, 9999, "B"); !whole {
		t.Fatal("another contest's links are not sections")
	}

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
	if err != nil || res.Status != OK || res.Whole || res.Spoilers() < 2 || res.URL != "https://codeforces.com/blog/entry/122677" || !strings.Contains(res.Title, "Editorial") {
		t.Fatalf("%+v %v", res, err)
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
