package apalache

import (
	"context"
	"testing"
)

func assertCherryPickerCandidates(t *testing.T, label string, candidates []Expr) {
	t.Helper()
	for selected := range candidates {
		rewriter := NewStratifiedRewriter()
		scope := InitialRewriterScope()
		cells := make([]ArenaCell, len(candidates))
		for index, candidate := range candidates {
			var err error
			scope, cells[index], err = rewriter.Rewrite(candidate, scope)
			if err != nil {
				t.Fatalf("%s candidate %d: %v", label, index, err)
			}
		}
		scope, oracle, err := CreateIntOracle(scope, len(cells))
		if err != nil {
			t.Fatal(err)
		}
		scope, picked, err := NewCherryPicker(rewriter).Pick(scope, oracle, cells)
		if err != nil {
			t.Fatal(err)
		}
		choice, err := oracle.Chosen(scope, selected)
		if err != nil {
			t.Fatal(err)
		}
		equal, err := rewriter.equality.Equal(scope.Arena, picked, cells[selected])
		if err != nil {
			t.Fatal(err)
		}
		constraints := append(rewriter.Constraints(), choice,
			MustOperExpr(Op("NOT"), Typed(BoolT1), equal))
		solver := newTestG3Solver(t)
		if _, err := EncodeRewriterScope(solver, scope, constraints); err != nil {
			t.Fatal(err)
		}
		if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
			t.Fatalf("%s position %d differs from candidate: %v, %v", label, selected, status, err)
		}
	}
}

func cherryInt(value int64) Expr {
	return NewValueExpr(Int64Value(value), Typed(IntT1))
}

func cherryTuple(typ Type, elements ...Expr) Expr {
	return MustOperExpr(Op("TUPLE"), Typed(typ), elements...)
}

func cherrySet(elementType Type, elements ...Expr) Expr {
	return MustOperExpr(Op("SET_ENUM"), Typed(mustSet(elementType)), elements...)
}

func TestCherryPickerPreservesTupleCorrelationThroughG3(t *testing.T) {
	tupleType, _ := NewTupleType(IntT1, IntT1)
	tag := Typed(tupleType)
	tuple := func(left, right int64) Expr {
		return MustOperExpr(Op("TUPLE"), tag,
			NewValueExpr(Int64Value(left), Typed(IntT1)), NewValueExpr(Int64Value(right), Typed(IntT1)))
	}
	rewriter := NewStratifiedRewriter()
	scope, first, err := rewriter.Rewrite(tuple(1, 10), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope, second, err := rewriter.Rewrite(tuple(2, 20), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope, mixed, err := rewriter.Rewrite(tuple(1, 20), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope, oracle, err := CreateIntOracle(scope, 2)
	if err != nil {
		t.Fatal(err)
	}
	scope, picked, err := NewCherryPicker(rewriter).Pick(scope, oracle, []ArenaCell{first, second})
	if err != nil {
		t.Fatal(err)
	}
	firstChoice, _ := oracle.Chosen(scope, 0)
	secondChoice, _ := oracle.Chosen(scope, 1)
	rewriter.assert(booleanOr(firstChoice, secondChoice))
	equalMixed, err := rewriter.equality.Equal(scope.Arena, picked, mixed)
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	constraints := append(rewriter.Constraints(), equalMixed)
	encoder, err := EncodeRewriterScope(solver, scope, constraints)
	if err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("correlation-breaking tuple pick = %v, %v", status, err)
	}
	_ = encoder
}

func TestCherryPickerSelectsSetsAndSequencesWithOneOracle(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	setTag := Typed(mustSet(IntT1))
	set := func(value int64) Expr {
		return MustOperExpr(Op("SET_ENUM"), setTag, NewValueExpr(Int64Value(value), Typed(IntT1)))
	}
	sequenceType, _ := NewSeqType(IntT1)
	sequenceTag := Typed(sequenceType)
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("TUPLE"), sequenceTag, elements...)
	}
	var sets, sequences []ArenaCell
	for _, expression := range []Expr{set(1), set(2), sequence(1), sequence(2, 3)} {
		var cell ArenaCell
		var err error
		scope, cell, err = rewriter.Rewrite(expression, scope)
		if err != nil {
			t.Fatal(err)
		}
		if isFiniteSetCellType(cell.Type) {
			sets = append(sets, cell)
		} else {
			sequences = append(sequences, cell)
		}
	}
	oracle, _ := NewMockOracle(1)
	picker := NewCherryPicker(rewriter)
	var pickedSet, pickedSequence ArenaCell
	var err error
	scope, pickedSet, err = picker.Pick(scope, oracle, sets)
	if err != nil {
		t.Fatal(err)
	}
	scope, pickedSequence, err = picker.Pick(scope, oracle, sequences)
	if err != nil {
		t.Fatal(err)
	}
	setEqual, _ := rewriter.equality.Equal(scope.Arena, pickedSet, sets[1])
	sequenceEqual, _ := rewriter.equality.Equal(scope.Arena, pickedSequence, sequences[1])
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, append(rewriter.Constraints(),
		MustOperExpr(Op("NOT"), Typed(BoolT1), booleanAnd(setEqual, sequenceEqual))))
	if err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("fixed structural picks differ from second candidates = %v, %v", status, err)
	}
	_ = encoder
}

func TestCherryPickerBasicScalaOracleCase(t *testing.T) {
	assertCherryPickerCandidates(t, "basic duplicate choices", []Expr{cherryInt(1), cherryInt(2), cherryInt(2)})
}

func TestCherryPickerScalaTupleCases(t *testing.T) {
	pairType := mustTuple(IntT1, IntT1)
	assertCherryPickerCandidates(t, "tuple", []Expr{
		cherryTuple(pairType, cherryInt(1), cherryInt(2)),
		cherryTuple(pairType, cherryInt(3), cherryInt(4)),
	})

	nestedType := mustTuple(IntT1, pairType)
	assertCherryPickerCandidates(t, "nested tuple", []Expr{
		cherryTuple(nestedType, cherryInt(1), cherryTuple(pairType, cherryInt(2), cherryInt(3))),
		cherryTuple(nestedType, cherryInt(3), cherryTuple(pairType, cherryInt(4), cherryInt(5))),
	})
}

func TestCherryPickerScalaSequenceCases(t *testing.T) {
	sequenceType, err := NewSeqType(IntT1)
	if err != nil {
		t.Fatal(err)
	}
	sequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = cherryInt(value)
		}
		return cherryTuple(sequenceType, elements...)
	}
	assertCherryPickerCandidates(t, "same-length sequences", []Expr{
		sequence(1, 2), sequence(3, 4),
	})
	assertCherryPickerCandidates(t, "differing-length sequences", []Expr{
		sequence(1, 2), sequence(3, 4, 5), sequence(),
	})
}

func TestCherryPickerScalaSetCases(t *testing.T) {
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = cherryInt(value)
		}
		return cherrySet(IntT1, elements...)
	}
	assertCherryPickerCandidates(t, "nonempty sets", []Expr{
		set(1, 2), set(3, 4),
	})
	assertCherryPickerCandidates(t, "nonempty and empty sets", []Expr{
		set(1, 2), set(),
	})
	assertCherryPickerCandidates(t, "singleton empty-set choice", []Expr{
		set(),
	})
	assertCherryPickerCandidates(t, "nested sets", []Expr{
		cherrySet(mustSet(IntT1), set(1, 2), set(3, 4)),
		cherrySet(mustSet(IntT1), set(5, 6)),
	})
}
