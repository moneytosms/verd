# Install

verd is one static binary for **Linux and macOS** (amd64 and arm64). On Windows, run it inside WSL.

## One line

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/verd/main/install.sh | sh
```

It installs the latest release to `~/.local/bin` after checking its SHA-256. Make sure that directory is on your `PATH`. Set `VERD_VERSION=v0.1.4` to pin a version, or `VERD_INSTALL_DIR` to change the target.

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
