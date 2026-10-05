package submit

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/moneytosms/verd/internal/cf"
)

// Req and Resp are the HTTP surface Direct needs; the real Doer sits behind a Chrome TLS client.
type Req struct {
	Method, URL string
	Header      map[string]string
	Body        string
}

type Resp struct {
	Status   int
	Location string
	Body     []byte
}

// Doer performs one request without following redirects.
type Doer interface {
	Do(ctx context.Context, r Req) (Resp, error)
}

// Compiler is one programTypeId option on the submit page.
type Compiler struct {
	ID   int
	Name string
}

// Form is what the submit page offers.
type Form struct {
	CSRF      string
	Fields    url.Values // hidden inputs of the submit form
	Compilers []Compiler
}

// FallbackError means direct submit did not happen and the caller should use browser handoff.
type FallbackError struct{ Reason string }

func (e *FallbackError) Error() string { return e.Reason }

func fallback(f string, a ...any) error { return &FallbackError{fmt.Sprintf(f, a...)} }

// ParseForm reads the submit page. No csrf_token means the session is not logged in.
func ParseForm(page []byte) (Form, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return Form{}, err
	}
	form := Form{Fields: url.Values{}}
	doc.Find("form.submit-form input[name]").Each(func(_ int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		if t, _ := s.Attr("type"); name != "" && t != "file" {
			v, _ := s.Attr("value")
			form.Fields.Set(name, v)
		}
	})
	form.CSRF = form.Fields.Get("csrf_token")
	doc.Find(`select[name="programTypeId"] option`).Each(func(_ int, s *goquery.Selection) {
		if v, _ := s.Attr("value"); v != "" {
			if id, err := strconv.Atoi(v); err == nil {
				form.Compilers = append(form.Compilers, Compiler{id, strings.TrimSpace(s.Text())})
			}
		}
	})
	if form.CSRF == "" {
		return form, errors.New("no csrf_token on the submit page")
	}
	return form, nil
}

// Direct posts Submissions with a browser session the user pasted into `verd login`.
type Direct struct {
	Doer      Doer
	Cookie    string
	UserAgent string
}

func (d Direct) header(extra map[string]string) map[string]string {
	h := map[string]string{
		"accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8",
		"accept-language": "en-US,en;q=0.9",
		"cookie":          d.Cookie,
		"user-agent":      d.UserAgent,
	}
	for k, v := range extra {
		h[k] = v
	}
	return h
}

// Submit posts src as a Submission. Every failure is a *FallbackError carrying the reason.
func (d Direct) Submit(ctx context.Context, contest int, index string, compilerID int, src []byte) error {
	page := cf.BaseURL + "/problemset/submit"
	resp, err := d.Doer.Do(ctx, Req{Method: "GET", URL: page, Header: d.header(nil)})
	switch {
	case err != nil:
		return fallback("submit page: %v", err)
	case resp.Status == 403 || resp.Status == 503:
		return fallback("Codeforces blocked the request (HTTP %d): the session or browser fingerprint was rejected", resp.Status)
	case resp.Status != 200:
		return fallback("submit page returned HTTP %d: session expired? run `verd login`", resp.Status)
	}
	form, err := ParseForm(resp.Body)
	if err != nil {
		return fallback("%v: session expired? run `verd login`", err)
	}
	var names []string
	known := false
	for _, c := range form.Compilers {
		known = known || c.ID == compilerID
		names = append(names, fmt.Sprintf("%d=%s", c.ID, c.Name))
	}
	if !known {
		return fallback("cf_compiler_id %d is not offered by Codeforces; available: %s", compilerID, strings.Join(names, ", "))
	}
	f := form.Fields
	f.Set("action", "submitSolutionFormSubmitted")
	f.Set("contestId", strconv.Itoa(contest))
	f.Set("submittedProblemIndex", index)
	f.Set("programTypeId", strconv.Itoa(compilerID))
	f.Set("source", string(src))
	f.Set("tabSize", "4")
	// The page fills these in with JS; Codeforces accepted placeholders in the #40 spike.
	if f.Get("ftaa") == "" {
		f.Set("ftaa", "0123456789abcdef01")
	}
	if f.Get("bfaa") == "" {
		f.Set("bfaa", "f1b3f18c715565b589b7823cda7448ce")
	}
	post, err := d.Doer.Do(ctx, Req{Method: "POST", URL: page + "?csrf_token=" + url.QueryEscape(form.CSRF), Body: f.Encode(),
		Header: d.header(map[string]string{"content-type": "application/x-www-form-urlencoded", "origin": cf.BaseURL, "referer": page})})
	switch {
	case err != nil:
		return fallback("posting the Solution: %v", err)
	case post.Status == 403 || post.Status == 503:
		return fallback("Codeforces blocked the submission (HTTP %d)", post.Status)
	case (post.Status == 302 || post.Status == 303) && strings.Contains(post.Location, "/status"):
		return nil
	case post.Status == 302 || post.Status == 303:
		return fallback("redirected to %s: session expired? run `verd login`", post.Location)
	}
	return fallback("Codeforces did not accept the Solution (HTTP %d): %s", post.Status, formError(post.Body))
}

// formError pulls the first visible error message out of a rejected submit page.
func formError(page []byte) string {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return "unknown reason"
	}
	var msg string
	doc.Find("span.error").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		if t := strings.TrimSpace(s.Text()); t != "" {
			msg = t
		}
		return msg == ""
	})
	if msg == "" {
		return "unknown reason"
	}
	return msg
}
