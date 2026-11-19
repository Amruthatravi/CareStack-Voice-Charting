package parser

import (
	"reflect"
	"testing"

	"perioassist/internal/chart"
)

func kinds(r Result) []string {
	var k []string
	for _, c := range r.Commands {
		k = append(k, c.Kind)
	}
	return k
}

func TestFastPath(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		nllm bool
	}{
		{"tooth 14", []string{"goto"}, false},
		{"tooth fourteen three two four", []string{"goto", "digits"}, false},
		{"three two four", []string{"digits"}, false},
		{"324", []string{"digits"}, false},
		{"bleeding", []string{"flag"}, false},
		{"three two three bleeding", []string{"digits", "flag"}, false},
		{"recession two", []string{"recession"}, false},
		{"furcation class two", []string{"furcation"}, false},
		{"mobility one", []string{"mobility"}, false},
		{"plaque positive", []string{"flag"}, false},
		{"scratch that", []string{"undo"}, false},
		{"make site 2 a 5", []string{"setsite"}, false},
		{"change last to 5", []string{"setsite"}, false},
		{"tooth thirty one missing", []string{"goto", "missing"}, false},
		{"no wait that was a five", []string{"digits"}, true},
		{"sorry blorp", nil, true},
	}
	for _, c := range cases {
		r := Parse(c.in)
		if got := kinds(r); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q: kinds %v, want %v", c.in, got, c.want)
		}
		if got := NeedsLLM(r); got != c.nllm {
			t.Errorf("%q: NeedsLLM=%v, want %v", c.in, got, c.nllm)
		}
	}
}

func TestDetails(t *testing.T) {
	r := Parse("make site 2 a 5")
	if c := r.Commands[0]; c.Site != 2 || c.Value != 5 {
		t.Errorf("setsite: %+v", c)
	}
	r = Parse("tooth twenty one")
	if c := r.Commands[0]; c.Tooth != 21 {
		t.Errorf("goto: %+v", c)
	}
	r = Parse("bleeding on lingual site 2")
	if c := r.Commands[0]; c.Surface != chart.Lingual || c.Site != 2 {
		t.Errorf("flag suffix: %+v", c)
	}
}

func TestHeuristic(t *testing.T) {
	h := Heuristic(Parse("no wait that was a five"))
	if len(h) != 1 || h[0].Kind != chart.KSetSite || h[0].Value != 5 {
		t.Errorf("heuristic: %+v", h)
	}
}

func TestAliasesAndFuzzy(t *testing.T) {
	for _, in := range []string{"seperation", "sepuration", "suppuratoin"} {
		r := Parse(in)
		if len(r.Commands) != 1 || r.Commands[0].Flag != "supp" {
			t.Errorf("%q -> %+v", in, r.Commands)
		}
	}
	for _, in := range []string{"banana", "hello there", "tomorrow"} {
		if r := Parse(in); len(r.Commands) != 0 {
			t.Errorf("%q should not parse: %+v", in, r.Commands)
		}
	}
}
