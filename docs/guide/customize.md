# Customize

Everything is configurable, live, from the Settings tab (`5`) or from `~/.config/verd/config.toml`. Edits to the file apply within about a second, so you can keep the editor open next to verd.

## Three ways to change a setting

```sh
verd config                        # print the effective config
verd config get theme              # read one key
verd config set theme nord         # change one key; your comments in the file stay
verd config keys                   # every key set accepts
```

In the TUI, `5` opens Settings: `j`/`k` move, `left`/`right` change a value, `enter` edits text, `e` opens `config.toml` in your editor. A bad value is refused (or named in an error), never silently ignored.

## Themes

Eight ship: `terminal` (inherits your palette, the default), `tokyo-night`, `dracula`, `catppuccin`, `gruvbox`, `nord`, `one-dark`, `solarized`.

```sh
verd themes                        # list them
verd config set theme tokyo-night
verd config set background dark    # auto | dark | light
```

Make your own: start from a built-in and change what you like.

```sh
verd themes show nord >> ~/.config/verd/config.toml    # appends a [themes.my-nord] block
verd config set theme my-nord
```

```toml
[themes.my-nord.dark]
accent = "#88c0d0"       # titles, focused borders, the active tab
good   = "#a3be8c"       # AC and solved marks
```

## Shortcuts

Every key is an action in a context, written `context.action` (for example `problem.stress`).

```sh
verd keys                          # all shortcuts: default and current
verd keys set problem.run_tests ctrl+t
verd keys set problem.submit s ctrl+s      # several keys are allowed
verd keys reset problem.run_tests
verd keys check                    # conflicts and unknown actions
verd keys presets                  # default, vim, arrows, emacs
verd keys preset vim
```

A rebound action stops answering to its old key, and two actions on one key within a screen are refused. `ctrl+c` always quits. In the TUI: Settings > **Keyboard shortcuts** (`/` searches, `enter` rebinds, `a` adds a second key, `backspace` resets). The full table is [Keys](../keys.md).

**Command palette.** `ctrl+k` (action `common.palette`) opens one fuzzy box over every action on the current screen and every setting. `enter` runs the action (using your own binding), flips a yes/no setting such as `Show footer` in place, or jumps to any other setting. In Settings, `/` fuzzy-searches settings by name, key and description; `?` then `/` searches all shortcuts.

## Reading

How statements look:

```sh
verd config set images blocks         # auto | kitty | blocks | off: statement images in the terminal
verd config set reading_size large     # compact | normal | large: a narrower, roomier column
verd config set reading_width 90         # cap the line length (0 = the whole pane)
verd config set reading_spacing relaxed  # compact | normal | relaxed
verd config set reading_headings bar     # plain | bold | bar | underline
verd config set reading_math latex       # unicode (default) shows 1 ≤ w ≤ 10⁹; latex shows the TeX source
verd config set reading_emphasis false   # no italics or bold, for fonts that render them badly
```

Your terminal sets the font.

**Line length** is how many characters fit across one line of text. Lines of 60 to 75 columns are the easiest to read: the eye finds the start of the next line without losing its place. Much wider lines make the eye hunt. `reading_width` caps the statement at a number of columns (`0` uses the whole pane). `reading_margin` adds empty columns on the left. `reading_size` sets both at once: `large` is a narrower column with a wider margin.

Two more settings:

```sh
verd config set reading_split cap       # full | cap: in the split view, cap caps the statement at 76 columns (when reading_width is 0)
verd config set reading_palette sepia   # colors for the statement only
verd config set reading_night_from 21   # hour (0-23) when auto switches to night
```

`reading_palette` takes `app` (your theme, the default), `paper`, `sepia`, `cool`, `night`, `mono`, `contrast` or `auto`. `auto` uses `night` from `reading_night_from` (default 20) until 07:00, then `app`. In a Problem, `P` cycles through the palettes.

## Layout, mouse and tabs

```sh
verd config set tabs problems,stats,settings   # which tabs, in what order
verd config set header false                   # hide the tab bar
verd config set footer false                   # hide the key hints
verd config set border square                  # rounded | square | heavy | double | ascii | none
verd config set mouse false                    # native terminal selection everywhere
```

## Editor and workspace

```sh
verd config set editor hx                  # any editor command; "code -w" works too
verd config set split auto                 # auto | tmux | herdr | embedded | suspend
verd config set embed_side left            # which column the embedded editor takes
verd config set workspace ~/cp             # where solutions live: <workspace>/<contest>/<index>/
verd --here                                # keep solutions in the current directory for one run
```

## Languages

Add or override languages under `[lang.<name>]`. Placeholders: `{src}` the source file, `{bin}` the compiled output, `{dir}` the problem directory.

```toml
[lang.rust]
ext = "rs"
compile = ["rustc", "-O", "-o", "{bin}", "{src}"]
run = ["{bin}"]
cf_compiler_id = 75          # used by direct submit
```

An interpreted language leaves `compile` empty and sets `run`:

```toml
[lang.ruby]
ext = "rb"
run = ["ruby", "{src}"]
```

Then `verd config set default_lang rust`. Set `default_lang ask` to pick a language per Problem from a list instead. Scale limits per language with `time_multiplier`. Reference: [Configuration](../config.md), [Customizing](../customizing.md).

## Check your work

```sh
verd config          # does it load, with your values?
verd keys check      # no conflicts?
verd doctor          # compilers, editor, terminal
verd schema > ~/.config/verd/schema.json   # editor completion for config.toml
```

If `config.toml` cannot be loaded, verd names the file and the problem and does not start; a bad edit never falls back to defaults silently.

Next: [Command line reference](./cli.md).
