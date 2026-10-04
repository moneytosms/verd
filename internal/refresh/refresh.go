// Package refresh keeps the SQLite cache fresh from Codeforces.
package refresh

import (
	"context"
	"time"

	"github.com/moneytosms/verd/internal/cf"
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
