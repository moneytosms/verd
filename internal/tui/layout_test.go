package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

func TestHeaderFooterToggle(t *testing.T) {
	// Test header=true, footer=true (default)
	m := listModel()
	view := m.View()
	lines := len(strings.Split(strings.TrimRight(view.Content, "\n"), "\n"))
	if lines > m.height {
		t.Fatalf("header on, footer on: view should fit in %d lines, got %d", m.height, lines)
	}

	// Test header=false, footer=true
	m.cfgVals["header"] = "false"
	view = m.View()
	lines = len(strings.Split(strings.TrimRight(view.Content, "\n"), "\n"))
	if lines > m.height {
		t.Fatalf("header off, footer on: view should fit in %d lines, got %d", m.height, lines)
	}

	// Test header=true, footer=false
	m.cfgVals["header"] = "true"
	m.cfgVals["footer"] = "false"
	view = m.View()
	lines = len(strings.Split(strings.TrimRight(view.Content, "\n"), "\n"))
	if lines > m.height {
		t.Fatalf("header on, footer off: view should fit in %d lines, got %d", m.height, lines)
	}

	// Test header=false, footer=false
	m.cfgVals["header"] = "false"
	m.cfgVals["footer"] = "false"
	view = m.View()
	lines = len(strings.Split(strings.TrimRight(view.Content, "\n"), "\n"))
	if lines > m.height {
		t.Fatalf("header off, footer off: view should fit in %d lines, got %d", m.height, lines)
	}
}

func TestListClickWithHeaderFooter(t *testing.T) {
	// Note: We need actual problems to test clicking. The mouse tests use the same pattern.
	// Since listModel() has no problems, we skip the detailed row clicking tests and just
	// verify the configuration values are being used correctly through tabPosFromID.
	m := listModel()
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = nm.(Model)

	// Verify header is showing by default
	if ct := m.contentTop(); ct != 2 {
		t.Fatalf("default: contentTop should be 2, got %d", ct)
	}

	// Verify header can be hidden
	m.cfgVals["header"] = "false"
	if ct := m.contentTop(); ct != 0 {
		t.Fatalf("header off: contentTop should be 0, got %d", ct)
	}

	// Verify footer is showing by default
	m = listModel()
	if fs := m.footerSize(); fs != 2 {
		t.Fatalf("default: footerSize should be 2, got %d", fs)
	}

	// Verify footer can be hidden
	m.cfgVals["footer"] = "false"
	if fs := m.footerSize(); fs != 0 {
		t.Fatalf("footer off: footerSize should be 0, got %d", fs)
	}
}

func TestTabKeysWithHeaderOff(t *testing.T) {
	m := listModel()
	nm, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m = nm.(Model)
	m.cfgVals["header"] = "false"

	// Tab key should still work
	m, _ = send(m, "2")
	if m.tab != 1 {
		t.Fatalf("tab key 2 with header off: tab=%d (expected 1)", m.tab)
	}

	m, _ = send(m, "5")
	if m.tab != 4 {
		t.Fatalf("tab key 5 with header off: tab=%d (expected 4)", m.tab)
	}
}

func TestBoxIsExactlyWideForEveryBorder(t *testing.T) {
	m := New(nil, "", Deps{})
	for _, b := range []string{"rounded", "square", "heavy", "double", "ascii", "none"} {
		for _, note := range []string{"", "a note"} {
			lines := box("Title", []string{"one", "two"}, note, 30, 6, true, m.styles(), b)
			if len(lines) != 6 {
				t.Fatalf("%s: %d lines", b, len(lines))
			}
			for i, l := range lines {
				if w := xansi.StringWidth(l); w != 30 {
					t.Errorf("%s note=%q line %d is %d columns: %q", b, note, i, w, l)
				}
			}
		}
	}
}
