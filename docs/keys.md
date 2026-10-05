# Keys

Press `?` on any screen to see its keys. Keys are the same everywhere the same action exists.

## Global

| Key | Action |
| --- | --- |
| `1`-`5`, `tab` | Switch tab: Problems, Contests, Stats, Picker, Settings |
| `ctrl+r` | Refresh from the network |
| `?` | Toggle help |
| `q`, `ctrl+c` | Quit |

## Problems

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Move |
| `enter` | Open the Problem |
| `f` | Filters modal: rating range, status, sort and a tag picker (`+` include, `−` exclude) |
| `:` | Filter expression, e.g. `800-1200 +dp -graphs unsolved` |
| `X` | Clear all filters |
| `/` | Live fuzzy search: the list narrows and re-ranks as you type |

Search: type letters in order (`wmln` finds Watermelon), several words must all match (`water 4a`), and `#dp` matches tags. `tab` inside the prompt cycles what is searched: all, name, tag, id. `enter` keeps the search, `esc` undoes it.

Filter syntax: a rating range (`800-1200`, `800-`, `-1200`, or one value), `+tag` to include, `-tag` to exclude (prefix match, `_` for a space: `+two_pointers`), and `unsolved` (or `u`).

The filter bar above the list shows what is active as chips; click it to open the filters. In the filters modal `tab` moves between fields, digits type a rating, `←`/`→` change status and sort, typing in the tag list searches it, `space` cycles a tag (include, exclude, off), `ctrl+u` resets and `esc` closes.

## Contests

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Move |
| `enter` | Open a modal with the contest's Problems and your per-Problem status |
| `q`, `esc` | Close the modal |

## Stats

| Key | Action |
| --- | --- |
| `j`/`k`, `pgup`/`pgdn` | Scroll |
| `n`/`N` | Select next / previous attempted Problem |
| `enter` | Open it |
| `p` | Problem Picker with the weak-topics preset |

## Settings

| Key | Action |
| --- | --- |
| `j`/`k` | Move |
| `←`/`→`, `space` | Change an option or toggle (saved to `config.toml` at once) |
| `enter` | Edit a text value; `enter` saves, `esc` cancels |
| `e` | Open `config.toml` in your editor |

Theme, background, default language and autotest apply immediately. Settings marked `↻` apply the next time verd starts. Comments in `config.toml` are kept.

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
| `tab`, `shift+tab` | Move focus between the statement, the test list and the test detail |
| `j`/`k`, `pgup`/`pgdn`, `g`/`G` | Scroll the focused pane; in the test list, move the selection |
| `e` | Edit the Solution in Neovim |
| `T` | Test manager: browse, add, edit, copy and delete tests |
| `a` | Add a Custom Test (opens the in-place editor) |
| `l` | Switch language for this Problem |
| `t` | Run tests |
| `c` | Cycle Comparison Mode: tokens, exact, float, none |
| `n`/`p` | Select the next / previous test |
| `d` | Diff the selected failing test |
| `S` | Stress test. First press creates and opens `gen` and `brute`; press again to run. `esc` cancels. |
| `w` | Save the stress counterexample as the next Custom Test |
| `s` | Submit |
| `r` | Refetch the statement |
| `o` | Open the Problem in your browser |
| `esc` | Back to the list |

On a terminal at least 100 columns wide and 16 rows tall the Problem view is split: the statement on the left, and on the right the Problem's rating, limits and tags, every test with its Local Verdict, and the selected test's input, expected and actual output. Narrower terminals get a single column.

## Mouse

Click a tab pill to switch screens. Click a row to select it and the selected row again to open it. Click the filter bar to open the filters, a pane to focus it, a test to select it, a setting to select it and its value to change it. Click outside a modal to close it. The wheel scrolls whatever is under the pointer. While verd uses the mouse, hold `shift` to select text with your terminal.

## Modals

Small things open in a modal over the screen you were on. `q` or `esc` closes a modal; `q` only quits verd when no modal is open.

| Modal | Opened by | Keys |
| --- | --- | --- |
| Help | `?` | `←`/`→` page (this screen, everywhere, mouse, guide), `j`/`k` scroll, `q` closes |
| Filters | `f` | see Problems |
| Diff | `d` on a failing test, or a stress counterexample | `j`/`k` scroll, `w` saves a counterexample, `q` closes |
| Submission | `s` | Live status and final Verdict; `q` closes, tracking continues |
| Contest Problems | `enter` on a contest | `j`/`k` move, `enter` opens, `q` closes |
| Test manager | `T` | `j`/`k` move, `a` add, `e` edit, `c` copy (also works on samples), `d` delete (asks first), `E` add in Neovim, `t` run, `q` closes |
| Test editor | `a`, `e` or `c` in the manager | Type, arrows move, `tab` switches input / expected, `ctrl+s` saves, `esc` cancels. Paste works. |

Samples are read-only; copy one to get an editable Custom Test.
