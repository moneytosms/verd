# Keys

Press `?` on any screen to see its keys. Keys are the same everywhere the same action exists.

## Rebinding

Every shortcut can be changed: Settings (`5`) > **Keyboard shortcuts**. `enter` rebinds the selected action (press the new key), `a` adds a second key, `backspace` resets it, and changes apply at once. They are saved to `config.toml`:

```toml
[keys.problem]
run_tests = "ctrl+t"
submit = ["s", "ctrl+s"]   # a list gives several keys
```

Contexts: `common`, `tabs`, `app`, `problems`, `contests`, `contest`, `stats`, `picker`, `settings`, `problem`, `editorial`, `testmgr`, `submission`, `help`, `editor`. A rebound action stops answering to its old key; two actions on one key (in the same screen) are refused. `ctrl+c` always quits. Text entry (prompts, the filters modal, the test editor, confirmations) is not rebindable. The tables below list the defaults; `?` shows your current keys.

Other changes in this release: opening a Problem and switching tabs no longer closes it (it is hidden until you return to its tab; `esc` closes it).

<!-- keys:begin -->
## Everywhere

| Key(s) | Action | What it does |
| --- | --- | --- |
| `?` | `help` | toggle help |
| `ctrl+k` | `palette` | command palette: fuzzy-run any action or setting |
| `Z` | `zen` | zen mode: hide everything but the statement and tests |
| `x` | `dismiss_toast` | dismiss a notification |

## Tabs

| Key(s) | Action | What it does |
| --- | --- | --- |
| `1` | `tab_1` | switch tab: Problems |
| `2` | `tab_2` | switch tab: Contests |
| `3` | `tab_3` | switch tab: Stats |
| `4` | `tab_4` | switch tab: Picker |
| `5` | `tab_5` | switch tab: Settings |

## Tab screens

| Key(s) | Action | What it does |
| --- | --- | --- |
| `ctrl+r` | `refresh` | refresh from the network |
| `tab` | `next_tab` | next tab |

## Problems

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q` | `quit` | quit |
| `k`, `up` | `up` | move up |
| `j`, `down` | `down` | move down |
| `pgup` | `page_up` | page up |
| `pgdown` | `page_down` | page down |
| `enter` | `open` | open Problem |
| `f` | `filters` | filters modal (rating, status, sort, tags) |
| `:` | `filter_expr` | filter expression: 800-1200 +dp -graphs unsolved |
| `X` | `clear_filters` | clear all filters |
| `/` | `search` | live fuzzy search |
| `m` | `mark_solved` | mark a CSES task solved or not |
| `Y` | `sync_cses` | sync solved marks from CSES |

## Contests

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q` | `quit` | quit |
| `k`, `up` | `up` | move up |
| `j`, `down` | `down` | move down |
| `pgup` | `page_up` | page up |
| `pgdown` | `page_down` | page down |
| `enter` | `open` | list the contest's Problems |

## Contest modal

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q`, `esc` | `close` | close |
| `s` | `start` | start a session: virtual (finished contest), live (running) or wait for the start (upcoming) |
| `p` | `practice` | start a practice session: no timer or penalty |
| `e` | `end_session` | show the session summary and end it |
| `H` | `editorial` | editorial of the selected problem |
| `k`, `up` | `up` | move up |
| `j`, `down` | `down` | move down |
| `enter` | `open` | open the Problem |

## Stats

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q` | `quit` | quit |
| `j`, `down` | `down` | scroll down |
| `k`, `up` | `up` | scroll up |
| `pgdown` | `page_down` | page down |
| `pgup` | `page_up` | page up |
| `n` | `next_problem` | select next attempted Problem |
| `N` | `prev_problem` | select previous attempted Problem |
| `enter` | `open` | open the selected Problem |
| `p` | `picker` | Problem Picker with the weak-topics preset |

## Picker

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q` | `quit` | quit |
| `r`, `space` | `reroll` | re-roll |
| `enter` | `open` | open the Problem |
| `f` | `filters` | filters: 800-1200 +dp -graphs |
| `w` | `weak_topics` | weak-topics preset |

## Settings

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q` | `quit` | quit |
| `/` | `search` | fuzzy-find a setting |
| `j`, `down` | `down` | move down |
| `k`, `up` | `up` | move up |
| `home`, `g` | `top` | first setting |
| `end`, `G` | `bottom` | last setting |
| `right`, `l`, `space` | `next_value` | next value / toggle |
| `left`, `h` | `prev_value` | previous value |
| `enter` | `edit` | edit a text value / open the shortcut editor |
| `L` | `login` | paste a browser session (direct submit) |
| `e` | `edit_config` | open config.toml in your editor |

## Problem view

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q` | `quit` | quit |
| `esc` | `back` | back (also cancels a stress run) |
| `tab` | `pane_next` | next pane: statement, tests, detail |
| `shift+tab` | `pane_prev` | previous pane |
| `k`, `up` | `up` | scroll up / previous test |
| `j`, `down` | `down` | scroll down / next test |
| `pgup` | `page_up` | page up |
| `pgdown` | `page_down` | page down |
| `home`, `g` | `top` | jump to the top |
| `end`, `G` | `bottom` | jump to the bottom |
| `e` | `edit` | edit the Solution in your editor |
| `N` | `notes` | edit the Problem's notes.md |
| `y` | `copy` | copy the focused pane |
| `I` | `copy_input` | copy the selected test's input only (tests/detail pane) |
| `E` | `copy_expected` | copy the selected test's expected output only (tests/detail pane) |
| `O` | `copy_output` | copy the selected test's actual output only (tests/detail pane) |
| `T` | `manage_tests` | manage tests: view, add, edit, copy, delete |
| `a` | `add_test` | add a Custom Test |
| `l` | `language` | switch language |
| `s` | `submit` | submit the Solution |
| `t` | `run_tests` | run tests |
| `i` | `snippets` | snippet library: copy bfs, dijkstra, modpow, ... |
| `H` | `editorial` | editorial: hints and solution, revealed one at a time |
| `S` | `stress` | stress test |
| `w` | `stress_save` | save the stress counterexample as a test |
| `c` | `cycle_mode` | cycle Comparison Mode |
| `n` | `next_test` | select next test |
| `p` | `prev_test` | select previous test |
| `space` | `mark_test` | mark or unmark the selected test for a partial run |
| `R` | `run_marked` | run only the marked tests (or the selected one if none is marked) |
| `F` | `run_failed` | rerun only the tests that did not pass |
| `f` | `fullview` | full-screen view of the selected test: input, expected, stdout, stderr |
| `d` | `diff` | diff the selected failing test |
| `r` | `refetch` | refetch statement |
| `o` | `open_browser` | open in the browser |
| `m` | `mark_solved` | mark a CSES task solved or not |
| `v` | `toggle_tags` | show or hide the tags |
| `>`, `.` | `widen` | widen the right column |
| `<`, `,` | `narrow` | narrow the right column |
| `+`, `=` | `grow_tests` | grow the test list |
| `-` | `shrink_tests` | shrink the test list |

## Full-screen test view

| Key(s) | Action | What it does |
| --- | --- | --- |
| `esc`, `f`, `q` | `close` | close the full-screen test view |
| `j`, `down` | `down` | scroll down |
| `k`, `up` | `up` | scroll up |
| `pgdown` | `page_down` | page down |
| `pgup` | `page_up` | page up |
| `home`, `g` | `top` | jump to the top |
| `end`, `G` | `bottom` | jump to the bottom |
| `l`, `right` | `pan_right` | pan right (no wrap) |
| `h`, `left` | `pan_left` | pan left (no wrap) |
| `w` | `wrap` | toggle wrap and pan |
| `1` | `section_1` | jump to Input |
| `2` | `section_2` | jump to Expected |
| `3` | `section_3` | jump to Output |
| `4` | `section_4` | jump to Stderr |
| `y` | `copy` | copy the section at the top |
| `r` | `rerun` | rerun just this test |

## Editorial

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q`, `esc` | `close` | close the editorial |
| `enter`, `space`, `n` | `reveal` | reveal the next hint or the solution |
| `j`, `down` | `down` | scroll down |
| `k`, `up` | `up` | scroll up |
| `pgdown` | `page_down` | page down |
| `pgup` | `page_up` | page up |
| `home`, `g` | `top` | jump to the top |
| `end`, `G` | `bottom` | jump to the bottom |
| `o` | `browser` | open the post in the browser |
| `r` | `retry` | retry after a failed load |

## Test manager

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q`, `esc` | `close` | close |
| `j`, `down` | `down` | move down |
| `k`, `up` | `up` | move up |
| `a` | `add` | add a Custom Test here |
| `e`, `enter` | `edit` | edit the selected Custom Test |
| `c` | `copy` | copy the selected test into a new Custom Test |
| `d`, `x` | `delete` | delete the selected Custom Test |
| `E` | `add_in_editor` | add a Custom Test in your editor |
| `t` | `run_tests` | run tests |

## Submission modal

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q`, `esc`, `enter` | `close` | close the Submission modal |

## Help

| Key(s) | Action | What it does |
| --- | --- | --- |
| `q`, `esc` | `close` | close help |
| `/` | `search` | fuzzy-search every shortcut |
| `right`, `l`, `tab` | `page_next` | next page |
| `left`, `h`, `shift+tab` | `page_prev` | previous page |
| `j`, `down` | `scroll_down` | scroll down |
| `k`, `up` | `scroll_up` | scroll up |
| `1` | `page_1` | page: This screen |
| `2` | `page_2` | page: Everywhere |
| `3` | `page_3` | page: Mouse |
| `4` | `page_4` | page: Guide |

## Embedded editor

| Key(s) | Action | What it does |
| --- | --- | --- |
| `ctrl+\` | `focus` | toggle the keyboard between verd and the embedded editor |
| `alt+h` | `focus_left` | keyboard to the left column |
| `alt+l` | `focus_right` | keyboard to the right column |
| `alt+z` | `zoom` | hide or show verd (editor fullscreen) |
| `alt+]` | `wider` | grow the editor column |
| `alt+[` | `narrower` | shrink the editor column |
| `alt+s` | `swap_side` | move editor between left and right |


<!-- keys:end -->

## Search

Type letters in order (`wmln` finds Watermelon), several words must all match (`water 4a`), and `#dp` matches tags. `tab` inside the prompt cycles what is searched: all, name, tag, id. `enter` keeps the search, `esc` undoes it.

## Filter syntax

A rating range (`800-1200`, `800-`, `-1200`, or one value), `+tag` to include, `-tag` to exclude (prefix match, `_` for a space: `+two_pointers`), and `unsolved` (or `u`).

The filter bar above the list shows what is active as chips; click it to open the filters. In the filters modal `tab` moves between fields, digits type a rating, `←`/`→` change status, sort and source, typing in the tag list searches it, `space` cycles a tag (include, exclude, off), `ctrl+u` resets and `esc` closes.

## Mouse

Click a tab pill to switch screens. Click a row to select it and the selected row again to open it. Click the filter bar to open the filters, a pane to focus it, a test to select it, a setting to select it and its value to change it. Click outside a modal to close it. The wheel scrolls whatever is under the pointer. In the Problem view, drag over the statement or the test detail to select whole lines; they are copied on release (OSC 52, so it works over ssh and tmux if your terminal allows it). Anywhere else, hold `shift` while dragging to select with your terminal.

## Modals

Small things open in a modal over the screen you were on. `q` or `esc` closes a modal; `q` only quits verd when no modal is open.

| Modal | Opened by | Keys |
| --- | --- | --- |
| Help | `?` | `←`/`→` page (this screen, everywhere, mouse, guide), `j`/`k` scroll, `q` closes |
| Filters | `f` | see Problems |
| Diff | `d` on a failing test, or a stress counterexample | `j`/`k` scroll, `w` saves a counterexample, `q` closes |
| Submission | `s` | Live status and final Verdict; `q` closes, tracking continues |
| Contest Problems | `enter` on a contest | `j`/`k` move, `enter` opens, `s` starts a [session](./contests.md), `H` editorial, `q` closes |
| Editorial | `H` | `enter` reveals the next hint, `j`/`k` scroll, `o` opens the post, `q` closes. See [Editorials](./editorials.md) |
| Snippets | `i` | Type to filter, `enter` copies, `tab` switches language, `esc` closes. See [Snippets](./config.md#snippets) |
| Session summary | `e` in the contest modal, or when the time is up | `e`/`enter` ends the session, `esc` keeps it |
| Test manager | `T` | `j`/`k` move, `a` add, `e` edit, `c` copy (also works on samples), `d` delete (asks first), `E` add in Neovim, `t` run, `q` closes |
| Test editor | `a`, `e` or `c` in the manager | Type, arrows move, `tab` switches input / expected, `ctrl+s` saves, `esc` cancels. Paste works. |

Samples are read-only; copy one to get an editable Custom Test.

## Problem view notes

On a terminal at least 100 columns wide and 16 rows tall the Problem view is split: the statement on the left, and on the right the Problem's rating, limits and tags, every test with its Local Verdict, and the selected test's input, expected and actual output. Narrower terminals get a single column.
