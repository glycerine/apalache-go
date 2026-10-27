package apalache

import "testing"

func makeUnaryIntegerLambda(body Expr) Expr {
	operatorType := mustOperator([]Type{IntT1}, IntT1)
	declaration := NewOperDecl("Lambda", []OperatorParam{ValueParam("i")}, body, Typed(operatorType))
	return NewLetInExpr(NewNameExpr("Lambda", Typed(operatorType)), Typed(operatorType), declaration)
}

func TestStratifiedMkSeqThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	sequenceType, _ := NewSeqType(IntT1)
	body := MustOperExpr(Op("MULT"), Typed(IntT1), NewNameExpr("i", Typed(IntT1)), intValue(2))
	expression := MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType), intValue(3), makeUnaryIntegerLambda(body))
	expected := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(2), intValue(4), intValue(6))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), expression, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedMkSeqUsesExistingCapacity(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	sequenceType, _ := NewSeqType(IntT1)
	base := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(10), intValue(20))
	capacity := MustOperExpr(Op("PLUS"), Typed(IntT1),
		MustOperExpr(Op("ApalacheInternal!__ApalacheSeqCapacity"), Typed(IntT1), base), intValue(1))
	expression := MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType), capacity,
		makeUnaryIntegerLambda(NewNameExpr("i", Typed(IntT1))))
	expected := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(1), intValue(2), intValue(3))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), expression, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedMkSeqRejectsNegativeCapacity(t *testing.T) {
	sequenceType, _ := NewSeqType(IntT1)
	expression := MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType),
		NewValueExpr(Int64Value(-1), Typed(IntT1)), makeUnaryIntegerLambda(NewNameExpr("i", Typed(IntT1))))
	if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil {
		t.Fatal("negative MkSeq capacity was accepted")
	}
}
