# Sources

verd shows Problems from more than one problem set in one list. Turn each on or off in Settings (`5`, the **Sources** group) or with `source_cf` / `source_cses` in `config.toml`. Codeforces is on by default; CSES is off.

| Source | What you get | Submitting | Solved marks |
| --- | --- | --- | --- |
| Codeforces | Ratings, tags, statements, Verdict tracking, stats | Browser handoff, or [direct](./submit.md#direct-mode) | From your Submissions |
| [CSES](https://cses.fi/problemset/) | 400 tasks. The topic section (for example "Dynamic Programming") is the tag; the solver count is the popularity. Limits, statement and the example from the task page | Browser handoff to `cses.fi/problemset/submit/<id>/` (CSES needs a login and has no API) | By hand: `m` |

## Using CSES

1. Settings, **CSES**, turn it on. Press `ctrl+r` on a list to load the tasks (24 hours cache, then refreshed on start).
2. CSES tasks appear in the same list as `CSES1068`. Search by name, topic or id. Filter with the **Source** field in the filter modal (`f`) or `src:cses` in the `:` prompt, for example `src:cses +graph`.
3. `enter` opens a task. `e` edits `<workspace>/cses/<id>/main.<ext>`, `t` runs the example and your Custom Tests, `S` stress tests it.
4. `s` copies the Solution and opens the CSES submit page. verd cannot read the verdict there, so press `m` on the task to mark it solved (`m` again clears it).

CSES shows a single example per task, so local runs cover that plus the tests you add. Marks live only in verd's own database. Codeforces stats do not include CSES.

Statements are fetched once and cached. verd waits a second between requests to cses.fi and identifies itself in the User-Agent.

## USACO

Not implemented. Its statements are public (`usaco.org/index.php?page=viewproblem2&cpid=N`) and contests list their problems on results pages, but submitting needs a login, so it would use the same browser handoff and manual marks as CSES. Official test data is published as per-contest zip files, which could give full local tests; that is the main reason to add it. A new source needs one Source tag in `internal/cf/source.go`, a fetcher beside `internal/cses`, and a line in `refresh.Source`.
