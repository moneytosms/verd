package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/creds"
	"github.com/moneytosms/verd/internal/theme"
	"golang.org/x/term"
)

// setupCmd walks through the settings a new install needs and writes them into config.toml
// (comments and other lines stay). Enter keeps the value in brackets. It is safe to re-run.
func setupCmd(out io.Writer, in *os.File, path string, store creds.Store) error {
	if _, err := os.Stat(path); err != nil {
		if err := config.Init(path, false); err != nil {
			return err
		}
		fmt.Fprintln(out, "wrote", path)
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	r := bufio.NewReader(in)
	langs := make([]string, 0, len(cfg.Lang))
	for k := range cfg.Lang {
		langs = append(langs, k)
	}
	slices.Sort(langs)
	fmt.Fprintln(out, "verd setup: enter keeps the value in brackets.")
	for _, q := range []struct {
		key, label, def string
		opts            []string
	}{
		{"handle", "Codeforces handle", cfg.Handle, nil},
		{"workspace", "Workspace (where Solutions live)", cfg.Workspace, nil},
		{"default_lang", "Default language", cfg.DefaultLang, langs},
		{"theme", "Theme", cfg.Theme, theme.Names()},
		{"editor", "Editor command (nvim, vim, hx, nano, code...)", cfg.Editor, nil},
		{"source_cses", "Also show CSES problems", fmt.Sprint(cfg.SourceCSES), []string{"true", "false"}},
		{"split", "Editor split", cfg.Split, config.Options("split")},
		{"submit_mode", "Submit mode (direct is experimental)", cfg.SubmitMode, config.Options("submit_mode")},
	} {
		if err := askSetting(out, r, path, q.key, q.label, q.def, q.opts); err != nil {
			return err
		}
	}
	if m, _ := config.Load(path); m.SubmitMode == "direct" {
		checkDirectSession(out, in, store)
	}
	fmt.Fprintf(out, "saved to %s. Run `verd` to start.\n", path)
	return nil
}

// askSetting prompts until the answer is valid, then writes it to config.toml. It gives up
// at end of input.
func askSetting(out io.Writer, r *bufio.Reader, path, key, label, def string, opts []string) error {
	for {
		fmt.Fprintf(out, "%s [%s]: ", label, settingHint(def, opts))
		line, rerr := r.ReadString('\n')
		v := strings.TrimSpace(line)
		if v == "" {
			v = def
		}
		if v == "" {
			if rerr != nil {
				return fmt.Errorf("%s: no input", label)
			}
			fmt.Fprintln(out, "  required")
			continue
		}
		if len(opts) > 0 && !slices.Contains(opts, v) {
			fmt.Fprintf(out, "  %q is not one of %s\n", v, strings.Join(opts, ", "))
			if rerr != nil {
				return fmt.Errorf("%s: %q is not one of %s", label, v, strings.Join(opts, ", "))
			}
			continue
		}
		if v != def || key == "handle" {
			if err := config.Set(path, key, v); err != nil {
				fmt.Fprintln(out, " ", err)
				if rerr != nil {
					return err
				}
				continue
			}
		}
		return nil
	}
}

// settingHint is the text in brackets: the allowed options, then the current value.
func settingHint(def string, opts []string) string {
	if len(opts) > 0 {
		return strings.Join(opts, "/") + ", now " + def
	}
	return def
}

// checkDirectSession tells the user whether direct submit has a saved browser session, and
// offers to log in when it does not and a terminal is attached.
func checkDirectSession(out io.Writer, in *os.File, store creds.Store) {
	if _, err := store.Load(); err == nil {
		fmt.Fprintln(out, "direct submit: a session is already saved (verd login replaces it)")
	} else if term.IsTerminal(int(in.Fd())) {
		fmt.Fprintln(out, "direct submit needs your browser session, see docs/submit.md.")
		if err := loginCmd(out, in, store); err != nil {
			fmt.Fprintln(out, " ", err, "(run verd login later)")
		}
	} else {
		fmt.Fprintln(out, "direct submit: run `verd login` to save your browser session")
	}
}
