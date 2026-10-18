package img

import (
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func solid(w, h int, c color.Color) image.Image {
	m := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			m.Set(x, y, c)
		}
	}
	return m
}

func TestResolve(t *testing.T) {
	env := func(kv ...string) func(string) string {
		return func(k string) string {
			for i := 0; i < len(kv); i += 2 {
				if kv[i] == k {
					return kv[i+1]
				}
			}
			return ""
		}
	}
	for name, tc := range map[string]struct {
		setting string
		env     func(string) string
		want    Mode
	}{
		"kitty":         {"auto", env("TERM", "xterm-kitty"), Kitty},
		"ghostty":       {"auto", env("TERM_PROGRAM", "ghostty"), Kitty},
		"alacritty":     {"auto", env("TERM", "alacritty"), Blocks},
		"tmux in kitty": {"auto", env("TERM", "xterm-kitty", "TMUX", "/tmp/x"), Blocks},
		"plain":         {"auto", env(), Blocks},
		"forced blocks": {"blocks", env("TERM", "xterm-kitty"), Blocks},
		"forced kitty":  {"kitty", env(), Kitty},
		"off":           {"off", env("TERM", "xterm-kitty"), Off},
	} {
		if got := Resolve(tc.setting, tc.env); got != tc.want {
			t.Errorf("%s: %v want %v", name, got, tc.want)
		}
	}
}

func TestFitKeepsShapeAndLimits(t *testing.T) {
	if c, r := Fit(400, 200, 100, 30); c != 50 || r != 13 { // 8 px per column, cells twice as tall
		t.Fatalf("%d x %d", c, r)
	}
	if c, r := Fit(2000, 2000, 60, 10); r != 10 || c != 20 {
		t.Fatalf("tall image should be limited by rows: %d x %d", c, r)
	}
	if c, r := Fit(2000, 100, 60, 10); c != 60 || r != 2 {
		t.Fatalf("wide image limited by columns: %d x %d", c, r)
	}
}

func TestBlockLinesColoursTopAndBottom(t *testing.T) {
	m := image.NewRGBA(image.Rect(0, 0, 1, 2))
	m.Set(0, 0, color.RGBA{255, 0, 0, 255})
	m.Set(0, 1, color.RGBA{0, 0, 255, 255})
	got := BlockLines(m, 1, 1)
	if len(got) != 1 || !strings.Contains(got[0], "38;2;255;0;0;48;2;0;0;255m▀") {
		t.Fatalf("%q", got)
	}
}

func TestTransparencyIsFlattenedOnWhite(t *testing.T) {
	px := Scale(solid(2, 2, color.NRGBA{0, 0, 0, 0}), 1, 1)
	if c := px.RGBAAt(0, 0); c.R != 255 || c.G != 255 || c.B != 255 {
		t.Fatalf("transparent should become white: %v", c)
	}
}

func TestKittySequences(t *testing.T) {
	id := ID("https://x/y.png")
	if id == 0 || id > 0xFFFFFF {
		t.Fatalf("id %d", id)
	}
	tx := Transmit(id, solid(10, 10, color.Black))
	if !strings.HasPrefix(tx, "\x1b_Ga=t,f=100,t=d,i=") || !strings.Contains(tx, "q=2") || !strings.HasSuffix(tx, "\x1b\\") {
		t.Fatalf("transmit: %.80q", tx)
	}
	if p := Place(id, 7, 3); !strings.Contains(p, "U=1") || !strings.Contains(p, "c=7,r=3") {
		t.Fatalf("place: %q", p)
	}
	ph := Placeholders(id, 4, 2)
	if len(ph) != 2 || strings.Count(ph[0], string(rune(0x10EEEE))) != 4 || !strings.Contains(ph[0], "38;2;") {
		t.Fatalf("placeholders: %q", ph)
	}
}

func TestFetchCachesAndRejectsBadAddresses(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		png.Encode(w, solid(3, 3, color.Black))
	}))
	defer srv.Close()
	dir := t.TempDir()
	for i := 0; i < 2; i++ {
		m, err := Fetch(srv.URL+"/a.png", dir)
		if err != nil || m.Bounds().Dx() != 3 {
			t.Fatalf("fetch: %v", err)
		}
	}
	if hits != 1 {
		t.Fatalf("second fetch should come from the cache: %d hits", hits)
	}
	if _, err := Fetch("file:///etc/passwd", dir); err == nil {
		t.Fatal("only http(s) addresses are fetched")
	}
}
