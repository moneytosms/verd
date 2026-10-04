# The TUI process is the server; no daemon

`verd test` / `verd submit` invoked from Neovim talk to the running TUI over a Unix socket and run headless in-process when no TUI is running. We chose this over a background daemon so there is one process lifecycle to manage and nothing left running after the user quits.
