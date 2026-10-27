package apalache

import "testing"

func makeBinaryIntegerLambda(body Expr) Expr {
	operatorType := mustOperator([]Type{IntT1, IntT1}, IntT1)
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
