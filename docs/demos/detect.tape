# Heramba live-detection demo — render with: vhs docs/demos/detect.tape
# Requires: vhs, ttyd, ffmpeg. Output: docs/demos/detect.gif
# Shows a honeypot-only edge catching a /.env probe as a zero-FP detection.
Output docs/demos/detect.gif

Set Shell bash
Set FontSize 17
Set Width 1200
Set Height 720
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "# Start a honeypot-only edge in the background"
Enter
Sleep 800ms
Type "heramba serve --listen 127.0.0.1:8080 & sleep 1"
Enter
Sleep 2s

Type "# An attacker/agent probes a classic secret path..."
Enter
Sleep 800ms
Type "curl -s -o /dev/null -w 'status=%{http_code}\\n' http://127.0.0.1:8080/.env"
Enter
Sleep 2s

Type "# ...and Heramba logs it as a zero-false-positive decoy hit (conf=1.00)"
Enter
Sleep 2s

Type "kill %1"
Enter
Sleep 1s
