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
./synch        # sync + open the TUI dashboard
./synch --json # emit the raw Profile
```

## Design

Fair and true by construction: scores are 100% deterministic heuristics, computed on
**language-agnostic structural signals** (paths, code fences, error patterns,
turn-shape) so prompts in English, Hindi, or Hinglish are graded on communication
quality, not English proficiency. Every score is explainable — the underlying counts
are retained and shown.
