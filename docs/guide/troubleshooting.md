# Troubleshooting

Start with the doctor:

```sh
verd doctor
```

It prints `ok`, `warn` or `fail` for your config, handle, workspace, the compiler of every language, the editor (and Neovim 0.9 or newer), tmux or herdr, truecolor support and `$TERM`, and exits `1` if anything failed.

## verd will not start

| You see | Do |
| --- | --- |
| `no handle set: run verd init ...` | `verd config set handle <your handle>` |
| an error naming `config.toml` and a key | fix that line; verd never starts on a config it cannot load |
| `verd is already running` | only one TUI runs per user; close the other one (the message names the socket) |
| `TERM is 'dumb'` | run it in a real terminal |

## Problems and statements

- **"offline: statement not cached"**: no network and that statement was never fetched. Open it once online; after that it works offline. `ctrl+r` refreshes the lists.
- **"Codeforces blocked the request (Cloudflare)"**: Codeforces sometimes challenges non-browser clients. Press `o` to read the problem in your browser; verd retries on the next fetch. The API (problem list, submissions, standings, editorials) is not affected.
- **The list is empty**: the first start has to download the problem list. Wait for the sync or press `ctrl+r`.
- **CSES tasks are missing**: `verd config set source_cses true`, then `ctrl+r` in verd.

## Tests

- **`CE` with a wall of paths**: the compiler output is shown with long paths shortened; `e` jumps to the first error.
- **`bits/stdc++.h` not found on macOS**: plain `g++` there is Apple Clang. Install GNU gcc (`brew install gcc`); verd prefers `g++-15` ... `g++-9` when present.
- **Wrong verdict on a float problem**: `c` cycles the comparison mode; `float` compares within `float_eps`.
- **Tests are slow to start**: the first build compiles; later runs of unchanged code use the cache.
- **An interactive problem**: local runs are not supported; verd says so.

## Editor

- **`e` does nothing / "editor not found"**: `verd config set editor <command>` and check `verd doctor`.
- **Neovim looks wrong inside verd**: set `split = "suspend"` (or run verd inside tmux) and report the terminal on [issue #45](https://github.com/moneytosms/verd/issues/45).
- **A chord does not reach Neovim**: only the Kitty keyboard protocol can send some `ctrl+shift` combinations; use tmux or `suspend`.
- **Keys go to the wrong side**: `ctrl+\` hands the keyboard between verd and the embedded editor.

## Submitting

- **Nothing is copied**: install `wl-copy`, `xclip`, `xsel` (Linux) or use a terminal with OSC 52; verd prints the URL when no tool is found.
- **Direct mode falls back to the browser**: the status notes say why (an expired session, a Cloudflare check). `verd login` again, or use the default browser mode.

## Contests

- **`s` says "set your handle first"**: live sessions find your row in the standings by handle.
- **"not in the standings"**: register for the contest on codeforces.com.
- **A verdict is slow to show in a live session**: the standings refresh every minute (two minutes for very big rounds).

## Files and data

| Where | What |
| --- | --- |
| `~/.config/verd/config.toml` | your config |
| `~/.config/verd/snippets/` | Templates and the snippet library |
| `~/.local/share/verd/verd.db` | cache: problems, submissions, editorials, the running session |
| `~/.cache/verd/` | compiled binaries |
| `<workspace>/<contest>/<index>/` | your solutions and tests |

`$XDG_CONFIG_HOME`, `$XDG_DATA_HOME` and `$XDG_CACHE_HOME` are honored. Deleting `verd.db` resets the cache, which is rebuilt on the next start, along with your per-problem language and comparison choices and manual CSES marks. Your solutions and config are separate files and are untouched.

On WSL keep the workspace on the Linux filesystem (`/home/...`); `/mnt/c/...` is slow and `verd doctor` warns about it.

## Still stuck

`verd help <command>` has the exact options of each command. Report bugs, with the output of `verd doctor` and `verd --version`, on [GitHub](https://github.com/moneytosms/verd/issues).
