# Headless client verification

`scripts/ptyhost.py` runs an interactive `amux` client inside a pseudo-terminal
and answers the terminal queries the client makes at attach time (`CSI 6n`,
OSC 10/11/12 colour queries, DA). This lets an agent or CI job drive a real
attached client without a human terminal:

```bash
export AMUX_NO_WATCH=1
setsid python3 scripts/ptyhost.py ~/.local/bin/amux -s scratch &
amux -s scratch spawn
amux -s scratch mouse click 2          # goes through the attached client
PTYHOST_LOG=/tmp/frames.ansi python3 scripts/ptyhost.py amux -s scratch   # log raw frames
```

`scripts/e2e/focus-click.sh <binary> <session>` uses it to prove that clicking
an inactive pane whose app enabled mouse tracking focuses the pane without
writing the click into its pty (`client.mouse_focus_click`).
