// Package refresh keeps the SQLite cache fresh from Codeforces.
package refresh

import (
	"context"
	"errors"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/cses"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
)

const (
	ProblemsetTTL = 24 * time.Hour
	ContestsTTL   = time.Hour
	RatingTTL     = 24 * time.Hour
)

// Problemset fetches and stores the problemset when the cache is older than ProblemsetTTL.
// It reports whether a fetch happened. On a fetch error the stale cache is left intact.
func Problemset(ctx context.Context, s *store.Store, c *cf.Client, now time.Time) (bool, error) {
	at, err := s.ProblemsetSyncedAt()
	if err != nil {
		return false, err
	}
	if !at.IsZero() && now.Sub(at) < ProblemsetTTL {
		return false, nil
	}
	ps, err := c.Problemset(ctx)
	if err != nil {
		return false, err
	}
	return true, s.SaveProblemset(ps, now)
}

// CSES is the client for the CSES source; tests point it at a local server.
var CSES = cses.New()

// Source fetches and stores a non-Codeforces source's problem list when its cache is older than
// ProblemsetTTL. It reports whether a fetch happened; a failed fetch leaves the cache intact.
func Source(ctx context.Context, s *store.Store, source string, now time.Time) (bool, error) {
	at, err := s.SourceSyncedAt(source)
	if err != nil {
		return false, err
	}
	if !at.IsZero() && now.Sub(at) < ProblemsetTTL {
		return false, nil
	}
	var ps []cf.Problem
	switch source {
	case cf.SourceCSES:
		ps, err = CSES.Problems(ctx)
	default:
		return false, errors.New("unknown source " + source)
	}
	if err != nil {
		return false, err
	}
	return true, s.SaveSource(source, ps, now)
}

// Detail returns a Problem's scraped detail, cache-first. Statements never expire;
// force (the `r` key) re-fetches. Errors from a failed fetch leave the cache intact.
func Detail(ctx context.Context, s *store.Store, c *cf.Client, p cf.Problem, force bool, now time.Time) (*scrape.Detail, error) {
	if !force {
		if d, err := s.Detail(p.ContestID, p.Index); err != nil || d != nil {
			return d, err
		}
	}
	var d *scrape.Detail
	if p.Source() == cf.SourceCSES {
		var err error
		if d, err = CSES.Page(ctx, p.ContestID); err != nil {
			return nil, err
		}
	} else {
		page, err := c.Page(ctx, p.ContestID, p.Index)
		if err != nil {
			return nil, err
		}
		if d, err = scrape.Parse(page); err != nil {
			return nil, err
		}
	}
	return d, s.SaveDetail(p.ContestID, p.Index, d, now)
}

const submissionPage = 100

// Submissions syncs the user's Submissions incrementally: newest pages first, stopping at the
// first Submission already stored with a final verdict (everything older is already known).
// ponytail: a long-ago rejudge of an old Submission is not picked up; manual full resync if it matters.
func Submissions(ctx context.Context, s *store.Store, c *cf.Client, handle string) (int, error) {
	added := 0
	for from := 1; ; from += submissionPage {
		page, err := c.UserStatus(ctx, handle, from, submissionPage)
		if err != nil {
			return added, err
		}
		var fresh []cf.Submission
		stop := len(page) < submissionPage
		for _, sub := range page {
			known, err := s.KnownFinal(sub.ID)
			if err != nil {
				return added, err
			}
			if known {
				stop = true
				break
			}
			fresh = append(fresh, sub)
		}
		if err := s.SaveSubmissions(fresh); err != nil {
			return added, err
		}
		added += len(fresh)
		if stop {
			return added, nil
		}
	}
}

// ContestProblems fetches one contest's Problems and adds them to the cache. The problemset omits
// running contests, so this is how their Problems show up.
func ContestProblems(ctx context.Context, s *store.Store, c *cf.Client, contest int) (int, error) {
	ps, err := c.ContestProblems(ctx, contest)
	if err != nil {
		return 0, err
	}
	return len(ps), s.AddProblems(ps)
}

// Contests fetches and stores the contest list when the cache is older than ContestsTTL.
func Contests(ctx context.Context, s *store.Store, c *cf.Client, now time.Time) (bool, error) {
	at, err := s.ContestsSyncedAt()
	if err != nil {
		return false, err
	}
	if !at.IsZero() && now.Sub(at) < ContestsTTL {
		return false, nil
	}
	cs, err := c.Contests(ctx)
	if err != nil {
		return false, err
	}
	return true, s.SaveContests(cs, now)
}

// Rating fetches and stores the rating history when the cache is older than RatingTTL.
func Rating(ctx context.Context, s *store.Store, c *cf.Client, handle string, now time.Time) (bool, error) {
	at, err := s.RatingSyncedAt()
	if err != nil {
		return false, err
	}
	if !at.IsZero() && now.Sub(at) < RatingTTL {
		return false, nil
	}
	rs, err := c.UserRating(ctx, handle)
	if err != nil {
		return false, err
	}
	return true, s.SaveRating(rs, now)
}

// Core refreshes the stale metadata (problemset, contests, rating). A failing step does not stop
// the others, except a network failure, which means offline: stop at once instead of timing out
// three times. errors.Is(err, cf.ErrNetwork) means offline. Submissions sync separately: first
// launch for a heavy user takes many pages and must not delay showing the problemset.
func Core(ctx context.Context, s *store.Store, c *cf.Client, handle string, now time.Time) error {
	steps := []func() error{
		func() (err error) { _, err = Problemset(ctx, s, c, now); return },
		func() (err error) { _, err = Contests(ctx, s, c, now); return },
		func() (err error) { _, err = Rating(ctx, s, c, handle, now); return },
	}
	var errs []error
	for _, step := range steps {
		if err := step(); err != nil {
			errs = append(errs, err)
			if errors.Is(err, cf.ErrNetwork) {
				break
			}
		}
	}
	return errors.Join(errs...)
}
