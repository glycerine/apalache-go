package apalache

import "testing"

func TestGroupElementPointersAndPointingSets(t *testing.T) {
	intType := BasicCellType{Type: IntT1}
	setType := BasicCellType{Type: mustSet(IntT1)}
	a, b := NewArenaCell(1, intType), NewArenaCell(2, intType)
	left, right := NewArenaCell(3, setType), NewArenaCell(4, setType)
	pA := FixedElementPointer{Cell: a}
	pB := SMTElementPointer{Cell: b, Expression: NewNameExpr("inB", Typed(BoolT1))}
	grouped, sets := GroupPointersAndSets([]SetElementPointers{
		{Set: left, Pointers: []ElementPointer{pA, pB}},
		{Set: right, Pointers: []ElementPointer{pA}},
	})
	if len(grouped[a.ID]) != 2 || len(grouped[b.ID]) != 1 {
		t.Fatalf("pointer groups = %#v", grouped)
	}
	if len(sets[a.ID]) != 2 || !sets[a.ID][0].Equal(left) || !sets[a.ID][1].Equal(right) {
		t.Fatalf("pointing sets for a = %#v", sets[a.ID])
	}
	if len(sets[b.ID]) != 1 || !sets[b.ID][0].Equal(left) {
		t.Fatalf("pointing sets for b = %#v", sets[b.ID])
	}
}

func TestMergeElementPointers(t *testing.T) {
	cell := NewArenaCell(1, BasicCellType{Type: IntT1})
	p := SMTElementPointer{Cell: cell, Expression: NewNameExpr("p", Typed(BoolT1))}
	q := SMTElementPointer{Cell: cell, Expression: NewNameExpr("q", Typed(BoolT1))}
	merged, err := MergeElementPointers(cell, []ElementPointer{p, q})
	if err != nil || merged.ToSMT().(*OperExpr).Operator().Name() != "OR" {
		t.Fatalf("merged guarded pointers = %#v, %v", merged, err)
	}
	fixed, err := MergeElementPointers(cell, []ElementPointer{p, FixedElementPointer{Cell: cell}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fixed.(FixedElementPointer); !ok {
		t.Fatalf("fixed pointer did not dominate: %#v", fixed)
	}
	other := NewArenaCell(2, BasicCellType{Type: IntT1})
	if _, err := MergeElementPointers(cell, []ElementPointer{FixedElementPointer{Cell: other}}); err == nil {
		t.Fatal("mismatched destination was accepted")
	}
	if _, err := MergeElementPointers(cell, nil); err == nil {
		t.Fatal("empty pointer collection was accepted")
	}
}

func TestMergeRetargetAndTuplePointers(t *testing.T) {
	typ := BasicCellType{Type: IntT1}
	a, b, tuple := NewArenaCell(2, typ), NewArenaCell(1, typ), NewArenaCell(3, typ)
	p := NewNameExpr("p", Typed(BoolT1))
	merged, err := MergeElementPointersByCell([]ElementPointer{
		FixedElementPointer{Cell: a}, SMTElementPointer{Cell: b, Expression: p}, FixedElementPointer{Cell: a},
	})
	if err != nil || len(merged) != 2 || merged[0].Element().ID != 1 || merged[1].Element().ID != 2 {
		t.Fatalf("merged pointers = %#v, %v", merged, err)
	}
	retargeted := PointerWithSameCondition(merged[0], tuple)
	if !retargeted.Element().Equal(tuple) || retargeted.ToSMT() != p {
		t.Fatalf("retargeted pointer = %#v", retargeted)
	}
	fixedTuple, err := TupleElementPointer([]ElementPointer{
		FixedElementPointer{Cell: a}, FixedElementPointer{Cell: b},
	}, tuple)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := fixedTuple.(FixedElementPointer); !ok {
		t.Fatalf("all-fixed tuple pointer = %#v", fixedTuple)
	}
	guardedTuple, err := TupleElementPointer([]ElementPointer{
		FixedElementPointer{Cell: a}, SMTElementPointer{Cell: b, Expression: p},
	}, tuple)
	if err != nil || guardedTuple.ToSMT().(*OperExpr).Operator().Name() != "AND" {
		t.Fatalf("guarded tuple pointer = %#v, %v", guardedTuple, err)
	}
	if _, err := TupleElementPointer(nil, tuple); err == nil {
		t.Fatal("empty tuple pointer was accepted")
	}
}
