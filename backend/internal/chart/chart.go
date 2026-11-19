// Package chart is the deterministic periodontal-chart state machine.
// All mutations are recorded as Changes so every one can be undone and audited.
package chart

import (
	"errors"
	"fmt"
	"strings"
)

type Surface string

const (
	Buccal  Surface = "buccal"
	Lingual Surface = "lingual"
)

// Fields that can change.
const (
	FPD        = "pd"
	FRec       = "rec"
	FBOP       = "bop"
	FSupp      = "supp"
	FPlaque    = "plaque"
	FMobility  = "mobility"
	FFurcation = "furcation"
	FMissing   = "missing"
)

// Command kinds.
const (
	KGoto      = "goto"
	KDigits    = "digits"
	KFlag      = "flag"
	KRecession = "recession"
	KMobility  = "mobility"
	KFurcation = "furcation"
	KSetSite   = "setsite"
	KUndo      = "undo"
	KNext      = "next"
	KSurface   = "surface"
	KMissing   = "missing"
	KClear     = "clear"
)

var (
	ErrEnd         = errors.New("end of arch reached: say 'tooth N', 'next' or switch surface")
	ErrNoTarget    = errors.New("no site to apply that to: chart a tooth first or say 'site 1/2/3'")
	ErrNothingUndo = errors.New("nothing to undo")
)

type Side struct {
	PD     [3]int  `json:"pd"`
	Rec    [3]int  `json:"rec"`
	BOP    [3]bool `json:"bop"`
	Supp   [3]bool `json:"supp"`
	Plaque [3]bool `json:"plaque"`
}

type Tooth struct {
	Buccal    Side `json:"buccal"`
	Lingual   Side `json:"lingual"`
	Mobility  int  `json:"mobility"`
	Furcation int  `json:"furcation"`
	Missing   bool `json:"missing"`
}

type Cursor struct {
	Tooth   int     `json:"tooth"`
	Surface Surface `json:"surface"`
	Site    int     `json:"site"` // 0..2 next site to be written
	End     bool    `json:"end"`
}

type Ref struct {
	Tooth   int     `json:"tooth"`
	Surface Surface `json:"surface"`
	Site    int     `json:"site"`
}

type Change struct {
	Tooth   int     `json:"tooth"`
	Surface Surface `json:"surface,omitempty"`
	Site    int     `json:"site"` // 0-based
	Field   string  `json:"field"`
	Old     int     `json:"old"`
	New     int     `json:"new"`
}

type Entry struct {
	Kind       string   `json:"kind"`
	Changes    []Change `json:"changes"`
	PrevCursor Cursor   `json:"prevCursor"`
	PrevLast   *Ref     `json:"prevLast,omitempty"`
	PrevNav    bool     `json:"prevNav,omitempty"`
}

type Command struct {
	Kind    string  `json:"kind"`
	Tooth   int     `json:"tooth,omitempty"`
	Site    int     `json:"site,omitempty"` // 1..3
	Value   int     `json:"value,omitempty"`
	Digits  []int   `json:"digits,omitempty"`
	Flag    string  `json:"flag,omitempty"`
	Surface Surface `json:"surface,omitempty"`
}

type Chart struct {
	Teeth   map[int]*Tooth `json:"teeth"`
	Cursor  Cursor         `json:"cursor"`
	Last    *Ref           `json:"last,omitempty"`
	Nav     bool           `json:"nav,omitempty"` // cursor moved by voice since the last write
	History []Entry        `json:"-"`
}

// Order is the charting order: upper right to left, then lower left to right.
var Order = func() []int {
	o := make([]int, 0, 32)
	for i := 1; i <= 16; i++ {
		o = append(o, i)
	}
	for i := 32; i >= 17; i-- {
		o = append(o, i)
	}
	return o
}()

func New() *Chart {
	return &Chart{Teeth: map[int]*Tooth{}, Cursor: Cursor{Tooth: 1, Surface: Buccal}}
}

func (c *Chart) tooth(n int) *Tooth {
	t, ok := c.Teeth[n]
	if !ok {
		t = &Tooth{}
		c.Teeth[n] = t
	}
	return t
}

func side(t *Tooth, s Surface) *Side {
	if s == Lingual {
		return &t.Lingual
	}
	return &t.Buccal
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (c *Chart) get(tn int, s Surface, site int, f string) int {
	t, ok := c.Teeth[tn]
	if !ok {
		return 0
	}
	switch f {
	case FMobility:
		return t.Mobility
	case FFurcation:
		return t.Furcation
	case FMissing:
		return b2i(t.Missing)
	}
	sd := side(t, s)
	switch f {
	case FPD:
		return sd.PD[site]
	case FRec:
		return sd.Rec[site]
	case FBOP:
		return b2i(sd.BOP[site])
	case FSupp:
		return b2i(sd.Supp[site])
	case FPlaque:
		return b2i(sd.Plaque[site])
	}
	return 0
}

func (c *Chart) put(tn int, s Surface, site int, f string, v int) {
	t := c.tooth(tn)
	switch f {
	case FMobility:
		t.Mobility = v
		return
	case FFurcation:
		t.Furcation = v
		return
	case FMissing:
		t.Missing = v != 0
		return
	}
	sd := side(t, s)
	switch f {
	case FPD:
		sd.PD[site] = v
	case FRec:
		sd.Rec[site] = v
	case FBOP:
		sd.BOP[site] = v != 0
	case FSupp:
		sd.Supp[site] = v != 0
	case FPlaque:
		sd.Plaque[site] = v != 0
	}
}

func (c *Chart) begin(kind string) *Entry {
	e := &Entry{Kind: kind, PrevCursor: c.Cursor, PrevNav: c.Nav}
	if c.Last != nil {
		l := *c.Last
		e.PrevLast = &l
	}
	return e
}

func (c *Chart) set(e *Entry, tn int, s Surface, site int, f string, v int) {
	old := c.get(tn, s, site, f)
	if old == v {
		return
	}
	c.put(tn, s, site, f, v)
	e.Changes = append(e.Changes, Change{Tooth: tn, Surface: s, Site: site, Field: f, Old: old, New: v})
}

func (c *Chart) commit(e *Entry) {
	if len(e.Changes) == 0 {
		return
	}
	c.History = append(c.History, *e)
	if len(c.History) > 200 {
		c.History = c.History[len(c.History)-200:]
	}
}

func (c *Chart) nextTooth(from int) int {
	idx := -1
	for i, t := range Order {
		if t == from {
			idx = i
			break
		}
	}
	for j := idx + 1; j < len(Order); j++ {
		if t, ok := c.Teeth[Order[j]]; ok && t.Missing {
			continue
		}
		return Order[j]
	}
	return 0
}

func (c *Chart) advance() {
	c.Cursor.Site++
	if c.Cursor.Site < 3 {
		return
	}
	c.Cursor.Site = 0
	if n := c.nextTooth(c.Cursor.Tooth); n == 0 {
		c.Cursor.End = true
	} else {
		c.Cursor.Tooth = n
	}
}

func validTooth(n int) bool { return n >= 1 && n <= 32 }

// target resolves which site a flag / correction applies to. site is -1 when the
// whole surface of a tooth is meant (e.g. "tooth 14 bleeding" with no site spoken).
func (c *Chart) target(cmd Command) (int, Surface, int, error) {
	tn, s, site := c.Cursor.Tooth, c.Cursor.Surface, -1
	if c.Last != nil && !c.Nav {
		tn, s, site = c.Last.Tooth, c.Last.Surface, c.Last.Site
	}
	if cmd.Tooth != 0 {
		if c.Last == nil || c.Last.Tooth != cmd.Tooth || c.Nav {
			s, site = c.Cursor.Surface, -1
		}
		tn = cmd.Tooth
	}
	if cmd.Surface != "" {
		s = cmd.Surface
	}
	if cmd.Site != 0 {
		site = cmd.Site - 1
	}
	if !validTooth(tn) {
		return 0, "", 0, fmt.Errorf("invalid tooth %d", tn)
	}
	if site > 2 {
		return 0, "", 0, ErrNoTarget
	}
	return tn, s, site, nil
}

func (c *Chart) toothOf(cmd Command) int {
	switch {
	case cmd.Tooth != 0:
		return cmd.Tooth
	case c.Last != nil && !c.Nav:
		return c.Last.Tooth
	}
	return c.Cursor.Tooth
}

// Apply executes one command. The returned Entry (nil for pure navigation)
// carries the Changes that were made; the string is a short spoken summary.
func (c *Chart) Apply(cmd Command) (*Entry, string, error) {
	switch cmd.Kind {
	case KUndo:
		e, err := c.Undo()
		if err != nil {
			return nil, "", err
		}
		return e, "undone", nil

	case KGoto:
		if !validTooth(cmd.Tooth) {
			return nil, "", fmt.Errorf("invalid tooth %d", cmd.Tooth)
		}
		c.Cursor.Tooth, c.Cursor.Site, c.Cursor.End = cmd.Tooth, 0, false
		c.Nav = true
		return nil, fmt.Sprintf("tooth %d", cmd.Tooth), nil

	case KNext:
		c.Nav = true
		c.Cursor.Site = 0
		if n := c.nextTooth(c.Cursor.Tooth); n == 0 {
			c.Cursor.End = true
		} else {
			c.Cursor.Tooth = n
		}
		return nil, fmt.Sprintf("tooth %d", c.Cursor.Tooth), nil

	case KSurface:
		c.Cursor.Surface, c.Cursor.Site, c.Cursor.End = cmd.Surface, 0, false
		c.Nav = true
		return nil, string(cmd.Surface), nil

	case KDigits:
		if c.Cursor.End {
			return nil, "", ErrEnd
		}
		e := c.begin(KDigits)
		c.Nav = false
		var written []string
		for _, d := range cmd.Digits {
			if c.Cursor.End {
				break
			}
			if d < 1 || d > 15 {
				continue
			}
			cu := c.Cursor
			c.set(e, cu.Tooth, cu.Surface, cu.Site, FPD, d)
			c.Last = &Ref{Tooth: cu.Tooth, Surface: cu.Surface, Site: cu.Site}
			written = append(written, fmt.Sprint(d))
			c.advance()
		}
		c.commit(e)
		return e, strings.Join(written, " "), nil

	case KSetSite:
		if cmd.Value < 1 || cmd.Value > 15 {
			return nil, "", fmt.Errorf("pocket depth %d out of range", cmd.Value)
		}
		tn, s, site, err := c.target(cmd)
		if err != nil {
			return nil, "", err
		}
		if site < 0 {
			return nil, "", ErrNoTarget
		}
		e := c.begin(KSetSite)
		c.Nav = false
		c.set(e, tn, s, site, FPD, cmd.Value)
		c.Last = &Ref{Tooth: tn, Surface: s, Site: site}
		c.commit(e)
		return e, fmt.Sprintf("site %d is %d", site+1, cmd.Value), nil

	case KFlag:
		f := map[string]string{"bop": FBOP, "supp": FSupp, "plaque": FPlaque}[cmd.Flag]
		if f == "" {
			return nil, "", fmt.Errorf("unknown flag %q", cmd.Flag)
		}
		tn, s, site, err := c.target(cmd)
		if err != nil {
			return nil, "", err
		}
		e := c.begin(KFlag)
		if site < 0 {
			for i := 0; i < 3; i++ {
				c.set(e, tn, s, i, f, 1)
			}
		} else {
			c.set(e, tn, s, site, f, 1)
		}
		c.commit(e)
		return e, cmd.Flag, nil

	case KRecession:
		if cmd.Value < 0 || cmd.Value > 15 {
			return nil, "", fmt.Errorf("recession %d out of range", cmd.Value)
		}
		tn, s, site, err := c.target(cmd)
		if err != nil {
			return nil, "", err
		}
		if site < 0 {
			return nil, "", ErrNoTarget
		}
		e := c.begin(KRecession)
		c.set(e, tn, s, site, FRec, cmd.Value)
		c.commit(e)
		return e, fmt.Sprintf("recession %d", cmd.Value), nil

	case KMobility, KFurcation:
		f := FMobility
		if cmd.Kind == KFurcation {
			f = FFurcation
		}
		if cmd.Value < 0 || cmd.Value > 3 {
			return nil, "", fmt.Errorf("%s %d out of range (0-3)", cmd.Kind, cmd.Value)
		}
		tn := c.toothOf(cmd)
		e := c.begin(cmd.Kind)
		c.set(e, tn, "", 0, f, cmd.Value)
		c.commit(e)
		return e, fmt.Sprintf("%s %d", cmd.Kind, cmd.Value), nil

	case KMissing:
		tn := cmd.Tooth
		if tn == 0 {
			tn = c.Cursor.Tooth
		}
		if !validTooth(tn) {
			return nil, "", fmt.Errorf("invalid tooth %d", tn)
		}
		e := c.begin(KMissing)
		c.set(e, tn, "", 0, FMissing, 1)
		if c.Cursor.Tooth == tn {
			c.Cursor.Site = 0
			if n := c.nextTooth(tn); n == 0 {
				c.Cursor.End = true
			} else {
				c.Cursor.Tooth = n
			}
		}
		c.commit(e)
		return e, fmt.Sprintf("tooth %d missing", tn), nil

	case KClear:
		tn := cmd.Tooth
		if tn == 0 {
			tn = c.Cursor.Tooth
		}
		if !validTooth(tn) {
			return nil, "", fmt.Errorf("invalid tooth %d", tn)
		}
		e := c.begin(KClear)
		for _, s := range []Surface{Buccal, Lingual} {
			for site := 0; site < 3; site++ {
				for _, f := range []string{FPD, FRec, FBOP, FSupp, FPlaque} {
					c.set(e, tn, s, site, f, 0)
				}
			}
		}
		c.set(e, tn, "", 0, FMobility, 0)
		c.set(e, tn, "", 0, FFurcation, 0)
		c.set(e, tn, "", 0, FMissing, 0)
		if c.Last != nil && c.Last.Tooth == tn {
			c.Last = nil
		}
		c.commit(e)
		return e, fmt.Sprintf("cleared tooth %d", tn), nil
	}
	return nil, "", fmt.Errorf("unknown command %q", cmd.Kind)
}

// Undo reverts the most recent data-changing entry. It returns an Entry whose
// Changes are the inverse of the reverted one (for the audit log).
func (c *Chart) Undo() (*Entry, error) {
	if len(c.History) == 0 {
		return nil, ErrNothingUndo
	}
	e := c.History[len(c.History)-1]
	c.History = c.History[:len(c.History)-1]
	inv := &Entry{Kind: KUndo}
	for i := len(e.Changes) - 1; i >= 0; i-- {
		ch := e.Changes[i]
		c.put(ch.Tooth, ch.Surface, ch.Site, ch.Field, ch.Old)
		inv.Changes = append(inv.Changes, Change{Tooth: ch.Tooth, Surface: ch.Surface, Site: ch.Site, Field: ch.Field, Old: ch.New, New: ch.Old})
	}
	c.Cursor = e.PrevCursor
	c.Last = e.PrevLast
	c.Nav = e.PrevNav
	return inv, nil
}
