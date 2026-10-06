package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/creds"
	"github.com/moneytosms/verd/internal/cses"
	"github.com/moneytosms/verd/internal/store"
	"golang.org/x/term"
)

// provider is a source whose solved tasks can be pulled in with `verd sync`. Adding one (say
// LeetCode) is a login and a solved function here, plus the source itself in internal/cf.
type provider struct {
	source string
	name   string
	creds  creds.Store
	// login trades a username and password for a session cookie. The password is never stored.
	login func(ctx context.Context, user, pass string) (string, error)
	// solved lists the ids of the tasks the session's user has solved.
	solved func(ctx context.Context, cookie string) ([]int, error)
	// expired is the error solved returns when the session no longer works.
	expired error
}

func providers() []provider {
	c := cses.New()
	return []provider{{
		source: cf.SourceCSES, name: "CSES",
		creds:   creds.Store{File: filepath.Join(config.Dir(), "credentials-cses.json"), Account: "cses"},
		login:   c.Login,
		solved:  c.Solved,
		expired: cses.ErrNotLoggedIn,
	}}
}

var errNeedLogin = errors.New("not logged in")

// syncProvider pulls the solved tasks into the store as marks. With a saved session it needs no
// input; otherwise it calls ask for a username and password (nil: fail with errNeedLogin).
func syncProvider(ctx context.Context, p provider, s *store.Store, ask func() (user, pass string, err error)) (solved, added int, err error) {
	var ids []int
	if c, lerr := p.creds.Load(); lerr == nil {
		ids, err = p.solved(ctx, c.Cookie)
		if err != nil && !errors.Is(err, p.expired) {
			return 0, 0, err
		}
	} else {
		err = p.expired
	}
	if errors.Is(err, p.expired) { // no session, or it expired: sign in again
		if ask == nil {
			return 0, 0, errNeedLogin
		}
		user, pass, aerr := ask()
		if aerr != nil {
			return 0, 0, aerr
		}
		cookie, lerr := p.login(ctx, user, pass)
		if lerr != nil {
			return 0, 0, lerr
		}
		if _, _, serr := p.creds.Save(creds.Creds{Cookie: cookie}); serr != nil {
			return 0, 0, serr
		}
		if ids, err = p.solved(ctx, cookie); err != nil {
			return 0, 0, err
		}
	}
	added, err = s.MarkSolved(p.source, ids)
	return len(ids), added, err
}

// promptLogin reads a username and a hidden password (two plain lines when in is not a terminal).
func promptLogin(out io.Writer, in *os.File, name string) func() (string, string, error) {
	return func() (string, string, error) {
		r := bufio.NewReader(in)
		fmt.Fprintf(out, "%s username: ", name)
		user, _ := r.ReadString('\n')
		var pass string
		if term.IsTerminal(int(in.Fd())) {
			fmt.Fprintf(out, "%s password (hidden, not stored): ", name)
			b, err := term.ReadPassword(int(in.Fd()))
			fmt.Fprintln(out)
			if err != nil {
				return "", "", err
			}
			pass = string(b)
		} else {
			pass, _ = r.ReadString('\n')
		}
		user, pass = strings.TrimSpace(user), strings.TrimSpace(pass)
		if user == "" || pass == "" {
			return "", "", &exitError{2, "need a username and a password"}
		}
		return user, pass, nil
	}
}

// syncCmd is `verd sync [provider]`: every provider when none is named.
func syncCmd(ctx context.Context, out io.Writer, in *os.File, args []string, s *store.Store) error {
	var todo []provider
	for _, p := range providers() {
		if len(args) == 0 || strings.EqualFold(args[0], p.source) || strings.EqualFold(args[0], p.name) {
			todo = append(todo, p)
		}
	}
	if len(todo) == 0 {
		return &exitError{2, "unknown provider " + args[0] + " (have: cses)"}
	}
	for _, p := range todo {
		solved, added, err := syncProvider(ctx, p, s, promptLogin(out, in, p.name))
		if err != nil {
			return &exitError{2, p.name + ": " + err.Error()}
		}
		fmt.Fprintf(out, "%s: %d solved, %d new marks\n", p.name, solved, added)
	}
	return nil
}
