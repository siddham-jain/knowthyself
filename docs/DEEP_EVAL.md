# Deep Eval — architecture & grading rubric

Status: **implemented.** `internal/insight/deepeval` (pipeline, rubric, redaction,
client), `internal/tui/consent.go` (the gate), `internal/tui/deepread.go` (the tab).

Two things differ from the original design and are reflected below: session
allocation needed an explicit per-session cap on top of sqrt weighting, and the
judge is asked for JSON in-prompt rather than via native structured-output, because
BYOK cannot assume every "OpenAI-compatible" endpoint implements it.

## 1. Why it exists

The five radar dimensions are computed by `internal/score` from *structure*: does a
prompt contain a file path, a code fence, an error trace; how many turns to
resolution; cache-read ratio. That is cheap, deterministic, private, and language
agnostic — and it is deliberately blind to meaning.

Structure cannot answer the question a developer actually cares about:

> *Was what I asked for understandable?*

A prompt can carry a file path, a stack trace and a code fence and still be a bad
ask, because it never said what "done" looks like. Deep-eval exists to read the
actual words and judge that. Nothing else justifies sending data off the machine.

## 2. Hard boundaries

1. **Deep-eval never changes a score.** It consumes an already-computed `Profile`.
   The radar, the overall, and the archetype are byte-identical whether or not
   deep-eval ran.
2. **Its output is separately labelled.** Model-judged results are never averaged
   into deterministic ones, and the surface always states the model and sample size.
3. **Opt-in per endpoint.** Explicit consent, showing the exact redacted payload,
   before the first byte leaves for a given (host, model).
4. **Failure degrades, never breaks.** Any error falls back to the heuristic engine
   with a specific, actionable message. `knowthyself --deep-eval` on a plane still
   renders a dashboard.

## 3. The rubric

### 3.1 Shape

Five criteria, judged **per sampled prompt**, on an anchored **0–4 ordinal scale**.

The scale is anchored — every level has a behavioural descriptor — because
unanchored numeric scales ("rate this 1–10") have poor agreement between models and
between runs of the same model. Five levels is about the limit at which distinct
descriptors can still be written without overlap. The UI shows the raw 0–4 mean and
labels the scale on the panel; it deliberately does not rescale to 0–10, which would
invite exactly the comparison with the radar that must not be made.

Criteria were chosen to be (a) genuinely beyond structural heuristics, (b) judgeable
from one prompt plus minimal neighbouring context, (c) actionable tomorrow.

### 3.2 Criteria

**C1 — Goal clarity.** Does the prompt state the outcome wanted, or only an action?

| L | Anchor |
|:-:|:-------|
| 0 | No discernible goal. "fix this", "continue", "make it better". |
| 1 | An action is named; the outcome is left implied. |
| 2 | An outcome is stated but its scope is ambiguous. |
| 3 | A concrete outcome is stated. |
| 4 | Outcome **and** why it matters, so the model can resolve trade-offs unaided. |

**C2 — Context sufficiency.** Is what the model needs supplied, or assumed?

| L | Anchor |
|:-:|:-------|
| 0 | Relies entirely on the model inferring or re-discovering the context. |
| 1 | Gestures at context without locating it ("the usual file", "that thing"). |
| 2 | Partial: some context located, load-bearing parts assumed. |
| 3 | The relevant location, state and inputs are supplied. |
| 4 | Supplies context **and** pre-empts the obvious follow-up question. |

**C3 — Constraints & acceptance.** Are boundaries, non-goals and done-conditions stated?

| L | Anchor |
|:-:|:-------|
| 0 | None. |
| 1 | One implicit constraint, inferable but unstated. |
| 2 | Some constraints, no done-condition. |
| 3 | Explicit constraints **or** an explicit done-condition. |
| 4 | Both, plus explicit non-goals ("don't touch the schema"). |

**C4 — Scope discipline.** Is this one coherent unit of work?

| L | Anchor |
|:-:|:-------|
| 0 | Several unrelated asks bundled into one message. |
| 1 | Multiple loosely-related asks. |
| 2 | A single ask with unbounded scope ("refactor the backend"). |
| 3 | One coherent, bounded ask. |
| 4 | Bounded and sequenced, with the ordering made explicit. |

**C5 — Correction quality.** *Conditional — judged only on prompts that correct a
previous response.*

| L | Anchor |
|:-:|:-------|
| 0 | Bare re-assertion. "no", "still broken", "that's wrong". |
| 1 | Says it is wrong, not what is wrong. |
| 2 | Identifies the symptom. |
| 3 | Symptom **and** expected behaviour. |
| 4 | Symptom, expected behaviour, and a hypothesis or the missing constraint that caused it. |

C5 carries an `applicable` flag. Aggregation divides by the applicable count, never
by sample size — otherwise a developer who rarely needs to correct the model is
punished for it.

### 3.3 Single source of truth

Criteria and anchors live once, as Go data in `internal/insight/deepeval/rubric.go`.
The system prompt (via `describe()`) and the validator (via `Lookup`/`MaxLevel`) are
both **derived** from that data. This is the reason the rubric is data and not prose: three
hand-maintained copies of a rubric drift, and a drifted validator silently accepts
judgments the prompt never asked for.

`RubricVersion` is an integer constant. Bumping it invalidates every cache entry.

## 4. Pipeline

```
sessions ──▶ sample ──▶ redact ──▶ consent ──▶ chunk ──▶ judge ──▶ validate
                                                            │          │
                                                            └── repair ┘
                                                                       │
                          heuristic fallback ◀── abandon ◀── aggregate ─┴─▶ synthesise
                                                                              │
                                                                            cache
                                                                              │
                                                                            render
```

### 4.1 Sample

Universe: user turns where `Turn.Scorable()` is true — synthetic, sidechain,
slash-command and pasted-command turns are already excluded there.

- **Deterministic.** Seed derives from a hash of the sorted session IDs plus
  `RubricVersion`. The same corpus always yields the same sample, so two runs are
  comparable and a cache hit is meaningful.
- **Stratified by session,** allocated proportional to `sqrt(prompts)` and then
  capped at `2 * budget / sessions`, with the leftover budget redistributed one
  prompt per session per round. The cap is not optional: sqrt weighting alone still
  handed a 400-prompt session 70% of a 40-prompt budget in testing.
- **Bounded** by both a prompt count (default 60) and a character budget (default
  40 000), whichever binds first. Cost is therefore predictable before any call.
- Each prompt is truncated to 1 200 characters, head and tail retained, middle
  elided — the shape of a long prompt survives, the bulk does not.
- For C5, the preceding assistant turn's first 300 characters travel with the prompt
  as `prior_context`; a correction cannot be judged without knowing what was
  corrected.

### 4.2 Redact

A standalone package with its own test suite, applied before anything is displayed
or sent. Order matters — secret patterns run first, before path and URL rules can
partially mangle a token and defeat the match.

| Class | Becomes |
|:------|:--------|
| API keys, `ghp_*`, AWS `AKIA*`, JWTs, `BEGIN … PRIVATE KEY` blocks | `<secret>` |
| `password=`, `token=`, `api_key=` assignments, connection strings | `<secret>` |
| Absolute paths | `<path>/basename.ext` |
| Home directory | `~` |
| Email addresses | `<email>` |
| URLs | scheme + host only |
| Hex/base64 runs ≥ 32 chars | `<blob>` |
| IP addresses | `<ip>` |

Redaction is conservative: a false positive costs a little judging signal, a false
negative leaks a credential.

### 4.3 Consent

Before the first send to a given (host, model): a screen stating the endpoint host,
the model, the number of prompts, the character count, and the estimated input
tokens — with the exact redacted payload available to page through. Approval is
recorded per (host, model), so changing endpoint or model asks again.
`--yes` skips it for scripted use.

### 4.4 Judge

Chunks of 12 prompts, judged independently, temperature 0, fixed `max_tokens`.
Independence means one poisoned chunk is droppable without discarding the run.

The response shape is specified in the prompt and the first JSON object is extracted
from the reply (tolerating code fences), rather than relying on native
structured-output support. BYOK is the reason: support for it varies widely across
endpoints that advertise OpenAI compatibility, and an unsupported parameter is a hard
failure rather than a degradation. The validator below is what makes this safe.

### 4.5 Validate — the load-bearing reliability control

Every per-prompt judgment must return a `prompt_id` and a `quote`. A judgment is
**rejected** unless:

- `prompt_id` is in the sample that was sent;
- `quote` is a literal substring of that prompt's redacted text;
- `level` is an integer within the criterion's declared range;
- the criterion is one the rubric declares.

Requiring a resolvable literal quote is the single most effective anti-fabrication
measure available: a model cannot invent evidence that must match text we already
hold.

Escalation:

| Condition | Action |
|:----------|:-------|
| Any judgment in a chunk fails | one repair retry naming the specific failures |
| Repair still fails | drop the chunk, continue |
| Valid judgments < 60 % of sample | keep, mark confidence `low` |
| Valid judgments < 30 % of sample | abandon, fall back to heuristics, explain why |

### 4.6 Aggregate & synthesise

Per criterion: mean level, applicable count, and the sample size it was judged over
— all three displayed, so small `n` is visible rather than hidden. Then at most four
findings, each required to cite at least one valid `prompt_id`; uncited findings are
dropped.

### 4.7 Cache

SQLite table keyed by a fingerprint over `RubricVersion` + model + the sorted hashes
of the sampled redacted prompts. Re-running with no new sessions is free and
instant. Entries record which model and rubric version produced them.

## 5. BYOK

Any OpenAI-compatible or Anthropic-compatible chat endpoint. Plain `net/http` +
`encoding/json` — no SDK, keeping the build CGO-free and dependency-light.

Resolution order: **flag → env → config file → default.**

| Setting | Flag | Env | Fallback env |
|:--------|:-----|:----|:-------------|
| Key | `--api-key` | `KNOWTHYSELF_API_KEY` | `ANTHROPIC_API_KEY`, `OPENAI_API_KEY` |
| Base URL | `--base-url` | `KNOWTHYSELF_BASE_URL` | — |
| Model | `--model` | `KNOWTHYSELF_MODEL` | — |
| Dialect | `--api-dialect` | `KNOWTHYSELF_API_DIALECT` | auto-detected from host |

Config file at `~/.config/knowthyself/config.json`, mode 0600. The key is never
logged, never printed, and is redacted from every error string.

Two adapters behind one interface:

```go
type Client interface {
	Judge(ctx context.Context, req JudgeRequest) (JudgeResponse, error)
}
```

## 6. Failure taxonomy

Every failure is a typed error carrying a `Remedy()` line. The user is told what
went wrong and what to do about it — never a bare HTTP status.

| Condition | Message |
|:----------|:--------|
| No key configured | how to set one, all three sources listed |
| 401 / 403 | key rejected by `<host>` — check it matches this endpoint |
| 404 on model | `<model>` not available at `<host>`; how to list models |
| 429 | rate limited; honour `Retry-After`, one backoff retry |
| 5xx | provider error; one backoff retry, then give up |
| Timeout / DNS / refused | can't reach `<host>` — offline, or wrong base URL |
| TLS failure | certificate problem reaching `<host>` |
| Non-JSON body | `<host>` didn't answer in a supported format — check `--api-dialect` |
| Schema invalid after repair | `<model>` couldn't follow the response schema; try a stronger model |
| Context-length exceeded | halve the sample, retry once, then report |

## 7. Output contract

Additive — no `SchemaVersion` bump, since existing consumers ignore an unknown
`omitempty` field.

```go
type DeepRead struct {
	Model      string            `json:"model"`
	Endpoint   string            `json:"endpoint"`   // host only, never the key
	RubricVer  int               `json:"rubric_version"`
	JudgedAt   time.Time         `json:"judged_at"`
	Sample     SampleInfo        `json:"sample"`     // judged, sessions covered, available
	Criteria   []CriterionResult `json:"criteria"`
	Findings   []Insight         `json:"findings"`   // Source: "deep-eval", each cited
	Confidence string            `json:"confidence"` // high | medium | low
}
```

Surfaced as a fourth TUI tab, present only when a deep read exists, headed with the
model and sample size so a model-judged number is never mistaken for a deterministic
one.

## 8. Cost

At defaults: 60 prompts in 5 chunks ≈ 50 000 input + 8 000 output tokens, once, then
cached. The estimate is shown on the consent screen before anything is sent.
