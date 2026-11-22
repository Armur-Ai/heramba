# Records demo/assets/watch.gif — the live "you're being attacked right now" dashboard.
#
# `heramba watch` is shipped (internal/watch + cmd/heramba/watch.go). This tape
# replays the recorded swarm attack (demo/attack.jsonl) through the real TUI, so
# the gif shows genuine motion with no live edge needed. It walks the full
# dashboard: the live overview, then the Sources / Campaigns / Timeline views,
# then the per-event inspector.
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
Sleep 700ms
# --replay plays the recorded attack paced by its own timestamps (demo).
# In production you'd run:  heramba watch   (it tails the live alert sink)
Type "heramba watch --replay --speed 2 demo/attack.jsonl"
Sleep 300ms
Enter

# Overview: events stream in, three sources cluster into one SWARM, the THREAT
# gauge climbs to CRITICAL, decoy/canary hits light up, the header sparkline
# tracks the rate.
Sleep 7s

# Pause the stream, then tour the focused views.
Type "p"
Sleep 900ms
Type "2"
Sleep 2600ms
Type "3"
Sleep 2600ms
Type "4"
Sleep 2800ms

# Back to the overview and inspect the selected detection.
Type "1"
Sleep 1200ms
Enter
Sleep 2600ms
Escape
Sleep 700ms

# Leave on a clean overview frame, then quit.
Type "q"
Sleep 800ms
