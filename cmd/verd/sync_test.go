package main

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/creds"
	"github.com/moneytosms/verd/internal/store"
	"github.com/zalando/go-keyring"
)

func TestSyncSignsInOnceThenReusesTheSession(t *testing.T) {
	keyring.MockInit()
	s, _ := store.Open(":memory:")
	defer s.Close()
	logins, expired := 0, errors.New("expired")
	p := provider{
		source: cf.SourceCSES, name: "CSES", expired: expired,
		creds: creds.Store{File: filepath.Join(t.TempDir(), "c.json"), Account: "cses-test"},
		login: func(_ context.Context, u, pw string) (string, error) {
			logins++
			if u != "me" || pw != "pw" {
				return "", errors.New("bad password")
			}
			return "PHPSESSID=ok", nil
		},
		solved: func(_ context.Context, cookie string) ([]int, error) {
			if cookie != "PHPSESSID=ok" {
				return nil, expired
			}
			return []int{1068, 1083}, nil
		},
	}
	ctx := context.Background()
	if _, _, err := syncProvider(ctx, p, s, nil); !errors.Is(err, errNeedLogin) {
		t.Fatalf("no session and no prompt: %v", err)
	}
	ask := func() (string, string, error) { return "me", "pw", nil }
	solved, added, err := syncProvider(ctx, p, s, ask)
	if err != nil || solved != 2 || added != 2 || logins != 1 {
		t.Fatalf("first sync: %d %d %v logins=%d", solved, added, err, logins)
	}
	if st, _ := s.Statuses(); st["1068cses"] != store.StatusSolved || st["1083cses"] != store.StatusSolved {
		t.Fatalf("marks: %v", st)
	}
	// the saved session works without a prompt, and nothing is added twice
	solved, added, err = syncProvider(ctx, p, s, nil)
	if err != nil || solved != 2 || added != 0 || logins != 1 {
		t.Fatalf("second sync: %d %d %v logins=%d", solved, added, err, logins)
	}
	// a hand-made mark survives a sync, and an expired session asks again
	s.SetMark(1069, cf.SourceCSES, true)
	p.creds.Save(creds.Creds{Cookie: "PHPSESSID=stale"})
	if _, _, err := syncProvider(ctx, p, s, nil); !errors.Is(err, errNeedLogin) {
		t.Fatalf("expired: %v", err)
	}
	if _, _, err := syncProvider(ctx, p, s, ask); err != nil || logins != 2 {
		t.Fatalf("re-login: %v logins=%d", err, logins)
	}
	if st, _ := s.Statuses(); st["1069cses"] != store.StatusSolved {
		t.Fatal("sync must not clear manual marks")
	}
}
