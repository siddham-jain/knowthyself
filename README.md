# synch

A developer-first CLI that reads the session logs your AI coding assistants already
write to disk and builds a profile of **how you collaborate with AI** — not the code
you ship, but the quality of your communication.

`synch` grades you across five deterministic dimensions, renders a radar chart and a
collaboration archetype in the terminal, and surfaces concrete, actionable tips.

## Status

v1 in development. Source: **Claude Code** (`~/.claude/projects/*/*.jsonl`).
Deterministic, local, private — nothing leaves your machine unless you opt into
`--deep-eval` with your own API key.

## Dimensions

- **Prompt Quality** — do you give file paths, error traces, concrete asks?
- **Iteration Efficiency** — turns-to-resolution, clarification loops, re-prompts.
- **Tool Leverage** — tool diversity, sub-agents, skills, slash commands.
- **Context Management** — compaction hygiene, `/clear` discipline, cache reuse.
- **Token Economy** — cache-hit rate, output/input ratio, tokens per task.

## Build

```
make build     # -> ./synch
make test      # unit + golden tests
./synch        # sync + open the interactive TUI dashboard
./synch --json # emit the raw Profile (piped/non-TTY also renders a static frame)
```

## Interactive dashboard

On a terminal, `synch` opens an industrial-instrument dashboard that boots with a
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

Piped or non-interactive output falls back to a single static frame.

## Design

Fair and true by construction: scores are 100% deterministic heuristics, computed on
**language-agnostic structural signals** (paths, code fences, error patterns,
turn-shape) so prompts in English, Hindi, or Hinglish are graded on communication
quality, not English proficiency. Every score is explainable — the underlying counts
are retained and shown.
