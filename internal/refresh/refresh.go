// Package refresh keeps the SQLite cache fresh from Codeforces.
package refresh

import (
	"context"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
	"github.com/moneytosms/verd/internal/store"
)

const ProblemsetTTL = 24 * time.Hour

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

// Detail returns a Problem's scraped detail, cache-first. Statements never expire;
// force (the `r` key) re-fetches. Errors from a failed fetch leave the cache intact.
func Detail(ctx context.Context, s *store.Store, c *cf.Client, p cf.Problem, force bool, now time.Time) (*scrape.Detail, error) {
	if !force {
		if d, err := s.Detail(p.ContestID, p.Index); err != nil || d != nil {
			return d, err
		}
	}
	page, err := c.Page(ctx, p.ContestID, p.Index)
	if err != nil {
		return nil, err
	}
	d, err := scrape.Parse(page)
	if err != nil {
		return nil, err
	}
	return d, s.SaveDetail(p.ContestID, p.Index, d, now)
}
