package apalache

import (
	"context"
	"testing"
)

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
