# knowthyself — project instructions

Go CLI that reads AI coding-assistant session logs from disk and profiles *how* the
developer collaborates. Local-first, deterministic, terminal-native.

## Style

- **Comments: lowercase, one line, rare.** Only where genuinely necessary — a non-obvious
  decision, trade-off, or constraint. No banner headers, no restating signatures, no
  narrating steps, no marketing prose. Code that reads clearly gets no comment.
- **Package doc comments** are the exception: keep the `// Package x ...` sentence, and keep
  it factual.
- **No AI slop.** No filler, no flourish, no copy written to sound impressive.
- **Commits:** imperative subject with a category prefix, nothing else. No body unless asked.

## Load-bearing contracts

Two types are depended on by everything else. Changing either is a breaking change.

- `internal/model.Session` — the provider-neutral normalized session. No provider's on-disk
  format may leak into it.
- `internal/profile.Profile` — the serializable analysis result, the only thing presentation
  layers read. Bump `SchemaVersion` on a breaking change.

## Rules that must not be broken

- **Scores are deterministic and local.** `internal/score` is pure computation over parsed
  sessions: no network, no clock beyond the injected `Clock`, no randomness. The same corpus
  must always produce the same radar.
- **Deep-eval may never change a score.** It consumes a computed `Profile` and produces
  qualitative output only. It is opt-in, requires explicit consent, and any failure degrades
  to the heuristic engine rather than failing the run.
- **Fairness gate.** Too little data renders as `Sufficient: false` ("insufficient data"),
  never as a low score.
- **Language-agnostic scoring.** Grade communication quality on structural signals (paths,
  code fences, error shapes, turn shape) — never on English proficiency.
- **Nothing leaves the machine** unless the user explicitly opted in for that run.

## Design system

`internal/design` is the single source of truth. Monochrome graphite/ink/paper plus one
oxide amber accent; squared borders; structure carried by rules, not color. Explicitly
avoided: matrix green, neon, cyberpunk, gradients, emoji-as-decoration.

The identity mark is the Delphic inscription (`ΓΝΩΘΙ ΣΕΑΥΤΟΝ`) in `internal/tui/art.go`.

## Layout

```
cmd/knowthyself      CLI entry, flag parsing, subcommands
internal/provider    Discover + Parse per assistant (claude only today)
internal/model       normalized Session/Turn/ToolCall
internal/store       SQLite cache + delta sync
internal/engine      orchestration: sync, analyze, stats
internal/score       the five deterministic dimension scorers + archetype
internal/insight     tip generation (heuristic; deep-eval)
internal/profile     the output contract
internal/report      JSON reporter
internal/tui         Bubble Tea dashboard
internal/design      palette, borders, shared styles
```

## Working

- `make build` / `make test` / `make lint` (`go vet`). All three green before done.
- Update `CHANGELOG.md` after each meaningful unit of work.
