package refresh

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/store"
)

func TestSecondLaunchMakesNoRequests(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"status":"OK","result":{"problems":[{"contestId":1,"index":"A","name":"X"}],"problemStatistics":[]}}`))
	}))
	defer srv.Close()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	c := cf.New(srv.URL)
	c.Interval = 0
	now := time.Now()

	if ok, err := Problemset(context.Background(), s, c, now); !ok || err != nil {
		t.Fatalf("first launch should fetch: %v %v", ok, err)
	}
	if ok, err := Problemset(context.Background(), s, c, now.Add(23*time.Hour)); ok || err != nil {
		t.Fatalf("within 24h should not fetch: %v %v", ok, err)
	}
	if hits.Load() != 1 {
		t.Fatalf("want 1 request, got %d", hits.Load())
	}
	if ok, _ := Problemset(context.Background(), s, c, now.Add(25*time.Hour)); !ok {
		t.Fatal("stale cache should refetch")
	}
}

func TestDetailCachedAndChallenge(t *testing.T) {
	page, _ := os.ReadFile("../scrape/testdata/1A.html")
	var hits atomic.Int32
	challenge := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if challenge {
			w.Header().Set("cf-mitigated", "challenge")
			w.WriteHeader(403)
			return
		}
		w.Write(page)
	}))
	defer srv.Close()
	s, _ := store.Open(":memory:")
	defer s.Close()
	c := cf.New(srv.URL)
	c.Interval = 0
	p := cf.Problem{ContestID: 1, Index: "A"}
	ctx := context.Background()

	d, err := Detail(ctx, s, c, p, false, time.Now())
	if err != nil || len(d.Samples) != 1 {
		t.Fatalf("%v %+v", err, d)
	}
	if _, err := Detail(ctx, s, c, p, false, time.Now()); err != nil || hits.Load() != 1 {
		t.Fatalf("reopen should hit cache: err=%v hits=%d", err, hits.Load())
	}
	challenge = true
	if _, err := Detail(ctx, s, c, p, true, time.Now()); !errors.Is(err, cf.ErrChallenge) {
		t.Fatalf("want ErrChallenge, got %v", err)
	}
	if d, _ := s.Detail(1, "A"); d == nil {
		t.Fatal("failed refetch must keep cache")
	}
	if _, err := Detail(ctx, s, c, cf.Problem{ContestID: 2, Index: "B"}, false, time.Now()); !errors.Is(err, cf.ErrChallenge) {
		t.Fatalf("uncached + challenge: %v", err)
	}
}
