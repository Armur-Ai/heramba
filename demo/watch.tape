# Records demo/assets/watch.gif — the live "you're being attacked right now" dashboard.
#
# ⚠ BLOCKED: `heramba watch` (the live TUI) is NOT shipped yet. Build it first
#    (see IMPLEMENTATION_PLAN.md §E + §"FIRST TASKS"): a bubbletea altscreen dashboard
#    with panels — live events · top sources · active campaigns/swarm · decoy map ·
#    mode (observe/protect). Seed it by replaying a recorded attack against the
#    deception app (`heramba serve --deception`) so the gif shows real motion.
#
# Once built, record with:
#   vhs demo/watch.tape

Output demo/assets/watch.gif

Set Shell bash
Set FontSize 14
Set Width 1500
Set Height 860
Set Padding 24
Set Theme { "name": "Heramba", "background": "#0b0e14", "foreground": "#e6e9ef", "cursor": "#3ddc97", "selection": "#16243c", "black": "#0b0e14", "red": "#ff6b6b", "green": "#3ddc97", "yellow": "#f5a524", "blue": "#57c7ff", "magenta": "#a78bfa", "cyan": "#57c7ff", "white": "#e6e9ef", "brightBlack": "#5c6577", "brightGreen": "#5ef0b0", "brightCyan": "#7fd8ff", "brightWhite": "#ffffff" }
Set TypingSpeed 32ms

Hide
Type "export PATH=$PWD/bin:$PATH PS1='$ ' && clear"
Enter
Show

# Start a protect-mode edge with the deception app, replay an attack in the background,
# then open the live dashboard. (Fill in exact commands once `heramba watch` exists.)
Sleep 500ms
Type "# Live defense dashboard — watch an autonomous attacker walk into the decoys."
Enter
Sleep 700ms
Type "heramba watch"
Sleep 500ms
Enter
# -> altscreen TUI: events streaming, sources scoring agentic, a coordinated swarm flagged,
#    decoy/canary hits lighting up, campaign timeline building.
Sleep 20s
