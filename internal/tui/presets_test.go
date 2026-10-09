package tui

import (
	"testing"
)

func TestPresetsValid(t *testing.T) {
	presets := KeyPresets()
	for name, binding := range presets {
		if err := CheckKeys(binding); err != nil {
			t.Errorf("preset %q: %v", name, err)
		}
		// Verify all keys parse correctly
		for id, keys := range binding {
			for _, key := range keys {
				if _, err := parseKey(key); err != nil {
					t.Errorf("preset %q, %s: invalid key %q: %v", name, id, key, err)
				}
			}
		}
	}
}
