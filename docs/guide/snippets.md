# Snippets and Templates

Two different things live in `~/.config/verd/snippets/` (change it with `snippets_dir`).

| | What | Used |
| --- | --- | --- |
| **Template** | the starting text of a new solution, one per language | automatically, when you press `e` the first time |
| **Snippet** | a reusable piece of code (bfs, dijkstra, modpow) | only when you ask |

## Templates

`templates/<lang>.<ext>` is a Go `text/template`. Variables: `{{.Problem.ID}}`, `{{.Problem.Name}}`, `{{.Problem.URL}}`, `{{.Handle}}`, `{{.Contest}}`, `{{.Index}}`, `{{.Date}}`, `{{.Time}}`, `{{.DateTime}}`. `{{cursor}}` marks where the editor cursor starts.

```cpp
// {{.Problem.ID}} {{.Problem.Name}}
// {{.Problem.URL}}
#include <bits/stdc++.h>
using namespace std;

int main() {
    ios::sync_with_stdio(false);
    cin.tie(nullptr);
    {{cursor}}
}
```

Edit one from Settings (`5`) > **Snippets**, or open the file yourself. `gen.<lang>.<ext>` and `brute.<lang>.<ext>` are the [stress](./stress.md) helpers' Templates. A language's `template = "/path"` in `[lang.*]` overrides all of this.

## The snippet library

verd ships 35 snippets for C++, C and Python: input readers (`vecinput`, `matinput`, `gridinput`, `graphinput`), `prefix_sum`, `prefix_sum_2d`, `diff_array`, `modpow`, `modinv`, `extgcd`, `sieve`, `spf_factorize`, `miller_rabin`, `divisors`, `ncr_mod`, `matrix_pow`, `bfs`, `grid_bfs`, `dfs`, `dijkstra`, `topo_sort`, `floyd_warshall`, `dsu`, `fenwick`, `segment_tree`, `sparse_table`, `binary_search`, `lis`, `compress`, `prefix_function`, `z_function`, `string_hash`, `debug`.

### Use one

- In the Problem view press `i`: type to filter, `tab` switches language, `enter` copies the snippet to the clipboard. Paste it into your editor.
- From the shell: 

```sh
verd snippet --list                  # names and descriptions (C++)
verd snippet --lang python --list
verd snippet dijkstra                # print it
verd snippet --copy modpow           # to the clipboard
```

In Neovim, `:r !verd snippet dijkstra` inserts it at the cursor. A unique prefix is enough (`verd snippet dijk`).

### Add your own

```sh
verd snippet add --desc "double it" twice twice.cpp     # from a file
echo 'x = 1' | verd snippet add --lang python xone -    # from stdin
verd snippet add --desc "my helper" helper              # no source: a stub opens in your editor
```

In Neovim, `:'<,'>w !verd snippet add mysnip -` saves the selected lines.

| Command | Does |
| --- | --- |
| `verd snippet edit <name>` | edit one (a built-in gets an editable copy first) |
| `verd snippet rm <name>` | remove your file; a built-in of that name comes back |
| `verd snippet open` | open the library folder in your editor |
| `verd snippet path` | print the snippets directory |

A snippet is a file `library/<lang>/<name>.<ext>`; the comment on its first line is its description. A file with a built-in's name replaces it. They are plain code, with no `{{...}}` variables.

### For LuaSnip or VSCode

```sh
verd snippet --export --lang cpp > ~/.config/nvim/snippets/verd-cpp.json
```

```lua
require("luasnip.loaders.from_vscode").load_standalone({ path = "~/.config/nvim/snippets/verd-cpp.json", lang = "cpp" })
```

Re-export after you edit the library.

### In C: graphs

`bfs`, `dfs`, `dijkstra` and `topo_sort` build on `graphinput` (edge arrays); paste that one first. Everything else is self-contained.

Next: [Customize](./customize.md). Reference: [Configuration](../config.md#snippets).
