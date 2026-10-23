# Contests and sessions

The Contests tab (`2`) lists upcoming contests (soonest first) and past ones. `enter` opens a contest: its problems with your status, each openable like any other.

Press `/` to search by contest name, id or problem name. The chips `a` (all), `u` (upcoming), `p` (past), `r` (running) and `s` (at least one solve) narrow the list, and `o` sorts least-solved first. Each row shows `solved/total` for cached problems (`-` when not fetched yet).

A **session** turns a contest into a timed run inside verd. Start one from the contest modal.

| Key | For | Does |
| --- | --- | --- |
| `s` on a finished contest | replay | **virtual contest**: a timer for the contest's real length, starting now |
| `p` on a finished contest | replay | **practice**: same view, no timer, no penalty |
| `s` on a running contest | now | **live**: the real clock, results from the Codeforces standings |
| `s` on an upcoming contest | soon | **wait**: a countdown, then the problems load by themselves at the start |

Each key asks to be pressed twice, the first press tells you what will happen. One session runs at a time and survives restarting verd.

## While it runs

- The header shows the clock and score on every tab: `virtual  1:12:03 left  solved 2/6  penalty 47`.
- For that contest, **ratings, tags and solved counts are hidden** and your old solved marks are ignored. Only submissions made after the start count, so replaying a contest you solved before starts clean.
- The contest modal lists each problem with `12m +1` (solved at minute 12 after one wrong attempt) or `-2` (two wrong attempts, still open).
- The contest's [editorial](./editorials.md) is hidden.
- Submit as usual. Scoring updates as soon as the verdict lands.

## Contest mode

While a session runs, verd shows the contest's Problems on a rail at the left. `alt+n` / `alt+p` move between them, `alt+1` to `alt+9` jump to one, `alt+b` cycles the rail (full, compact, hidden) and `alt+x` opens the summary. Each session also writes a Markdown report to the workspace when it ends. Details: [Contest sessions](../contests.md).

## Scoring

**Virtual and practice** are scored locally, ICPC style: a solve costs its minute plus 20 minutes per wrong attempt before it. Compile errors and attempts after the first accepted one are free. It is an approximation of Codeforces' own scoring.

**Live** uses the real thing. Every minute verd downloads the standings and keeps your row:

- Codeforces-type rounds: `rank 5  490 pts`, and `490* +1` per problem (the star means system testing has not confirmed it yet).
- ICPC-type rounds: solved count and penalty.
- Not in the standings? The header says `not in the standings (register on the site)`. Registering happens on codeforces.com; verd cannot do it for you.

The full standings are several MB for a big round, so verd streams the response and keeps one row, and polls every two minutes after a download over 8 MB.

## The end

When the time is up (or when you press `e` in the contest modal) the summary opens: solved problems with times, penalty, and a list to upsolve. `e` or `enter` ends the session and shows everything again; `esc` keeps it.

```text
Session summary
  ✓ A   4m
  ✓ B   31m +1
  · C
  solved 2/3  penalty 75
  upsolve: C
```

Details and limits: [Contest sessions](../contests.md).

Next: [Editorials](./editorials.md).
