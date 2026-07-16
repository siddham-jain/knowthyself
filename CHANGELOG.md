# Changelog

All notable changes to this project are documented here so that any session,
editor, or agent can pick up with full context.

## [Unreleased]

### Added
- **The Reveal** — a first-run persona portrait the boot animation lands on, built to make the very first run irresistible (identity + awe, zero judgment). Combines the collaboration archetype ("YOU ARE A CONDUCTOR") with a "by the numbers" band and deterministic trait badges. `internal/tui/reveal.go`.
- Expanded, cooler archetype catalog matched by radar *shape* via cosine similarity over the graded dimensions: Architect, Surgeon, Conductor, Pathfinder, Economist, Marathoner, plus Conversationalist and a balanced Generalist fallback. Deterministic and explainable. `internal/score/archetype.go`.
- Trait badges (deterministic, positive-only): Polyglot, Veteran, Deep Diver, Night Owl / Early Bird. `internal/engine/analyze.go` (`deriveTraits`).
- New "by the numbers" stats: total collaboration time, first-session date (tenure), peak collaboration hour, language mix (English/Hindi/Other via `text.DominantScript`), distinct project count. `internal/profile/profile.go` (`Stats`) + `internal/engine/analyze.go` (`computeStats`).
- Tests: archetype shape-matching + trait/stat computation + reveal render/reflow across a width sweep.

### Changed
- `Archetype` gains a `Traits []string` field; archetype blurbs rewritten as punchier identity lines.
- Boot animation now resolves to the reveal; the graded dashboard (overview/sessions/trends) is one keystroke away (`→`/any key), with the archetype-jump keys still honored.

---

## Work Log

Chronological notes for cross-session context. Newest first.

### 2026-07-17 — The Reveal: first-run persona + numbers
- **What:** Reframed the first run from a dashboard/report-card into a celebratory identity portrait. Added a shape-matched archetype catalog (cosine similarity over sufficient dims), deterministic trait badges, and a set of awe stats (hours, tenure, peak hour, languages, projects, token analogy). New `internal/tui/reveal.go` renders the hero + numbers; boot lands here, dashboard is one key away.
- **Why:** The product goal shifted to *first-time conviction*. An after-action "coach" was explicitly rejected (it judges/exposes the user's corrections and missing file paths). The reveal is pure curiosity + flattery + near-zero friction (local, ~5s), which is what actually gets someone to run the tool the first time.
- **State:** Builds, `go vet` clean, all tests green. Verified against real `~/.claude` data (Conversationalist; Deep Diver + Night Owl; ~73 hrs; 630M tokens; 98% cache) and eyeballed the reveal at widths 100 and 60 — no overflow. Capped the "≈ N novels" analogy to a graspable range so an inflated (cache-heavy) token count doesn't read as absurd.
- **Notes:** Per-session "after-action coach" and its per-session evidence retention were deliberately NOT built. The percentile / "where you stand" option was dropped (needs a cohort baseline we don't have). Known soft spot: "hours together" is wall-clock session span (EndedAt-StartedAt) so it overcounts idle time — acceptable for a fun stat. Plan file: `~/.claude/plans/synch-is-a-developer-first-precious-sonnet.md` (feature plan prepended).
