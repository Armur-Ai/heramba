# Heramba four-baseline experiment demo — render with: vhs docs/demos/experiment.tape
# Requires: vhs, ttyd, ffmpeg. Output: docs/demos/experiment.gif
Output docs/demos/experiment.gif

Set Shell bash
Set FontSize 18
Set Width 1100
Set Height 760
Set Padding 20
Set Theme "Catppuccin Mocha"

Type "# Reproducible evaluation: four defenses, identical seeded traffic"
Enter
Sleep 1500ms

Type "heramba experiment --trials 20"
Enter
Sleep 4s

Type "# No defense lets the attacker finish; deception contains it at zero FP."
Enter
Sleep 2s

Type "# Adaptive detects earlier than fixed — measured, not asserted."
Enter
Sleep 2s

Type "# Same seeds always reproduce the same report:"
Enter
Sleep 1s

Type "heramba experiment --trials 20 --json | shasum | cut -c1-16"
Enter
Sleep 3s

Type "heramba experiment --trials 20 --json | shasum | cut -c1-16"
Enter
Sleep 3s
