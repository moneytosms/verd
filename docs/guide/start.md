# Quick start

From nothing to a submitted solution in about five minutes.

## 1. Install

```sh
curl -fsSL https://raw.githubusercontent.com/moneytosms/verd/main/install.sh | sh
verd --version
```

Other ways (`go install`, tarballs) are on the [Install](https://moneytosms.github.io/verd/install.html) page. You also need a compiler or interpreter for the language you solve in (`g++`, `gcc` or `python3` by default) and, for editing, `nvim` (any editor works).

## 2. Set up

```sh
verd init            # writes config.toml, Templates and the snippet library
verd doctor          # checks compilers, editor and terminal
```

`verd init` asks for your Codeforces handle, the only required setting. To skip the questions:

```sh
verd init --no-setup
verd config set handle tourist
```

## 3. Open verd

```sh
verd
```

The first start downloads the problem list, then works offline from its cache. Five tabs sit along the top, switched with `1` to `5`:

| Tab | What it is |
| --- | --- |
| `1` Problems | every problem, with search, filters and solved marks |
| `2` Contests | contests, and timed [sessions](./contests.md) |
| `3` Stats | your rating, topics and streaks |
| `4` Picker | a random unsolved problem in your rating band |
| `5` Settings | every option, live |

Press `?` on any screen for its keys, or `ctrl+k` to fuzzy-find and run any action, setting or snippet.

## 4. Solve one problem

1. On Problems press `/`, type `water`, press `enter` to keep the search, then `enter` again to open a problem.
2. Read the statement on the left. The right side lists the sample tests.
3. Press `e`. verd creates `main.cpp` from your Template and opens Neovim next to verd.
4. Write the solution and save. With `autotest` on (default) the tests run by themselves; or press `t`.
5. All `AC`? Press `s`. verd copies the solution, opens the submit page and shows the verdict as it arrives.

```text
~/verd/1900/A/
├── main.cpp          your solution
├── tests/            sample-N and custom-N .in/.ans files
└── notes.md          your notes (N in the Problem view)
```

## What next

- [Finding problems](./problems.md): search, filters, the Picker and your stats.
- [Read, edit and test](./solve.md): the everyday loop in detail.
- [Stress testing](./stress.md): when a solution passes the samples and still fails.
- [Customize](./customize.md): themes, shortcuts, layout.
