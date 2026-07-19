<div align="center">

<img src="assets/logo.png" alt="reflect" width="340" />

### See how you actually collaborate with your AI coding assistant.

reflect reads the session logs your AI coding tools already keep on disk and turns them into a clear picture of how you work. Not the code you ship, but the quality of the conversation behind it: your strengths, your habits, and the shape of your collaboration.

[![npm](https://img.shields.io/npm/v/knowthyself?color=e08a3c&label=npm)](https://www.npmjs.com/package/knowthyself)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![built with Go](https://img.shields.io/badge/built%20with-Go-00ADD8)](https://go.dev)

</div>

## What it does

Every time you work with an AI coding assistant, it writes a full transcript to disk. reflect reads those transcripts locally, grades five dimensions of how you collaborate, and renders the result as an interactive terminal dashboard: a radar chart, a collaboration archetype that names your style, and concrete tips for getting more out of the tool.

Everything runs on your machine. Nothing leaves it unless you explicitly opt into `--deep-eval` with your own API key.

## Demo

<!--
  Add the demo video here. A short edited clip with music works well.
  Two easy options:
    1. Drag an .mp4 into a GitHub issue or PR comment, then paste the
       generated https://github.com/user-attachments/... link below
       (GitHub plays uploaded mp4s inline).
    2. Save a GIF at assets/demo.gif and reference it:
       <img src="assets/demo.gif" alt="reflect demo" width="820">
-->

> Demo video coming soon.

## Screens

Screenshots of each screen will live here. Save the images under `assets/` and uncomment the tags below.

### The Reveal

The first thing you see. The boot animation resolves into a persona portrait: your collaboration archetype, your numbers, and your trait badges.

<!-- <img src="assets/reveal.png" alt="The Reveal screen" width="820"> -->

> Screenshot coming soon.

### Overview

A radar of the five dimensions with an evidence inspector. Select any axis to see the exact counts behind its score, plus the matching tip.

<!-- <img src="assets/overview.png" alt="Overview screen" width="820"> -->

> Screenshot coming soon.

### Sessions

Drill into any single session for its own mini radar and per dimension bars.

<!-- <img src="assets/sessions.png" alt="Sessions screen" width="820"> -->

> Screenshot coming soon.

### Trends

Chronological sparklines per dimension, so you can answer the one question that matters: am I getting better?

<!-- <img src="assets/trends.png" alt="Trends screen" width="820"> -->

> Screenshot coming soon.

## Install

### npm (recommended)

```sh
npm install -g knowthyself
reflect
```

Or run it once without installing anything:

```sh
npx knowthyself
```

The package installs the `reflect` command. On install it downloads the prebuilt binary for your platform from the GitHub release, so no Go toolchain is required.

### Direct download

Grab a prebuilt binary for your OS and architecture from the [releases page](https://github.com/siddham-jain/reflect/releases), unpack the archive, and put `reflect` on your PATH.

### From source

Requires Go 1.25 or newer.

```sh
git clone https://github.com/siddham-jain/reflect
cd reflect
make build   # produces ./reflect
```

More package managers (Homebrew, Scoop, winget, AUR) are planned.

## First run

reflect reads `~/.claude` by default. Override the location with `CLAUDE_CONFIG_DIR`.

The first time you run it on a real terminal, reflect asks whether you want to profile now before it reads anything. Choose to see your results, or decline and it exits without touching your history. After that first prompt it goes straight to the dashboard.

If there is no Claude history yet, reflect greets you with a friendly starting screen instead of an error.

## Usage

```sh
reflect            # sync, then open the interactive dashboard
reflect --json     # print the raw profile as JSON (piped output stays scriptable)
reflect --sync     # refresh the local cache, print a summary, then exit
reflect --version  # print the version
```

On a real terminal, reflect opens a dashboard that boots with a short power on animation and then lets you explore:

| Key | Action |
|:---:|:-------|
| `↑` `↓` or `j` `k` | move the cursor (select a dimension or session) |
| `←` `→` or `tab` | switch view |
| `1` `2` `3` | jump to Overview, Sessions, or Trends |
| `r` | replay the boot animation |
| `q` or `esc` | quit |

The layout reflows to your terminal size and never overflows the screen. Piped or non interactive output falls back to a single static frame.

## The five dimensions

reflect grades every session on five deterministic dimensions and averages them into an overall score.

| Dimension | What it measures |
|:----------|:-----------------|
| **Prompt Quality** | Whether your prompts carry concrete signal: file paths, error traces, and specific asks. |
| **Iteration Efficiency** | How directly you reach a result: turns to resolution, clarification loops, and re prompts. |
| **Tool Leverage** | How well you use the platform: tool diversity, sub agents, skills, and slash commands. |
| **Context Management** | Your context hygiene: compaction, `/clear` discipline, and cache reuse. |
| **Token Economy** | How much value you extract per token: cache hit rate, output to input ratio, and tokens per task. |

Every score is explainable. reflect keeps the underlying counts and shows them in the evidence inspector, so no number is a black box.

## Collaboration archetypes

After grading the dimensions, reflect names the persona whose signature best matches the shape of your radar. The match uses cosine similarity over the dimensions it actually graded, so the result is deterministic, reproducible, and never rests on a single axis in isolation.

| Archetype | Signature |
|:----------|:----------|
| **Architect** | You plan in precise strokes. Grounded briefs and clean context, with the work specced before a line is written. |
| **Surgeon** | Precise, minimal incisions. You say exactly what is needed and land it in a few clean moves. |
| **Conductor** | You direct a whole orchestra of tools, agents, and servers, delegating the work in concert. |
| **Pathfinder** | You map unknown territory by probing. You read, search, and trace until its shape appears. |
| **Economist** | You extract maximum signal per token. Cache savvy, context stable, and quietly cost lean. |
| **Marathoner** | You go deep for hours and keep context clean the whole way. Sustained, disciplined sessions. |
| **Conversationalist** | You think out loud with AI. Fast, iterative, exploratory. You get there by dialogue, not by spec. |
| **Generalist** | No single mode. You are fluent across the board and adapt to whatever the task needs. |

Before there is enough history to judge, reflect shows a **Newcomer** placeholder and invites you to come back once you have collaborated a bit more.

## How it works

reflect is fair and true by construction. Scores are one hundred percent deterministic heuristics computed on language agnostic structural signals: paths, code fences, error patterns, and turn shape. That means prompts written in English, Hindi, or Hinglish are graded on communication quality, not on English proficiency.

The local cache lives next to your config so repeat runs are fast. The optional `--deep-eval` flag uses your own API key to phrase qualitative tips more naturally. It never changes a score.

## Contributing

Issues and pull requests are welcome. Build and test locally with:

```sh
make build   # produces ./reflect
make test    # unit and golden tests
```

## License

MIT. See [LICENSE](LICENSE).
