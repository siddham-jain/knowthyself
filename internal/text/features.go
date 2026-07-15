package text

import "strings"

// LengthBand classifies how substantive a prompt is by rune length, normalized per
// script so a compact Devanagari prompt is not judged "vague" against Latin-tuned
// thresholds. Bands: "vague" (too terse to act on), "short", "substantial", "long".
func LengthBand(s string) string {
	n := RuneLen(strings.TrimSpace(s))
	// Devanagari/other scripts pack more meaning per rune than Latin; scale the
	// thresholds down so equivalent intent lands in the same band.
	factor := 1.0
	switch DominantScript(s) {
	case "devanagari":
		factor = 0.55
	case "other":
		factor = 0.7
	}
	vague := int(15 * factor)
	short := int(45 * factor)
	long := int(600 * factor)
	switch {
	case n < vague:
		return "vague"
	case n < short:
		return "short"
	case n < long:
		return "substantial"
	default:
		return "long"
	}
}

// correctionCues are language-agnostic + multilingual markers that a user turn is a
// correction/re-prompt (dissatisfaction with the previous answer). English, Hindi
// (Devanagari), and Hinglish (Latin-script Hindi) are all covered so no language is
// penalized more than another. These only ADD a correction signal; their absence
// never lowers a score.
var correctionCues = []string{
	// English
	"no,", "no ", "not what", "that's wrong", "thats wrong", "incorrect", "actually",
	"instead", "still not", "still broken", "still failing", "again", "redo", "revert",
	"undo", "that didn't", "that did not", "doesn't work", "does not work", "wrong",
	// Hinglish (Latin-script Hindi)
	"nahi", "nahin", "galat", "phir se", "dobara", "dubara", "theek nahi", "ruko",
	"nahi chal", "kaam nahi",
	// Hindi (Devanagari)
	"नहीं", "गलत", "फिर से", "दोबारा", "ठीक नहीं", "रुको", "काम नहीं",
}

// IsCorrection reports whether a user turn reads as a correction/re-prompt. It is a
// booster signal for Iteration Efficiency; multilingual by construction.
func IsCorrection(s string) bool {
	l := strings.ToLower(strings.TrimSpace(s))
	if l == "" {
		return false
	}
	for _, cue := range correctionCues {
		if strings.Contains(l, cue) {
			return true
		}
	}
	return false
}
