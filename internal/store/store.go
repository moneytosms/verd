// Package store is verd's SQLite cache and state (plain database/sql, embedded migrations).
package store

import (
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"github.com/moneytosms/verd/internal/cf"
	"github.com/moneytosms/verd/internal/scrape"
	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrations embed.FS

const (
	keyProblemsetSync = "problemset_synced_at"
	keyContestsSync   = "contests_synced_at"
	keyRatingSync     = "rating_synced_at"
)

type Store struct{ db *sql.DB }

// Open opens (creating) the DB at path and applies pending migrations.
func Open(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, err
		}
	}
	dsn := path
	if path != ":memory:" {
		dsn += "?_pragma=busy_timeout(5000)" // `verd test` may share the file with a running TUI
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // single writer; also keeps :memory: on one connection
	s := &Store{db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	files, err := migrations.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name() < files[j].Name() })
	var cur int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&cur); err != nil {
		return err
	}
	for i, f := range files {
		v := i + 1
		if v <= cur {
			continue
		}
		b, _ := migrations.ReadFile("migrations/" + f.Name())
		tx, err := s.db.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(string(b)); err != nil {
			tx.Rollback()
			return fmt.Errorf("migration %s: %w", f.Name(), err)
		}
		if _, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", v)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

// Meta returns the value for key, or "" if unset.
func (s *Store) Meta(key string) (string, error) {
	var v string
	err := s.db.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return v, err
}

func (s *Store) setMeta(tx *sql.Tx, key, value string) error {
	_, err := tx.Exec("INSERT INTO meta(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value", key, value)
	return err
}

// SaveProblemset replaces all problems and stamps the sync time, atomically.
func (s *Store) SaveProblemset(ps []cf.Problem, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM problems"); err != nil {
		return err
	}
	st, err := tx.Prepare("INSERT INTO problems(contest_id, idx, name, rating, tags, solved_count) VALUES(?,?,?,?,?,?)")
	if err != nil {
		return err
	}
	defer st.Close()
	for _, p := range ps {
		tags, _ := json.Marshal(p.Tags)
		if _, err := st.Exec(p.ContestID, p.Index, p.Name, p.Rating, string(tags), p.SolvedCount); err != nil {
			return err
		}
	}
	if err := s.setMeta(tx, keyProblemsetSync, strconv.FormatInt(at.Unix(), 10)); err != nil {
		return err
	}
	return tx.Commit()
}

// ProblemsetSyncedAt returns the last problemset sync time; zero if never.
func (s *Store) ProblemsetSyncedAt() (time.Time, error) { return s.syncedAt(keyProblemsetSync) }

// ContestsSyncedAt returns the last contest list sync time; zero if never.
func (s *Store) ContestsSyncedAt() (time.Time, error) { return s.syncedAt(keyContestsSync) }

func (s *Store) syncedAt(key string) (time.Time, error) {
	v, err := s.Meta(key)
	if err != nil || v == "" {
		return time.Time{}, err
	}
	n, err := strconv.ParseInt(v, 10, 64)
	return time.Unix(n, 0), err
}

// SaveContests replaces the contest list and stamps the sync time, atomically.
func (s *Store) SaveContests(cs []cf.Contest, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM contests"); err != nil {
		return err
	}
	for _, c := range cs {
		if _, err := tx.Exec("INSERT INTO contests(id, name, phase, start_time, duration) VALUES(?,?,?,?,?)", c.ID, c.Name, c.Phase, c.Start, c.Duration); err != nil {
			return err
		}
	}
	if err := s.setMeta(tx, keyContestsSync, strconv.FormatInt(at.Unix(), 10)); err != nil {
		return err
	}
	return tx.Commit()
}

// Contests returns cached contests, newest start first.
func (s *Store) Contests() ([]cf.Contest, error) {
	rows, err := s.db.Query("SELECT id, name, phase, start_time, duration FROM contests ORDER BY start_time DESC, id DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cf.Contest
	for rows.Next() {
		var c cf.Contest
		if err := rows.Scan(&c.ID, &c.Name, &c.Phase, &c.Start, &c.Duration); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Problems returns the cached problemset, newest contest first.
func (s *Store) Problems() ([]cf.Problem, error) {
	rows, err := s.db.Query("SELECT contest_id, idx, name, rating, tags, solved_count FROM problems ORDER BY contest_id DESC, idx")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cf.Problem
	for rows.Next() {
		var p cf.Problem
		var tags string
		if err := rows.Scan(&p.ContestID, &p.Index, &p.Name, &p.Rating, &tags, &p.SolvedCount); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(tags), &p.Tags); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// SaveDetail replaces a Problem's scraped detail and Sample Tests, atomically.
func (s *Store) SaveDetail(contest int, idx string, d *scrape.Detail, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`INSERT OR REPLACE INTO problem_details(contest_id, idx, statement, time_limit_ms, memory_limit_mb, interactive, hint, fetched_at)
		VALUES(?,?,?,?,?,?,?,?)`, contest, idx, d.Statement, d.TimeLimitMS, d.MemoryLimitMB, d.Interactive, d.Hint, at.Unix()); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM sample_tests WHERE contest_id = ? AND idx = ?", contest, idx); err != nil {
		return err
	}
	for i, sm := range d.Samples {
		if _, err := tx.Exec("INSERT INTO sample_tests(contest_id, idx, n, input, output) VALUES(?,?,?,?,?)", contest, idx, i+1, sm.Input, sm.Output); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Detail returns the cached detail, or nil if the Problem was never fetched.
func (s *Store) Detail(contest int, idx string) (*scrape.Detail, error) {
	d := &scrape.Detail{}
	err := s.db.QueryRow("SELECT statement, time_limit_ms, memory_limit_mb, interactive, hint FROM problem_details WHERE contest_id = ? AND idx = ?", contest, idx).
		Scan(&d.Statement, &d.TimeLimitMS, &d.MemoryLimitMB, &d.Interactive, &d.Hint)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query("SELECT input, output FROM sample_tests WHERE contest_id = ? AND idx = ? ORDER BY n", contest, idx)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sm scrape.Sample
		if err := rows.Scan(&sm.Input, &sm.Output); err != nil {
			return nil, err
		}
		d.Samples = append(d.Samples, sm)
	}
	return d, rows.Err()
}

// Status is the user's standing on a Problem, derived from their Submissions.
type Status int

const (
	StatusNone Status = iota
	StatusAttempted
	StatusSolved
)

// SaveSubmissions upserts Submissions (a verdict can change from TESTING to final).
func (s *Store) SaveSubmissions(subs []cf.Submission) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	st, err := tx.Prepare(`INSERT OR REPLACE INTO submissions(id, contest_id, idx, language, verdict, passed_tests, time_ms, memory_bytes, created_at)
		VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer st.Close()
	for _, x := range subs {
		if _, err := st.Exec(x.ID, x.Problem.ContestID, x.Problem.Index, x.Language, x.Verdict, x.PassedTests, x.TimeMS, x.MemoryBytes, x.Created); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// KnownFinal reports whether Submission id is stored with a final verdict.
func (s *Store) KnownFinal(id int64) (bool, error) {
	var n int
	err := s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE id = ? AND verdict NOT IN ('', 'TESTING')", id).Scan(&n)
	return n > 0, err
}

// Statuses maps "<contest><index>" (e.g. "1900A") to Solved or Attempted. Unsubmitted Problems are absent.
func (s *Store) Statuses() (map[string]Status, error) {
	rows, err := s.db.Query("SELECT contest_id, idx, MAX(verdict = 'OK') FROM submissions GROUP BY contest_id, idx")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Status{}
	for rows.Next() {
		var c int
		var i string
		var ok bool
		if err := rows.Scan(&c, &i, &ok); err != nil {
			return nil, err
		}
		out[fmt.Sprintf("%d%s", c, i)] = map[bool]Status{true: StatusSolved, false: StatusAttempted}[ok]
	}
	return out, rows.Err()
}

// RatingSyncedAt returns the last rating history sync time; zero if never.
func (s *Store) RatingSyncedAt() (time.Time, error) { return s.syncedAt(keyRatingSync) }

// SaveRating replaces the rating history and stamps the sync time, atomically.
func (s *Store) SaveRating(rs []cf.RatingChange, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec("DELETE FROM rating_changes"); err != nil {
		return err
	}
	for _, r := range rs {
		if _, err := tx.Exec("INSERT INTO rating_changes(contest_id, old_rating, new_rating, at) VALUES(?,?,?,?)", r.ContestID, r.OldRating, r.NewRating, r.At); err != nil {
			return err
		}
	}
	if err := s.setMeta(tx, keyRatingSync, strconv.FormatInt(at.Unix(), 10)); err != nil {
		return err
	}
	return tx.Commit()
}

// Rating returns the rating history, oldest first.
func (s *Store) Rating() ([]cf.RatingChange, error) {
	rows, err := s.db.Query("SELECT contest_id, old_rating, new_rating, at FROM rating_changes ORDER BY at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cf.RatingChange
	for rows.Next() {
		var r cf.RatingChange
		if err := rows.Scan(&r.ContestID, &r.OldRating, &r.NewRating, &r.At); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// LastSync is the oldest of the TTL-driven sync times (the staleness of the cache as a whole); zero if any never synced.
func (s *Store) LastSync() (time.Time, error) {
	var oldest time.Time
	for i, f := range []func() (time.Time, error){s.ProblemsetSyncedAt, s.ContestsSyncedAt, s.RatingSyncedAt} {
		t, err := f()
		if err != nil || t.IsZero() {
			return time.Time{}, err
		}
		if i == 0 || t.Before(oldest) {
			oldest = t
		}
	}
	return oldest, nil
}
