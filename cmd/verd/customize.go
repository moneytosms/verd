package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/theme"
	"github.com/moneytosms/verd/internal/tui"
)

// configSub handles `verd config path|keys|get|set`; no arguments prints the effective config.
func configSub(out io.Writer, path string, args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(out, path)
	case "keys":
		for _, k := range config.SettableKeys() {
			fmt.Fprintln(out, k)
		}
	case "get":
		if len(args) != 2 {
			return true, fmt.Errorf("usage: verd config get <key>")
		}
		cfg, err := loadConfig(path)
		if err != nil {
			return true, err
		}
		v, ok := cfg.Get(args[1])
		if !ok {
			return true, fmt.Errorf("unknown key %q (verd config keys)", args[1])
		}
		fmt.Fprintln(out, v)
	case "set":
		if len(args) != 3 {
			return true, fmt.Errorf("usage: verd config set <key> <value>")
		}
		if err := config.Set(path, args[1], args[2]); err != nil {
			return true, err
		}
		if _, err := loadConfig(path); err != nil { // never leave a config that does not load
			return true, err
		}
		fmt.Fprintf(out, "%s = %s\n", args[1], args[2])
	default:
		return true, fmt.Errorf("unknown config command %q (path, keys, get, set)", args[0])
	}
	return true, nil
}

// keysCmd handles `verd keys [list|set|reset|check|presets|export|import|markdown] [--json] [--write <file>]`.
func keysCmd(out io.Writer, path string, args []string) error {
	asJSON := slices.Contains(args, "--json")
	writeFile := ""
	skipNext := false
	for idx, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--write" && idx+1 < len(args) {
			writeFile = args[idx+1]
			skipNext = true
		}
	}
	// Remove --json, --write, and --write targets from args
	var cleaned []string
	skipNext = false
	for _, a := range args {
		if skipNext {
			skipNext = false
			continue
		}
		if a == "--json" || a == "--write" {
			if a == "--write" {
				skipNext = true
			}
			continue
		}
		cleaned = append(cleaned, a)
	}
	args = cleaned
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	// Handle preset commands (may need stdin)
	switch sub {
	case "presets": // verd keys presets [list|preset <name>|export|import]
		return presetsCmd(out, path, os.Stdin, args[1:])
	case "preset", "export", "import": // shorthand: verd keys preset vim
		return presetsCmd(out, path, os.Stdin, args)
	}
	// Handle markdown
	if sub == "markdown" {
		md := tui.KeyTableMarkdown()
		if writeFile != "" {
			if err := updateKeysMarkdown(writeFile, md); err != nil {
				return err
			}
			fmt.Fprintf(out, "Updated %s\n", writeFile)
		} else {
			fmt.Fprint(out, md)
		}
		return nil
	}
	// Rest of keys commands
	cfg, err := loadConfig(path)
	if err != nil {
		return err
	}
	user, err := cfg.KeyBindings()
	if err != nil {
		return err
	}
	switch sub {
	case "list":
		infos := tui.KeyActions(user)
		if asJSON {
			type row struct {
				Context  string   `json:"context"`
				Action   string   `json:"action"`
				Desc     string   `json:"description"`
				Default  []string `json:"default"`
				Current  []string `json:"keys"`
				Modified bool     `json:"modified"`
			}
			rows := make([]row, len(infos))
			for i, k := range infos {
				rows[i] = row{k.Context, k.Action, k.Desc, k.Default, k.Current, !slices.Equal(k.Default, k.Current)}
			}
			b, _ := json.MarshalIndent(rows, "", "  ")
			fmt.Fprintln(out, string(b))
			return nil
		}
		for _, k := range infos {
			mark := " "
			if !slices.Equal(k.Default, k.Current) {
				mark = "*"
			}
			fmt.Fprintf(out, "%s %-28s %-24s %s\n", mark, k.Context+"."+k.Action, strings.Join(k.Current, " "), k.Desc)
		}
		fmt.Fprintln(out, "\n* = changed. Set: verd keys set <context.action> <key>...   Reset: verd keys reset <context.action>")
	case "contexts":
		for _, c := range tui.KeyContexts() {
			fmt.Fprintf(out, "%-11s %s\n", c[0], c[1])
		}
	case "check":
		fmt.Fprintf(out, "ok: %d custom shortcuts, no conflicts\n", len(user))
	case "set", "reset":
		need := 3
		if sub == "reset" {
			need = 2
		}
		if len(args) < need || (sub == "reset" && len(args) != 2) {
			return fmt.Errorf("usage: verd keys set <context.action> <key>...  |  verd keys reset <context.action>")
		}
		ctx, action, ok := strings.Cut(args[1], ".")
		if !ok {
			return fmt.Errorf("%q: want context.action (verd keys)", args[1])
		}
		keys := args[2:]
		next := map[string][]string{}
		for k, v := range user {
			next[k] = v
		}
		if sub == "reset" {
			delete(next, args[1])
			keys = nil
		} else {
			next[args[1]] = keys
		}
		if err := tui.CheckKeys(next); err != nil { // unknown action, bad key or conflict: nothing is written
			return err
		}
		if err := config.SetKeys(path, ctx, action, keys); err != nil {
			return err
		}
		if sub == "reset" {
			fmt.Fprintln(out, "reset", args[1])
		} else {
			fmt.Fprintf(out, "%s = %s\n", args[1], strings.Join(keys, ", "))
		}
	default:
		return fmt.Errorf("unknown keys command %q (list, contexts, check, set, reset, presets, export, import, markdown)", sub)
	}
	return nil
}

// themesCmd handles `verd themes [list|show <name>]`. show prints a ready-to-edit [themes.<name>] block.
func themesCmd(out io.Writer, args []string) error {
	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "list":
		for _, n := range theme.Names() {
			fmt.Fprintln(out, n)
		}
	case "show":
		if len(args) != 2 {
			return fmt.Errorf("usage: verd themes show <name>")
		}
		t, ok := theme.Get(args[1])
		if !ok {
			return fmt.Errorf("unknown theme %q (verd themes)", args[1])
		}
		fmt.Fprintf(out, "[themes.my-%s]\nbase = %q\nglamour_dark = %q\nglamour_light = %q\n", t.Name, t.Name, t.GlamourDark, t.GlamourLight)
		for _, side := range []struct {
			name string
			p    theme.Palette
		}{{"dark", t.Dark}, {"light", t.Light}} {
			fmt.Fprintf(out, "[themes.my-%s.%s]\n", t.Name, side.name)
			d := side.p.Describe()
			for _, k := range theme.PaletteKeys {
				fmt.Fprintf(out, "%s = %q\n", k, d[k])
			}
		}
	default:
		return fmt.Errorf("unknown themes command %q (list, show)", sub)
	}
	return nil
}

// updateKeysMarkdown replaces the section between <!-- keys:begin --> and <!-- keys:end --> in a file.
func updateKeysMarkdown(path, content string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(b)
	const begin, end = "<!-- keys:begin -->", "<!-- keys:end -->"
	beginIdx := strings.Index(text, begin)
	endIdx := strings.Index(text, end)
	if beginIdx == -1 || endIdx == -1 || beginIdx >= endIdx {
		return fmt.Errorf("markers not found in %s (need <!-- keys:begin --> and <!-- keys:end -->)", path)
	}
	newText := text[:beginIdx+len(begin)] + "\n" + content + "\n" + text[endIdx:]
	return os.WriteFile(path, []byte(newText), 0o644)
}
