# Neovim runs in a multiplexer pane, not inside verd

verd opens Neovim in a tmux or herdr split next to itself rather than emulating a terminal inside Bubble Tea, because a real pane gives full Neovim fidelity for free. An embedded pane (`creack/pty` + `charmbracelet/x/vt`, pinned commits, missing Kitty keyboard support) is an opt-in fallback outside a multiplexer, and suspend/resume via `tea.ExecProcess` is the last resort. The embedded pane becomes the default only after it passes the promotion checklist.
