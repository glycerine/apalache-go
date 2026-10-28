package apalache

import "testing"

func makeBinaryIntegerLambda(body Expr) Expr {
	return makeIntegerAccumulatorLambda(body, IntT1)
}

func makeIntegerAccumulatorLambda(body Expr, elementType Type) Expr {
	operatorType := mustOperator([]Type{IntT1, elementType}, IntT1)
	declaration := NewOperDecl("Fold", []OperatorParam{ValueParam("acc"), ValueParam("elem")}, body, Typed(operatorType))
	return NewLetInExpr(NewNameExpr("Fold", Typed(operatorType)), Typed(operatorType), declaration)
}

func TestStratifiedFoldSequenceThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	sequenceType, _ := NewSeqType(IntT1)
	add := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	sequence := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1), intValue(2), intValue(3))
	fold := MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Typed(IntT1), add, intValue(0), sequence)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), fold, intValue(6)), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedFoldSequenceScalaOracleCases(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSequenceType, _ := NewSeqType(IntT1)
	stringSequenceType, _ := NewSeqType(StrT1)
	intSequence := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intValue(value)
		}
		return MustOperExpr(Op("TUPLE"), Typed(intSequenceType), elements...)
	}
	constant := makeBinaryIntegerLambda(intValue(0))
	plusOne := makeIntegerAccumulatorLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), intValue(1)), StrT1)
	sum := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	strings := MustOperExpr(Op("TUPLE"), Typed(stringSequenceType),
		NewValueExpr(StringValue("x"), Typed(StrT1)), NewValueExpr(StringValue("y"), Typed(StrT1)))
	for _, test := range []struct {
		name     string
		operator Expr
		base     int64
		sequence Expr
		want     int64
	}{
		{"constant", constant, 1, intSequence(2, 3), 0},
		{"empty", constant, 1, intSequence(), 1},
		{"length", plusOne, 0, strings, 2},
		{"sum", sum, 0, intSequence(2, 93, 4), 99},
	} {
		t.Run(test.name, func(t *testing.T) {
			fold := MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Typed(IntT1), test.operator, intValue(test.base), test.sequence)
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				MustOperExpr(Op("EQ"), Typed(BoolT1), fold, intValue(test.want)), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedFoldSequenceHonorsSymbolicLength(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	sequenceType, _ := NewSeqType(IntT1)
	add := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	predicate := NewNameExpr("p", Typed(BoolT1))
	long := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1), intValue(2))
	short := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1))
	sequence := MustOperExpr(Op("IF_THEN_ELSE"), Typed(sequenceType), predicate, long, short)
	fold := MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Typed(IntT1), add, intValue(0), sequence)
	expected := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), predicate, intValue(3), intValue(1))
	claim := MustOperExpr(Op("EQ"), Typed(BoolT1), fold, expected)
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	newArena, predicateCell := scope.Arena.AppendCellType(BasicCellType{Type: BoolT1}, true)
	scope.Arena, scope.Binding = newArena, scope.Binding.With("p", predicateCell)
	scope, result, err := rewriter.Rewrite(claim, scope)
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedFoldSequenceRejectsMalformedTypesWithoutMutation(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	sequenceType, _ := NewSeqType(IntT1)
	add := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	sequence := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1))
	for _, expression := range []Expr{
		MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Untyped(), add, intValue(0), sequence),
		MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Typed(IntT1), add, intValue(0), intValue(1)),
		MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Typed(BoolT1), add, intValue(0), sequence),
	} {
		base := InitialRewriterScope()
		returned, _, err := NewStratifiedRewriter().Rewrite(expression, base)
		if err == nil || returned.Arena != base.Arena {
			t.Fatalf("invalid FoldSeq returned scope=%v, err=%v", returned, err)
		}
	}
}
