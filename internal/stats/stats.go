// Package stats computes Profile Stats from cached Problems, Submissions and rating history.
package stats

import (
	"fmt"
	"sort"
	"time"

	"github.com/moneytosms/verd/internal/cf"
)

// Band limits and thresholds from the Profile Stats design.
const (
	bandBelow    = 200
	bandAbove    = 300
	minBandTag   = 20 // tags with fewer Problems in the band are ignored for strengths/weaknesses
	unratedBandL = 800
	unratedBandH = 1200
)

type Input struct {
	Problems    []cf.Problem
	Submissions []cf.Submission
	Rating      []cf.RatingChange // oldest first
	Now         time.Time
	Loc         *time.Location // for streak days; nil = Local
}

type TagStat struct {
	Tag          string
	Solved       int
	AvgRating    float64 // of solved, rated Problems; 0 if none
	FirstTryRate float64 // share of solved Problems whose first Submission was OK
	InBand       int     // Problems with this tag in the user's rating band
	SolvedInBand int
	Coverage     float64 // SolvedInBand / InBand
}

type Attempt struct {
	ContestID int
	Index     string
	Name      string
	Attempts  int
	Last      time.Time
}

type Stats struct {
	Solved, Attempted, Submissions int
	ACRate                         float64
	Rating, MaxRating              int
	Rank                           string
	RatingHistory                  []cf.RatingChange
	SolvedByRating                 map[int]int // bucket (800, 900, ...) -> solved
	SolvedUnrated                  int
	Tags                           []TagStat // by solved desc, then name
	Strengths, Weaknesses          []TagStat
	Streak, MaxStreak              int
	Days                           map[string]int // "2006-01-02" -> AC Submissions that day
	Verdicts                       map[string]int
	Unsolved                       []Attempt // most recently attempted first
	Band                           [2]int
}

type key struct {
	contest int
	index   string
}

func final(v string) bool { return v != "" && v != "TESTING" }

// Rank is the Codeforces title for a rating; "unrated" for 0.
func Rank(r int) string {
	switch {
	case r <= 0:
		return "unrated"
	case r < 1200:
		return "newbie"
	case r < 1400:
		return "pupil"
	case r < 1600:
		return "specialist"
	case r < 1900:
		return "expert"
	case r < 2100:
		return "candidate master"
	case r < 2300:
		return "master"
	case r < 2400:
		return "international master"
	case r < 2600:
		return "grandmaster"
	case r < 3000:
		return "international grandmaster"
	}
	return "legendary grandmaster"
}

// hist is one Problem's Submission history.
type hist struct {
	firstOK  bool
	solved   bool
	attempts int
	last     time.Time
}

// tagAgg sums the solved Problems of one tag.
type tagAgg struct {
	solved, firstTry, rated int
	ratingSum               float64
}

// Compute derives Profile Stats. Submissions still TESTING are ignored.
func Compute(in Input) Stats {
	loc := in.Loc
	if loc == nil {
		loc = time.Local
	}
	st := Stats{SolvedByRating: map[int]int{}, Verdicts: map[string]int{}, RatingHistory: in.Rating}
	byID := make(map[key]cf.Problem, len(in.Problems))
	for _, p := range in.Problems {
		byID[key{p.ContestID, p.Index}] = p
	}

	h, days := tallySubmissions(&st, in.Submissions, loc)

	// Rating.
	if n := len(in.Rating); n > 0 {
		st.Rating = in.Rating[n-1].NewRating
		for _, r := range in.Rating {
			st.MaxRating = max(st.MaxRating, r.NewRating)
		}
	}
	st.Rank = Rank(st.Rating)
	lo, hi := unratedBandL, unratedBandH
	if st.Rating > 0 {
		lo, hi = st.Rating-bandBelow, st.Rating+bandAbove
	}
	st.Band = [2]int{lo, hi}

	solvedKeys, tags := tallyProblems(&st, h, byID)
	sortUnsolved(st.Unsolved)

	inBand, solvedInBand := bandCoverage(in.Problems, lo, hi, solvedKeys)
	st.Tags = tagStats(tags, inBand, solvedInBand)
	sort.Slice(st.Tags, func(i, j int) bool {
		a, b := st.Tags[i], st.Tags[j]
		if a.Solved != b.Solved {
			return a.Solved > b.Solved
		}
		return a.Tag < b.Tag
	})
	st.Strengths, st.Weaknesses = pickTags(st.Tags)

	st.Streak, st.MaxStreak = streaks(days, in.Now.In(loc))
	return st
}

// tallySubmissions folds the final Submissions into st (verdicts, days, AC rate) and returns
// the per-Problem history and the days with an AC.
func tallySubmissions(st *Stats, submissions []cf.Submission, loc *time.Location) (map[key]*hist, map[string]bool) {
	// Per-Problem history, oldest first (Submission ids grow with time).
	subs := make([]cf.Submission, 0, len(submissions))
	for _, s := range submissions {
		if final(s.Verdict) {
			subs = append(subs, s)
		}
	}
	sort.Slice(subs, func(i, j int) bool { return subs[i].ID < subs[j].ID })
	h := map[key]*hist{}
	days := map[string]bool{}
	st.Days = map[string]int{}
	ok := 0
	for _, s := range subs {
		k := key{s.Problem.ContestID, s.Problem.Index}
		x := h[k]
		if x == nil {
			x = &hist{firstOK: s.Verdict == "OK"}
			h[k] = x
		}
		x.attempts++
		x.last = time.Unix(s.Created, 0)
		st.Verdicts[s.Verdict]++
		if s.Verdict == "OK" {
			ok++
			x.solved = true
			d := time.Unix(s.Created, 0).In(loc).Format("2006-01-02")
			days[d] = true
			st.Days[d]++
		}
	}
	st.Submissions = len(subs)
	if len(subs) > 0 {
		st.ACRate = float64(ok) / float64(len(subs))
	}
	return h, days
}

// tallyProblems sets the totals and difficulty buckets in st, collects unsolved attempts,
// and returns the solved Problem keys and the per-tag sums.
func tallyProblems(st *Stats, h map[key]*hist, byID map[key]cf.Problem) (map[key]bool, map[string]*tagAgg) {
	tags := map[string]*tagAgg{}
	solvedKeys := map[key]bool{}
	for k, x := range h {
		p, known := byID[k]
		if !x.solved {
			st.Attempted++
			name := ""
			if known {
				name = p.Name
			}
			st.Unsolved = append(st.Unsolved, Attempt{k.contest, k.index, name, x.attempts, x.last})
			continue
		}
		st.Solved++
		solvedKeys[k] = true
		if known && p.Rating > 0 {
			st.SolvedByRating[p.Rating/100*100]++
		} else {
			st.SolvedUnrated++
		}
		if known {
			addTagSums(tags, p, x.firstOK)
		}
	}
	return solvedKeys, tags
}

// addTagSums adds one solved Problem to the sums of each of its tags.
func addTagSums(tags map[string]*tagAgg, p cf.Problem, firstOK bool) {
	for _, t := range p.Tags {
		a := tags[t]
		if a == nil {
			a = &tagAgg{}
			tags[t] = a
		}
		a.solved++
		if firstOK {
			a.firstTry++
		}
		if p.Rating > 0 {
			a.rated++
			a.ratingSum += float64(p.Rating)
		}
	}
}

// sortUnsolved orders attempts most recently attempted first.
func sortUnsolved(u []Attempt) {
	sort.Slice(u, func(i, j int) bool {
		a, b := u[i], u[j]
		if !a.Last.Equal(b.Last) {
			return a.Last.After(b.Last)
		}
		if a.ContestID != b.ContestID {
			return a.ContestID > b.ContestID
		}
		return a.Index < b.Index
	})
}

// bandCoverage counts Problems per tag inside the rating band [lo, hi], and how many are solved.
func bandCoverage(problems []cf.Problem, lo, hi int, solvedKeys map[key]bool) (map[string]int, map[string]int) {
	inBand := map[string]int{}
	solvedInBand := map[string]int{}
	for _, p := range problems {
		if p.Rating < lo || p.Rating > hi {
			continue
		}
		for _, t := range p.Tags {
			inBand[t]++
			if solvedKeys[key{p.ContestID, p.Index}] {
				solvedInBand[t]++
			}
		}
	}
	return inBand, solvedInBand
}

// tagStats returns one TagStat per tag seen in solves or in the band, unsorted.
func tagStats(tags map[string]*tagAgg, inBand, solvedInBand map[string]int) []TagStat {
	var out []TagStat
	for t, a := range tags {
		out = append(out, tagStat(t, a, inBand, solvedInBand))
	}
	for t := range inBand {
		if tags[t] == nil {
			out = append(out, tagStat(t, nil, inBand, solvedInBand))
		}
	}
	return out
}

// tagStat builds one TagStat; a is nil when the tag has no solves.
func tagStat(t string, a *tagAgg, inBand, solvedInBand map[string]int) TagStat {
	ts := TagStat{Tag: t, InBand: inBand[t], SolvedInBand: solvedInBand[t]}
	if a != nil {
		ts.Solved = a.solved
		ts.FirstTryRate = float64(a.firstTry) / float64(a.solved)
		if a.rated > 0 {
			ts.AvgRating = a.ratingSum / float64(a.rated)
		}
	}
	if ts.InBand > 0 {
		ts.Coverage = float64(ts.SolvedInBand) / float64(ts.InBand)
	}
	return ts
}

// pickTags returns the top 3 coverage tags (strengths) and the 3 weakest of the rest (weakest first).
// Only tags with enough Problems in the band qualify.
func pickTags(all []TagStat) (strengths, weaknesses []TagStat) {
	var eligible []TagStat
	for _, t := range all {
		if t.InBand >= minBandTag {
			eligible = append(eligible, t)
		}
	}
	sort.Slice(eligible, func(i, j int) bool {
		a, b := eligible[i], eligible[j]
		if a.Coverage != b.Coverage {
			return a.Coverage > b.Coverage
		}
		return a.Tag < b.Tag
	})
	n := min(3, len(eligible))
	strengths = append([]TagStat(nil), eligible[:n]...)
	rest := eligible[n:] // disjoint from strengths when few tags qualify
	w := min(3, len(rest))
	for i := len(rest) - 1; i >= len(rest)-w; i-- {
		weaknesses = append(weaknesses, rest[i])
	}
	return strengths, weaknesses
}

// streaks counts consecutive days with at least one solve. The current streak is still alive if
// the last solve was yesterday (today may not have happened yet).
func streaks(days map[string]bool, now time.Time) (cur, best int) {
	var all []string
	for d := range days {
		all = append(all, d)
	}
	sort.Strings(all)
	run := 0
	var prev time.Time
	for _, d := range all {
		t, _ := time.ParseInLocation("2006-01-02", d, now.Location())
		if run > 0 && sameNextDay(prev, t) {
			run++
		} else {
			run = 1
		}
		best = max(best, run)
		prev = t
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	for d := today; ; d = d.AddDate(0, 0, -1) {
		if days[d.Format("2006-01-02")] {
			cur++
			continue
		}
		if d.Equal(today) {
			continue // no solve yet today: yesterday may still be the end of the streak
		}
		break
	}
	return cur, best
}

// sameNextDay reports whether b is the calendar day after a (DST-safe).
func sameNextDay(a, b time.Time) bool {
	y, m, d := a.AddDate(0, 0, 1).Date()
	by, bm, bd := b.Date()
	return y == by && m == bm && d == bd
}

// String renders a TagStat's coverage for lists, e.g. "dp 62% (31/50)".
func (t TagStat) String() string {
	return fmt.Sprintf("%s %.0f%% (%d/%d)", t.Tag, t.Coverage*100, t.SolvedInBand, t.InBand)
}
