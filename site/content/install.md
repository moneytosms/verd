# Install

verd is one static binary for **Linux and macOS** (amd64 and arm64). On Windows, run it inside WSL.

## One line

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/verd/main/install.sh | sh
```

It installs the latest release to `~/.local/bin` after checking its SHA-256. Make sure that directory is on your `PATH`. Set `VERD_VERSION=v0.2.0` to pin a version, or `VERD_INSTALL_DIR` to change the target.

## Other ways

```sh
go install github.com/moneytosms/verd/cmd/verd@latest   # needs Go
```

Or download a tarball from the [Releases](https://github.com/moneytosms/verd/releases) page and put `verd` on your `PATH`. Packages for the AUR (`verd-bin`) and Homebrew (`moneytosms/tap/verd`) are planned.

## What else you need

| Tool | For | Notes |
| --- | --- | --- |
| `nvim` | Editing | Any editor works; Neovim is the one verd integrates deeply. Version 0.9 or newer. |
| `g++`, `gcc`, `python3` | Building and running your Solutions | Only the languages you use. Add others under `[lang.*]`. |
| `tmux` or `herdr` | The editor split | Optional. Without them verd can embed Neovim in its own window. |

## First run

```sh
verd --version
verd init            # writes config.toml and starter Templates, offers guided setup
verd doctor          # checks your compilers, editor and terminal
verd                 # open the TUI
```

`verd init` asks for your Codeforces handle (the only required setting). Everything else has a default. To script the setup instead of answering prompts:

```sh
verd init --no-setup
verd config set handle tourist
verd config set default_lang cpp
verd config set theme nord
```

## Shell completions

```sh
verd completions zsh  > ~/.zfunc/_verd                        # zsh (add ~/.zfunc to fpath)
verd completions bash > ~/.local/share/bash-completion/completions/verd
verd completions fish > ~/.config/fish/completions/verd.fish
```

## Update and uninstall

```sh
verd update --check   # is there a newer release?
verd update           # replace the binary (checksum verified)
```

Uninstall by deleting the binary. verd keeps its data in `~/.config/verd`, `~/.local/share/verd` and `~/.cache/verd`; see [Configuration](docs/config.html#files-verd-writes).

## Build from source (for development)

You need **Go 1.27 or newer** (see `go` in `go.mod`), `git`, and the same tools as above to try it end to end.

```sh
git clone https://github.com/moneytosms/verd && cd verd
go build -o verd ./cmd/verd     # a single static binary, no CGO
./verd --version
```

Run it against a throwaway config so your real one is untouched:

```sh
export XDG_CONFIG_HOME=$(mktemp -d) XDG_DATA_HOME=$(mktemp -d) XDG_CACHE_HOME=$(mktemp -d)
./verd init --no-setup && ./verd config set handle tourist
./verd                           # the TUI, using the temp config and cache
```

### Check your change

```sh
gofmt -l .                       # must print nothing
go vet ./...
go test ./...                    # unit tests; embedded-pane tests use a pseudo-terminal
```

Tests use temporary directories and fake servers, not a Codeforces account.

### Where things live

| Path | What |
| --- | --- |
| `cmd/verd` | The CLI: command dispatch, `test`/`submit`/`stress`, `config`, `keys`, `themes`, `doctor`, `sync`. |
| `internal/tui` | The Bubble Tea model: screens, the keymap (`keymap.go`), the shortcut editor, the embedded pane. |
| `internal/config` | `config.toml` loading, validation and the comment-preserving writer (`Set`, `SetKeys`). |
| `internal/theme` | Built-in themes, custom `[themes.*]`, palette and styles. |
| `internal/scrape`, `internal/cf`, `internal/cses` | Fetching and rendering statements; the Codeforces and CSES clients. |
| `internal/runner`, `internal/stress` | Compiling and running tests locally; the stress loop. |
| `internal/editor`, `internal/mux`, `internal/embed` | Driving Neovim, tmux/herdr panes, and the embedded terminal. |
| `internal/store`, `internal/refresh` | The SQLite cache and background refresh. |
| `docs/` | The documentation. The site in `site/` is generated from it. |

### Common tasks

- **Add a shortcut.** Add a row to `keyActions` in `internal/tui/keymap.go`, handle its canonical key in the screen's handler, then regenerate the key reference: `go run ./cmd/verd keys markdown --write docs/keys.md`. A test fails if the docs drift.
- **Add a config key.** Add the field and default in `internal/config/config.go`, register it in `settable` in `set.go`, document it in `default.toml` and `docs/config.md`, and, if it should be live in the TUI, add a row to `settingDefs` and a case in `applyLive` in `internal/tui/settings.go`.
- **Add a theme.** Add an entry to `themes` in `internal/theme/theme.go` (a dark and a light palette), or try colors first with a `[themes.*]` block in your config; `verd themes show nord` prints a template.
- **Preview the site.** `go run ./site/gen -out _site && python3 -m http.server -d _site 8000`.
- **Try a release build.** `goreleaser release --snapshot --clean` builds every target without publishing.

Issues and pull requests are welcome on [GitHub](https://github.com/moneytosms/verd/issues); the vocabulary used in the code (Problem, Solution, Sample Test, Verdict) is defined in [CONTEXT.md](https://github.com/moneytosms/verd/blob/main/CONTEXT.md).
