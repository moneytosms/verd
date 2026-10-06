# Neovim and other editors

Neovim is the default and gets the most (pane reuse, `verd test` from inside it). Set `editor` in Settings or `config.toml` to use another: `vim`, `hx`, `nano`, `micro`, `emacs`, `kak`, `code`, `subl`, `zed` or any command. verd passes the line number the way each editor expects. Other terminal editors open in the same split but a second `e` replaces the pane; GUI editors (`code`, `subl`, `zed`) open their own window.

verd edits Solutions in Neovim (`nvim` must be on your `PATH`). How the editor appears is the `split` setting.

## Split modes

| `split` | Behavior |
| --- | --- |
| `auto` (default) | tmux if you are in tmux, else herdr if you are in herdr, else suspend. |
| `tmux` / `herdr` | Force that multiplexer. Neovim opens in a split pane next to verd. verd must be running inside it; otherwise it falls back to `suspend`. Changing `split` in Settings applies at once. |
| `suspend` | verd hands the terminal to Neovim and resumes when you quit. |
| `embedded` | Neovim is drawn inside verd's own window. Opt-in. See [Embedded pane](./embedded-pane.md). |

A new split opens without taking focus. Pressing `e` again reuses the open Neovim (`:edit` at the right line) instead of opening a second pane, and moves focus to it (in herdr by focusing the pane to the right of verd's, as herdr can only focus by direction). verd talks to it over a `--listen` socket in the runtime directory.

## Commands from inside Neovim

`verd test`, `verd submit` and `verd stress` take a Solution path. If the verd TUI is running they are handed to it and the run appears in its pane; otherwise they run headless and print. Either way the exit code is `0` on success, `1` on a failing result, `2` on an error.

You do not need any plugin. Plain jobs are enough.

### Copy-paste keymaps

Put this in `init.lua` or a file under `lua/config/`. It saves the file, runs the command in the background, and reports the output as a notification (a warning when the exit code is not `0`).

```lua
local function verd(cmd)
  vim.cmd("silent write")
  local out = {}
  local function collect(_, data) vim.list_extend(out, data) end
  vim.fn.jobstart({ "verd", cmd, vim.fn.expand("%:p") }, {
    stdout_buffered = true,
    stderr_buffered = true,
    on_stdout = collect,
    on_stderr = collect,
    on_exit = function(_, code)
      vim.notify(
        vim.trim(table.concat(out, "\n")),
        code == 0 and vim.log.levels.INFO or vim.log.levels.WARN,
        { title = "verd " .. cmd }
      )
    end,
  })
end

vim.keymap.set("n", "<leader>vt", function() verd("test") end,   { desc = "verd test" })
vim.keymap.set("n", "<leader>vs", function() verd("submit") end, { desc = "verd submit" })
vim.keymap.set("n", "<leader>vx", function() verd("stress") end, { desc = "verd stress" })
```

`verd submit` stays running until the Verdict is decided, so the notification arrives when judging finishes.

### Prefer a terminal split?

```lua
vim.keymap.set("n", "<leader>vT", function()
  vim.cmd("silent write")
  vim.cmd("botright 12split | terminal verd test " .. vim.fn.shellescape(vim.fn.expand("%:p")))
end, { desc = "verd test (terminal)" })
```

## Notes

- Only one TUI runs per user; a second `verd` refuses to start and names the socket.
- The Solution path must be inside your `workspace`. Outside it, `verd test` exits `2`.
- With no TUI running, the commands still work, so the same keymaps are usable in a plain Neovim session.
