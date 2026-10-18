package cf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
)

// ProblemResult is one contestant's result on one problem in a standings row.
type ProblemResult struct {
	Points   float64 `json:"points"`
	Rejected int     `json:"rejectedAttemptCount"`
	Type     string  `json:"type"` // PRELIMINARY until system tests finish, then FINAL
	BestSecs *int64  `json:"bestSubmissionTimeSeconds"`
}

// Solved reports whether the problem scored: an accepted time (ICPC) or points (CF, IOI).
func (r ProblemResult) Solved() bool { return r.BestSecs != nil || r.Points > 0 }

// StandingRow is a contestant's line of the standings.
type StandingRow struct {
	Rank    int             `json:"rank"`
	Points  float64         `json:"points"`
	Penalty int             `json:"penalty"`
	Results []ProblemResult `json:"problemResults"`
}

// Standing is a contest's problems and, when the handle took part, its row.
type Standing struct {
	Contest  Contest
	Problems []Problem
	Row      *StandingRow // nil: the handle is not in the standings (not registered, or none asked for)
	Bytes    int64        // size of the download: big contests are many MB
}

type countReader struct {
	r io.Reader
	n *int64
}

func (c countReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	*c.n += int64(n)
	return n, err
}

// Standings downloads contest.standings and keeps only the contest, its problems and the row of
// handle. Codeforces only serves non-admins the complete standings (no from/count/handles), so
// this streams the body and drops every other row. With an empty handle it stops after the
// problems, which come first. A contest that has not started is an APIError.
func (c *Client) Standings(ctx context.Context, contest int, handle string) (*Standing, error) {
	res, err := c.Get(ctx, "/api/contest.standings", url.Values{"contestId": {strconv.Itoa(contest)}})
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	out := &Standing{}
	dec := json.NewDecoder(countReader{res.Body, &out.Bytes})
	bad := func(err error) (*Standing, error) {
		return nil, fmt.Errorf("%w: http %d: %v", ErrNetwork, res.StatusCode, err)
	}
	if err := expectDelim(dec, '{'); err != nil {
		return bad(err)
	}
	status, comment := "", ""
	for dec.More() {
		key, err := keyOf(dec)
		if err != nil {
			return bad(err)
		}
		switch key {
		case "status":
			err = dec.Decode(&status)
		case "comment":
			err = dec.Decode(&comment)
		case "result":
			if status != "OK" {
				return nil, &APIError{comment}
			}
			done, rerr := readResult(dec, handle, out)
			if rerr != nil {
				return bad(rerr)
			}
			if done {
				return out, nil
			}
		default:
			err = skip(dec)
		}
		if err != nil {
			return bad(err)
		}
	}
	if status != "OK" {
		return nil, &APIError{comment}
	}
	return out, nil
}

// readResult walks the result object; done means the wanted data was read and the rest can be left.
func readResult(dec *json.Decoder, handle string, out *Standing) (done bool, err error) {
	if err := expectDelim(dec, '{'); err != nil {
		return false, err
	}
	for dec.More() {
		key, err := keyOf(dec)
		if err != nil {
			return false, err
		}
		switch key {
		case "contest":
			err = dec.Decode(&out.Contest)
		case "problems":
			err = dec.Decode(&out.Problems)
		case "rows":
			if handle == "" {
				return true, nil
			}
			return false, readRows(dec, handle, out)
		default:
			err = skip(dec)
		}
		if err != nil {
			return false, err
		}
	}
	return false, expectDelim(dec, '}')
}

func readRows(dec *json.Decoder, handle string, out *Standing) error {
	if err := expectDelim(dec, '['); err != nil {
		return err
	}
	for dec.More() {
		var row struct {
			Party struct {
				Members []struct {
					Handle string `json:"handle"`
				} `json:"members"`
			} `json:"party"`
			StandingRow
		}
		if err := dec.Decode(&row); err != nil {
			return err
		}
		if out.Row != nil {
			continue
		}
		for _, m := range row.Party.Members {
			if strings.EqualFold(m.Handle, handle) {
				r := row.StandingRow
				out.Row = &r
			}
		}
	}
	return nil // the rest of the object is not needed
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	if d, ok := t.(json.Delim); !ok || d != want {
		return fmt.Errorf("want %q, got %v", want, t)
	}
	return nil
}

func keyOf(dec *json.Decoder) (string, error) {
	t, err := dec.Token()
	if err != nil {
		return "", err
	}
	s, ok := t.(string)
	if !ok {
		return "", fmt.Errorf("want a key, got %v", t)
	}
	return s, nil
}

func skip(dec *json.Decoder) error {
	var v json.RawMessage
	return dec.Decode(&v)
}

// BlogEntry is a Codeforces blog post (editorials are blog posts).
type BlogEntry struct {
	ID             int    `json:"id"`
	Title          string `json:"title"` // HTML, usually <p>...</p>
	Content        string `json:"content"`
	Locale         string `json:"locale"`
	OriginalLocale string `json:"originalLocale"`
}

// BlogEntry fetches a post's HTML through blogEntry.view, which works anonymously (the blog
// pages themselves sit behind the Cloudflare challenge).
func (c *Client) BlogEntry(ctx context.Context, id int) (*BlogEntry, error) {
	var b BlogEntry
	if err := c.api(ctx, "blogEntry.view", url.Values{"blogEntryId": {strconv.Itoa(id)}}, &b); err != nil {
		return nil, err
	}
	return &b, nil
}
