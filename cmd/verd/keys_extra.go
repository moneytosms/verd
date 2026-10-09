package main

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/tui"
)

// presetsCmd handles `verd keys presets [list|preset <name>|export|import <file>]`.
func presetsCmd(out io.Writer, path string, in io.Reader, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: verd keys presets [list | preset <name> | export | import <file|->]")
	}
	sub := args[0]
	switch sub {
	case "list":
		presets := tui.KeyPresets()
		names := make([]string, 0, len(presets))
		for n := range presets {
			names = append(names, n)
		}
		slices.Sort(names)
		for _, n := range names {
			desc := map[string]string{
				"default": "use default bindings",
				"vim":     "vim-style: j/k/up/down, ctrl+d/u for paging",
				"arrows":  "arrow keys only for navigation",
				"emacs":   "emacs-style: ctrl+n/p, alt+v/ctrl+v for paging",
			}[n]
			fmt.Fprintf(out, "%-12s %s\n", n, desc)
		}
	case "preset":
		if len(args) != 2 {
			return fmt.Errorf("usage: verd keys presets preset <name>")
		}
		presets := tui.KeyPresets()
		preset, ok := presets[args[1]]
		if !ok {
			return fmt.Errorf("unknown preset %q (verd keys presets list)", args[1])
		}
		cfg, err := loadConfig(path)
		if err != nil {
			return err
		}
		user, err := cfg.KeyBindings()
		if err != nil {
			return err
		}
		// Remove all current user bindings by resetting each action
		for id := range user {
			parts := strings.Split(id, ".")
			if len(parts) != 2 {
				continue
			}
			if err := config.SetKeys(path, parts[0], parts[1], nil); err != nil {
				return err
			}
		}
		// Write the preset
		for id, keys := range preset {
			parts := strings.Split(id, ".")
			if len(parts) != 2 {
				continue
			}
			if err := config.SetKeys(path, parts[0], parts[1], keys); err != nil {
				return err
			}
		}
		// Validate the result before declaring success
		cfg, err = loadConfig(path)
		if err != nil {
			return err
		}
		newUser, err := cfg.KeyBindings()
		if err != nil {
			return err
		}
		if err := tui.CheckKeys(newUser); err != nil {
			return err
		}
		fmt.Fprintf(out, "preset %q applied (%d bindings)\n", args[1], len(preset))
	case "export":
		cfg, err := loadConfig(path)
		if err != nil {
			return err
		}
		user, err := cfg.KeyBindings()
		if err != nil {
			return err
		}
		// Group by context and sort for output
		byCtx := map[string]map[string]any{}
		for id, keys := range user {
			parts := strings.Split(id, ".")
			if len(parts) != 2 {
				continue
			}
			ctx, act := parts[0], parts[1]
			if byCtx[ctx] == nil {
				byCtx[ctx] = map[string]any{}
			}
			if len(keys) == 1 {
				byCtx[ctx][act] = keys[0]
			} else {
				byCtx[ctx][act] = keys
			}
		}
		// Output in sorted context order
		for _, ctx := range tui.KeyContexts() {
			name := ctx[0]
			if data, ok := byCtx[name]; ok && len(data) > 0 {
				fmt.Fprintf(out, "[keys.%s]\n", name)
				// Sort keys in this context
				actNames := make([]string, 0, len(data))
				for a := range data {
					actNames = append(actNames, a)
				}
				slices.Sort(actNames)
				for _, a := range actNames {
					v := data[a]
					b, _ := toml.Marshal(map[string]any{a: v})
					fmt.Fprint(out, string(b))
				}
				fmt.Fprintln(out)
			}
		}
	case "import":
		if len(args) != 2 {
			return fmt.Errorf("usage: verd keys presets import <file|->")
		}
		var data []byte
		var err error
		if args[1] == "-" {
			data, err = io.ReadAll(in)
		} else {
			data, err = os.ReadFile(args[1])
		}
		if err != nil {
			return err
		}
		// Parse the [keys.*] blocks the same way config.toml does.
		var parsed config.Config
		if err := toml.Unmarshal(data, &parsed); err != nil {
			return fmt.Errorf("invalid TOML: %w", err)
		}
		imported, err := parsed.KeyBindings()
		if err != nil {
			return err
		}
		// Validate before writing anything
		if err := tui.CheckKeys(imported); err != nil {
			return err
		}
		// Import replaces: drop current bindings the file does not mention, then write the file's.
		if cur, err := loadConfig(path); err == nil {
			old, _ := cur.KeyBindings()
			for id := range old {
				if _, keep := imported[id]; !keep {
					if ctx, act, ok := strings.Cut(id, "."); ok {
						if err := config.SetKeys(path, ctx, act, nil); err != nil {
							return err
						}
					}
				}
			}
		}
		for id, keys := range imported {
			parts := strings.Split(id, ".")
			if len(parts) != 2 {
				continue
			}
			if err := config.SetKeys(path, parts[0], parts[1], keys); err != nil {
				return err
			}
		}
		fmt.Fprintf(out, "imported %d bindings\n", len(imported))
	default:
		return fmt.Errorf("unknown presets command %q (list, preset, export, import)", sub)
	}
	return nil
}
