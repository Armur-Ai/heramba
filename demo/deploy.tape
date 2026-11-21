# Records demo/assets/deploy.gif — "deploy in minutes, protected the same afternoon".
# Shows: drop Heramba in front of an app, observe mode (safe), and a decoy firing on the first probe.
#
# Prereqs (from repo root):
#   make build            # -> ./bin/heramba   (or: go install .../cmd/heramba@latest)
# Then:
#   vhs demo/deploy.tape
#
# Docker variant (once the image is published — see IMPLEMENTATION_PLAN §A):
#   docker run -p 8080:8080 -e UPSTREAM=http://app:3000 -e MODE=observe armur/heramba

Output demo/assets/deploy.gif

Set Shell bash
Set FontSize 14
Set Width 1500
Set Height 820
Set Padding 24
# Heramba identity: terminal void + green + cyan
Set Theme { "name": "Heramba", "background": "#0b0e14", "foreground": "#e6e9ef", "cursor": "#3ddc97", "selection": "#16243c", "black": "#0b0e14", "red": "#ff6b6b", "green": "#3ddc97", "yellow": "#f5a524", "blue": "#57c7ff", "magenta": "#a78bfa", "cyan": "#57c7ff", "white": "#e6e9ef", "brightBlack": "#5c6577", "brightGreen": "#5ef0b0", "brightCyan": "#7fd8ff", "brightWhite": "#ffffff" }
Set TypingSpeed 32ms

Hide
Type "export PATH=$PWD/bin:$PATH PS1='$ ' && clear"
Enter
Show

Sleep 500ms
Type "# 1. Point Heramba in front of your app. Observe mode is safe in production."
Enter
Sleep 700ms
Type "heramba config init"
Sleep 400ms
Enter
Sleep 1.5s

Type "# 2. Run the edge. It forwards real traffic to your app, untouched."
Enter
Sleep 600ms
Type "heramba serve --listen 127.0.0.1:8080 --upstream http://127.0.0.1:3000 &"
Sleep 500ms
Enter
Sleep 2s

Type "# 3. An attacker (or an autonomous agent) probes for secrets..."
Enter
Sleep 700ms
Type "curl -s -o /dev/null http://127.0.0.1:8080/.env"
Sleep 500ms
Enter
# -> prints: high decoy_hit src=127.0.0.1 target="/.env" conf=1.00 — zero false positives
Sleep 2.5s

Type "# One container. Observe mode. Protected the same afternoon."
Enter
Sleep 2.5s
