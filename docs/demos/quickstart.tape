# Heramba quickstart demo — render with: vhs docs/demos/quickstart.tape
# Requires: vhs, ttyd, ffmpeg. Output: docs/demos/quickstart.gif
Output docs/demos/quickstart.gif

Set Shell bash
Set FontSize 18
Set Width 1100
Set Height 640
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "# Heramba — stand up an observe-mode sensor in 30 seconds"
Enter
Sleep 1s

Type "heramba config init"
Enter
Sleep 1500ms

Type "heramba doctor"
Enter
Sleep 2s

Type "# observe mode is safe: detection only, nothing inline"
Enter
Sleep 1s

Type "heramba version"
Enter
Sleep 2s
