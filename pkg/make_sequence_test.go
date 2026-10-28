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
	body := MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("i", Typed(IntT1)), intValue(1))
	for _, test := range []struct {
		name     string
		capacity int64
		expected Expr
	}{
		{"three elements", 3, MustOperExpr(Op("TUPLE"), Typed(sequenceType), intValue(2), intValue(3), intValue(4))},
		{"empty", 0, MustOperExpr(Op("TUPLE"), Typed(sequenceType))},
	} {
		t.Run(test.name, func(t *testing.T) {
			expression := MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType), intValue(test.capacity), makeUnaryIntegerLambda(body))
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				MustOperExpr(Op("EQ"), Typed(BoolT1), expression, test.expected), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
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

func TestStratifiedMkSeqCapacityArithmeticAndErrors(t *testing.T) {
	sequenceType := mustSeq(IntT1)
	capacity := MustOperExpr(Op("DIV"), Typed(IntT1),
		MustOperExpr(Op("MULT"), Typed(IntT1), intLiteral(2), intLiteral(3)), intLiteral(2))
	expression := MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType), capacity,
		makeUnaryIntegerLambda(NewNameExpr("i", Typed(IntT1))))
	expected := MustOperExpr(Op("TUPLE"), Typed(sequenceType), intLiteral(1), intLiteral(2), intLiteral(3))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), expression, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)

	binaryType := mustOperator([]Type{IntT1, IntT1}, IntT1)
	binaryDecl := NewOperDecl("Binary", []OperatorParam{ValueParam("x"), ValueParam("y")},
		NewNameExpr("x", Typed(IntT1)), Typed(binaryType))
	binary := NewLetInExpr(NewNameExpr("Binary", Typed(binaryType)), Typed(binaryType), binaryDecl)
	invalid := []Expr{
		MustOperExpr(Op("Apalache!MkSeq"), Typed(IntT1), intLiteral(1), makeUnaryIntegerLambda(intLiteral(1))),
		MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType), intLiteral(1), binary),
		MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType), NewNameExpr("n", Typed(IntT1)), makeUnaryIntegerLambda(intLiteral(1))),
		MustOperExpr(Op("Apalache!MkSeq"), Typed(sequenceType),
			MustOperExpr(Op("DIV"), Typed(IntT1), intLiteral(1), intLiteral(0)), makeUnaryIntegerLambda(intLiteral(1))),
	}
	for _, invalidExpression := range invalid {
		if _, _, err := NewStratifiedRewriter().Rewrite(invalidExpression, InitialRewriterScope()); err == nil {
			t.Fatalf("invalid MkSeq was accepted: %v", invalidExpression)
		}
	}
}
