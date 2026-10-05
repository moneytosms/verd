<p align="center">
  <img src="./assets/readme/hero.svg" width="100%" alt="verd: Codeforces in your terminal. The TUI shows a Problem with local tests all AC, then a stress run that found a counterexample at seed 7.">
</p>

<p align="center">
  <a href="https://github.com/moneytosms/verd/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/moneytosms/verd/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/go-1.27-7aa2f7?labelColor=1a1b26">
  <img alt="Platforms" src="https://img.shields.io/badge/linux%20%C2%B7%20macos-9ece6a?labelColor=1a1b26">
</p>

**verd** is a terminal companion for [Codeforces](https://codeforces.com). Find a Problem, read the statement, write the Solution in Neovim, run it against Sample Tests and your own, hunt for counterexamples with a stress test, submit, and watch the Verdict arrive. One keyboard-driven TUI, no browser tab for the loop.

<p align="center">
  <img src="./assets/readme/loop.svg" width="100%" alt="The verd loop: pick, read, edit, test, stress, submit.">
</p>

## Why

- **Local Verdicts first.** AC, WA, TLE, RE, MLE and CE are decided on your machine, with time and memory per test, before you spend a Codeforces submission.
- **Neovim stays Neovim.** It opens in a tmux or herdr split next to verd (or embedded in the pane, opt-in). `verd test`, `verd submit` and `verd stress` run from inside Neovim and report in verd's pane.
- **Stress testing built in.** `gen` and `brute` Templates per language; `S` finds a counterexample, shows the diff, and `w` saves it as the next Custom Test.
- **Your history, used.** Solved marks, per-topic stats, rating chart, and a Problem Picker with a weak-topics preset.
- **Offline-tolerant.** Everything renders from a local cache; the network refreshes it in the background.
- **Safe submit by default.** Browser handoff copies the Solution, opens the submit page and tracks the Verdict. Direct submit is opt-in and [experimental](./docs/submit.md#direct-mode).

## See it

<p align="center">
  <img src="./assets/readme/screens.svg" width="100%" alt="The Problems tab with solved marks, and the stress counterexample diff overlay.">
</p>

These are real frames rendered by verd's own model (Tokyo Night theme), not mock-ups.

## Install

```sh
go install github.com/moneytosms/verd/cmd/verd@latest
```

Or download a Linux/macOS binary (amd64, arm64) from the [Releases](https://github.com/moneytosms/verd/releases) page once a tag is published. Packages for the AUR (`verd-bin`) and Homebrew (`moneytosms/tap/verd`) are planned, see [#49](https://github.com/moneytosms/verd/issues/49).

You also need: `nvim` for editing, a compiler or interpreter for your languages (`g++`, `gcc`, `python3` by default), and optionally `tmux` or `herdr` for the editor split.

## Quickstart (five minutes)

```sh
verd init                       # writes ~/.config/verd/config.toml and starter Templates
$EDITOR ~/.config/verd/config.toml   # set: handle = "your_codeforces_handle"
verd                            # open the TUI
```

1. **Pick.** On the Problems tab press `enter` on a Problem, or press `4` for the Picker and `w` for your weak topics.
2. **Edit.** Press `e`. Neovim opens beside verd on `~/verd/<contest>/<index>/main.cpp`, created from your Template.
3. **Test.** Press `t` (or just save, `autotest` is on by default). Press `n`/`p` to select a test and `d` for a diff on a failing one.
4. **Add a case.** Press `a` to create the next `custom-N.in` / `.ans` pair.
5. **Stress.** Press `S`. The first time, verd creates `gen.cpp` and `brute.cpp` and opens them; fill them in and press `S` again. It runs until it finds a counterexample, and `w` saves it as a Custom Test.
6. **Submit.** Press `s`. The Solution is copied, the submit page opens, and the Verdict streams into the pane.

Press `?` on any screen for its key list. Full reference: [docs/keys.md](./docs/keys.md).

## Run it from Neovim

verd's TUI is the server. A `verd test`, `verd submit` or `verd stress` started from Neovim talks to the running TUI over a Unix socket and shows up in its pane; with no TUI running it runs headless and prints the result.

```lua
local function verd(cmd)
  vim.cmd("silent write")
  local out = {}
  local function collect(_, data) vim.list_extend(out, data) end
  vim.fn.jobstart({ "verd", cmd, vim.fn.expand("%:p") }, {
    stdout_buffered = true, stderr_buffered = true,
    on_stdout = collect, on_stderr = collect,
    on_exit = function(_, code)
      vim.notify(vim.trim(table.concat(out, "\n")),
        code == 0 and vim.log.levels.INFO or vim.log.levels.WARN, { title = "verd " .. cmd })
    end,
  })
end
vim.keymap.set("n", "<leader>vt", function() verd("test") end, { desc = "verd test" })
vim.keymap.set("n", "<leader>vs", function() verd("submit") end, { desc = "verd submit" })
vim.keymap.set("n", "<leader>vx", function() verd("stress") end, { desc = "verd stress" })
```

More in [docs/neovim.md](./docs/neovim.md).

## Commands

| Command | What it does |
| --- | --- |
| `verd` | Open the TUI. |
| `verd init [--force]` | Write the default config and Templates. |
| `verd config` | Print the effective, merged config. |
| `verd test <file>` | Run Sample and Custom Tests. Exit `0` only if every test is AC. |
| `verd stress [--iter N] [--time S] <file>` | Search for a counterexample. Exit `0` only if none was found. |
| `verd submit <file>` | Submit and track the Verdict. Exit `0` only on Accepted. |
| `verd login` / `verd logout` | Save or delete the browser session used by [direct submit](./docs/submit.md#direct-mode). |
| `verd --version` | Print the version. |

Exit codes: `0` success, `1` the thing you asked about failed (WA, counterexample, rejected), `2` verd could not do it (bad input, build error, offline).

## Documentation

| Page | Covers |
| --- | --- |
| [Configuration](./docs/config.md) | Every config key, languages, Templates, file locations. |
| [Keys](./docs/keys.md) | Key bindings for each screen. |
| [Testing and stress](./docs/testing.md) | Test Runs, Comparison Modes, Custom Tests, stress workflow. |
| [Neovim](./docs/neovim.md) | Split modes and copy-pasteable keymaps. |
| [Submitting](./docs/submit.md) | Browser handoff, direct mode, `verd login`, risks. |
| [Embedded pane](./docs/embedded-pane.md) | Running Neovim inside verd, known gaps, promotion checklist. |
| [Design decisions](./docs/adr) | Why submit defaults to the browser, why a multiplexer split, why no daemon. |
| [Glossary](./CONTEXT.md) | Problem, Solution, Local Verdict, Comparison Mode and the rest. |

## Limits

- Linux and macOS only. Windows is not supported.
- Interactive Problems cannot be run locally; verd tells you so and you can still submit.
- Direct submit is experimental and carries account risk. Read [the risks](./docs/submit.md#direct-mode) before turning it on.
- verd is not affiliated with Codeforces. It reads the public API and statement pages, and respects rate limits.

## Development

```sh
go vet ./... && go test ./...
```

CI runs both on Linux and macOS. Releases are built by GoReleaser when a `v*.*.*` tag is pushed. Issues and specs live in [GitHub Issues](https://github.com/moneytosms/verd/issues).

## License

[MIT](./LICENSE)
