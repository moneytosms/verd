# Contest sessions

Run a Codeforces contest as a timed session: open the Contests tab (`2`), press `enter` on a contest, then:

Finished contests are replayed:

| Key | Starts |
| --- | --- |
| `s` | **Virtual contest**: a timer for the contest's real duration, starting now. Penalty is scored. |
| `p` | **Practice**: same view, no timer, no penalty. |

Each key asks you to press it again to confirm. The first press tells you what will be hidden.

## Finding contests

On the Contests tab, press `/` to search. Each word must match a contest's name or id, or a problem's name inside a contest whose problems are cached. Chips narrow the list: `a` all, `u` upcoming, `p` past, `r` in progress, `s` contests with at least one solve. Press a chip's key again to go back to all. `o` sorts least-solved first.

Each row shows `solved/total` for the contest's cached problems. `-` means the problems are not fetched yet. `?` appears during the contest's own session, where the count is hidden.

## Contest mode

Starting a session, of any kind, switches verd into contest mode, and it stays there (across restarts too) until the session ends:

- The tabs are hidden. The header shows the contest, its clock and the score.
- The contest's Problems sit on a rail at the left with each result (`✓ 12m +1`, `-2`); the open one is marked `▸`. Problem A opens first (a Problem you had open from outside the session gives way to it, but one opened from outside afterwards, such as `verd test` on another contest, stays), and every statement is downloaded in the background so switching is instant.
- `alt+n` / `alt+p` go to the next / previous Problem, `alt+1` to `alt+9` jump straight to one, and a click on the rail does the same. These work even while the embedded editor has the keyboard, and an open editor pane follows to the new Problem's Solution. Each Problem keeps its scroll, selected test and last Test Run, so switching back loses nothing.
- `alt+b` cycles the rail: full (name, clock, score, problems, standings), compact (letters and results only) and hidden. Until you pick one it is compact next to an embedded editor or in a window under 140 columns, full otherwise. Zen mode hides it. The choice is remembered.
- For a live contest the rail adds a standings panel: your rank out of everyone (`rank 512/18034`), the top three, and the two rows above and below yours, with your handle highlighted. A virtual or practice session has no live standings; its score is verd's own.
- `alt+x` opens the session summary at any time: `e` ends the session there, `esc` goes back to it.

All of these keys can be rebound under `[keys.contest_mode]`.

### Session stats and report

While a session runs, verd records for each Problem when you first opened it, how long it was the open Problem and how many Test Runs you made (and how many passed). The summary shows them next to each result. A report is never overwritten: a second session of the same mode starting in the same minute is saved as `…-1500-2.md`.

Ending the session writes a Markdown report to `<workspace>/<contest>/<mode>-<date>.md` (for example `~/verd/500/virtual-2026-10-22-1500.md`): the date and length, the score, a table with each Problem's result, wrong attempts, time spent, first open and Test Runs, the totals, and links to what is left to upsolve. The footer says where it went.

While a session runs:

- The header shows `virtual  1:12:03 left  solved 2/6  penalty 47`; the contest's row in the Contests tab is marked `● session`.
- That contest's ratings, tags and solved counts are hidden everywhere, and your old solved marks for it are ignored. Only Submissions made after the start count, so replaying a contest you solved before starts clean. Other contests are unaffected.
- The contest modal lists the problems with `12m +1` (solved at minute 12 after one wrong attempt) or `-2` (two wrong attempts, still open).
- Submit as usual (`s` in the Problem view, `verd submit`). verd tracks the Verdict and rescoring happens as soon as it lands.
- When the time is up, the summary opens: solved problems with times, penalty and an upsolve list. `e` or `enter` ends the session and shows everything again; `esc` keeps it. In the contest modal, `e` opens the summary at any time.

Scoring is local and ICPC style: a solve costs its minute plus 20 minutes per wrong attempt before it. Compilation errors and attempts after the first AC are free. It is an approximation: Codeforces' own point decay and standings rank are not used.

One session runs at a time and survives restarting verd.

## Live contests

Press `s` on a contest that is running or upcoming (register on the Codeforces site first; verd cannot do that for you).

- **Upcoming:** the session waits. The header counts down (`live  starts in 0:09:41`); when the contest begins verd downloads the Problems by itself.
- **Running:** the clock is the real one (`live  1:50:00 left`), not a fresh timer.
- **Results come from the Codeforces standings, not from verd's own scoring.** Every minute verd downloads the standings and keeps your row: rank and points for Codeforces-type rounds (`rank 5  490 pts`), solved and penalty for ICPC-type ones. The modal shows each problem's points and wrong attempts (`490* +1`; a star means system testing has not confirmed it yet) and the marks follow your row. Between polls nothing changes, so a fresh Verdict can take up to a minute to show; submitting is unchanged.
- Codeforces only serves the complete standings, which is several MB for a big round, so verd streams the response and keeps one row. After a download over 8 MB it polls every two minutes instead of every minute.
- If your handle is not in the standings the header says `not in the standings (register on the site)`; set `handle` first, since the row is found by it.
- At the end a last download feeds the summary, which warns that results can change after system testing.

Codeforces' own virtual participation (the register button on the site) is separate and does not see replayed Submissions as virtual.
