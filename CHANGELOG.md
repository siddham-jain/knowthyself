# Changelog

All notable changes to this project are documented here so that any session,
editor, or agent can pick up with full context.

## [Unreleased]

### Added
- **First-run consent gate** — the first time `reflect` runs on a real terminal it asks "Meet how you work with AI?" with two choices: run the analysis now, or "No, I don't want to know" (which prints how to run it later and exits without reading anything). Shown once; a `.welcomed` sentinel beside the cache records it. `internal/tui/firstrun.go`, wired in `cmd/reflect/main.go`.
- **Rebrand to `reflect`** — the tool, module path (`github.com/siddham/reflect`), binary, command dir (`cmd/reflect`), cache path (`…/reflect/store.db`), and all user-facing copy. "know how you build."
- **`reflect` wordmark art** — a block-letter wordmark with a dimmed, vertically mirrored reflection beneath a waterline (the name made literal). Reused by the installer banner and the cold-start screens. `internal/tui/art.go`.
- **Playful cold-start screens** — instead of a bare error: a wide-eyed reaction for "no Claude on this machine" (with a get-Claude / `CLAUDE_CONFIG_DIR` nudge) and an empty-radar "you're early" for not-enough-history. Interactive TTY only; piped/JSON stays a plain error. `internal/tui/onboarding.go`, wired in `cmd/reflect/main.go`.
- **`--version` flag** — prints version/commit/date injected at release time via ldflags. `cmd/reflect/main.go` (`main.version`).
- **Distribution** — `.goreleaser.yaml` (cross-compiled binaries for darwin/linux/windows × amd64/arm64, cosign-signed checksums, Homebrew + Scoop + winget + AUR publishers), `.github/workflows/release.yml` (tag-triggered), `install.sh` + `install.ps1` (banner → checksum-verified download → first-run prompt), `LICENSE` (MIT).
- **npm distribution** — `npm/` wrapper package published as `knowthyself` (the bare `reflect` name is taken on npm). `npm i -g knowthyself` / `npx knowthyself` installs the `reflect` command; `scripts/postinstall.js` downloads the matching prebuilt binary from the GitHub Release for this OS/arch, `bin/run.js` execs it. No Go toolchain needed on the user's machine.
- **`claude.Available()`** — detects whether `~/.claude` exists, to drive the no-Claude greeting. `internal/provider/claude/claude.go`.
- Tests: `TestNeverOverflowsHeight` (the alt-screen never renders more lines than the terminal), `TestTooShortMessage`.
- **The Reveal** — a first-run persona portrait the boot animation lands on, built to make the very first run irresistible (identity + awe, zero judgment). Combines the collaboration archetype ("YOU ARE A CONDUCTOR") with a "by the numbers" band and deterministic trait badges. `internal/tui/reveal.go`.
- Expanded, cooler archetype catalog matched by radar *shape* via cosine similarity over the graded dimensions: Architect, Surgeon, Conductor, Pathfinder, Economist, Marathoner, plus Conversationalist and a balanced Generalist fallback. Deterministic and explainable. `internal/score/archetype.go`.
- Trait badges (deterministic, positive-only): Polyglot, Veteran, Deep Diver, Night Owl / Early Bird. `internal/engine/analyze.go` (`deriveTraits`).
- New "by the numbers" stats: total collaboration time, first-session date (tenure), peak collaboration hour, language mix (English/Hindi/Other via `text.DominantScript`), distinct project count. `internal/profile/profile.go` (`Stats`) + `internal/engine/analyze.go` (`computeStats`).
- Tests: archetype shape-matching + trait/stat computation + reveal render/reflow across a width sweep.

### Changed
- **Professional README rewrite** — logo-led header with badges, a clear "what it does" section, npm-first install, placeholder slots for a demo video and per-screen TUI screenshots, and a dedicated "Collaboration archetypes" section (all eight personas) placed after the core docs. Dash-free prose. Fixed the owner references to `siddham-jain`. New `assets/` folder with a guide for the media files to drop in. `README.md`, `assets/README.md`.
- **Responsive TUI** — the dashboard now budgets vertical space: the radar shrinks to a height cap, the session list and overview detail row fit to the terminal height, a "terminal too short" notice mirrors the too-narrow one, and every frame is hard-clipped to the viewport. This fixes the bug where a small terminal rendered a partial frame and corrupted the scrollback until a manual resize. `internal/tui/view.go`, `radar.go`, `tui.go`.
- `Archetype` gains a `Traits []string` field; archetype blurbs rewritten as punchier identity lines.
- Boot animation now resolves to the reveal; the graded dashboard (overview/sessions/trends) is one keystroke away (`→`/any key), with the archetype-jump keys still honored.

---

## Work Log

### 2026-07-18 — npm distribution + release owner fix
- **What:** Added an `npm/` wrapper package (`knowthyself`) that downloads the prebuilt `reflect` binary from GitHub Releases on postinstall. Fixed the `siddham` → `siddham-jain` owner mismatch in `.goreleaser.yaml` (`release.github.owner`) so releases and download URLs hit the real repo, and commented out the Homebrew/Scoop/winget/AUR publisher blocks (they need a `TAP_GITHUB_TOKEN`/`AUR_KEY` not yet configured) so the first binary-only release succeeds. Gitignored the `/reflect` build artifact.
- **Why:** Enable `npm i -g knowthyself` / `npx knowthyself` without shipping a Go toolchain.
- **State:** Wrapper + config committed. Not yet released/published — needs (1) push tag `v0.1.0` to trigger the release workflow, then (2) `npm publish` from `npm/`. Package version (0.1.0) must equal the release tag.
- **Notes:** `go.mod` module path is still `github.com/siddham/reflect` (internal only, doesn't affect goreleaser builds); `go install` from the network would need it changed to `siddham-jain`, but npm/Homebrew/etc. don't care.


Chronological notes for cross-session context. Newest first.

### 2026-07-17 — Rebrand to reflect, distribution, responsive TUI, cold-start moments
- **What:** Renamed synch → **reflect** across the codebase (module path, binary, `cmd/reflect`, cache dir, all copy; protected `PRAGMA synchronous`). Added the mirrored-wordmark ASCII identity (`art.go`), two playful cold-start screens (`onboarding.go`: no-Claude reaction + not-enough-data empty radar), a `--version` flag with ldflags build metadata, and a full distribution setup: GoReleaser (all-target cross-compile, cosign-signed checksums, Homebrew/Scoop/winget/AUR), a tag-triggered release workflow, `install.sh`/`install.ps1` (banner → verified download → "run now?" prompt), and an MIT `LICENSE`. Fixed the responsive-TUI overflow bug (small terminal → partial render + garbled scrollback) by budgeting height, shrinking the radar/lists, adding a too-short notice, and hard-clipping each frame to the viewport.
- **Why:** The tool needed a real name and a professional, cross-OS install story so the commands can drop into a future landing page. The cold-start screens make a first-timer feel greeted, not error-messaged. The overflow bug made the dashboard unusable in small windows.
- **State:** `go build`, `go vet`, and all tests green (incl. new height/overflow tests). Verified all six cross-compile targets build CGO-free. Eyeballed the no-Claude and no-data screens and the installer banner via a pty. Distribution is release-ready but **inert until**: repo is public at `github.com/siddham/reflect`, helper repos exist (`homebrew-tap`, `scoop-bucket`, `winget-pkgs` fork), and CI secrets are set (`TAP_GITHUB_TOKEN`, `AUR_KEY`). `<domain>` in the install scripts/README is a placeholder.
- **Notes:** The no-Claude "shocked" moment is a generic wide-eyed reaction, not a caricature of a real person, and carries no fabricated quote — the humor is in the tool's voice ("shipping code with no AI pair in this economy?"). Swap the art freely. Landing page intentionally deferred (will be React/Next, designed separately).

### 2026-07-17 — The Reveal: first-run persona + numbers
- **What:** Reframed the first run from a dashboard/report-card into a celebratory identity portrait. Added a shape-matched archetype catalog (cosine similarity over sufficient dims), deterministic trait badges, and a set of awe stats (hours, tenure, peak hour, languages, projects, token analogy). New `internal/tui/reveal.go` renders the hero + numbers; boot lands here, dashboard is one key away.
- **Why:** The product goal shifted to *first-time conviction*. An after-action "coach" was explicitly rejected (it judges/exposes the user's corrections and missing file paths). The reveal is pure curiosity + flattery + near-zero friction (local, ~5s), which is what actually gets someone to run the tool the first time.
- **State:** Builds, `go vet` clean, all tests green. Verified against real `~/.claude` data (Conversationalist; Deep Diver + Night Owl; ~73 hrs; 630M tokens; 98% cache) and eyeballed the reveal at widths 100 and 60 — no overflow. Capped the "≈ N novels" analogy to a graspable range so an inflated (cache-heavy) token count doesn't read as absurd.
- **Notes:** Per-session "after-action coach" and its per-session evidence retention were deliberately NOT built. The percentile / "where you stand" option was dropped (needs a cohort baseline we don't have). Known soft spot: "hours together" is wall-clock session span (EndedAt-StartedAt) so it overcounts idle time — acceptable for a fun stat. Plan file: `~/.claude/plans/synch-is-a-developer-first-precious-sonnet.md` (feature plan prepended).
