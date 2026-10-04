package refresh

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
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

func TestSubmissionsIncremental(t *testing.T) {
	// 150 submissions, ids 150..1 newest first; serve by from/count.
	total := 150
	var froms []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		from, _ := strconv.Atoi(r.URL.Query().Get("from"))
		count, _ := strconv.Atoi(r.URL.Query().Get("count"))
		froms = append(froms, r.URL.Query().Get("from"))
		var items []string
		for i := from; i < from+count && i <= total; i++ {
			id := total - i + 1
			v := "OK"
			if id == total {
				v = "TESTING"
			}
			items = append(items, fmt.Sprintf(`{"id":%d,"creationTimeSeconds":%d,"programmingLanguage":"C++","verdict":%q,"problem":{"contestId":1,"index":"A"}}`, id, id, v))
		}
		fmt.Fprintf(w, `{"status":"OK","result":[%s]}`, strings.Join(items, ","))
	}))
	defer srv.Close()
	s, _ := store.Open(":memory:")
	defer s.Close()
	c := cf.New(srv.URL)
	c.Interval = 0
	ctx := context.Background()

	if n, err := Submissions(ctx, s, c, "u"); err != nil || n != 150 {
		t.Fatalf("full sync: %d %v", n, err)
	}
	// #151 arrives; #150 (was TESTING) is now OK: both fetched, stops at known #149
	total = 151
	froms = nil
	if n, err := Submissions(ctx, s, c, "u"); err != nil || n != 2 {
		t.Fatalf("incremental sync: %d %v", n, err)
	}
	if len(froms) != 1 {
		t.Fatalf("should stop on first page at known id, fetched from=%v", froms)
	}
	st, _ := s.Statuses()
	if st["1A"] != store.StatusSolved {
		t.Fatalf("status %v", st)
	}
}

func TestContestsTTL(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.Write([]byte(`{"status":"OK","result":[{"id":2,"name":"Soon","phase":"BEFORE","startTimeSeconds":2000,"durationSeconds":7200},{"id":1,"name":"Old","phase":"FINISHED","startTimeSeconds":1000,"durationSeconds":7200}]}`))
	}))
	defer srv.Close()
	s, _ := store.Open(":memory:")
	defer s.Close()
	c := cf.New(srv.URL)
	c.Interval = 0
	now := time.Now()
	ctx := context.Background()
	if ok, err := Contests(ctx, s, c, now); !ok || err != nil {
		t.Fatalf("first: %v %v", ok, err)
	}
	if ok, _ := Contests(ctx, s, c, now.Add(59*time.Minute)); ok {
		t.Fatal("within 1h must not refetch")
	}
	if ok, _ := Contests(ctx, s, c, now.Add(61*time.Minute)); !ok || hits.Load() != 2 {
		t.Fatalf("after 1h must refetch, hits=%d", hits.Load())
	}
	cs, _ := s.Contests()
	if len(cs) != 2 || cs[0].ID != 2 || cs[1].Finished() != true {
		t.Fatalf("%+v", cs)
	}
}

func TestCoreOfflineKeepsCache(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		switch {
		case strings.Contains(r.URL.Path, "problemset"):
			w.Write([]byte(`{"status":"OK","result":{"problems":[{"contestId":1,"index":"A","name":"X"}],"problemStatistics":[]}}`))
		case strings.Contains(r.URL.Path, "contest.list"):
			w.Write([]byte(`{"status":"OK","result":[]}`))
		case strings.Contains(r.URL.Path, "user.rating"):
			w.Write([]byte(`{"status":"OK","result":[{"contestId":1,"oldRating":1000,"newRating":1100,"ratingUpdateTimeSeconds":5}]}`))
		default:
			w.Write([]byte(`{"status":"OK","result":[]}`))
		}
	}))
	s, _ := store.Open(":memory:")
	defer s.Close()
	c := cf.New(srv.URL)
	c.Interval = 0
	now := time.Now()
	if err := Core(context.Background(), s, c, "u", now); err != nil {
		t.Fatal(err)
	}
	if at, _ := s.LastSync(); at.Unix() != now.Unix() {
		t.Fatalf("LastSync %v", at)
	}
	if r, _ := s.Rating(); len(r) != 1 || r[0].NewRating != 1100 {
		t.Fatalf("rating %+v", r)
	}
	// server dies; a day later everything is stale; offline error, cache intact, stops after one try
	srv.Close()
	hits.Store(0)
	err := Core(context.Background(), s, c, "u", now.Add(25*time.Hour))
	if !errors.Is(err, cf.ErrNetwork) {
		t.Fatalf("want ErrNetwork, got %v", err)
	}
	if ps, _ := s.Problems(); len(ps) != 1 {
		t.Fatal("cache must survive failed refresh")
	}
	if at, _ := s.LastSync(); at.Unix() != now.Unix() {
		t.Fatalf("LastSync must not advance on failure: %v", at)
	}
}
