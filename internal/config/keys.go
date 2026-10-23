package config

import (
	"fmt"
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
	b, err := readConfig(path)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(b) == 0 {
		lines = nil
	}
	line, err := keyLine(action, keys)
	if err != nil {
		return err
	}
	header := "[keys." + ctx + "]"
	at, end := findTable(lines, header)
	switch {
	case at < 0 && line == "":
		return nil
	case at < 0:
		lines = append(lines, "", header, line)
	default:
		lines = setEntry(lines, at, end, action, line)
	}
	return saveFile(path, strings.Join(lines, "\n")+"\n")
}

// keyLine renders `action = keys` as one TOML line; a single key is written as a string.
// No keys gives "", which means remove.
func keyLine(action string, keys []string) (string, error) {
	if len(keys) == 0 {
		return "", nil
	}
	var v any = keys
	if len(keys) == 1 {
		v = keys[0]
	}
	enc, err := toml.Marshal(map[string]any{action: v})
	if err != nil {
		return "", err
	}
	return strings.TrimRight(string(enc), "\n"), nil
}

// findTable returns the index of the header line and of the line after its last entry.
// at is -1 when the table is absent.
func findTable(lines []string, header string) (at, end int) {
	at, end = -1, len(lines)
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if at < 0 {
			if t == header {
				at = i
			}
			continue
		}
		if strings.HasPrefix(t, "[") {
			return at, i
		}
	}
	return at, end
}

// setEntry replaces the action's line inside the table at lines[at:end], or removes it when line
// is empty. A missing entry gets line right under the header.
func setEntry(lines []string, at, end int, action, line string) []string {
	live := regexp.MustCompile(`^\s*` + regexp.QuoteMeta(action) + `\s*=`)
	for i := at + 1; i < end; i++ {
		if live.MatchString(lines[i]) {
			if line == "" {
				return append(lines[:i], lines[i+1:]...)
			}
			lines[i] = line
			return lines
		}
	}
	if line == "" {
		return lines
	}
	return append(lines[:at+1], append([]string{line}, lines[at+1:]...)...)
}
