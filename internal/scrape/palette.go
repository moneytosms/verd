package scrape

import "charm.land/glamour/v2/ansi"

// Palette is a reading colour scheme: text, page background and accent, as hex.
type Palette struct {
	Fg, Bg, Accent string
	Underline      bool // headings underlined (mono)
}

// palettes are the named reading palettes. app (the theme's own colours) and auto have none here.
var palettes = map[string]Palette{
	"paper":    {"#3a3226", "#f7f1e3", "#8a5a2b", false},
	"sepia":    {"#4b3a24", "#efe2c6", "#7a4a1e", false},
	"cool":     {"#263238", "#eceff1", "#1e5a78", false},
	"night":    {"#d8cfc0", "#1c1a17", "#d9a86c", false},
	"mono":     {"#111111", "#ffffff", "#111111", true},
	"contrast": {"#ffffff", "#000000", "#ffe066", false},
}

// PaletteNamed returns the named palette; false for app, auto and unknown names.
func PaletteNamed(name string) (Palette, bool) {
	p, ok := palettes[name]
	return p, ok
}

// applyPalette recolours a glamour style: text, headings, emphasis, links and code.
func applyPalette(cfg *ansi.StyleConfig, p Palette) {
	fg, bg, ac := p.Fg, p.Bg, p.Accent
	cfg.Document.Color, cfg.Document.BackgroundColor = &fg, &bg
	cfg.Text.Color = &fg
	for _, h := range []*ansi.StyleBlock{&cfg.Heading, &cfg.H1, &cfg.H2, &cfg.H3, &cfg.H4, &cfg.H5, &cfg.H6} {
		h.Color, h.BackgroundColor = &ac, nil
		if p.Underline {
			u := true
			h.Underline = &u
		}
	}
	cfg.Emph.Color, cfg.Strong.Color = &fg, &fg
	cfg.Link.Color, cfg.LinkText.Color = &ac, &ac
	cfg.Code.Color, cfg.Code.BackgroundColor = &ac, nil
}
