# Assets

Media referenced by the root `README.md`.

| File | Purpose |
|:-----|:--------|
| `logo.png` | The logo shown at the top of the README. |
| `demo.gif` | The demo shown under "Demo". Keep it small (a few MB) so the README stays fast; compress with ffmpeg or gifsicle before committing. |
| `overview.png` | The Overview screen (radar plus evidence inspector). |
| `sessions.png` | The Sessions screen (per session mini radar and bars). |
| `trends.png` | The Trends screen (sparklines per dimension). |

## Capturing clean terminal screenshots

For crisp, consistent captures of the TUI, [VHS](https://github.com/charmbracelet/vhs) is the tool of choice (`brew install vhs`). It scripts a terminal session and exports PNGs, GIFs, and MP4s at a fixed size, so every screen looks uniform.
