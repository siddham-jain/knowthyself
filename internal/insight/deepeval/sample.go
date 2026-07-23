package deepeval

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"sort"

	"github.com/siddham-jain/knowthyself/internal/model"
)

const (
	// DefaultMaxPrompts and DefaultCharBudget bound cost before any call is made,
	// whichever binds first.
	DefaultMaxPrompts = 60
	DefaultCharBudget = 40_000

	// maxPromptChars truncates one prompt; the shape of a long prompt survives, the
	// bulk does not.
	maxPromptChars = 1_200
	// priorChars is how much of the preceding assistant turn travels with a prompt.
	// A correction cannot be judged without knowing what was corrected.
	priorChars = 300
)

// Prompt is one redacted, truncated prompt offered to the judge.
type Prompt struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Prior   string `json:"prior,omitempty"` // tail of the preceding assistant turn
	session string
}

// Sample is the bounded, redacted evidence a deep read is based on.
type Sample struct {
	Prompts   []Prompt
	Sessions  int
	Available int // scorable prompts the sample was drawn from
}

// Chars is the total size of the sample text, for the consent screen's estimate.
func (s Sample) Chars() int {
	n := 0
	for _, p := range s.Prompts {
		n += len(p.Text) + len(p.Prior)
	}
	return n
}

// Fingerprint identifies this exact sample under this exact rubric, so a cached read
// can only be reused for identical evidence.
func (s Sample) Fingerprint(model string) string {
	h := sha256.New()
	fmt.Fprintf(h, "v%d\x00%s\x00", RubricVersion, model)
	for _, p := range s.Prompts {
		fmt.Fprintf(h, "%s\x00%s\x00", p.ID, p.Text)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Build selects the prompts to judge. Selection is deterministic: the seed derives
// from the corpus itself, so the same history always yields the same sample and two
// runs are comparable.
//
// Allocation across sessions is proportional to sqrt(prompts) rather than to
// prompts, so one 500-prompt session cannot drown twenty small ones.
func Build(sessions []model.Session, maxPrompts, charBudget int) Sample {
	if maxPrompts <= 0 {
		maxPrompts = DefaultMaxPrompts
	}
	if charBudget <= 0 {
		charBudget = DefaultCharBudget
	}

	var buckets []bucket
	available := 0

	for _, s := range sessions {
		var ps []Prompt
		for i, t := range s.Turns {
			if !t.Scorable() || len(t.Text) == 0 {
				continue
			}
			ps = append(ps, Prompt{
				ID:      fmt.Sprintf("%s#%d", shortID(s.ID), i),
				Text:    Redact(truncate(t.Text, maxPromptChars)),
				Prior:   Redact(truncate(priorAssistantText(s.Turns, i), priorChars)),
				session: s.ID,
			})
		}
		if len(ps) == 0 {
			continue
		}
		available += len(ps)
		buckets = append(buckets, bucket{id: s.ID, prompts: ps})
	}
	if len(buckets) == 0 {
		return Sample{}
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].id < buckets[j].id })

	ids := make([]string, len(buckets))
	for i, b := range buckets {
		ids[i] = b.id
	}
	rng := rand.New(rand.NewSource(seed(ids)))

	// Sessions are weighted by sqrt(size) so a busy project counts for more without
	// counting for everything, and then capped, because sqrt alone still lets one
	// 400-prompt session take most of the budget. The read is meant to describe the
	// developer, not their largest repository.
	weights := make([]float64, len(buckets))
	var total float64
	for i, b := range buckets {
		weights[i] = math.Sqrt(float64(len(b.prompts)))
		total += weights[i]
	}
	perSessionCap := maxPrompts
	if len(buckets) > 1 {
		perSessionCap = maxInt(2, 2*maxPrompts/len(buckets))
	}

	// Draw order within each session is fixed up front, so the redistribution pass
	// continues where the first pass stopped instead of re-picking.
	order := make([][]int, len(buckets))
	taken := make([]int, len(buckets))
	for i, b := range buckets {
		order[i] = rng.Perm(len(b.prompts))
	}

	var picked []Prompt
	chars := 0
	draw := func(i int) bool {
		if taken[i] >= len(order[i]) || len(picked) >= maxPrompts {
			return false
		}
		p := buckets[i].prompts[order[i][taken[i]]]
		if chars+len(p.Text)+len(p.Prior) > charBudget {
			return false
		}
		taken[i]++
		picked = append(picked, p)
		chars += len(p.Text) + len(p.Prior)
		return true
	}

	for i := range buckets {
		quota := minInt(int(math.Round(float64(maxPrompts)*weights[i]/total)), perSessionCap)
		quota = minInt(maxInt(quota, 1), len(buckets[i].prompts))
		for n := 0; n < quota; n++ {
			if !draw(i) {
				break
			}
		}
	}

	// Hand any budget the caps left over back out, one prompt per session per round.
	for progress := true; progress && len(picked) < maxPrompts; {
		progress = false
		for i := range buckets {
			if draw(i) {
				progress = true
			}
		}
	}

	// Stable order so the request, the cache key, and the validator agree.
	sort.Slice(picked, func(i, j int) bool { return picked[i].ID < picked[j].ID })

	covered := map[string]bool{}
	for _, p := range picked {
		covered[p.session] = true
	}
	return Sample{Prompts: picked, Sessions: len(covered), Available: available}
}

// bucket is one session's scorable prompts, the unit sampling is stratified over.
type bucket struct {
	id      string
	prompts []Prompt
}

// seed derives a stable seed from the corpus, so sampling is reproducible without
// persisting anything.
func seed(ids []string) int64 {
	h := sha256.New()
	fmt.Fprintf(h, "v%d", RubricVersion)
	for _, id := range ids {
		h.Write([]byte(id))
	}
	return int64(binary.BigEndian.Uint64(h.Sum(nil)[:8]))
}

// priorAssistantText returns the assistant turn immediately before index i.
func priorAssistantText(turns []model.Turn, i int) string {
	for j := i - 1; j >= 0; j-- {
		if turns[j].Role == model.RoleAssistant && turns[j].Text != "" {
			return turns[j].Text
		}
	}
	return ""
}

// truncate keeps the head and tail of an over-long prompt, eliding the middle, so
// both the ask and any trailing constraints survive.
func truncate(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	head := max * 2 / 3
	tail := max - head
	return string(r[:head]) + "\n…\n" + string(r[len(r)-tail:])
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
