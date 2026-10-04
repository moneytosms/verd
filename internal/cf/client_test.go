package cf

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestProblemsetJoin(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.UserAgent()
		w.Write([]byte(`{"status":"OK","result":{"problems":[{"contestId":1,"index":"A","name":"Theatre Square","rating":1000,"tags":["math"]},{"contestId":1,"index":"B","name":"Spreadsheet"}],"problemStatistics":[{"contestId":1,"index":"B","solvedCount":7},{"contestId":1,"index":"A","solvedCount":42}]}}`))
	}))
	defer srv.Close()
	ps, err := New(srv.URL).Problemset(context.Background())
	if err != nil || len(ps) != 2 {
		t.Fatalf("%v %v", ps, err)
	}
	if ps[0].SolvedCount != 42 || ps[1].SolvedCount != 7 || ps[0].Tags[0] != "math" {
		t.Fatalf("bad join: %+v", ps)
	}
	if ua == "" || ua == "Go-http-client/1.1" {
		t.Fatalf("UA not browser-like: %q", ua)
	}
}

func TestTypedErrors(t *testing.T) {
	status := 200
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		w.Write([]byte(`{"status":"FAILED","comment":"boom"}`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	c.sleep = func(time.Duration) {}
	var ae *APIError
	if _, err := c.Problemset(context.Background()); !errors.As(err, &ae) || ae.Comment != "boom" {
		t.Fatalf("want APIError, got %v", err)
	}
	status = 403
	if _, err := c.Problemset(context.Background()); !errors.Is(err, ErrChallenge) {
		t.Fatalf("want ErrChallenge, got %v", err)
	}
	srv.Close()
	if _, err := c.Problemset(context.Background()); !errors.Is(err, ErrNetwork) {
		t.Fatalf("want ErrNetwork, got %v", err)
	}
}

func TestLimiterSpacing(t *testing.T) {
	c := New("")
	now := time.Unix(0, 0)
	var slept []time.Duration
	c.now = func() time.Time { return now }
	c.sleep = func(d time.Duration) { slept = append(slept, d); now = now.Add(d) }
	c.wait() // first: no wait
	c.wait() // immediate second: full 2s
	now = now.Add(500 * time.Millisecond)
	c.wait() // 500ms elapsed: 1.5s
	if len(slept) != 2 || slept[0] != 2*time.Second || slept[1] != 1500*time.Millisecond {
		t.Fatalf("slept %v", slept)
	}
}
