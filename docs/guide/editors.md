# Neovim and other editors

## How the editor opens

`e` opens the solution in `editor` (default `nvim`), at the right place:

| `split` | Result |
| --- | --- |
| `auto` (default) | a tmux or herdr split next to verd when you are inside one, otherwise Neovim drawn inside verd's window |
| `tmux`, `herdr` | force that multiplexer (verd must be running inside it) |
| `embedded` | Neovim inside verd, even in tmux |
| `suspend` | verd hands the whole terminal to the editor and resumes when you quit |

A new split opens without taking focus. `e` again reuses the open Neovim (`:edit` at the right line). GUI editors (`code`, `subl`, `zed`) open their own window.

### The embedded pane

Neovim runs in a pseudo-terminal inside verd. While it has the keyboard everything goes to Neovim; these verd keys always work:

| Key | Does |
| --- | --- |
| `ctrl+\` | hand the keyboard between verd and the editor |
| `alt+h` / `alt+l` | keyboard to the left / right column |
| `alt+z` | zoom whichever side has the keyboard (editor or verd) to fill the window |
| `alt+]` / `alt+[` | grow / shrink the editor column |
| `alt+s` | swap the editor to the other side |

Clicking a column focuses it. Chords that only the Kitty keyboard protocol can send (some `ctrl+shift` combinations) are not passed through; use tmux or `split = "suspend"` if you depend on them. Details: [Embedded pane](../embedded-pane.md).

## Run verd from inside Neovim

`verd test`, `verd stress` and `verd submit` need no plugin, only a job. Put this in `init.lua`:

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

vim.keymap.set("n", "<leader>vt", function() verd("test") end,   { desc = "verd test" })
vim.keymap.set("n", "<leader>vs", function() verd("submit") end, { desc = "verd submit" })
vim.keymap.set("n", "<leader>vx", function() verd("stress") end, { desc = "verd stress" })
```

If the TUI is running the result also shows in its pane; otherwise the command runs headless and prints. Useful one-liners:

```vim
:r !verd snippet dijkstra                    " insert a snippet at the cursor
:'<,'>w !verd snippet add mysnip -           " save the selected lines as a snippet
:!verd editorial --reveal 1 1900/A           " peek at the first hint of a problem
```

## Other editors

Any editor works. verd knows how each takes a line number:

| Editor | Notes |
| --- | --- |
| `nvim` | the one verd integrates deeply (pane reuse, `--listen` socket) |
| `vim`, `hx`, `nano`, `micro`, `emacs`, `kak` | open in the same split or embedded pane; a second `e` replaces the pane |
| `code`, `subl`, `zed` | open their own window; use `"code -w"` to wait |

```sh
verd config set editor "code -w"
```

## LuaSnip

```sh
verd snippet --export --lang cpp > ~/.config/nvim/snippets/verd-cpp.json
```

```lua
require("luasnip.loaders.from_vscode").load_standalone({ path = "~/.config/nvim/snippets/verd-cpp.json", lang = "cpp" })
```

Reference: [Neovim](../neovim.md). Next: [Troubleshooting](./troubleshooting.md).
