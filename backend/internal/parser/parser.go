// Package parser is the regex/keyword fast path: spoken text -> chart commands.
// It is deterministic, allocation-light and runs in microseconds.
package parser

import (
	"regexp"
	"strconv"
	"strings"

	"perioassist/internal/chart"
)

type Result struct {
	Commands   []chart.Command
	Unparsed   []string
	Normalized string
}

var units = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6, "seven": 7,
	"eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12, "thirteen": 13,
	"fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19,
}

var tens = map[string]int{"twenty": 20, "thirty": 30}

var fillers = map[string]bool{
	"and": true, "then": true, "okay": true, "ok": true, "the": true, "a": true, "an": true,
	"um": true, "uh": true, "please": true, "on": true, "at": true, "is": true, "be": true,
	"to": true, "as": true, "of": true, "it": true, "that": true, "positive": true,
	"present": true, "yes": true, "site": true, "class": true, "grade": true,
}

func tokenize(s string) []string {
	toks := make([]string, 0, 8)
	start := -1
	for i := 0; i <= len(s); i++ {
		var ch byte
		if i < len(s) {
			ch = s[i]
		}
		isWord := (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9')
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
			isWord = true
		}
		if isWord && start < 0 {
			start = i
		}
		if !isWord && start >= 0 {
			toks = append(toks, strings.ToLower(s[start:i]))
			start = -1
		}
	}
	return toks
}

func isNumeral(t string) bool {
	if t == "" {
		return false
	}
	for _, r := range t {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// parseNum reads a full number ("14", "fourteen", "twenty one").
func parseNum(toks []string, i int) (n, used int, ok bool) {
	if i >= len(toks) {
		return 0, 0, false
	}
	t := toks[i]
	if isNumeral(t) {
		v, err := strconv.Atoi(t)
		if err != nil {
			return 0, 0, false
		}
		return v, 1, true
	}
	if v, ok := units[t]; ok {
		return v, 1, true
	}
	if v, ok := tens[t]; ok {
		if i+1 < len(toks) {
			if u, ok2 := units[toks[i+1]]; ok2 && u >= 1 && u <= 9 {
				return v + u, 2, true
			}
		}
		return v, 1, true
	}
	return 0, 0, false
}

// parseDigits reads a run of single digits: "three two four" or "324".
func parseDigits(toks []string, i int) (ds []int, used int) {
	j := i
	for j < len(toks) {
		t := toks[j]
		if isNumeral(t) {
			for _, r := range t {
				if r > '0' {
					ds = append(ds, int(r-'0'))
				}
			}
			j++
			continue
		}
		if v, ok := units[t]; ok && v >= 1 && v <= 9 {
			ds = append(ds, v)
			j++
			continue
		}
		break
	}
	return ds, j - i
}

func surfaceOf(t string) chart.Surface {
	switch t {
	case "buccal", "facial":
		return chart.Buccal
	case "lingual", "palatal":
		return chart.Lingual
	}
	return ""
}

// flagSuffix handles "site 2" and "on lingual" after a flag word.
func flagSuffix(toks []string, i int, cmd *chart.Command) int {
	for i < len(toks) {
		switch {
		case toks[i] == "site":
			if n, u, ok := parseNum(toks, i+1); ok && n >= 1 && n <= 3 {
				cmd.Site = n
				i += 1 + u
				continue
			}
		case toks[i] == "on" && i+1 < len(toks) && surfaceOf(toks[i+1]) != "":
			cmd.Surface = surfaceOf(toks[i+1])
			i += 2
			continue
		case toks[i] == "positive" || toks[i] == "present" || toks[i] == "yes":
			i++
			continue
		}
		break
	}
	return i
}

func Parse(text string) Result {
	toks := tokenize(text)
	for i, t := range toks {
		toks[i] = canon(t)
	}
	res := Result{Normalized: strings.Join(toks, " ")}
	add := func(c chart.Command) { res.Commands = append(res.Commands, c) }

	for i := 0; i < len(toks); {
		t := toks[i]
		switch t {
		case "tooth", "number", "jump", "go":
			j := i + 1
			for j < len(toks) && (toks[j] == "to" || toks[j] == "tooth" || toks[j] == "number") {
				j++
			}
			if n, u, ok := parseNum(toks, j); ok && n >= 1 && n <= 32 {
				add(chart.Command{Kind: chart.KGoto, Tooth: n})
				i = j + u
				continue
			}
			res.Unparsed = append(res.Unparsed, t)
			i++

		case "clear", "reset":
			j := i + 1
			if j < len(toks) && toks[j] == "tooth" {
				j++
			}
			cmd := chart.Command{Kind: chart.KClear}
			if n, u, ok := parseNum(toks, j); ok && n >= 1 && n <= 32 {
				cmd.Tooth = n
				j += u
			}
			add(cmd)
			i = j

		case "scratch", "undo":
			add(chart.Command{Kind: chart.KUndo})
			i++
			for i < len(toks) && (toks[i] == "that" || toks[i] == "it") {
				i++
			}

		case "make", "change", "set", "correct":
			j := i + 1
			cmd := chart.Command{Kind: chart.KSetSite}
		loop:
			for j < len(toks) {
				switch toks[j] {
				case "tooth":
					j++
					if n, u, ok := parseNum(toks, j); ok && n >= 1 && n <= 32 {
						cmd.Tooth = n
						j += u
					}
				case "site":
					j++
					if n, u, ok := parseNum(toks, j); ok && n >= 1 && n <= 3 {
						cmd.Site = n
						j += u
					}
				case "last", "that", "it", "previous", "this", "to", "a", "an", "as", "the", "be", "is", "equal", "equals":
					j++
				default:
					break loop
				}
			}
			if n, u, ok := parseNum(toks, j); ok && n >= 1 && n <= 15 {
				cmd.Value = n
				add(cmd)
				i = j + u
				continue
			}
			res.Unparsed = append(res.Unparsed, t)
			i++

		case "bleeding", "bleed", "bleeds", "bop":
			cmd := chart.Command{Kind: chart.KFlag, Flag: "bop"}
			i = flagSuffix(toks, i+1, &cmd)
			add(cmd)

		case "suppuration", "suppurating", "pus", "supp":
			cmd := chart.Command{Kind: chart.KFlag, Flag: "supp"}
			i = flagSuffix(toks, i+1, &cmd)
			add(cmd)

		case "plaque":
			cmd := chart.Command{Kind: chart.KFlag, Flag: "plaque"}
			i = flagSuffix(toks, i+1, &cmd)
			add(cmd)

		case "recession", "recede", "receded":
			j := i + 1
			for j < len(toks) && (toks[j] == "of" || toks[j] == "is") {
				j++
			}
			if n, u, ok := parseNum(toks, j); ok && n >= 0 && n <= 15 {
				cmd := chart.Command{Kind: chart.KRecession, Value: n}
				i = flagSuffix(toks, j+u, &cmd)
				add(cmd)
				continue
			}
			res.Unparsed = append(res.Unparsed, t)
			i++

		case "mobility", "mobile", "furcation":
			kind := chart.KMobility
			if t == "furcation" {
				kind = chart.KFurcation
			}
			j := i + 1
			for j < len(toks) && (toks[j] == "class" || toks[j] == "grade" || toks[j] == "is") {
				j++
			}
			if n, u, ok := parseNum(toks, j); ok && n >= 0 && n <= 3 {
				add(chart.Command{Kind: kind, Value: n})
				i = j + u
				continue
			}
			res.Unparsed = append(res.Unparsed, t)
			i++

		case "next":
			add(chart.Command{Kind: chart.KNext})
			i++

		case "buccal", "facial", "lingual", "palatal":
			add(chart.Command{Kind: chart.KSurface, Surface: surfaceOf(t)})
			i++

		case "missing", "extracted", "implant":
			add(chart.Command{Kind: chart.KMissing})
			i++

		default:
			if ds, used := parseDigits(toks, i); used > 0 {
				if len(ds) > 0 {
					add(chart.Command{Kind: chart.KDigits, Digits: ds})
				}
				i += used
				continue
			}
			if !fillers[t] {
				res.Unparsed = append(res.Unparsed, t)
			}
			i++
		}
	}
	return res
}

var cue = regexp.MustCompile(`\b(wait|actually|sorry|i mean|i meant|that was|that should be|should be|correction|no)\b`)

// NeedsLLM reports whether the utterance is a spoken correction or something the
// fast path could not understand. Clean "make site 2 a 5" / "scratch that" stay local.
func NeedsLLM(res Result) bool {
	for _, c := range res.Commands {
		if c.Kind == chart.KSetSite || c.Kind == chart.KUndo {
			return false
		}
	}
	if cue.MatchString(res.Normalized) {
		return true
	}
	return len(res.Commands) == 0 && len(res.Unparsed) > 0
}

// Heuristic is the deterministic stand-in for the LLM when it is disabled,
// rate-limited or offline: "no wait, that was a five" -> change last site to 5.
func Heuristic(res Result) []chart.Command {
	if !cue.MatchString(res.Normalized) {
		return nil
	}
	toks := strings.Fields(res.Normalized)
	for i := len(toks) - 1; i >= 0; i-- {
		if n, _, ok := parseNum(toks, i); ok && n >= 1 && n <= 15 {
			// "twenty one" style: step back if the previous token is a tens word
			if i > 0 {
				if _, isTens := tens[toks[i-1]]; isTens {
					continue
				}
			}
			return []chart.Command{{Kind: chart.KSetSite, Value: n}}
		}
	}
	return nil
}
