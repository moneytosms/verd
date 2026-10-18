# Command line reference

`verd` with no arguments opens the TUI. Every command has a built-in help page, colored on a terminal and plain text when piped:

```sh
verd help                  # the overview
verd help stress           # one command
verd snippet add --help    # a subcommand
```

`NO_COLOR` and `TERM=dumb` give plain text; `FORCE_COLOR=1` forces colors. A usage error ends with `run verd help <command> for details`.

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | success (tests all `AC`, no counterexample, submission Accepted, editorial printed) |
| `1` | the thing you asked about failed or there is nothing to show (WA, counterexample, Rejected, no editorial) |
| `2` | verd could not do it (bad input, build error, offline, unknown argument) |

This makes the commands usable in scripts, git hooks and editor jobs.

## Everyday

| Command | Does | Example |
| --- | --- | --- |
| `verd test <file>` | run Sample and Custom Tests | `verd test ~/verd/1900/A/main.cpp` |
| `verd stress [--iter N] [--time S] [--jobs N] [--batch N] [--multi N] <file>` | search for a counterexample | `verd stress --iter 5000 main.cpp` |
| `verd submit <file>` | submit and track the verdict | `verd submit main.cpp` |
| `verd editorial [--reveal N \| --all] <file\|contest/index>` | read the editorial, hints withheld | `verd editorial --reveal 1 1900/A` |
| `verd snippet ...` | print, copy, add, edit, remove, export snippets | `verd snippet --copy dijkstra` |
| `verd daily` | today's unsolved pick | |

`<file>` is a solution inside the workspace (`<workspace>/<contest>/<index>/main.<ext>`); the problem and language come from its path. When the TUI is running, `test`, `stress` and `submit` are handed to it and show in its pane; otherwise they run headless.

## Setup and diagnosis

| Command | Does |
| --- | --- |
| `verd init [--force] [--no-setup]` | write the config, Templates and snippet library, offer the guided setup |
| `verd setup` | guided setup for handle, workspace, language, theme, editor split, submit mode; safe to re-run |
| `verd doctor` | check config, handle, workspace, compilers, editor, tmux/herdr, terminal |
| `verd update [--check]` | replace verd with the latest release (checksum verified) |
| `verd version`, `verd --version` | print the version and commit |

## Configuration

| Command | Does |
| --- | --- |
| `verd config [path \| keys \| get <key> \| set <key> <value>]` | show or change the config |
| `verd keys [--json] [list \| contexts \| check]` | list shortcuts |
| `verd keys set <ctx.action> <key>...` / `reset <ctx.action>` | rebind / restore |
| `verd keys presets \| preset <name> \| export \| import <file\|->` | presets, and moving bindings between machines |
| `verd themes [show <name>]` | list themes / print an editable block |
| `verd completions bash\|zsh\|fish` | shell completions |
| `verd schema` | JSON Schema of `config.toml` |

## Accounts

| Command | Does |
| --- | --- |
| `verd login` / `verd logout` | save or delete the browser session used by direct submit |
| `verd sync [cses]` | pull solved CSES tasks into verd as marks |
| `verd logout cses` | forget the CSES session |

## Global flag

`--here` uses the current directory as the workspace for this run, so solutions land in `<dir>/<contest>/<index>/`.

## Scripting examples

```sh
# fail a pre-push hook if the samples do not pass
verd test main.cpp || { echo "samples failing"; exit 1; }

# hunt for a counterexample for two minutes, quietly
verd stress --time 120 --jobs 8 main.cpp >/dev/null && echo "none found"

# every config key and its current value
for k in $(verd config keys); do printf '%s = %s\n' "$k" "$(verd config get $k)"; done

# submit and notify when judged
verd submit main.cpp && notify-send "verd" "Accepted"
```

Next: [Neovim and other editors](./editors.md).
