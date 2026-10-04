package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/refresh"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
	"github.com/moneytosms/verd/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "verd:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load(filepath.Join(config.Dir(), "config.toml"))
	if err != nil {
		return err
	}
	if cfg.Handle == "" {
		return fmt.Errorf("no handle set: add `handle = \"...\"` to %s", filepath.Join(config.Dir(), "config.toml"))
	}
	s, err := store.Open(filepath.Join(config.DataDir(), "verd.db"))
	if err != nil {
		return err
	}
	defer s.Close()

	// ponytail: blocking refresh before the TUI; background refresh is ticket #28.
	client := cf.New(cf.BaseURL)
	note := ""
	if _, err := refresh.Problemset(context.Background(), s, client, time.Now()); err != nil {
		note = "offline: " + err.Error()
	}
	if _, err := refresh.Submissions(context.Background(), s, client, cfg.Handle); err != nil && note == "" {
		note = "offline: " + err.Error()
	}
	if _, err := refresh.Contests(context.Background(), s, client, time.Now()); err != nil && note == "" {
		note = "offline: " + err.Error()
	}
	contests, err := s.Contests()
	if err != nil {
		return err
	}
	ps, err := s.Problems()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		return fmt.Errorf("no cached problems and fetch failed: %s", note)
	}
	statuses, err := s.Statuses()
	if err != nil {
		return err
	}
	deps := tui.Deps{
		Load: func(p cf.Problem, force bool) (*scrape.Detail, error) {
			return refresh.Detail(context.Background(), s, client, p, force, time.Now())
		},
		OpenURL: openURL,
	}
	_, err = tea.NewProgram(tui.New(ps, note, deps).WithStatuses(statuses).WithContests(contests)).Run()
	return err
}

func openURL(url string) error {
	switch {
	case os.Getenv("WSL_DISTRO_NAME") != "":
		return exec.Command("wslview", url).Start()
	case runtime.GOOS == "darwin":
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}
