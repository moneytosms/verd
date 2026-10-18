package tui

import (
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/config"
	"github.com/moneytosms/verd/internal/img"
	"github.com/moneytosms/verd/internal/scrape"
)

var imgToken = regexp.MustCompile(`VERDIMG(\d+)Z`)

// imagesMsg is one statement image, fetched (or not: m == nil).
type imagesMsg struct {
	p   cf.Problem
	url string
	m   image.Image
}

// imgMode is how images are drawn, from the images setting and the terminal.
func (m Model) imgMode() img.Mode { return img.Resolve(m.setting("images"), os.Getenv) }

// fetchImages downloads the statement's images that are not loaded yet, one command each.
func (m Model) fetchImages() []tea.Cmd {
	if m.detail == nil || m.open == nil || m.imgMode() == img.Off {
		return nil
	}
	fetch := m.deps.FetchImage
	if fetch == nil {
		dir := filepath.Join(config.CacheDir(), "images")
		fetch = func(u string) (image.Image, error) { return img.Fetch(u, dir) }
	}
	var cmds []tea.Cmd
	p := *m.open
	for _, u := range scrape.ImageURLs(m.detail.Statement) {
		if _, tried := m.imgs[u]; tried {
			continue
		}
		cmds = append(cmds, func() tea.Msg {
			pic, err := fetch(u)
			if err != nil {
				pic = nil
			}
			return imagesMsg{p: p, url: u, m: pic}
		})
	}
	return cmds
}

// onImages stores a fetched image and redraws the statement with it.
func (m Model) onImages(msg imagesMsg) Model {
	if m.imgs == nil {
		m.imgs = map[string]image.Image{}
	}
	m.imgs[msg.url] = msg.m
	if m.open == nil || m.open.ContestID != msg.p.ContestID || m.open.Index != msg.p.Index {
		return m
	}
	return m.rerender()
}

// rerender draws the open statement again at its current width.
func (m Model) rerender() Model {
	if m.detail == nil || m.bodyW == 0 {
		return m
	}
	if body, err := scrape.Render(m.detail.Statement, m.bodyW, m.styles().Glamour, m.readingOptions()); err == nil {
		m = m.setBody(body, m.bodyW)
	}
	return m
}

// setBody makes body (rendered at width w) the shown statement, with its images drawn in.
func (m Model) setBody(body string, w int) Model {
	lines, raw := m.withImages(strings.Split(body, "\n"), w)
	m.body, m.bodyW = lines, w
	m.rawOut += raw
	return m
}

// withImages replaces each image token line with the picture (or "[image]" while it is not there),
// and returns the terminal commands the pictures need (Kitty uploads and placements).
func (m Model) withImages(lines []string, w int) ([]string, string) {
	if m.detail == nil || !strings.Contains(strings.Join(lines, "\n"), "VERDIMG") {
		return lines, ""
	}
	urls := scrape.ImageURLs(m.detail.Statement)
	mode := m.imgMode()
	if m.imgSent == nil {
		m.imgSent = map[string]bool{}
	}
	var out []string
	var raw strings.Builder
	for _, l := range lines {
		plain := xansi.Strip(l)
		tm := imgToken.FindStringSubmatch(plain)
		if tm == nil {
			out = append(out, l)
			continue
		}
		i, _ := strconv.Atoi(tm[1])
		var pic image.Image
		if i < len(urls) && mode != img.Off {
			pic = m.imgs[urls[i]]
		}
		if pic == nil {
			out = append(out, strings.Replace(plain, tm[0], "[image]", 1))
			continue
		}
		margin := len(plain) - len(strings.TrimLeft(plain, " "))
		b := pic.Bounds()
		cols, rows := img.Fit(b.Dx(), b.Dy(), max(8, w-margin-2), max(6, min(20, m.paneHeight()-6)))
		var rowsText []string
		if mode == img.Kitty {
			id := img.ID(urls[i])
			if !m.imgSent[urls[i]] {
				raw.WriteString(img.Transmit(id, pic))
				m.imgSent[urls[i]] = true
			}
			if key := urls[i] + "@" + strconv.Itoa(cols) + "x" + strconv.Itoa(rows); !m.imgSent[key] {
				raw.WriteString(img.Place(id, cols, rows))
				m.imgSent[key] = true
			}
			rowsText = img.Placeholders(id, cols, rows)
		} else {
			rowsText = img.BlockLines(pic, cols, rows)
		}
		pad := strings.Repeat(" ", margin)
		out = append(out, "")
		for _, r := range rowsText {
			out = append(out, pad+r)
		}
		out = append(out, "")
	}
	return out, raw.String()
}

// scrimText is a line with colours removed for the dimmed background of a modal; picture cells
// become spaces (without their colour they would show as unknown characters).
func scrimText(l string) string {
	s := xansi.Strip(l)
	if !strings.ContainsRune(s, 0x10EEEE) {
		return s
	}
	var sb strings.Builder
	for _, r := range s {
		switch {
		case r == 0x10EEEE:
			sb.WriteByte(' ')
		case r >= 0x300 && r <= 0x36F: // the row and column marks
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}
