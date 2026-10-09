# Customizing verd (for people and agents)

Everything verd does is driven by one file, `config.toml`, and every customization has a CLI command, so a person or an agent can set verd up and reshape it without clicking through the TUI. Each command below prints what it did and refuses a change that would leave a config verd cannot load.

Config location: `verd config path` (default `~/.config/verd/config.toml`, or `$XDG_CONFIG_HOME/verd/config.toml`).

## Contents

1. [Set up from scratch](#set-up-from-scratch)
2. [Read and change settings](#read-and-change-settings)
3. [Keyboard shortcuts](#keyboard-shortcuts)
4. [Themes and styling](#themes-and-styling)
5. [Layout and editor](#layout-and-editor)
6. [Languages and Templates](#languages-and-templates)
7. [Verify your changes](#verify-your-changes)
8. [Live reload](#live-reload)
9. [Reading and fonts](#reading-and-fonts)

## Set up from scratch

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/verd/main/install.sh | sh   # or: go install github.com/moneytosms/verd/cmd/verd@latest
verd --version
verd init --no-setup            # writes config.toml (every key commented) and starter Templates
verd config set handle tourist  # the only required setting
verd config set workspace ~/cp  # where Solutions live (default ~/verd)
verd config set default_lang cpp
verd config set editor nvim
verd                            # open the TUI
```

`verd setup` is the interactive version of the same questions. Two things need a human at a keyboard: `verd login` (pasting a browser session for [direct submit](./submit.md#direct-mode)) and `verd sync cses` (your CSES password, never stored). Everything else is scriptable. You need `nvim` (or another editor) and a compiler for your languages (`g++`, `gcc`, `python3` by default); `tmux` or `herdr` are optional for the editor split.

## Read and change settings

| Command | Does |
| --- | --- |
| `verd config` | Print the effective, merged config. |
| `verd config path` | Print the config file path. |
| `verd config keys` | List the keys `config set` accepts. |
| `verd config get <key>` | Print one value. |
| `verd config set <key> <value>` | Change one top-level key. Comments and other lines stay; a commented default is uncommented in place. Enum values are checked (`split`, `background`, `embed_side`, `submit_mode`), numbers and booleans are parsed. |

Tables (`[lang.*]`, `[keys.*]`, `[themes.*]`) are edited as text, or with `verd keys` for shortcuts. The full key list is in [Configuration](./config.md). The Settings tab (`5`) edits the same file live.

## Keyboard shortcuts

Every shortcut is rebindable. A shortcut is an **action** (`run_tests`) inside a **context** (`problem`), written `problem.run_tests`.

### Inspect

```sh
verd keys                 # every action: current keys, description, * if changed
verd keys --json          # same, machine readable: context, action, description, default, keys, modified
verd keys contexts        # the contexts and what they are
```

Always read `verd keys --json` for the authoritative, current action list; it is generated from the same table the TUI uses and cannot drift from the code.

### Presets

Quick-apply curated binding sets:

```sh
verd keys presets list                                    # show available presets
verd keys presets preset vim                             # apply vim preset (j/k, ctrl+d/u)
verd keys presets preset arrows                          # arrow keys only
verd keys presets preset emacs                           # emacs-style (ctrl+n/p, alt+v/ctrl+v)
verd keys presets preset default                         # clear all custom bindings
```

### Change

```sh
verd keys set problem.run_tests ctrl+t        # one key
verd keys set problem.down j down ctrl+n      # several keys for one action
verd keys set editor.focus_left alt+h
verd keys reset problem.run_tests             # back to the default
verd keys check                               # validate [keys.*] in config.toml
```

Export and import:

```sh
verd keys presets export                      # print current bindings as [keys.*] TOML blocks
verd keys presets import shortcuts.toml       # import from a file
verd keys presets export | ssh host verd keys presets import -  # save elsewhere or paste
```

or write the table yourself:

```toml
[keys.problem]
run_tests = "ctrl+t"
down = ["j", "down", "ctrl+n"]
```

### Rules

- **Key names:** a single character (`x`, `X`, `?`, `/`), `enter`, `esc`, `tab`, `space`, `backspace`, `delete`, `insert`, `up`, `down`, `left`, `right`, `home`, `end`, `pgup`, `pgdown`, and `ctrl+`, `alt+`, `shift+` prefixes (`ctrl+t`, `alt+z`, `shift+tab`). Uppercase letters are their own keys (`S` is not `s`).
- **A rebound action stops answering to its old key.** Rebinding `problem.run_tests` to `ctrl+t` makes `t` do nothing in the Problem view.
- **No two actions on one key** within the same screen. `verd keys set` and the editor refuse the change and name both actions.
- **`ctrl+c` always quits** and cannot be rebound.
- **Not rebindable:** text entry (the search and filter prompts, the filters modal, the test editor, the credential paste box) and y/n confirmations.
- The first key in an action's list is the one shown in footers, help and messages, which all follow your bindings.

### Contexts

| Context | Applies on |
| --- | --- |
| `common` | Every screen: help, dismiss a notification. |
| `tabs` | Every screen: switch tab `1`-`5`. |
| `app` | The tab screens (not the Problem view): refresh, next tab. |
| `problems`, `contests`, `stats`, `picker`, `settings` | That tab. |
| `contest` | The contest modal on the Contests tab. |
| `problem` | The Problem view, including the diff and stress overlays. |
| `testmgr` | The test manager modal (`T`). |
| `submission` | The Submission modal. |
| `help` | The help overlay. |
| `editor` | The embedded editor pane: `focus`, `focus_left`, `focus_right`, `zoom`, `wider`, `narrower`, `swap_side`. These work even while Neovim has the keyboard. |

The same shortcuts can be edited in the TUI: Settings (`5`) > **Keyboard shortcuts**. `/` fuzzy-searches by description, group, id or key; `enter` rebinds, `a` adds a key, `backspace` resets one action, `R` resets all (with y/n confirmation).

## Themes and styling

Eight themes ship: `terminal` (inherits your terminal's 16 colors, the default), `tokyo-night`, `dracula`, `catppuccin`, `gruvbox`, `nord`, `one-dark`, `solarized`.

```sh
verd themes                       # list every theme, built in and yours
verd themes show nord             # print an editable [themes.*] block for it
verd config set theme nord        # select one
verd config set background dark   # auto | dark | light
```

### Define your own

`verd themes show <name>` prints a complete block to start from. Paste it into `config.toml`, rename it, edit colors, then `verd config set theme <your-name>`:

```toml
[themes.midnight]
base = "tokyo-night"          # optional: start from a built-in, override only what you list
glamour_dark = "tokyo-night"  # statement/markdown style on a dark background
glamour_light = "light"       # ... and on a light one

[themes.midnight.dark]        # used when the terminal background is dark
accent = "#7aa2f7"
good = "#9ece6a"

[themes.midnight.light]       # used when it is light
accent = "#34548a"
```

A theme has a `dark` and a `light` palette; verd picks one from `background` (`auto` detects the terminal). Unset colors come from `base` (default `terminal`). Colors are `#rrggbb`, `#rgb`, or `ansiN` for terminal palette index N (0-255), which follows whatever palette your terminal uses.

| Color | Used for |
| --- | --- |
| `accent` | Titles, focused borders, the active tab fill, selected text. |
| `accent2` | Key caps, filter chips, section headings, the divider and badges. |
| `dim` | Secondary text, hints, unfocused borders. |
| `good` | AC, solved marks, success notes. |
| `bad` | WA, RE, CE, TLE, errors. |
| `warn` | Warnings, running state, "applies next start" notes. |
| `surface` | Background of the selected row and of chips. |
| `bar` | Header and footer background. |
| `on_accent` | Text drawn on an `accent` fill (the active tab). |

`glamour_dark` / `glamour_light` style the rendered statement: `dark`, `light`, `dracula`, `tokyo-night`, `pink`, `notty`, `ascii`, or a path to a [glamour](https://github.com/charmbracelet/glamour) style JSON file for full control of headings, code blocks and tables. An unknown theme name in `theme` falls back to `terminal` and shows a notice. A bad color or base is a config error that names the theme and key.

Pick a theme live in Settings (`5`): it previews as you move through the list.

## Layout and editor

| Setting | Values | Effect |
| --- | --- | --- |
| `editor` | `nvim`, `vim`, `hx`, `nano`, `micro`, `emacs`, `code`, any command | The editor `e` opens. |
| `split` | `auto`, `tmux`, `herdr`, `embedded`, `suspend` | Where the editor opens. See [Neovim](./neovim.md). |
| `embed_side` | `right` (default), `left` | Which column an embedded editor takes. Applies live. |
| `embed_ratio` | `0.1` to `0.9` | Share of the window verd keeps when the editor is embedded. |
| `embed_zoom` | `true`, `false` (default) | Start the editor fullscreen when it opens. |
| `autotest` | `true`, `false` | Re-run tests whenever you save the Solution. |
| `source_cf`, `source_cses` | `true`, `false` | Which Problem sources the lists show. |

Embedded editor keys (rebindable under `[keys.editor]`): `ctrl+\` toggles the keyboard between verd and the editor, `alt+h` / `alt+l` move the keyboard to the left / right column, `alt+z` hides verd so the editor fills the window, `alt+]` / `alt+[` grow/shrink the editor column, `alt+s` swaps the editor between left and right. See [Embedded pane](./embedded-pane.md).

A Problem you opened stays open when you switch tabs; `esc` closes it.

## Languages and Templates

Add or override languages under `[lang.<name>]` (`ext`, `compile`, `run`, `cf_compiler_id`) and edit Templates in `~/.config/verd/templates/`. Both are documented, with placeholders and Template variables, in [Configuration](./config.md#languages).

## Verify your changes

An agent should confirm each step rather than assume:

```sh
verd config                 # does the merged config load, and show your values?
verd keys check             # no unknown actions, bad keys or conflicts
verd keys --json | jq '.[] | select(.modified)'   # what differs from the defaults
verd themes show <name>     # does your theme resolve to the colors you meant?
```

If `config.toml` cannot be loaded, verd names the file and the problem (`themes.midnight: dark: accent: "red": want #rrggbb, #rgb or ansiN`) and does not start the TUI, so a bad edit never silently falls back to defaults. Footers, help and messages are built from the live bindings, so after changing a shortcut open the TUI and press `?` to confirm.

## Live reload

Changes to `config.toml` apply while verd runs, within about one second. Edit the file in your editor (or use `verd config set` / `verd keys set`), and you'll see the updates in the TUI: theme changes, reading options, shortcuts, autotest, source toggles, and embed settings all take effect immediately. Invalid edits show an error notice and are ignored; the old values stay in use.

## Reading and fonts

Font family and size belong to your terminal emulator; verd cannot change them. Change them in the terminal's own settings (for example `font` in kitty/alacritty/ghostty/foot, Preferences > Profiles > Text in iTerm2 and GNOME Terminal, `font.size` in Windows Terminal). What verd controls is how a statement is laid out and styled, live from Settings (`5`) > **Reading** or with `verd config set`:

| Key | Values | Effect |
| --- | --- | --- |
| `reading_width` | `0` or `40`-`200` | Cap the statement's line length (0 = the whole pane). Narrow columns read faster. |
| `reading_margin` | `0`-`8` | Left margin in columns. |
| `reading_spacing` | `compact`, `normal`, `relaxed` | Blank lines between paragraphs. |
| `reading_headings` | `plain`, `bold`, `bar`, `underline` | Input / Output / Note headings: text, bold, `▌ Input`, or with a rule under them. |
| `reading_math` | `unicode`, `raw` | TeX shown as symbols (`1 ≤ w ≤ 10⁹`) or as its source. |
| `reading_emphasis` | `true`, `false` | `false` turns italics and bold into plain text, for fonts that render them badly. |

Colors of headings and text come from the theme (see [Themes](#themes-and-styling)); `glamour_dark` / `glamour_light` in a theme pick the statement style.
