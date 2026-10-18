# Contest sessions

Run a Codeforces contest as a timed session: open the Contests tab (`2`), press `enter` on a contest, then:

Finished contests are replayed:

| Key | Starts |
| --- | --- |
| `s` | **Virtual contest**: a timer for the contest's real duration, starting now. Penalty is scored. |
| `p` | **Practice**: same view, no timer, no penalty. |

Each key asks you to press it again to confirm. The first press tells you what will be hidden.

While a session runs:

- The header shows `virtual  1:12:03 left  solved 2/6  penalty 47` on every tab; the contest's row in the Contests tab is marked `● session`.
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
