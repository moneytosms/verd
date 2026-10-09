# Configuration

verd reads `~/.config/verd/config.toml` (`$XDG_CONFIG_HOME/verd/config.toml` if set). Every key is optional. `verd init` writes a fully commented file and offers `verd setup`, a guided walk through the main keys that edits the file in place (re-run it any time); `verd config` prints the effective, merged result.

## Keys

| Key | Default | Meaning |
| --- | --- | --- |
| `handle` | none, **required** | Your Codeforces handle. Solved marks, stats and Verdict tracking use it. |
| `workspace` | `"~/verd"` | Where Solutions and tests live: `<workspace>/<contest>/<index>/`. A relative value such as `"."` is resolved against the directory verd is started in; `verd --here` does the same for one run. |
| `default_lang` | `"cpp"` | Language for new Solutions. Any `[lang.*]` key. |
| `time_multiplier` | `1.0` | Scales every Problem's time limit for local runs. Use `2.0` on a slow machine. |
| `float_eps` | `1e-6` | Absolute/relative tolerance for the `float` Comparison Mode. |
| `autotest` | `true` | Run a Test Run whenever the active Solution is saved. |
| `theme` | `"terminal"` | `terminal` (inherits your palette), `tokyo-night`, `dracula`, `catppuccin`, `gruvbox`, `nord`, `one-dark` or `solarized`. Change it live in the Settings tab. |
| `background` | `"auto"` | `auto` detects the terminal, or force `dark` / `light`. |
| `editor` | `"nvim"` | The editor command: `nvim`, `vim`, `hx`, `nano`, `micro`, `emacs`, `kak`, `code`, `subl`, `zed` or any other, with optional arguments (`"code -w"`). verd knows how each takes a line number. Only Neovim reuses its pane on the next `e`; GUI editors open their own window instead of a split. Change it live in Settings. |
| `source_cf` | `true` | Show Codeforces Problems. See [Sources](./sources.md). |
| `source_cses` | `false` | Show the CSES Problem Set next to them. See [Sources](./sources.md). |
| `split` | `"auto"` | How the editor opens: `auto`, `tmux`, `herdr`, `embedded`, `suspend`. See [Neovim](./neovim.md). |
| `embed_ratio` | `0.4` | Share of the window verd keeps when Neovim is embedded. |
| `embed_focus_key` | `"ctrl+\\"` | Hands keyboard focus between verd and the embedded Neovim. |
| `submit_mode` | `"browser"` | `browser` or `direct`. See [Submitting](./submit.md). |
| `border` | `"rounded"` | Box border glyphs: `rounded`, `square`, `heavy`, `double`, `ascii`, or `none` (no frame, title line only). |
| `header` | `true` | Show the top tab/header bar. |
| `footer` | `true` | Show the key-hint footer bar. |
| `density` | `"normal"` | Compact spacing: `normal` or `compact` (removes blank spacer rows where easy). |

An unknown `submit_mode` or `border` is a config error.

## Languages

Three languages ship by default, mirroring the Codeforces compilers:

| Key | Ext | Compile | Run | `cf_compiler_id` |
| --- | --- | --- | --- | --- |
| `c` | `c` | `gcc -std=c11 -O2 -Wall -o {bin} {src} -lm` | `{bin}` | 43 |
| `cpp` | `cpp` | `g++ -std=c++20 -O2 -Wall -o {bin} {src}` | `{bin}` | 89 |
| `python` | `py` | none (interpreted) | `python3 {src}` | 31 |

Override only what you change; omitted keys are inherited from the default of the same name:

```toml
[lang.cpp]
compile = ["g++", "-std=c++23", "-O2", "-o", "{bin}", "{src}"]

# Add a language
[lang.rust]
ext = "rs"
compile = ["rustc", "-O", "-o", "{bin}", "{src}"]
run = ["{bin}"]
cf_compiler_id = 75
```

| Field | Meaning |
| --- | --- |
| `ext` | File extension of Solutions. |
| `compile` | Build command. Omit for interpreted languages. |
| `run` | Run command. With no `compile`, it runs `{src}` directly. |
| `cf_compiler_id` | The Codeforces `programTypeId`, used by [direct submit](./submit.md#direct-mode). |

Placeholders: `{src}` the Solution, `{bin}` the compiled output, `{dir}` the Problem directory. Compiled binaries are cached by content, so an unchanged Solution is never rebuilt.

## Templates

`verd init` writes starter Templates to `~/.config/verd/templates/` and never overwrites existing ones unless you pass `--force`.

| File | Used for |
| --- | --- |
| `<lang>.<ext>` (`cpp.cpp`, `python.py`) | New Solutions. |
| `gen.<lang>.<ext>` | The stress test input generator; `argv[1]` is the seed. |
| `brute.<lang>.<ext>` | The slow, obviously correct Solution stress compares against. |

Solution Templates are Go `text/template` files. Variables: `{{.Problem.ID}}`, `{{.Problem.Name}}`, `{{.Problem.URL}}`, `{{.Handle}}`, `{{.Date}}`. `{{cursor}}` marks where Neovim places the cursor and is removed from the output.

## Files verd writes

| Path | Contents |
| --- | --- |
| `~/.config/verd/config.toml` | Your config. |
| `~/.config/verd/templates/` | Templates. |
| `~/.config/verd/credentials.json` | Only if the OS keyring was unavailable at `verd login`; mode `0600`. |
| `~/.local/share/verd/verd.db` | SQLite cache: Problems, Contests, Submissions, per-Problem choices. |
| `~/.cache/verd/` | Compiled binaries. |
| `$XDG_RUNTIME_DIR/verd/verd.sock` | The socket `verd test` / `submit` / `stress` use to reach the TUI. |
| `<workspace>/<contest>/<index>/` | `main.<ext>`, `gen.<ext>`, `brute.<ext>`, and `tests/`. |

`tests/` holds `sample-N.in/.ans` (written from the statement) and `custom-N.in/.ans` (yours). Only complete `.in`/`.ans` pairs run.

The workspace is created on first use. Under WSL, keep it on the Linux filesystem; verd warns when it is under `/mnt/`.
