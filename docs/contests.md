# Contest sessions

Replay a finished Codeforces contest as a timed session: open the Contests tab (`2`), press `enter` on a contest, then:

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

Not yet: joining a contest that is running right now, and waiting for one to start. Codeforces' own virtual participation (the register button on the site) is separate and does not see these Submissions as virtual.
