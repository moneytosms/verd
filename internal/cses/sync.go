package cses

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ErrNotLoggedIn means the session is missing or expired.
var ErrNotLoggedIn = errors.New("not logged in to CSES")

// Login signs in with a username and password and returns the session as a Cookie header value.
// CSES has a plain form: GET /login gives a csrf_token and a PHPSESSID cookie, POST sends
// nick, pass and the token. The password is used once and never stored.
func (c *Client) Login(ctx context.Context, nick, pass string) (string, error) {
	jar, _ := cookiejar.New(nil)
	hc := &http.Client{Timeout: c.HTTP.Timeout, Jar: jar}
	page, err := get(ctx, hc, c.Base+"/login")
	if err != nil {
		return "", err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return "", err
	}
	csrf, ok := doc.Find(`input[name="csrf_token"]`).Attr("value")
	if !ok {
		return "", errors.New("CSES login page has no csrf_token (did the site change?)")
	}
	form := url.Values{"csrf_token": {csrf}, "nick": {nick}, "pass": {pass}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Base+"/login", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", userAgent)
	res, err := hc.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if bytes.Contains(body, []byte(`name="nick"`)) { // the login form again
		return "", errors.New("CSES rejected the username or password")
	}
	base, _ := url.Parse(c.Base)
	var parts []string
	for _, ck := range jar.Cookies(base) {
		parts = append(parts, ck.Name+"="+ck.Value)
	}
	if len(parts) == 0 {
		return "", errors.New("CSES set no session cookie")
	}
	return strings.Join(parts, "; "), nil
}

func get(ctx context.Context, hc *http.Client, u string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s: http %d", u, res.StatusCode)
	}
	return io.ReadAll(res.Body)
}

// Solved returns the ids of the tasks the logged-in user has solved: the task list marks a task
// with .task-score icon "full" when it is solved and "zero" when it was tried.
func (c *Client) Solved(ctx context.Context, cookie string) ([]int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.Base+"/problemset/", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", cookie)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	page, _ := io.ReadAll(res.Body)
	return ParseSolved(page)
}

// ParseSolved reads the solved task ids from a logged-in task list page.
func ParseSolved(page []byte) ([]int, error) {
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return nil, err
	}
	if doc.Find(`a.account[href="/login"]`).Length() > 0 { // the header shows "Login" instead of the user
		return nil, ErrNotLoggedIn
	}
	if doc.Find("li.task").Length() == 0 {
		return nil, errors.New("no tasks on the CSES list page (did the site change?)")
	}
	var ids []int
	doc.Find("li.task").Each(func(_ int, li *goquery.Selection) {
		href, _ := li.Find("a").First().Attr("href")
		m := taskHref.FindStringSubmatch(href)
		if m == nil {
			return
		}
		if cls, _ := li.Find(".task-score").Attr("class"); strings.Contains(cls, "full") {
			id, _ := strconv.Atoi(m[1])
			ids = append(ids, id)
		}
	})
	return ids, nil
}
