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
	switch args[0] {
	case "list":
		presetsList(out)
		return nil
	case "preset":
		return applyPreset(out, path, args)
	case "export":
		return exportKeys(out, path)
	case "import":
		return importKeys(out, path, in, args)
	default:
		return fmt.Errorf("unknown presets command %q (list, preset, export, import)", args[0])
	}
}

// presetsList prints each preset name with a short description.
func presetsList(out io.Writer) {
	presets := tui.KeyPresets()
	names := make([]string, 0, len(presets))
	for n := range presets {
		names = append(names, n)
	}
	slices.Sort(names)
	descs := map[string]string{
		"default": "use default bindings",
		"vim":     "vim-style: j/k/up/down, ctrl+d/u for paging",
		"arrows":  "arrow keys only for navigation",
		"emacs":   "emacs-style: ctrl+n/p, alt+v/ctrl+v for paging",
	}
	for _, n := range names {
		fmt.Fprintf(out, "%-12s %s\n", n, descs[n])
	}
}

// applyPreset replaces all user bindings with the named preset, then checks the result.
func applyPreset(out io.Writer, path string, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: verd keys presets preset <name>")
	}
	preset, ok := tui.KeyPresets()[args[1]]
	if !ok {
		return fmt.Errorf("unknown preset %q (verd keys presets list)", args[1])
	}
	user, err := loadBindings(path)
	if err != nil {
		return err
	}
	// Remove all current user bindings by resetting each action
	for id := range user {
		if ctx, act, ok := splitBinding(id); ok {
			if err := config.SetKeys(path, ctx, act, nil); err != nil {
				return err
			}
		}
	}
	if err := writeBindings(path, preset); err != nil {
		return err
	}
	// Validate the result before declaring success
	newUser, err := loadBindings(path)
	if err != nil {
		return err
	}
	if err := tui.CheckKeys(newUser); err != nil {
		return err
	}
	fmt.Fprintf(out, "preset %q applied (%d bindings)\n", args[1], len(preset))
	return nil
}

// exportKeys prints the user's [keys.*] blocks, grouped by context in context order.
func exportKeys(out io.Writer, path string) error {
	user, err := loadBindings(path)
	if err != nil {
		return err
	}
	// Group by context and sort for output
	byCtx := map[string]map[string]any{}
	for id, keys := range user {
		ctx, act, ok := splitBinding(id)
		if !ok {
			continue
		}
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
		data := byCtx[name]
		if len(data) == 0 {
			continue
		}
		fmt.Fprintf(out, "[keys.%s]\n", name)
		// Sort keys in this context
		actNames := make([]string, 0, len(data))
		for a := range data {
			actNames = append(actNames, a)
		}
		slices.Sort(actNames)
		for _, a := range actNames {
			b, _ := toml.Marshal(map[string]any{a: data[a]})
			fmt.Fprint(out, string(b))
		}
		fmt.Fprintln(out)
	}
	return nil
}

// importKeys replaces the user's bindings with those in a TOML file (or stdin with "-").
func importKeys(out io.Writer, path string, in io.Reader, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: verd keys presets import <file|->")
	}
	imported, err := parseImport(in, args[1])
	if err != nil {
		return err
	}
	// Import replaces: drop current bindings the file does not mention, then write the file's.
	if err := dropStaleBindings(path, imported); err != nil {
		return err
	}
	if err := writeBindings(path, imported); err != nil {
		return err
	}
	fmt.Fprintf(out, "imported %d bindings\n", len(imported))
	return nil
}

// parseImport reads and validates the [keys.*] blocks of a file (or stdin with "-").
func parseImport(in io.Reader, file string) (map[string][]string, error) {
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(in)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, err
	}
	// Parse the [keys.*] blocks the same way config.toml does.
	var parsed config.Config
	if err := toml.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("invalid TOML: %w", err)
	}
	imported, err := parsed.KeyBindings()
	if err != nil {
		return nil, err
	}
	// Validate before writing anything
	if err := tui.CheckKeys(imported); err != nil {
		return nil, err
	}
	return imported, nil
}

// dropStaleBindings clears current bindings that the imported set does not mention.
func dropStaleBindings(path string, imported map[string][]string) error {
	cur, err := loadConfig(path)
	if err != nil {
		return nil
	}
	old, _ := cur.KeyBindings()
	for id := range old {
		if _, keep := imported[id]; keep {
			continue
		}
		if ctx, act, ok := strings.Cut(id, "."); ok {
			if err := config.SetKeys(path, ctx, act, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeBindings stores each binding in the config file. Ids without exactly one dot are skipped.
func writeBindings(path string, keys map[string][]string) error {
	for id, k := range keys {
		ctx, act, ok := splitBinding(id)
		if !ok {
			continue
		}
		if err := config.SetKeys(path, ctx, act, k); err != nil {
			return err
		}
	}
	return nil
}

// loadBindings loads the config file and returns its user key bindings.
func loadBindings(path string) (map[string][]string, error) {
	cfg, err := loadConfig(path)
	if err != nil {
		return nil, err
	}
	return cfg.KeyBindings()
}

// splitBinding splits a "context.action" id; ok is false unless there is exactly one dot.
func splitBinding(id string) (ctx, act string, ok bool) {
	parts := strings.Split(id, ".")
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}
