package img

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
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

func cache(t *testing.T, dir, addr string, m image.Image) {
	t.Helper()
	var buf bytes.Buffer
	png.Encode(&buf, m)
	sum := sha256.Sum256([]byte(addr))
	os.WriteFile(filepath.Join(dir, hex.EncodeToString(sum[:])), buf.Bytes(), 0o644)
}

func TestFetchReadsTheCache(t *testing.T) {
	dir, addr := t.TempDir(), "https://cses.fi/file/abc"
	cache(t, dir, addr, solid(3, 3, color.Black))
	m, err := Fetch(addr, dir) // served from disk: no network in this test
	if err != nil || m.Bounds().Dx() != 3 {
		t.Fatalf("fetch: %v", err)
	}
}

func TestHTTPSOnly(t *testing.T) {
	for addr, ok := range map[string]bool{
		"https://cses.fi/file/x":    true,
		"https://i.imgur.com/a.png": true,
		"http://cses.fi/file/x":     false,
		"file:///etc/passwd":        false,
		"https:///nohost":           false,
	} {
		if err := allowed(addr); (err == nil) != ok {
			t.Errorf("%s: %v", addr, err)
		}
	}
}

func TestPrivateAddressesAreRefused(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:443": false, "10.0.0.5:443": false, "192.168.1.1:443": false, "169.254.169.254:80": false,
		"[::1]:443": false, "0.0.0.0:443": false, "100.64.0.1:443": false, "198.18.0.1:443": false, "240.0.0.1:443": false,
		"[::ffff:10.0.0.1]:443": false, "[64:ff9b::a00:1]:443": false, "[2002:a00:1::]:443": false, "[fd00::1]:443": false, "[fec0::1]:443": false, "8.8.8.8:443": true, "[2606:4700::1111]:443": true,
	} {
		if err := publicOnly("tcp", addr, nil); (err == nil) != ok {
			t.Errorf("%s: %v", addr, err)
		}
	}
}

func TestHugePicturesAreRefused(t *testing.T) {
	dir, addr := t.TempDir(), "https://cses.fi/file/big"
	cache(t, dir, addr, image.NewGray(image.Rect(0, 0, 5000, 5000))) // 25 MP: compresses to almost nothing
	if _, err := Fetch(addr, dir); err == nil || !strings.Contains(err.Error(), "too large") {
		t.Fatalf("want too large, got %v", err)
	}
}
