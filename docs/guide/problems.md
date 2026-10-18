# Finding problems

## The Problems tab

Every Codeforces problem in one list with its rating, solver count and tags. `✓` marks a problem you solved and `✗` one you attempted, both taken from your Codeforces submissions. `j`/`k` move, `enter` opens.

### Search

Press `/` and type. It is a live fuzzy search.

```text
wmln          finds Watermelon (letters in order)
water 4a      several words must all match
#dp           matches tags
```

`tab` inside the prompt cycles what is searched: all, name, tag, id. `enter` keeps the search, `esc` clears it.

### Filters

Press `f` for the filter modal (rating range, status, sort, source, tags), or `:` to type a filter expression:

```text
800-1200 +dp -graphs unsolved
```

| Part | Means |
| --- | --- |
| `800-1200`, `800-`, `-1200`, `1500` | rating range or exact rating |
| `+dp` / `-graphs` | include / exclude a tag (prefix match; `_` for a space: `+two_pointers`) |
| `unsolved` or `u` | only problems you have not solved |
| `src:cses` | one source (see below) |

The chips above the list show what is active; `X` clears everything.

## The Picker

Press `4` for a random unsolved problem in your rating band. `r` or `space` re-rolls, `f` changes the filter, `w` picks from your weak topics, `enter` opens it.

```sh
verd daily      # today's pick, from the shell; the same all day
```

## Stats

`3` shows your totals, a rating chart, a difficulty histogram, per-topic strengths, streaks and a solve heatmap. Press `n`/`N` to walk through problems you attempted and did not solve, `enter` opens one, `p` opens the Picker on your weak topics.

## Other sources

verd can show the [CSES Problem Set](https://cses.fi/problemset/) next to Codeforces.

```sh
verd config set source_cses true     # then press ctrl+r in verd to load the tasks
verd sync cses                       # pull in what you already solved
```

CSES tasks appear as `CSES1068` and open, test and stress like any other. See [Sources](../sources.md) for how submitting and solved marks work there.

## Offline

The problem list, statements, your submissions and editorials are cached in `~/.local/share/verd/verd.db`. Without a network verd works from the cache and says "offline"; `ctrl+r` refreshes when you are back.

Next: [Read, edit and test](./solve.md).
