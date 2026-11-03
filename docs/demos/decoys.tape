# Heramba decoys demo — render with: vhs docs/demos/decoys.tape
# Requires: vhs, ttyd, ffmpeg. Output: docs/demos/decoys.gif
Output docs/demos/decoys.gif

Set Shell bash
Set FontSize 18
Set Width 1100
Set Height 700
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "# The deception surface: lures no real client ever touches"
Enter
Sleep 1s

Type "heramba decoys list"
Enter
Sleep 3s

Type "# Mint a canary token to plant in a fake config/secret"
Enter
Sleep 1s

Type "heramba decoys canary db-password"
Enter
Sleep 3s
