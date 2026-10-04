package refresh

import (
	"context"
	"net/http"
	"net/http/httptest"
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
