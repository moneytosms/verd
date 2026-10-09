# Usage

verd is a keyboard-driven TUI for the contest loop: **pick a problem, read it, write the solution, test it, stress it, submit it**. Press `?` on any screen for its key list; the mouse works too.

## The loop

1. **Pick.** Open verd and stay on the Problems tab. `/` fuzzy-searches (try `water 4a`, or `#dp` for tags), `f` opens the filters (rating range, status, sort, tags), `4` draws a random unsolved problem, and `enter` opens one.
2. **Read.** The statement is rendered in the left pane with the tests on the right. `tab` moves between panes, `j`/`k` scroll, `y` copies the whole question (with difficulty and tags) for pasting into a chat or notes.
3. **Edit.** `e` creates `main.cpp` from your Template and opens Neovim next to verd (tmux or herdr split, or embedded in verd's own window). Switch language per problem with `l`.
4. **Test.** `t` runs every Sample and Custom Test locally and shows a verdict per test with time and memory. With autotest on (default) just save the file. `d` shows a side-by-side diff of a failing test, `c` cycles how output is compared.
5. **Add cases.** `a` adds a Custom Test, `T` manages them all.
6. **Stress.** `S` creates a generator and a brute force from your Templates; fill them in, press `S` again, and verd searches for an input where your solution and the brute force disagree. `w` saves the counterexample as your next Custom Test.
7. **Submit.** `s` copies the solution, opens the submit page and streams the verdict into the pane.

## From inside Neovim

```sh
verd test main.cpp      # run tests, report in verd's pane
verd stress main.cpp    # search for a counterexample
verd submit main.cpp    # submit and track the verdict
```

These talk to the running TUI over a Unix socket. With no TUI running they run headless and print the result. Exit codes: `0` success, `1` the thing you asked about failed (WA, counterexample, rejected), `2` verd could not do it (bad input, build error, offline). Map them to keys in your editor; see [Neovim](docs/neovim.html).

## Screens

| Tab | What it is |
| --- | --- |
| `1` Problems | Codeforces (and optionally CSES) in one list with live fuzzy search, filter chips and solved marks. |
| `2` Contests | Browse contests; `enter` lists a contest's problems with your per-problem status. |
| `3` Stats | Totals, rating chart, difficulty histogram, per-topic strengths, streaks, a solve heatmap. |
| `4` Picker | A random unsolved problem in your rating band. `w` is the weak-topics preset. |
| `5` Settings | Every option, live: theme, reading layout, editor, shortcuts, mouse, layout. |

A Problem you opened stays open when you switch tabs; `esc` closes it.

## Everyday commands

| Command | Does |
| --- | --- |
| `verd` | Open the TUI. `--here` keeps solutions in the current directory. |
| `verd daily` | Today's pick: an unsolved problem near your rating, from your weak topics. |
| `verd sync cses` | Pull your solved CSES tasks into verd. |
| `verd config` | Print the effective config; `config set <key> <value>` changes one key. |
| `verd keys` | List every shortcut; `keys set <context.action> <key>` rebinds one. |
| `verd themes` | List themes; `themes show <name>` prints an editable block. |
| `verd doctor` | Check compilers, editor and terminal. |
| `verd update` | Update to the latest release. |

## Make it yours

Everything is configurable, live, from Settings (`5`) or from `config.toml` (edits apply within a second). Rebind any shortcut, define your own colour theme, tune how statements are laid out, put Neovim on the left, hide the footer, reorder tabs. The [Customizing guide](docs/customizing.html) covers each of these with the exact commands, and is written so an agent can configure verd for you.

## Where to go next

- [Customizing](docs/customizing.html) for shortcuts, themes, layout and reading options
- [Testing and stress](docs/testing.html) for Comparison Modes and the stress workflow
- [Submitting](docs/submit.html) for browser handoff versus direct submit, and the risks
- [Configuration](docs/config.html) for every key, languages and Templates
