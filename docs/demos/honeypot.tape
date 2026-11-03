# Heramba honeypot demo — render with: vhs docs/demos/honeypot.tape
# Requires: vhs, ttyd, ffmpeg. Output: docs/demos/honeypot.gif
Output docs/demos/honeypot.gif

Set Shell bash
Set FontSize 18
Set Width 1100
Set Height 780
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "# A live honeypot: edge decoys in front, a coherent synthetic app behind"
Enter
Sleep 1500ms

Type "heramba honeypot --listen 127.0.0.1:8080 --seed 7 &"
Enter
Sleep 2s

Type "# An agent enumerates the surface and trips a decoy..."
Enter
Sleep 1s

Type "curl -s -o /dev/null -w '%{http_code}\\n' http://127.0.0.1:8080/api/users/1"
Enter
Sleep 1500ms

Type "curl -s -o /dev/null -w '%{http_code}\\n' http://127.0.0.1:8080/.env   # decoy"
Enter
Sleep 1500ms

Type "curl -s -o /dev/null -w '%{http_code}\\n' http://127.0.0.1:8080/.git/config   # decoy"
Enter
Sleep 1500ms

Type "# Stop the honeypot and read the intelligence rollup:"
Enter
Sleep 1s

Type "kill -INT %1"
Enter
Sleep 3s
