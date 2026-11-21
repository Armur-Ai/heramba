# Records demo/assets/watch.gif — the live "you're being attacked right now" dashboard.
#
# `heramba watch` is shipped (internal/watch + cmd/heramba/watch.go). This tape
# replays the recorded swarm attack (demo/attack.jsonl) through the real TUI, so
# the gif shows genuine motion with no live edge needed.
#
# Record with:   vhs demo/watch.tape
# (run from the repo root; it builds bin/heramba itself)

Output demo/assets/watch.gif

Set Shell bash
Set FontSize 14
Set Width 1500
Set Height 900
Set Padding 24
Set Theme { "name": "Heramba", "background": "#0b0e14", "foreground": "#e6e9ef", "cursor": "#3ddc97", "selection": "#16243c", "black": "#0b0e14", "red": "#ff6b6b", "green": "#3ddc97", "yellow": "#f5a524", "blue": "#57c7ff", "magenta": "#a78bfa", "cyan": "#57c7ff", "white": "#e6e9ef", "brightBlack": "#5c6577", "brightGreen": "#5ef0b0", "brightCyan": "#7fd8ff", "brightWhite": "#ffffff" }
Set TypingSpeed 32ms

Hide
Type "go build -o bin/heramba ./cmd/heramba && export PATH=$PWD/bin:$PATH PS1='$ ' && clear"
Enter
Sleep 2s
Show

Sleep 500ms
Type "# Live defense dashboard — watch an autonomous swarm walk into the decoys."
Enter
Sleep 800ms
# --replay plays the recorded attack paced by its own timestamps (demo).
# In production you'd run:  heramba watch   (it tails the live alert sink)
Type "heramba watch --replay --speed 1 demo/attack.jsonl"
Sleep 400ms
Enter

# Altscreen TUI: events stream in, three sources cluster into one SWARM,
# decoy/canary hits light up, egress callbacks get blocked, tallies climb.
Sleep 13s

# Leave on the final frame (stream ended, totals + SWARM visible), then quit.
Type "q"
Sleep 800ms
