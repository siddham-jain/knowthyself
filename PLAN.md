# synch — Implementation Plan (v1)

## Context

**synch** is a developer-first CLI that reads the session logs AI coding assistants already write to disk and builds a profile of *how* a developer collaborates with AI — not the code they ship, but the quality of their communication. It grades the developer on five dimensions, renders a radar chart + collaboration archetype in the terminal, and surfaces concrete, actionable tips.

The value of the tool lives or dies on one thing: **the analysis engine must be fair and true.** A grade that is wrong, unreproducible, or biased against how a real person actually types (short prompts, pasted commands, Hindi/Hinglish/English code-switching, half-finished sessions) destroys trust instantly. So v1 is deliberately narrow in surface area and deep in robustness.

**v1 is narrow; the architecture is not.** The code must be structured so that new *providers* (Codex, OpenCode, Gemini…), new *scoring rubrics*, new *output consumers* (a future recruiter flow where candidates submit a portable report for grading), and new *rendering surfaces* (a landing page / web view) can be added **without touching the core.** Extensibility is achieved by depending on interfaces at every seam and keeping a pure, serializable `Profile` as the contract between computation and presentation. None of the future use-cases ship in v1 — but nothing in v1 may block them.

### Decisions locked (from discussion)
- **Source, v1:** **Claude Code only.** Codex CLI / OpenCode / Gemini CLI become later adapters behind the same normalized model. (Their storage was researched and is documented at the end for future reference.)
- **Surface:** **Terminal-only TUI.** No web dashboard. No shareable PNG/SVG card (explicitly cut). One render path.
- **Grading:** **100% deterministic heuristics** produce every numeric/radar score — reproducible, private, instant. An **optional `--deep-eval`** (bring-your-own API key, off by default) writes *qualitative prose tips only* and can never move a score.
- **Stack:** **Go** + Charm (Bubble Tea / Lip Gloss / ntcharts) + pure-Go SQLite (`modernc.org/sqlite`) + GoReleaser. Single static binary, <50ms boot, no cgo.
- **Prime directive:** robustness and fairness. Cover the edge cases. Be language-agnostic.

---

## Architecture (Go packages)

```
cmd/synch/main.go            # CLI: `synch` (sync + TUI), `synch sync`, `synch export`(future), flags (--deep-eval, --json, --since)
internal/provider/           # Provider interface + registry; provider/claude implements Discover+Parse
internal/model/              # unified Session → Turn → ToolCall (source-agnostic normalized model)
internal/store/              # Repository interface over SQLite; delta-sync by mtime/size; aggregates
internal/text/               # language-agnostic feature extraction (paths, code, errors, script detect)
internal/score/              # Scorer interface + registry; 5 deterministic scorers; aggregation; archetype
internal/profile/            # the pure, versioned, serializable Profile contract (schemaVersion)
internal/insight/            # InsightEngine interface; template tips; optional deep-eval LLM (BYO key)
internal/report/             # Reporter interface; TUI + JSON reporters (web/export = future impls)
internal/tui/                # Bubble Tea dashboard + ntcharts braille radar (a report.Reporter)
internal/design/             # design tokens (palette, type scale, borders, motion) shared with landing page
```

The **`model.Session` and the `Profile` are the two contracts**: everything above the provider is source-agnostic (adding Codex/OpenCode = a new `provider/<tool>` emitting the same `model.Session`), and everything below scoring consumes only the `Profile` (adding a surface = a new `report.Reporter`). Neither future extension touches the core.

### Normalized model (`internal/model`)
```
Session{ ID, Source, Cwd, GitBranch, Model(s), StartedAt, EndedAt,
         Turns[], TokenTotals, Version }
Turn{    Role(user|assistant|meta), Text, Blocks[], ToolCalls[],
         Timestamp, IsSidechain, IsSynthetic, PermissionMode,
         SlashCommand, Usage(tokens...) }
ToolCall{ Name, IsMCP, InputSummary, ResultOK, DurationMs }
```
Read the `cwd` **field inside records** to identify the project — never reverse the encoded folder name (the `/`→`-` and `.`→`-` encoding is lossy and collides).

---

## Design patterns & extensibility (structure for the long game)

The system is a **pipeline of swappable stages** bound by interfaces, with a pure data `Profile` as the contract between "compute" and "present." Applied deliberately, not as pattern-soup — each pattern earns its place by making a specific future extension a drop-in.

**The seams (interfaces) — anything new plugs in here without editing the core:**

```go
// Provider adapter (Strategy + Adapter). Add Codex/OpenCode/Gemini = new impl, no core change.
type Provider interface {
    ID() string                                  // "claude-code"
    Discover(ctx) ([]SessionRef, error)          // locate & enumerate sessions
    Parse(ctx, SessionRef) (model.Session, error)// raw logs -> normalized model
}

// Scoring dimension (Strategy). Add/replace a dimension or a whole rubric = register a Scorer.
type Scorer interface {
    Dimension() Dimension
    Score(model.Session) (Signal, error)         // returns score + retained raw counts
}

// Insight generation (Strategy). Template today; LLM deep-eval; a recruiter rubric later.
type InsightEngine interface { Generate(Profile) ([]Insight, error) }

// Output consumer (Strategy). TUI now; JSON; a portable signed report; a web payload later.
type Reporter interface { Render(Profile) error }
```

- **Registry / Factory** — `Provider`s and `Scorer`s self-register into registries; the engine iterates the registry, so enabling a source or rubric is one registration line, and weights/enabled-set come from config. This is what makes the "recruiter rubric" or a 6th dimension additive rather than invasive.
- **Pipeline** — `Discover → Parse → Normalize → Store → Score → Aggregate → Profile → Report`. Each stage consumes the previous stage's typed output; stages are independently testable and independently replaceable.
- **Repository** — `internal/store` hides SQLite behind a `Repository` interface (`UpsertSession`, `LoadAggregates`, `Since`). Swapping the cache backend or mocking it in tests never touches scoring.
- **Functional options + dependency injection** — constructors take options (`WithProviders`, `WithScorers`, `WithClock`, `WithFS`); no global singletons, so everything is unit-testable and deterministic (inject the clock/FS for golden tests).
- **The `Profile` is a pure, versioned, serializable value** (JSON-able struct, `schemaVersion` field) — the single contract every `Reporter` consumes. The TUI renders it; `--json` emits it; a future landing page, web dashboard, or recruiter portal consumes the *same* struct. Compute never knows who renders.

**Future use-cases this structure unlocks (NOT v1, but validated as drop-ins):**
- **More providers** — implement `Provider`, register it. The normalized model + all five scorers work unchanged. (Storage for each is documented in the appendix.)
- **Recruiter / candidate flow** — a candidate runs `synch export` → a portable, integrity-stamped `Profile` artifact; a recruiter tool ingests it and applies a recruiter-defined `Scorer` set / rubric. Both sides reuse the same `Profile` schema and `Scorer`/`Reporter` interfaces — no fork of the engine. (Design the `Profile` schema now with this in mind: stable field names, `schemaVersion`, no TUI-specific data leaking in.)
- **New surfaces** — landing page, web dashboard, CI check: each is just another `Reporter`/consumer of the `Profile` JSON.

---

## Parsing layer — the edge cases (this is the core)

The Claude Code format is an **undocumented internal JSONL that changes across versions**. Parse defensively; key off the top-level `type` discriminator and tolerate unknown fields. Every one of these must be handled, each with a golden-file test:

**Structural / format edge cases**
- **Malformed or truncated final line** — a session being written live. Skip the bad line, keep going; never crash the run.
- **Schema drift across `version`** — new/renamed fields appear frequently. Unknown `type` → ignore; unknown fields → tolerate. Branch on the per-record `version` only where a known break exists.
- **`message.content` is `string` XOR `array`** — user text is a bare string; tool results arrive as an array of `tool_result` blocks. Handle both shapes.
- **Huge files** — sessions can bloat to hundreds of MB. Always stream (`bufio.Scanner` with a raised buffer; fall back to a chunked reader if a single line exceeds the cap). Never read a whole file into memory.
- **Empty / single-record / interrupted sessions** — no assistant turn, no `stop_reason`. Must not divide-by-zero; contribute nothing rather than a fake score.
- **Timestamps missing or out of order** — sort defensively; tolerate absent `timestamp`.
- **`parentUuid` forms a tree, not a list** — sidechains (sub-agent/Task branches) fork off. Walk the tree; don't assume linear.

**Records that must NOT be scored as user prompts** (fairness-critical — over-counting these skews every dimension)
- `isMeta: true` records and `<local-command-caveat>…</local-command-caveat>` blocks (locally-run command output).
- `isCompactSummary: true` / synthetic `summary` records injected by `/compact` — count the *event* (context signal) but exclude the text from prompt-quality.
- **Slash-command envelopes** — captured as `<command-name>/clear</command-name>` inside user text. Extract the command (for Tool Leverage + "most-used command" stat), but the envelope itself is not a natural-language prompt.
- **Pasted shell commands / `!` invocations** — real example on disk: a user "prompt" that was `curl -L … && code --install-extension …`. Detect command-shaped / attachment / paste-cache content and classify it as *not a NL prompt* so it doesn't pollute Prompt Quality.
- **Sidechain turns** (`isSidechain: true`) — attribute to the parent session; don't count sub-agent internal prompts as the human's prompts.

**Pairing / integrity**
- Pair `tool_use.id` ↔ `tool_result.tool_use_id`; tolerate orphans on either side.
- MCP tools carry `attributionMcpServer`/`attributionMcpTool` — flag `IsMCP` for Tool Leverage.

---

## Multilingual & language-agnostic scoring (fairness core)

The user may prompt in **English, Hindi (Devanagari), Hinglish (Hindi in Latin script), or code-switch mid-sentence.** The engine must grade *communication quality*, not *English proficiency*. Design rules:

1. **Structural signals are primary and language-independent** — file paths, `@mentions`, code fences/backticks, URLs, stack-trace/error patterns (near-always English/code regardless of prompt language), numbers, tool/command references, and **turn-shape** (who spoke, how many turns, question-vs-statement). These carry ~80% of Prompt-Quality and all of Iteration/Tool/Context/Token signal, and they work identically in any language.
2. **Length/specificity is measured in runes with script awareness**, never bytes — Devanagari is multibyte, and word segmentation differs. Detect the dominant Unicode script per message; normalize "is this vague/terse" thresholds per script so a Hindi prompt isn't penalized for tokenizing differently.
3. **Lexical cues are optional boosters only, with multilingual dictionaries** — e.g. correction/clarification detection ("no, I meant" / "galat" / "नहीं" / "phir se" / "ruko"). These *add* confidence; their **absence never lowers a score** (a non-English prompt simply relies on the structural path). Keep dictionaries small, data-driven, and clearly separated so they're easy to extend.
4. **Assistant-question detection** (for clarification-loop counting) uses structure — a short assistant turn ending in `?` with no tool_use — plus the `AskUserQuestion` tool signal, both language-independent.
5. **Never penalize for not being English.** Explicitly tested: a Hindi/Hinglish prompt that names a file + pastes an error must score as high-quality.

---

## Scoring engine (`internal/score`) — deterministic

Five weighted dimensions, each 0–10. **Every score is explainable** — we retain the underlying counts and surface them ("file path present in 36% of first prompts"). Trust comes from transparency.

| Dimension | Primary (language-agnostic) signals |
|---|---|
| **Prompt Quality** | file-path presence (esp. first prompt of a task), error/stack-trace inclusion, code fences, concrete-ask structure, specificity (rune/script-aware), attachment vs typed |
| **Iteration Efficiency** | turns-to-resolution per task, clarification loops (assistant `?`/`AskUserQuestion`), correction re-prompts (structural + multilingual cues), redo/repeat detection |
| **Tool Leverage** | distinct tool diversity, tool-calls-per-turn, sub-agent/`Agent` use, `Skill`/MCP/slash-command usage, parallel tool calls *(note: 0 in sample — weight modestly)* |
| **Context Management** | `/compact` frequency & timing, `/clear` at task boundaries, session-length discipline, cache-read ratio (context reuse), file-reference vs giant-paste |
| **Token Economy** | cache-hit rate (`cache_read`/input — biggest lever), output/input ratio, tokens per resolved task, cost proxy |

**Fairness mechanics (baked in, not optional):**
- **Per-session normalization then aggregate** — a few giant sessions can't dominate the profile.
- **Minimum-data gating** — a dimension with too few observations renders as *"insufficient data,"* not a low score. No punishing sparse users.
- **Calibrated absolute thresholds** derived from real distributions (seed from this machine's corpus), with the raw signal always shown so the grade is auditable.
- **Graceful degradation** — if a signal is absent (e.g. a reasoning-only model, missing tokens), that dimension's weight redistributes rather than scoring zero. (Also future-proofs multi-tool.)
- Deterministic: same logs → same scores, always.

### Archetypes (`internal/score`)
Derived from the *shape* of the radar (which dimensions dominate), not a separate model — so it's deterministic and explainable:
- **Architect** — high Prompt Quality + Context Management (plans, precise, uses plan mode)
- **Conversationalist** — many turns, high iteration, lower first-prompt specificity
- **Prober** — high Tool Leverage / exploration-heavy (Read/Search/Bash dominant)
- **Operator** — high Token Economy + Iteration Efficiency (terse, fast, cache-savvy)
- **Delegator** — heavy sub-agent/`Agent` use
Rank dimensions, match to the nearest archetype signature; ties broken deterministically.

---

## Cache & delta-sync (`internal/store`)
- SQLite (`modernc.org/sqlite`, pure Go) at `~/.config/synch/store.db` (honor `XDG_CONFIG_HOME`), WAL mode, opened with a busy timeout.
- Track per file: `path, mtime, size, last_synced_at`. On sync, `stat` each `*.jsonl`; re-parse only when mtime/size changed. Store normalized turns + precomputed per-session aggregates.
- **<50ms boot:** if nothing changed, skip parsing entirely and load precomputed aggregates → straight into the TUI.

## Design language (shared system: TUI now, landing page later)

The interface reads as a **high-end industrial operating system for expert users** — mission-control / UNIX-workstation / aerospace-instrument / financial-terminal, not a SaaS website. Precision, structure, density, and technical authenticity over decoration. **Industrial, mechanical, engineered, functional, timeless, restrained, premium.** This is codified once as a **design-token spec** (palette, type scale, spacing grid, border/rule styles, motion rules, iconography) that *both* the terminal TUI and the future web landing page consume, so the product feels like one bespoke environment across surfaces.

**Tokens (single source of truth):**
- **Palette:** restrained **monochrome** (graphite/ink/paper neutrals) + **one industrial accent** (e.g. amber/oxide-orange or a signal color) used sparingly for live values, alerts, the active axis. No second accent.
- **Type:** **monospaced throughout**; strong hierarchy via weight/size/case and rules, not color.
- **Structure:** strict **grids**, split-window / panel / control-panel layouts, visible borders and rules as the primary structuring device (borders over shadows/effects).
- **Iconography:** custom **engineering-inspired** glyphs, technical diagrams, **wireframes, ASCII-inspired** elements — no stock icon libraries, no illustrations.
- **Motion:** **mechanical, purposeful** transitions (stepped, instrument-like), never bouncy/organic.
- **Explicitly banned** (both surfaces): matrix/hacker green, neon terminals, cyberpunk, RGB-gaming, glassmorphism, gradient-heavy SaaS, stock art, overused UI patterns.

### TUI (`internal/tui`) — v1
Bubble Tea + Lip Gloss, styled to the tokens above: a **control-panel layout** of bordered panels — braille **radar** (ntcharts `canvas` + a ~100-line `drawRadar` polygon over 5 axes, active axis in the accent), a dense dimensional-score panel with rule-separated rows, an archetype banner rendered as an instrument readout, top actionable tips, and fun stats (most-used slash command, top tools, cache-hit rate). Monochrome + single accent; information-dense, not spacious. `--json` emits the raw `Profile` for scripting/tests and future consumers.

### Landing page (NOT v1 — spec recorded so it's coherent)
A separate web deliverable (static site / Next.js) that is another **consumer of the same `Profile` schema and design tokens** — decoupled from the Go core. Same industrial-OS language: monospaced type, structured grids, wireframe/ASCII motifs, custom engineering iconography, mechanical animations, borders-not-effects, monochrome + one accent. It should feel like *booting the tool*, not visiting a marketing page. Deferred, but the token spec and `Profile` contract are built in v1 so the landing page drops onto them later without redesign.

## Optional deep-eval (`internal/insight`)
Default path = deterministic template tips from the retained signal counts. `--deep-eval` (BYO API key, opt-in) sends a *summary* (never raw secrets/code where avoidable) to an LLM purely to phrase qualitative insights. It **cannot alter any numeric score.** Uses the latest Claude models when the key is Anthropic.

## Distribution
GoReleaser from a git tag → GitHub release binaries (darwin/linux/windows × amd64/arm64), Homebrew tap/cask, Scoop + winget PR, and a `curl | sh` installer. Pure-Go (no cgo) means all targets cross-compile on one CI runner.

---

## Verification (how we prove it's robust & fair)

1. **Edge-case corpus + golden files** — a fixtures directory of hand-crafted `.jsonl` sessions, one per edge case above: truncated last line, unknown `type`, string-vs-array content, `isCompactSummary`, slash-command envelope, pasted `curl` "prompt", sidechain tree, empty session, out-of-order timestamps, huge synthetic file, and **multilingual** samples (Hindi Devanagari, Hinglish, code-switched). Each asserts the parser doesn't crash and classifies records correctly (e.g. the `curl` line is *not* counted as a NL prompt; the Hindi+file+error prompt scores high).
2. **Real-data smoke test** — run against this machine's live `~/.claude/projects/` (4 projects present) and eyeball the profile for sanity; assert boot <50ms on a warm cache.
3. **Determinism test** — run twice on identical input; assert byte-identical scores.
4. **Fairness assertions** — unit tests proving: (a) an all-Hindi high-signal prompt ≥ the equivalent English prompt; (b) a sparse user gets "insufficient data," not a 0; (c) a session full of `isMeta`/command records yields no Prompt-Quality inflation.
5. **TUI snapshot** — golden snapshot of the rendered dashboard; manual run for the braille radar.

## Milestones
1. **Interface seams first** — define `Provider`, `Scorer`, `InsightEngine`, `Reporter`, `Repository` interfaces and the versioned `Profile` struct (the skeleton everything plugs into).
2. `provider/claude` defensive parser + normalized model + **edge-case corpus/golden tests** (the hard, high-value core).
3. store (`Repository`/SQLite) + delta-sync + <50ms boot.
4. `internal/text` language-agnostic features + 5 registered scorers + fairness gating + archetype + **multilingual fairness tests**.
5. `internal/design` tokens + TUI reporter (radar + panels) + JSON reporter.
6. template tips; then optional `--deep-eval` `InsightEngine`.
7. GoReleaser distribution.

*(Post-v1, all additive: more `Provider`s, `synch export` + recruiter rubric `Scorer`s, landing page consuming `Profile` + design tokens.)*

---

## Appendix — researched storage for future adapters (not v1)
- **Codex CLI:** `~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl`, `{type,payload}` envelope; tokens in `event_msg`→`token_count`; **reasoning encrypted**; old pre-`RolloutLine` files won't parse; logs bloat to GBs. Root override `CODEX_HOME`.
- **OpenCode:** `~/.local/share/opencode/opencode.db` (SQLite/Drizzle/WAL). Denormalized `session.tokens_*`+`cost`; `message`/`part` bodies are JSON blobs in `data`; migrated from legacy `storage/*.json` (DB authoritative). XDG paths.
- **Gemini CLI:** `~/.gemini/tmp/<project_hash>/chats|checkpoints/`, structured JSON (prompts+tools+tokens+reasoning). Cheapest high-quality 4th adapter.
- **Aider:** per-repo `.aider.chat.history.md` (lossy) + opt-in `~/.aider/analytics.jsonl` (only good structured source). Needs graceful degradation.
- **Cursor CLI** (SQLite + hex-encoded blobs — painful), **Continue `cn`** (clean `~/.continue/sessions/*.json`), **Amp** (cloud-first, no local transcript). Cline/Roo/Kilo are IDE extensions — off-thesis for a terminal tool.