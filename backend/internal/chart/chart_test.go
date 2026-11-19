package chart

import "testing"

func TestTripletAdvanceAndFlag(t *testing.T) {
	c := New()
	c.Apply(Command{Kind: KDigits, Digits: []int{3, 2, 4}})
	if c.Teeth[1].Buccal.PD != [3]int{3, 2, 4} || c.Cursor.Tooth != 2 {
		t.Fatalf("state: %+v cursor %+v", c.Teeth[1], c.Cursor)
	}
	c.Apply(Command{Kind: KFlag, Flag: "bop"})
	if !c.Teeth[1].Buccal.BOP[2] {
		t.Fatal("bop should land on the last-written site of tooth 1")
	}
}

func TestSingleDigitsAcrossTeeth(t *testing.T) {
	c := New()
	c.Apply(Command{Kind: KDigits, Digits: []int{3, 2}})
	c.Apply(Command{Kind: KDigits, Digits: []int{4, 5}})
	if c.Teeth[1].Buccal.PD != [3]int{3, 2, 4} || c.Teeth[2].Buccal.PD[0] != 5 {
		t.Fatalf("got %+v %+v", c.Teeth[1], c.Teeth[2])
	}
}

func TestCorrectionAndUndo(t *testing.T) {
	c := New()
	c.Apply(Command{Kind: KGoto, Tooth: 14})
	c.Apply(Command{Kind: KDigits, Digits: []int{3, 2, 3}})
	c.Apply(Command{Kind: KSetSite, Value: 5}) // "that was a five"
	if c.Teeth[14].Buccal.PD[2] != 5 {
		t.Fatalf("correction failed: %+v", c.Teeth[14])
	}
	c.Apply(Command{Kind: KUndo})
	if c.Teeth[14].Buccal.PD[2] != 3 {
		t.Fatalf("undo of correction failed: %+v", c.Teeth[14])
	}
	c.Apply(Command{Kind: KUndo})
	if c.Teeth[14].Buccal.PD != [3]int{} || c.Cursor.Tooth != 14 || c.Cursor.Site != 0 {
		t.Fatalf("undo of triplet failed: %+v cursor %+v", c.Teeth[14], c.Cursor)
	}
	if _, _, err := c.Apply(Command{Kind: KUndo}); err != ErrNothingUndo {
		t.Fatalf("want ErrNothingUndo, got %v", err)
	}
}

func TestMissingSkippedAndEnd(t *testing.T) {
	c := New()
	c.Apply(Command{Kind: KGoto, Tooth: 2})
	c.Apply(Command{Kind: KMissing})
	if c.Cursor.Tooth != 3 || !c.Teeth[2].Missing {
		t.Fatalf("missing should skip to 3: %+v", c.Cursor)
	}
	c.Apply(Command{Kind: KGoto, Tooth: 17})
	c.Apply(Command{Kind: KDigits, Digits: []int{1, 1, 1}})
	if !c.Cursor.End {
		t.Fatal("expected end of arch")
	}
	if _, _, err := c.Apply(Command{Kind: KDigits, Digits: []int{2}}); err != ErrEnd {
		t.Fatalf("want ErrEnd got %v", err)
	}
}

func TestFlagWithoutHistoryUsesCursorTooth(t *testing.T) {
	c := New()
	if _, _, err := c.Apply(Command{Kind: KFlag, Flag: "bop"}); err != nil || c.Teeth[1].Buccal.BOP != [3]bool{true, true, true} {
		t.Fatalf("err=%v tooth=%+v", err, c.Teeth[1])
	}
}

func TestGotoThenBleedingHitsThatTooth(t *testing.T) {
	c := New()
	c.Apply(Command{Kind: KDigits, Digits: []int{3, 2, 3}}) // tooth 1 charted
	c.Apply(Command{Kind: KGoto, Tooth: 14})
	c.Apply(Command{Kind: KFlag, Flag: "bop"}) // "tooth 14 bleeding"
	if c.Teeth[14] == nil || c.Teeth[14].Buccal.BOP != [3]bool{true, true, true} {
		t.Fatalf("tooth 14 should bleed on all sites: %+v", c.Teeth[14])
	}
	if c.Teeth[1].Buccal.BOP != [3]bool{} {
		t.Fatalf("tooth 1 must be untouched: %+v", c.Teeth[1].Buccal)
	}
	c.Apply(Command{Kind: KUndo})
	if c.Teeth[14].Buccal.BOP != [3]bool{} {
		t.Fatal("undo should clear it")
	}
}
