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

// syncRig holds the CSES provider fake and the state the steps share.
type syncRig struct {
	p      provider
	s      *store.Store
	ctx    context.Context
	logins int
}

// askOK answers the password prompt with the account the fake accepts.
func askOK() (string, string, error) { return "me", "pw", nil }

func TestSyncSignsInOnceThenReusesTheSession(t *testing.T) {
	keyring.MockInit()
	s, _ := store.Open(":memory:")
	defer s.Close()
	r := &syncRig{s: s, ctx: context.Background()}
	expired := errors.New("expired")
	r.p = provider{
		source: cf.SourceCSES, name: "CSES", expired: expired,
		creds: creds.Store{File: filepath.Join(t.TempDir(), "c.json"), Account: "cses-test"},
		login: func(_ context.Context, u, pw string) (string, error) {
			r.logins++
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
	t.Run("no session and no prompt", func(t *testing.T) { checkSyncNeedsLogin(t, r) })
	t.Run("first sync signs in", func(t *testing.T) { checkFirstSync(t, r) })
	t.Run("saved session reused", func(t *testing.T) { checkSessionReused(t, r) })
	t.Run("expired session asks again", func(t *testing.T) { checkExpiredAsksAgain(t, r) })
}

func checkSyncNeedsLogin(t *testing.T, r *syncRig) {
	if _, _, err := syncProvider(r.ctx, r.p, r.s, nil); !errors.Is(err, errNeedLogin) {
		t.Fatalf("no session and no prompt: %v", err)
	}
}

func checkFirstSync(t *testing.T, r *syncRig) {
	solved, added, err := syncProvider(r.ctx, r.p, r.s, askOK)
	if err != nil || solved != 2 || added != 2 || r.logins != 1 {
		t.Fatalf("first sync: %d %d %v logins=%d", solved, added, err, r.logins)
	}
	if st, _ := r.s.Statuses(); st["1068cses"] != store.StatusSolved || st["1083cses"] != store.StatusSolved {
		t.Fatalf("marks: %v", st)
	}
}

// the saved session works without a prompt, and nothing is added twice
func checkSessionReused(t *testing.T, r *syncRig) {
	solved, added, err := syncProvider(r.ctx, r.p, r.s, nil)
	if err != nil || solved != 2 || added != 0 || r.logins != 1 {
		t.Fatalf("second sync: %d %d %v logins=%d", solved, added, err, r.logins)
	}
}

// a hand-made mark survives a sync, and an expired session asks again
func checkExpiredAsksAgain(t *testing.T, r *syncRig) {
	r.s.SetMark(1069, cf.SourceCSES, true)
	r.p.creds.Save(creds.Creds{Cookie: "PHPSESSID=stale"})
	if _, _, err := syncProvider(r.ctx, r.p, r.s, nil); !errors.Is(err, errNeedLogin) {
		t.Fatalf("expired: %v", err)
	}
	if _, _, err := syncProvider(r.ctx, r.p, r.s, askOK); err != nil || r.logins != 2 {
		t.Fatalf("re-login: %v logins=%d", err, r.logins)
	}
	if st, _ := r.s.Statuses(); st["1069cses"] != store.StatusSolved {
		t.Fatal("sync must not clear manual marks")
	}
}
