package submit

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
)

type fakeDoer struct {
	get, post Resp
	reqs      []Req
}

func (f *fakeDoer) Do(_ context.Context, r Req) (Resp, error) {
	f.reqs = append(f.reqs, r)
	if r.Method == "GET" {
		return f.get, nil
	}
	return f.post, nil
}

func fixture(t *testing.T) []byte {
	b, err := os.ReadFile("testdata/submit-page.html")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseFormFromRealPage(t *testing.T) {
	f, err := ParseForm(fixture(t))
	if err != nil || f.CSRF == "" {
		t.Fatalf("%+v %v", f, err)
	}
	if f.Fields.Get("action") != "submitSolutionFormSubmitted" || !f.Fields.Has("ftaa") || !f.Fields.Has("bfaa") {
		t.Fatalf("fields: %v", f.Fields)
	}
	var py bool
	for _, c := range f.Compilers {
		py = py || (c.ID == 31 && strings.HasPrefix(c.Name, "Python 3"))
	}
	if len(f.Compilers) < 40 || !py {
		t.Fatalf("compilers: %d %v", len(f.Compilers), f.Compilers[:3])
	}
}

func TestParseFormLoggedOut(t *testing.T) {
	if _, err := ParseForm([]byte(`<html><body><a href="/enter">Enter</a></body></html>`)); err == nil {
		t.Fatal("a page without csrf_token must error")
	}
}

func direct(f *fakeDoer) Direct { return Direct{Doer: f, Cookie: "c=1", UserAgent: "UA"} }

func TestDirectPostsFormFields(t *testing.T) {
	f := &fakeDoer{get: Resp{Status: 200, Body: fixture(t)}, post: Resp{Status: 302, Location: "https://codeforces.com/problemset/status?my=on"}}
	if err := direct(f).Submit(context.Background(), 4, "A", 31, []byte("print(1)\n")); err != nil {
		t.Fatal(err)
	}
	post := f.reqs[1]
	form, _ := url.ParseQuery(post.Body)
	for k, want := range map[string]string{"contestId": "4", "submittedProblemIndex": "A", "programTypeId": "31", "source": "print(1)\n", "action": "submitSolutionFormSubmitted", "tabSize": "4"} {
		if form.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, form.Get(k), want)
		}
	}
	if form.Get("ftaa") == "" || form.Get("bfaa") == "" || form.Get("csrf_token") == "" {
		t.Errorf("anti-bot fields missing: %v", form)
	}
	if post.Header["cookie"] != "c=1" || post.Header["user-agent"] != "UA" || !strings.Contains(post.URL, "csrf_token=") {
		t.Errorf("request: %+v", post)
	}
}

func TestDirectFallsBack(t *testing.T) {
	ok := fixture(t)
	for _, tc := range []struct {
		name     string
		f        *fakeDoer
		compiler int
		want     string
	}{
		{"403 on GET", &fakeDoer{get: Resp{Status: 403}}, 31, "blocked"},
		{"redirect to login", &fakeDoer{get: Resp{Status: 302}}, 31, "session expired"},
		{"logged out page", &fakeDoer{get: Resp{Status: 200, Body: []byte("<html></html>")}}, 31, "verd login"},
		{"unknown compiler lists options", &fakeDoer{get: Resp{Status: 200, Body: ok}}, 9999, "31=Python 3"},
		{"403 on POST", &fakeDoer{get: Resp{Status: 200, Body: ok}, post: Resp{Status: 403}}, 31, "blocked"},
		{"redirect to enter", &fakeDoer{get: Resp{Status: 200, Body: ok}, post: Resp{Status: 302, Location: "/enter"}}, 31, "/enter"},
		{"duplicate code", &fakeDoer{get: Resp{Status: 200, Body: ok}, post: Resp{Status: 200, Body: []byte(`<span class="error for__source">You have submitted exactly the same code before</span>`)}}, 31, "exactly the same code"},
	} {
		err := direct(tc.f).Submit(context.Background(), 4, "A", tc.compiler, []byte("x"))
		fb, isFB := err.(*FallbackError)
		if !isFB || !strings.Contains(fb.Reason, tc.want) {
			t.Errorf("%s: got %v, want fallback containing %q", tc.name, err, tc.want)
		}
	}
}
