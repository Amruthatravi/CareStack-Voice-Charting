package parser

import (
	"errors"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type ChartEvent struct {
	SessionID    string         `json:"session_id"`
	TenantID     string         `json:"tenant_id"`
	PatientID    string         `json:"patient_id"`
	VisitID      string         `json:"visit_id,omitempty"`
	EventType    string         `json:"event_type"` // "measurement"|"correction"|"navigate"|"transcript"
	ToothNum     int            `json:"tooth_num"`
	Surface      string         `json:"surface"`
	Measurements map[string]any `json:"measurements,omitempty"`
	Confidence   float64        `json:"confidence"`
	SourceText   string         `json:"source_text"`
	IsPartial    bool           `json:"is_partial"`
	Version      int64          `json:"version"`
	Source       string         `json:"source"` // "regex" | "groq"
	Timestamp    time.Time      `json:"timestamp"`
}

type SessionContext struct {
	SessionID     string
	TenantID      string
	PatientID     string
	VisitID       string
	ActiveTooth   int
	ActiveSurface string
	Version       int64
}

type PatternLibrary struct {
	ThreeReadings     *regexp.Regexp
	ThreeWordReadings *regexp.Regexp
	ToothNumber       *regexp.Regexp
	ToothNumberWord   *regexp.Regexp
	Bleeding          *regexp.Regexp
	Recession         *regexp.Regexp
	Furcation         *regexp.Regexp
	Mobility          *regexp.Regexp
	Suppuration       *regexp.Regexp
	Plaque            *regexp.Regexp
	SiteCorrection    *regexp.Regexp
	GenericCorrection *regexp.Regexp
	Navigate          *regexp.Regexp
	Surface           *regexp.Regexp
	wordToNumber      map[string]int
	toothWordToNumber map[string]int
}

var ErrNoMatch = errors.New("no pattern matched")

// buildToothWordAlternation builds the set of spoken tooth-number phrases
// (e.g. "fourteen", "twenty two", "thirty-two") valid for the universal
// numbering system (1-32), plus a regex alternation string that matches
// any of them. Longer (compound) phrases are ordered first so that, e.g.,
// "twenty two" is matched in full rather than stopping at "two".
func buildToothWordAlternation() (map[string]int, string) {
	type numWord struct {
		phrase string
		value  int
	}

	base := []numWord{
		{"one", 1}, {"two", 2}, {"three", 3}, {"four", 4}, {"five", 5},
		{"six", 6}, {"seven", 7}, {"eight", 8}, {"nine", 9}, {"ten", 10},
		{"eleven", 11}, {"twelve", 12}, {"thirteen", 13}, {"fourteen", 14},
		{"fifteen", 15}, {"sixteen", 16}, {"seventeen", 17}, {"eighteen", 18},
		{"nineteen", 19}, {"twenty", 20}, {"thirty", 30},
	}

	tens := []numWord{{"twenty", 20}, {"thirty", 30}}
	ones := []numWord{
		{"one", 1}, {"two", 2}, {"three", 3}, {"four", 4}, {"five", 5},
		{"six", 6}, {"seven", 7}, {"eight", 8}, {"nine", 9},
	}
	for _, t := range tens {
		for _, o := range ones {
			v := t.value + o.value
			if v <= 32 {
				base = append(base, numWord{t.phrase + " " + o.phrase, v})
			}
		}
	}

	all := make(map[string]int, len(base))
	valid := make([]numWord, 0, len(base))
	for _, nw := range base {
		if nw.value >= 1 && nw.value <= 32 {
			all[nw.phrase] = nw.value
			valid = append(valid, nw)
		}
	}

	// Longest phrase first so compound words ("twenty two") aren't
	// short-circuited by a shorter alternative earlier in the list.
	sort.Slice(valid, func(i, j int) bool {
		return len(valid[i].phrase) > len(valid[j].phrase)
	})

	parts := make([]string, len(valid))
	for i, nw := range valid {
		esc := regexp.QuoteMeta(nw.phrase)
		esc = strings.ReplaceAll(esc, " ", `[-\s]+`) // allow "twenty two" or "twenty-two"
		parts[i] = esc
	}

	return all, strings.Join(parts, "|")
}

func NewPatternLibrary() *PatternLibrary {
	toothWordToNumber, toothWordAlt := buildToothWordAlternation()

	return &PatternLibrary{
		// Numeric triplets: "3 2 4" or "3-2-4" or "3,2,4"
		ThreeReadings: regexp.MustCompile(`\b([0-9]|1[0-9])\s*[-,/]?\s*([0-9]|1[0-9])\s*[-,/]?\s*([0-9]|1[0-9])\b`),
		// Word-number triplets: "three two four" or "three-two-four"
		ThreeWordReadings: regexp.MustCompile(`(?i)\b(zero|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve)[-\s]+(zero|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve)[-\s]+(zero|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve)\b`),
		ToothNumber:       regexp.MustCompile(`(?i)\b(?:tooth|number|#|no\.?|upper|lower)\s*([1-9]|[12][0-9]|3[0-2])\b`),
		ToothNumberWord:   regexp.MustCompile(`(?i)\b(?:tooth|number|no\.?|upper|lower)\s+(` + toothWordAlt + `)\b`),
		Bleeding:          regexp.MustCompile(`(?i)\b(bleeding|bop|bleeds?|hemorrhag|bled|blood)\b`),
		Recession:         regexp.MustCompile(`(?i)(?:recession|recede[sd]?)\s+(\d+|zero|one|two|three|four|five|six|seven|eight|nine)|(\d+|zero|one|two|three|four|five|six|seven|eight|nine)\s*mm?\s+recession`),
		Furcation:         regexp.MustCompile(`(?i)furcation\s*(?:class|grade|involvement|level)?\s*([123iIvV]+)`),
		Mobility:          regexp.MustCompile(`(?i)mobilit[yies]+\s*(?:grade|class|level)?\s*([0-3]|zero|one|two|three)`),
		Suppuration:       regexp.MustCompile(`(?i)\b(suppuration|pus|exudate|purulent)\b`),
		Plaque:            regexp.MustCompile(`(?i)\b(plaque)\s*(positive|negative|present|absent|pos|neg)?\b`),
		SiteCorrection:    regexp.MustCompile(`(?i)(?:make|change|set|site|reading|point)\s+(?:site|reading|point\s+)?(\d|one|two|three)\s+(?:to|is|a|as)\s+(\d+)`),
		GenericCorrection: regexp.MustCompile(`(?i)\b(scratch\s+that|correction|no\s+wait|undo|change|incorrect|wrong|sorry\s+that)\b`),
		Navigate:          regexp.MustCompile(`(?i)\b(next|skip|move\s+on|advance|done\s+with|continue|proceed)\b`),
		Surface:           regexp.MustCompile(`(?i)\b(buccal|lingual|palatal|mesial|distal|facial)\b`),
		wordToNumber: map[string]int{
			"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4,
			"five": 5, "six": 6, "seven": 7, "eight": 8, "nine": 9,
			"ten": 10, "eleven": 11, "twelve": 12,
		},
		toothWordToNumber: toothWordToNumber,
	}
}

func (pl *PatternLibrary) resolveNum(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if val, err := strconv.Atoi(s); err == nil {
		return val, true
	}
	if val, ok := pl.wordToNumber[s]; ok {
		return val, true
	}
	return 0, false
}

// resolveToothWord normalizes a spoken tooth-number phrase (which may use a
// hyphen or space between compound words, e.g. "thirty-two" / "thirty two")
// and looks it up against the valid 1-32 tooth-number vocabulary.
func (pl *PatternLibrary) resolveToothWord(s string) (int, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.Join(strings.Fields(s), " ")
	val, ok := pl.toothWordToNumber[s]
	return val, ok
}

func (pl *PatternLibrary) romanToInt(s string) int {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "1", "i":
		return 1
	case "2", "ii":
		return 2
	case "3", "iii":
		return 3
	}
	return 0
}

func (pl *PatternLibrary) Parse(transcript string, ctx *SessionContext) (*ChartEvent, error) {
	matched := false
	event := &ChartEvent{
		SessionID:    ctx.SessionID,
		TenantID:     ctx.TenantID,
		PatientID:    ctx.PatientID,
		VisitID:      ctx.VisitID,
		SourceText:   transcript,
		Measurements: make(map[string]any),
		Timestamp:    time.Now(),
		Confidence:   1.0,
		Source:       "regex",
	}

	// ── Corrections (check first — highest priority) ───────────
	if pl.GenericCorrection.MatchString(transcript) {
		matched = true
		event.EventType = "correction"
		event.Confidence = 0.97
		if matches := pl.SiteCorrection.FindStringSubmatch(transcript); matches != nil {
			idx, _ := pl.resolveNum(matches[1])
			val, _ := pl.resolveNum(matches[2])
			event.Measurements["site_correction"] = map[string]int{
				"site_index": idx - 1,
				"new_value":  val,
			}
		} else {
			event.Measurements["undo_last"] = true
		}
	}

	// ── Tooth navigation ────────────────────────────────────────
	// STT frequently transcribes tooth numbers as words ("tooth thirty
	// two") rather than digits, so try the digit form first and fall
	// back to the spoken-word form before giving up.
	if matches := pl.ToothNumber.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if val, ok := pl.resolveNum(matches[1]); ok && val >= 1 && val <= 32 {
			ctx.ActiveTooth = val
			if event.EventType == "" {
				event.EventType = "navigate"
			}
		}
	} else if matches := pl.ToothNumberWord.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if val, ok := pl.resolveToothWord(matches[1]); ok && val >= 1 && val <= 32 {
			ctx.ActiveTooth = val
			if event.EventType == "" {
				event.EventType = "navigate"
			}
		}
	}

	// ── Surface ──────────────────────────────────────────────────
	if matches := pl.Surface.FindStringSubmatch(transcript); matches != nil {
		matched = true
		srf := strings.ToLower(matches[1])
		if srf == "palatal" || srf == "facial" {
			srf = "lingual"
		}
		ctx.ActiveSurface = srf
	}

	// ── Pocket depth — numeric triplets ─────────────────────────
	if matches := pl.ThreeReadings.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		r1, _ := strconv.Atoi(matches[1])
		r2, _ := strconv.Atoi(matches[2])
		r3, _ := strconv.Atoi(matches[3])
		event.Measurements["pocket_depth"] = []int{r1, r2, r3}
	} else if matches := pl.ThreeWordReadings.FindStringSubmatch(transcript); matches != nil {
		// Word-number triplets: "three two four"
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		r1, _ := pl.resolveNum(matches[1])
		r2, _ := pl.resolveNum(matches[2])
		r3, _ := pl.resolveNum(matches[3])
		event.Measurements["pocket_depth"] = []int{r1, r2, r3}
	}

	// ── Bleeding ─────────────────────────────────────────────────
	if pl.Bleeding.MatchString(transcript) {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		event.Measurements["bleeding"] = true
	}

	// ── Recession ────────────────────────────────────────────────
	if matches := pl.Recession.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		valStr := matches[1]
		if valStr == "" {
			valStr = matches[2]
		}
		if val, ok := pl.resolveNum(valStr); ok {
			event.Measurements["recession"] = val
		}
	}

	// ── Furcation ────────────────────────────────────────────────
	if matches := pl.Furcation.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		f := pl.romanToInt(matches[1])
		if f >= 1 && f <= 3 {
			event.Measurements["furcation"] = f
		}
	}

	// ── Mobility ─────────────────────────────────────────────────
	if matches := pl.Mobility.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		if val, ok := pl.resolveNum(matches[1]); ok {
			event.Measurements["mobility"] = val
		}
	}

	// ── Suppuration ──────────────────────────────────────────────
	if pl.Suppuration.MatchString(transcript) {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		event.Measurements["suppuration"] = true
	}

	// ── Plaque ───────────────────────────────────────────────────
	if matches := pl.Plaque.FindStringSubmatch(transcript); matches != nil {
		matched = true
		if event.EventType == "" || event.EventType == "navigate" {
			event.EventType = "measurement"
		}
		negWords := []string{"negative", "absent", "neg"}
		plaqueVal := true
		if len(matches) > 2 && matches[2] != "" {
			for _, w := range negWords {
				if strings.EqualFold(matches[2], w) {
					plaqueVal = false
					break
				}
			}
		}
		event.Measurements["plaque"] = plaqueVal
	}

	// ── Navigate / advance ───────────────────────────────────────
	if pl.Navigate.MatchString(transcript) {
		matched = true
		if event.EventType == "" {
			event.EventType = "navigate"
			event.Measurements["auto_advance"] = true
		}
	}

	if !matched {
		return nil, ErrNoMatch
	}

	event.ToothNum = ctx.ActiveTooth
	event.Surface = ctx.ActiveSurface
	ctx.Version++
	event.Version = ctx.Version

	return event, nil
}
