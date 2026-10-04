// Package cf is the single Codeforces client: browser UA, global rate limit, typed errors.
package cf

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

const (
	BaseURL   = "https://codeforces.com"
	userAgent = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/126.0.0.0 Safari/537.36"
	interval  = 2 * time.Second // default Client.Interval
)

var (
	ErrChallenge = errors.New("codeforces cloudflare challenge")
	ErrNetwork   = errors.New("codeforces network error")
)

// APIError is a status=FAILED envelope.
type APIError struct{ Comment string }

func (e *APIError) Error() string { return "codeforces api: " + e.Comment }

type Problem struct {
	ContestID   int      `json:"contestId"`
	Index       string   `json:"index"`
	Name        string   `json:"name"`
	Rating      int      `json:"rating"`
	Tags        []string `json:"tags"`
	SolvedCount int      `json:"-"`
}

type Client struct {
	Base     string
	HTTP     *http.Client
	Interval time.Duration // min spacing between requests

	mu    sync.Mutex
	last  time.Time
	now   func() time.Time
	sleep func(time.Duration)
}

func New(base string) *Client {
	return &Client{Base: base, HTTP: &http.Client{Timeout: 60 * time.Second}, Interval: interval, now: time.Now, sleep: time.Sleep}
}

// wait blocks until at least `Interval` after the previous request.
func (c *Client) wait() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.last.IsZero() {
		if d := c.Interval - c.now().Sub(c.last); d > 0 {
			c.sleep(d)
		}
	}
	c.last = c.now()
}

// Get performs a rate-limited GET and returns the body of a 200 response.
func (c *Client) Get(ctx context.Context, path string, q url.Values) (*http.Response, error) {
	c.wait()
	u := c.Base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	if res.Header.Get("cf-mitigated") != "" || res.StatusCode == http.StatusForbidden {
		res.Body.Close()
		return nil, ErrChallenge
	}
	return res, nil
}

type envelope struct {
	Status  string          `json:"status"`
	Comment string          `json:"comment"`
	Result  json.RawMessage `json:"result"`
}

func (c *Client) api(ctx context.Context, method string, q url.Values, out any) error {
	res, err := c.Get(ctx, "/api/"+method, q)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	var env envelope
	// Failed calls (e.g. 400 handle not found) still carry a JSON envelope.
	if err := json.NewDecoder(res.Body).Decode(&env); err != nil {
		return fmt.Errorf("%w: http %d: %v", ErrNetwork, res.StatusCode, err)
	}
	if env.Status != "OK" {
		return &APIError{env.Comment}
	}
	return json.Unmarshal(env.Result, out)
}

// Problemset fetches problemset.problems with solved counts joined on (contestId, index).
func (c *Client) Problemset(ctx context.Context) ([]Problem, error) {
	var r struct {
		Problems   []Problem `json:"problems"`
		Statistics []struct {
			ContestID   int    `json:"contestId"`
			Index       string `json:"index"`
			SolvedCount int    `json:"solvedCount"`
		} `json:"problemStatistics"`
	}
	if err := c.api(ctx, "problemset.problems", nil, &r); err != nil {
		return nil, err
	}
	type key struct {
		c int
		i string
	}
	solved := make(map[key]int, len(r.Statistics))
	for _, s := range r.Statistics {
		solved[key{s.ContestID, s.Index}] = s.SolvedCount
	}
	for i := range r.Problems {
		p := &r.Problems[i]
		p.SolvedCount = solved[key{p.ContestID, p.Index}]
	}
	return r.Problems, nil
}

// Page fetches a problem page. A Cloudflare challenge (header, 403, or interstitial body) is ErrChallenge.
func (c *Client) Page(ctx context.Context, contest int, index string) ([]byte, error) {
	res, err := c.Get(ctx, fmt.Sprintf("/problemset/problem/%d/%s", contest, index), nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNetwork, err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: http %d", ErrNetwork, res.StatusCode)
	}
	if bytes.Contains(b, []byte("Just a moment...")) || bytes.Contains(b, []byte("cf-chl")) {
		return nil, ErrChallenge
	}
	return b, nil
}

type Submission struct {
	ID          int64  `json:"id"`
	Created     int64  `json:"creationTimeSeconds"`
	Language    string `json:"programmingLanguage"`
	Verdict     string `json:"verdict"`
	PassedTests int    `json:"passedTestCount"`
	TimeMS      int    `json:"timeConsumedMillis"`
	MemoryBytes int64  `json:"memoryConsumedBytes"`
	Problem     struct {
		ContestID int    `json:"contestId"`
		Index     string `json:"index"`
	} `json:"problem"`
}

// UserStatus returns up to count of a user's Submissions starting at 1-based from, newest first.
func (c *Client) UserStatus(ctx context.Context, handle string, from, count int) ([]Submission, error) {
	var out []Submission
	err := c.api(ctx, "user.status", url.Values{"handle": {handle}, "from": {strconv.Itoa(from)}, "count": {strconv.Itoa(count)}}, &out)
	return out, err
}

type Contest struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Phase    string `json:"phase"`
	Duration int64  `json:"durationSeconds"`
	Start    int64  `json:"startTimeSeconds"`
}

// Finished reports whether the contest is over.
func (c Contest) Finished() bool { return c.Phase == "FINISHED" }

// Contests fetches contest.list (non-gym).
func (c *Client) Contests(ctx context.Context) ([]Contest, error) {
	var out []Contest
	err := c.api(ctx, "contest.list", nil, &out)
	return out, err
}
