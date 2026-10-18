// Package img fetches statement images and draws them in the terminal: as Kitty graphics
// (Unicode placeholders, so they scroll and clip like text) where the terminal supports that, and
// as half-block characters everywhere else.
package img

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi/kitty"
)

// Mode is how images are drawn.
type Mode int

const (
	Off    Mode = iota // links only
	Blocks             // half-block characters: any terminal with colors
	Kitty              // Kitty graphics protocol with Unicode placeholders
)

// Resolve turns the images setting (auto, kitty, blocks, off) into a Mode. auto picks Kitty for
// kitty and Ghostty (the terminals that implement Unicode placeholders), and blocks elsewhere,
// including inside tmux and zellij, which would need graphics passthrough.
func Resolve(setting string, getenv func(string) string) Mode {
	switch setting {
	case "off":
		return Off
	case "blocks":
		return Blocks
	case "kitty":
		return Kitty
	}
	if getenv("TMUX") != "" || getenv("ZELLIJ") != "" || getenv("STY") != "" {
		return Blocks
	}
	term, prog := getenv("TERM"), getenv("TERM_PROGRAM")
	if term == "xterm-kitty" || term == "xterm-ghostty" || getenv("KITTY_WINDOW_ID") != "" ||
		strings.EqualFold(prog, "ghostty") || getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return Kitty
	}
	return Blocks
}

// maxBytes bounds a download; statement images are small.
const maxBytes = 10 << 20

// Fetch downloads an image (cached on disk under dir) and decodes it.
func Fetch(addr, dir string) (image.Image, error) {
	if err := allowed(addr); err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(addr))
	file := filepath.Join(dir, hex.EncodeToString(sum[:]))
	data, err := os.ReadFile(file)
	if err != nil {
		req, _ := http.NewRequest("GET", addr, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "image/png,image/jpeg,image/gif,image/*;q=0.8")
		if h := hostOf(addr); strings.HasSuffix(h, "codeforces.com") || strings.HasSuffix(h, "cses.fi") {
			req.Header.Set("Referer", "https://"+h+"/") // the judges' CDNs can refuse hotlinking
		}
		resp, err := newClient().Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("image: HTTP %d", resp.StatusCode)
		}
		if data, err = io.ReadAll(io.LimitReader(resp.Body, maxBytes)); err != nil {
			return nil, err
		}
		if os.MkdirAll(dir, 0o755) == nil {
			os.WriteFile(file, data, 0o644) // a cache miss next time is fine
		}
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data)) // size first: a small file can claim a huge picture
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("image is %dx%d, too large", cfg.Width, cfg.Height)
	}
	m, _, err := image.Decode(bytes.NewReader(data))
	return m, err
}

// maxPixels bounds a decoded picture (16 megapixels).
const maxPixels = 16 << 20

// allowed accepts https addresses only. Any public host is fine (authors host statement images on
// imgur and the like); what must not happen is a statement steering verd at a private address, so
// the connection itself refuses those (see publicOnly).
func allowed(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return errors.New("only https images are fetched")
	}
	return nil
}

// publicOnly is a dialer control that refuses loopback, private, link-local and unspecified
// addresses, checked on the address actually dialled (so DNS tricks and redirects cannot get past it).
func publicOnly(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return fmt.Errorf("refusing non-public address %s", host)
	}
	return nil
}

func newClient() *http.Client {
	d := &net.Dialer{Timeout: 10 * time.Second, Control: publicOnly}
	return &http.Client{
		Timeout:   15 * time.Second,
		Transport: &http.Transport{DialContext: d.DialContext, TLSHandshakeTimeout: 10 * time.Second},
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) > 5 {
				return errors.New("too many redirects")
			}
			return allowed(r.URL.String())
		},
	}
}

// Fit is the cell size of an image of w x h pixels: at most maxCols x maxRows, keeping the shape
// (a cell is about twice as tall as wide) and never upscaling past about 8 pixels per column.
func Fit(w, h, maxCols, maxRows int) (cols, rows int) {
	if w <= 0 || h <= 0 || maxCols < 1 || maxRows < 1 {
		return 1, 1
	}
	cols = min(maxCols, max(1, (w+7)/8))
	rows = max(1, (cols*h+w)/(2*w)) // rounded
	if rows > maxRows {
		rows = maxRows
		cols = max(1, min(maxCols, rows*2*w/h))
	}
	return cols, rows
}

// Scale resamples src to w x h by averaging, flattening transparency onto white (statement
// diagrams are drawn for a white page).
func Scale(src image.Image, w, h int) *image.RGBA {
	b := src.Bounds()
	out := image.NewRGBA(image.Rect(0, 0, w, h))
	sw, sh := b.Dx(), b.Dy()
	for y := 0; y < h; y++ {
		y0, y1 := y*sh/h, max(y*sh/h+1, (y+1)*sh/h)
		for x := 0; x < w; x++ {
			x0, x1 := x*sw/w, max(x*sw/w+1, (x+1)*sw/w)
			var r, g, bl, n uint64
			for yy := y0; yy < y1; yy++ {
				for xx := x0; xx < x1; xx++ {
					cr, cg, cb, ca := src.At(b.Min.X+xx, b.Min.Y+yy).RGBA() // 16-bit, premultiplied
					r, g, bl, n = r+uint64(cr)+uint64(0xffff-ca), g+uint64(cg)+uint64(0xffff-ca), bl+uint64(cb)+uint64(0xffff-ca), n+1
				}
			}
			out.SetRGBA(x, y, color.RGBA{uint8(r / n >> 8), uint8(g / n >> 8), uint8(bl / n >> 8), 255})
		}
	}
	return out
}

// BlockLines draws src in cols x rows cells, two pixel rows per cell: the upper half block with the
// top pixel as foreground and the bottom pixel as background.
func BlockLines(src image.Image, cols, rows int) []string {
	px := Scale(src, cols, rows*2)
	lines := make([]string, rows)
	for r := 0; r < rows; r++ {
		var sb strings.Builder
		for c := 0; c < cols; c++ {
			t, b := px.RGBAAt(c, r*2), px.RGBAAt(c, r*2+1)
			fmt.Fprintf(&sb, "\x1b[38;2;%d;%d;%d;48;2;%d;%d;%dm▀", t.R, t.G, t.B, b.R, b.G, b.B)
		}
		sb.WriteString("\x1b[0m")
		lines[r] = sb.String()
	}
	return lines
}

// ID is the Kitty image number for a URL (24 bits, so it fits a placeholder's foreground colour).
func ID(url string) uint32 {
	sum := sha256.Sum256([]byte(url))
	return max(1, binary.BigEndian.Uint32(sum[:4])&0xFFFFFF)
}

// Transmit is the escape sequence that uploads src to the terminal as image id (not shown yet).
// q=2 silences the terminal's replies, which would otherwise arrive as typed input.
func Transmit(id uint32, src image.Image) string {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w > 1000 {
		w, h = 1000, max(1, h*1000/w)
	}
	var buf bytes.Buffer
	png.Encode(&buf, Scale(src, w, h))
	data := base64.StdEncoding.EncodeToString(buf.Bytes())
	var sb strings.Builder
	for first := true; len(data) > 0; first = false {
		n := min(4096, len(data))
		more := 0
		if n < len(data) {
			more = 1
		}
		if first {
			fmt.Fprintf(&sb, "\x1b_Ga=t,f=100,t=d,i=%d,q=2,m=%d;%s\x1b\\", id, more, data[:n])
		} else {
			fmt.Fprintf(&sb, "\x1b_Gm=%d,q=2;%s\x1b\\", more, data[:n])
		}
		data = data[n:]
	}
	return sb.String()
}

// Place is the escape sequence that makes image id a virtual placement of cols x rows cells,
// which the placeholder characters from Placeholders then show.
func Place(id uint32, cols, rows int) string {
	return fmt.Sprintf("\x1b_Ga=p,U=1,i=%d,c=%d,r=%d,q=2\x1b\\", id, cols, rows)
}

// Placeholders are the text lines that show image id: each cell is U+10EEEE plus the row and
// column marks, coloured with the image number.
func Placeholders(id uint32, cols, rows int) []string {
	fg := fmt.Sprintf("\x1b[38;2;%d;%d;%dm", id>>16&0xFF, id>>8&0xFF, id&0xFF)
	lines := make([]string, rows)
	for r := range lines {
		var sb strings.Builder
		sb.WriteString(fg)
		for c := 0; c < cols; c++ {
			sb.WriteRune(0x10EEEE)
			sb.WriteRune(kitty.Diacritic(r))
			sb.WriteRune(kitty.Diacritic(c))
		}
		sb.WriteString("\x1b[39m")
		lines[r] = sb.String()
	}
	return lines
}

func hostOf(addr string) string {
	u, err := url.Parse(addr)
	if err != nil {
		return ""
	}
	return strings.ToLower(u.Hostname())
}
