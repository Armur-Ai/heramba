# Terminal demos

These are [VHS](https://github.com/charmbracelet/vhs) tapes — the **source** for
Heramba's terminal demo GIFs. They're committed as code so the demos are
reproducible and reviewable; the rendered `.gif` files are git-ignored.

## Render

Install VHS and its runtime deps, then:

```bash
# deps: vhs, ttyd, ffmpeg  (e.g. `brew install vhs` pulls them)
make demos            # renders every tape in this directory
# or one at a time:
vhs docs/demos/quickstart.tape
```

## Tapes

| Tape | Shows |
|---|---|
| `quickstart.tape` | `config init` → `doctor` → `version` (observe mode in 30s) |
| `decoys.tape` | `decoys list` + minting a canary token |
| `detect.tape` | a honeypot edge catching a `/.env` probe as a zero-FP hit |

Build the binary first (`make build`) and ensure `heramba` is on `PATH` so the
tapes invoke the real CLI.
