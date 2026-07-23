<div align="center">

<img src="assets/logo.png" alt="knowthyself" width="150" height="150" />

### See how you actually collaborate with your AI coding assistant.

knowthyself reads the session logs your AI coding tools already keep on disk and turns them into a clear picture of how you work. Not the code you ship, but the quality of the conversation behind it: your strengths, your habits, and the shape of your collaboration. Everything runs locally. Nothing leaves your machine unless you opt into `--deep-eval` with your own API key.

[![npm](https://img.shields.io/npm/v/knowthyself?color=e08a3c&label=npm)](https://www.npmjs.com/package/knowthyself)
[![license](https://img.shields.io/badge/license-MIT-blue)](LICENSE)
[![built with Go](https://img.shields.io/badge/built%20with-Go-00ADD8)](https://go.dev)

</div>

## Demo

<div align="center">

<img src="assets/demo.gif" alt="knowthyself demo" width="640" />

</div>

## Install

### npm (recommended)

```sh
npm install -g knowthyself
knowthyself
```

Or run it once without installing anything:

```sh
npx knowthyself
```

The package installs the `knowthyself` command. On install it downloads the prebuilt binary for your platform from the GitHub release, so no Go toolchain is required.

### Direct download

Grab a prebuilt binary for your OS and architecture from the [releases page](https://github.com/siddham-jain/knowthyself/releases), unpack the archive, and put `knowthyself` on your PATH.

### From source

Requires Go 1.25 or newer.

```sh
git clone https://github.com/siddham-jain/knowthyself
cd knowthyself
make build   # produces ./knowthyself
```

More package managers (Homebrew, Scoop, winget, AUR) are planned.

## Usage

```sh
knowthyself                # sync, then open the interactive dashboard
knowthyself --json         # print the raw profile as JSON (piped output stays scriptable)
knowthyself --sync         # refresh the local cache, print a summary, then exit
knowthyself --version      # print the version
knowthyself update         # upgrade to the latest release
knowthyself update --check # report whether an update is available, then exit
knowthyself provider       # manage the endpoints --deep-eval can call
```

`update` figures out how the running binary was installed. A plain download is replaced in place, after verifying the archive checksum against the signed release manifest. A binary owned by npm, Homebrew, or `go install` is never touched — the right command for that manager is printed instead, so the two can't fight over the same file. The dashboard footer also shows `▲ <version>` when a newer release exists; that check is cached and refreshed in the background, so it never delays startup. Set `KNOWTHYSELF_NO_UPDATE_CHECK=1` to turn it off.

knowthyself reads `~/.claude` by default. Override the location with `CLAUDE_CONFIG_DIR`. The first time you run it on a real terminal, it asks whether you want to profile before it reads anything. Decline and it exits without touching your history. If there is no Claude history yet, it greets you with a friendly starting screen instead of an error.

On a real terminal the dashboard boots with a short power on animation, then lets you explore:

| Key | Action |
|:---:|:-------|
| `↑` `↓` or `j` `k` | move the cursor (select a dimension or session) |
| `←` `→` or `tab` | switch view |
| `1` `2` `3` `4` | jump to Overview, Sessions, Trends, or Deep Read |
| `r` | replay the boot animation |
| `q` or `esc` | quit |

The layout reflows to your terminal size and never overflows the screen. Piped or non interactive output falls back to a single static frame.

## Screens

**Overview.** A radar of the five dimensions with an evidence inspector. Select any axis to see the exact counts behind its score, plus the matching tip.

<div align="center"><img src="assets/overview.png" alt="Overview screen" width="820" /></div>

**Sessions.** Drill into any single session for its own mini radar and per dimension bars.

<div align="center"><img src="assets/sessions.png" alt="Sessions screen" width="820" /></div>

**Trends.** Chronological sparklines per dimension, so you can answer the one question that matters: am I getting better?

<div align="center"><img src="assets/trends.png" alt="Trends screen" width="820" /></div>

## The five dimensions

Every session is graded on five deterministic dimensions, then averaged into an overall score.

| Dimension | What it measures |
|:----------|:-----------------|
| **Prompt Quality** | Whether your prompts carry concrete signal: file paths, error traces, and specific asks. |
| **Iteration Efficiency** | How directly you reach a result: turns to resolution, clarification loops, and re prompts. |
| **Tool Leverage** | How well you use the platform: tool diversity, sub agents, skills, and slash commands. |
| **Context Management** | Your context hygiene: compaction, `/clear` discipline, and cache reuse. |
| **Token Economy** | How much value you extract per token: cache hit rate, output to input ratio, and tokens per task. |

## Collaboration archetypes

After grading the dimensions, knowthyself names the persona whose signature best matches the shape of your radar. The match uses cosine similarity over the dimensions it actually graded, so the result is deterministic, reproducible, and never rests on a single axis in isolation.

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

Before there is enough history to judge, knowthyself shows a **Newcomer** placeholder and invites you to come back once you have collaborated a bit more.

## How it works

Scores are one hundred percent deterministic heuristics computed on language agnostic structural signals: paths, code fences, error patterns, and turn shape. Prompts written in English, Hindi, or Hinglish are graded on communication quality, not on English proficiency. Every score is explainable, since the underlying counts are retained and shown in the evidence inspector.

## Deep read (optional)

Structural heuristics can tell whether a prompt carried a file path. They cannot tell whether the ask was understandable. `--deep-eval` reads the actual words and grades five things the deterministic scorers are blind to:

| Criterion | What it asks |
|:----------|:-------------|
| **Goal clarity** | Does the prompt state the outcome wanted, or only an action? |
| **Context sufficiency** | Is what the model needs supplied, or assumed? |
| **Constraints & acceptance** | Are boundaries, non goals and done conditions stated? |
| **Scope discipline** | Is this one coherent unit of work? |
| **Correction quality** | When correcting, does the prompt diagnose or merely re assert? |

Each is scored on an anchored 0 to 4 scale with written descriptors for every level, not a bare "rate this out of ten", because unanchored scales do not agree between models or between runs. Results appear in their own **Deep Read** tab, labelled with the model and sample size, and are never averaged into your radar. Your five dimension scores stay deterministic and entirely local whether or not you use this.

### Bring any provider

Register an endpoint once, then select it by name. Anything speaking the OpenAI or Anthropic wire format works, including a model running on your own machine.

```sh
knowthyself provider add          # guided: pick a starting point, then edit every field
knowthyself provider list         # what's configured, and which is the default
knowthyself provider test         # send one tiny request to check it works
knowthyself provider use groq     # change the default
knowthyself provider edit groq    # change any field, base URL included
```

`provider add` opens a picker (Anthropic, OpenAI, OpenRouter, Groq, Together, DeepSeek, Ollama, LM Studio, or anything else) and then a form where the **name, base URL, model, dialect and credential are all editable** — presets only save you a documentation lookup, they never lock a value down. Local endpoints need no key at all.

Credentials can be stored in the config file (written `0600`) or, better, read from an environment variable you name — in which case the secret never touches disk.

```sh
knowthyself --deep-eval                    # uses your default provider
knowthyself --deep-eval --provider ollama  # use a specific one
```

One-off overrides without saving anything: `--api-key`, `--base-url`, `--model`, `--api-dialect`. Environment: `KNOWTHYSELF_API_KEY`, `KNOWTHYSELF_BASE_URL`, `KNOWTHYSELF_MODEL`, `KNOWTHYSELF_API_DIALECT`; `ANTHROPIC_API_KEY` and `OPENAI_API_KEY` are honoured as fallbacks, and `ANTHROPIC_AUTH_TOKEN` is sent as an OAuth bearer token. Precedence is flags, then environment, then the selected provider, then defaults.

Note a **Claude Code subscription login is not an API key** and cannot authenticate the API — deep-eval needs a key from the console, or an OAuth token.

**Before anything is sent**, a consent screen shows the endpoint, the model, the exact volume, and a token estimate, and lets you page through the precise redacted text that would leave your machine. Nothing is transmitted until you accept, consent is remembered per endpoint and model, and there is no way to send anything without a terminal to approve it in.

Secrets, absolute paths, emails, URLs, IPs and opaque blobs are stripped first. Every judgment the model returns must quote text that appears verbatim in the prompt it is grading; anything that fails that check is discarded, which is what stops a model inventing evidence. Results are cached against the exact sample, so re running costs nothing. If the endpoint is unreachable, the key is rejected, or the model cannot follow the schema, you get a specific message and a remedy, and the dashboard still renders from the local scores.

See [`docs/DEEP_EVAL.md`](docs/DEEP_EVAL.md) for the full architecture.

## Contributing

Issues and pull requests are welcome. Build and test locally with:

```sh
make build   # produces ./knowthyself
make test    # unit and golden tests
```

## License

MIT. See [LICENSE](LICENSE).
