package tui

import (
	"strings"
	"testing"
	"time"
)

func TestHeatmap(t *testing.T) {
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC) // a Wednesday
	rows := Heatmap(map[string]int{"2026-10-07": 5, "2026-10-05": 1, "2026-09-30": 2}, now, 2)
	if len(rows) != 7 {
		t.Fatalf("rows = %d", len(rows))
	}
	// 2 weeks: Mon 09-28 .. Sun 10-11. Wed 09-30 = row 2 col 0 (▓); Mon 10-05 = row 0 col 1 (▒);
	// today Wed 10-07 = row 2 col 1 (█); Thu onward is blank.
	for _, c := range []struct {
		row  int
		want string
	}{{0, "· ▒ "}, {2, "▓ █ "}, {3, "·   "}} {
		if rows[c.row] != c.want {
			t.Errorf("row %d = %q, want %q", c.row, rows[c.row], c.want)
		}
	}
	if Heatmap(nil, now, 0) != nil || strings.Contains(strings.Join(Heatmap(nil, now, 3), ""), "█") {
		t.Fatal("empty input")
	}
}
