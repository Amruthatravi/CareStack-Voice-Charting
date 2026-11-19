package parser

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// aliases.json: editable map of known mis-hearings / misspellings -> intended command word.
//
//go:embed aliases.json
var aliasJSON []byte

var aliases = func() map[string]string {
	m := map[string]string{}
	_ = json.Unmarshal(aliasJSON, &m)
	return m
}()

// Command words the fuzzy matcher may snap to.
var vocab = []string{
	"tooth", "number", "jump", "clear", "reset", "scratch", "undo", "make", "change",
	"bleeding", "suppuration", "plaque", "recession", "mobility", "furcation",
	"buccal", "lingual", "facial", "palatal", "missing", "implant", "extracted",
	"positive", "next", "class",
}

var known = func() map[string]bool {
	k := map[string]bool{}
	for w := range units {
		k[w] = true
	}
	for w := range tens {
		k[w] = true
	}
	for w := range fillers {
		k[w] = true
	}
	for _, w := range vocab {
		k[w] = true
	}
	for _, w := range strings.Fields("bleed bleeds bop supp pus suppurating recede receded mobile go set correct " +
		"last previous this equal equals no wait actually sorry mean meant should was correction that site") {
		k[w] = true
	}
	return k
}()

// canon maps one token to its canonical form: alias file first, then (for words
// that are not already known) the closest command word within a strict edit distance.
func canon(t string) string {
	if known[t] || isNumeral(t) {
		return t
	}
	if a, ok := aliases[t]; ok {
		return a
	}
	if len(t) < 5 {
		return t
	}
	max := 1
	if len(t) >= 8 {
		max = 2
	}
	best, bd := "", max+1
	for _, w := range vocab {
		if d := lev(t, w, max); d < bd {
			best, bd = w, d
		}
	}
	if best != "" {
		return best
	}
	return t
}

// lev is a bounded Levenshtein distance (returns max+1 when it exceeds max).
func lev(a, b string, max int) int {
	if d := len(a) - len(b); d > max || -d > max {
		return max + 1
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= len(b); j++ {
			c := 1
			if a[i-1] == b[j-1] {
				c = 0
			}
			v := prev[j-1] + c
			if x := prev[j] + 1; x < v {
				v = x
			}
			if x := cur[j-1] + 1; x < v {
				v = x
			}
			cur[j] = v
			if v < rowMin {
				rowMin = v
			}
		}
		if rowMin > max {
			return max + 1
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}
