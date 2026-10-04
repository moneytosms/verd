package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/refresh"
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
	note := ""
	if _, err := refresh.Problemset(context.Background(), s, cf.New(cf.BaseURL), time.Now()); err != nil {
		note = "offline: " + err.Error()
	}
	ps, err := s.Problems()
	if err != nil {
		return err
	}
	if len(ps) == 0 {
		return fmt.Errorf("no cached problems and fetch failed: %s", note)
	}
	_, err = tea.NewProgram(tui.New(ps, note)).Run()
	return err
}
