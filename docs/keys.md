# Keys

Press `?` on any screen to see its keys. Keys are the same everywhere the same action exists.

## Global

| Key | Action |
| --- | --- |
| `1`-`4`, `tab` | Switch tab: Problems, Contests, Stats, Picker |
| `ctrl+r` | Refresh from the network |
| `?` | Toggle help |
| `q`, `ctrl+c` | Quit |

## Problems

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Move |
| `enter` | Open the Problem |
| `f` | Filter, e.g. `800-1200 +dp -graphs unsolved` |
| `/` | Search by ID or name |

Filter syntax: a rating range (`800-1200`, `800-`, `-1200`, or one value), `+tag` to include, `-tag` to exclude (prefix match, `_` for a space: `+two_pointers`), and `unsolved` (or `u`).

## Contests

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Move |
| `enter` | List the contest's Problems with your per-Problem status |
| `esc` | Back |

## Stats

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Scroll |
| `n`/`N` | Select next / previous attempted Problem |
| `enter` | Open it |
| `p` | Problem Picker with the weak-topics preset |

## Picker

| Key | Action |
| --- | --- |
| `space`, `r` | Re-roll |
| `enter` | Open the Problem |
| `f` | Filters, same syntax as the Problems tab |
| `w` | Weak-topics preset: your three weakest topics, in your rating band, unsolved. Needs 20+ Problems per topic. |

## Problem

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Scroll |
| `e` | Edit the Solution in Neovim |
| `a` | Add a Custom Test (opens `.in` and `.ans` side by side) |
| `l` | Switch language for this Problem |
| `t` | Run tests |
| `c` | Cycle Comparison Mode: tokens, exact, float, none |
| `n`/`p` | Select a test |
| `d` | Diff the selected failing test |
| `S` | Stress test. First press creates and opens `gen` and `brute`; press again to run. `esc` cancels. |
| `w` | Save the stress counterexample as the next Custom Test |
| `s` | Submit |
| `r` | Refetch the statement |
| `o` | Open the Problem in your browser |
| `esc` | Back |

In a diff overlay: `j`/`k` scroll, `esc` or `d` closes, and for a stress counterexample `w` saves it.
