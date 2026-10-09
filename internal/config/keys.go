package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// KeyBindings flattens [keys.<context>] into "context.action" -> keys. A value is one key
// ("x") or a list (["x", "ctrl+x"]).
func (c Config) KeyBindings() (map[string][]string, error) {
	out := map[string][]string{}
	for ctx, acts := range c.Keys {
		for act, v := range acts {
			id := ctx + "." + act
			switch v := v.(type) {
			case string:
				out[id] = []string{v}
			case []any:
				for _, k := range v {
					s, ok := k.(string)
					if !ok {
						return nil, fmt.Errorf("keys.%s: %v is not a string", id, k)
					}
					out[id] = append(out[id], s)
				}
			default:
				return nil, fmt.Errorf("keys.%s: want a key or a list of keys", id)
			}
			if len(out[id]) == 0 {
				delete(out, id)
			}
		}
	}
	return out, nil
}

// SetKeys writes `action = keys` under [keys.ctx] in the config file, keeping everything else.
// Empty keys removes the line (back to the default).
func SetKeys(path, ctx, action string, keys []string) error {
	b, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(b) == 0 {
		lines = nil
	}
	var line string
	if len(keys) > 0 {
		var v any = keys
		if len(keys) == 1 {
			v = keys[0]
		}
		enc, err := toml.Marshal(map[string]any{action: v})
		if err != nil {
			return err
		}
		line = strings.TrimRight(string(enc), "\n")
	}
	header := "[keys." + ctx + "]"
	live := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(action) + `\s*=`)
	at, end := -1, len(lines) // table header line and the line after its last entry
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if at < 0 {
			if t == header {
				at = i
			}
			continue
		}
		if strings.HasPrefix(t, "[") {
			end = i
			break
		}
	}
	switch {
	case at < 0 && line == "":
		return nil
	case at < 0:
		lines = append(lines, "", header, line)
	default:
		done := false
		for i := at + 1; i < end; i++ {
			if live.MatchString(lines[i]) {
				if line == "" {
					lines = append(lines[:i], lines[i+1:]...)
				} else {
					lines[i] = line
				}
				done = true
				break
			}
		}
		if !done && line != "" {
			lines = append(lines[:at+1], append([]string{line}, lines[at+1:]...)...)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	mode := fs.FileMode(0o644)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode()
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), mode)
}
