# reflect

A developer-first CLI that reads the session logs your AI coding assistants already
write to disk and shows you **how you actually collaborate with AI** — not the code
you ship, but the quality of your communication.

`reflect` grades you across five deterministic dimensions, renders a radar chart and a
collaboration archetype in the terminal, and surfaces concrete, actionable tips.
Everything runs locally; nothing leaves your machine unless you opt into `--deep-eval`
with your own API key.

## Install

**macOS / Linux** — universal one-liner (detects your OS/arch, verifies the checksum):

```sh
curl -fsSL https://<domain>/install.sh | sh
```

**Homebrew** (macOS + Linux):

```sh
brew install siddham/tap/reflect
```

**Windows** — PowerShell:

```powershell
irm https://<domain>/install.ps1 | iex
```

…or `winget install Siddham.reflect` · `scoop install reflect`

**Arch (AUR):** `yay -S reflect-bin`

**From source** (needs Go 1.25+):

```sh
go install github.com/siddham/reflect/cmd/reflect@latest
```

> Prebuilt binaries, Homebrew/Scoop/winget/AUR manifests, and checksums are produced
> by [GoReleaser](https://goreleaser.com) on each tagged release (see `.goreleaser.yaml`).
> Replace `<domain>` once the install scripts are hosted.

## First run

`reflect` reads `~/.claude` (override with `CLAUDE_CONFIG_DIR`), profiles your history
locally in a few seconds, and opens the dashboard on the persona **Reveal**. No Claude
history yet? It'll greet you instead of erroring.

## Dimensions

- **Prompt Quality** — do you give file paths, error traces, concrete asks?
- **Iteration Efficiency** — turns-to-resolution, clarification loops, re-prompts.
- **Tool Leverage** — tool diversity, sub-agents, skills, slash commands.
- **Context Management** — compaction hygiene, `/clear` discipline, cache reuse.
- **Token Economy** — cache-hit rate, output/input ratio, tokens per task.

## Usage

```sh
reflect            # sync + open the interactive TUI dashboard
reflect --json     # emit the raw Profile (piped/non-TTY also renders a static frame)
reflect --sync     # refresh the local cache and print a summary, then exit
reflect --version  # print version
```

Build from a checkout:

```sh
make build   # -> ./reflect
make test    # unit + golden tests
```

## Interactive dashboard

On a terminal, `reflect` opens an industrial-instrument dashboard that boots with a
short power-on animation, then lets you explore:

| Key | Action |
|-----|--------|
| `↑ ↓` / `j k` | move the cursor (select a dimension or session) |
| `← →` / `tab` | switch view |
| `1 2 3` | jump to Overview / Sessions / Trends |
| `r` | replay the boot animation |
| `q` / `esc` | quit |

- **Overview** — radar + an *evidence inspector*: select any axis to see the exact
  counts behind its score (auditable by design) plus the matching tip.
- **Sessions** — drill into any session for its own mini-radar and per-dimension bars.
- **Trends** — chronological sparklines per dimension ("am I improving?") + a
  session timeline.

The layout reflows to the terminal size and never overflows the screen; piped or
non-interactive output falls back to a single static frame.

## Design

Fair and true by construction: scores are 100% deterministic heuristics, computed on
**language-agnostic structural signals** (paths, code fences, error patterns,
turn-shape) so prompts in English, Hindi, or Hinglish are graded on communication
quality, not English proficiency. Every score is explainable — the underlying counts
are retained and shown.
