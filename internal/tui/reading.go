package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/scrape"
)

// readingPalette is the reading palette in effect now.
func (m Model) readingPalette() (scrape.Palette, bool) {
	return scrape.PaletteNamed(m.resolvedPalette())
}

// resolvedPalette is the name of the reading palette in effect now. auto is night from
// reading_night_from to 07:00, and app when that is not set.
func (m Model) resolvedPalette() string {
	name := m.setting("reading_palette")
	if name == "auto" {
		name = "app"
		if nightNow(m.setting("reading_night_from")) {
			name = "night"
		}
	}
	return name
}

// nightNow reports whether the local hour is in the night window that starts at from (default 20).
func nightNow(from string) bool {
	start := 20
	if v, err := strconv.Atoi(from); err == nil {
		start = v
	}
	h := time.Now().Hour()
	if start <= 7 {
		return h >= start && h < 7
	}
	return h >= start || h < 7
}

// cyclePalette steps reading_palette to the next palette and re-renders the statement.
func (m Model) cyclePalette() Model {
	cur := m.setting("reading_palette")
	if cur == "" {
		cur = "app"
	}
	return m.apply(settingDef{key: "reading_palette", kind: "enum", live: true}, nextOf(config.ReadingPalettes, cur))
}

// paintStmt gives statement lines the palette's background, when it has one.
func (m Model) paintStmt(lines []string) []string {
	p, ok := m.readingPalette()
	if !ok || p.Bg == "" {
		return lines
	}
	sgr, w := bgSGR(p.Bg), m.stmtWidth()+2
	out := make([]string, len(lines))
	for i, l := range lines {
		out[i] = paintLine(l, sgr, w)
	}
	return out
}

// paintLine keeps the background behind a line: each SGR reset re-opens it, and the line is padded
// to w columns so the colour fills the pane.
func paintLine(line, sgr string, w int) string {
	line = strings.ReplaceAll(line, "\x1b[0m", "\x1b[0m"+sgr)
	line = strings.ReplaceAll(line, "\x1b[m", "\x1b[m"+sgr)
	pad := max(0, w-xansi.StringWidth(line))
	return sgr + line + strings.Repeat(" ", pad) + "\x1b[0m"
}

// bgSGR is the escape that sets a #rrggbb background.
func bgSGR(hex string) string {
	n, _ := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	return fmt.Sprintf("\x1b[48;2;%d;%d;%dm", n>>16, n>>8&0xff, n&0xff)
}
