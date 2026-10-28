package apalache

import (
	"errors"
	"testing"
)

func TestStratifiedRepeatThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	addIndex := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	addOne := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), intValue(1)))
	tests := []struct {
		name        string
		operator    Expr
		bound, base int64
		want        int64
	}{
		{"zero", addIndex, 0, 7, 7},
		{"five increments", addOne, 5, 0, 5},
		{"five indexed additions", addIndex, 5, 0, 15},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repeat := MustOperExpr(Op("Apalache!Repeat"), Typed(IntT1),
				test.operator, intValue(test.bound), intValue(test.base))
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				MustOperExpr(Op("EQ"), Typed(BoolT1), repeat, intValue(test.want)), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedRepeatRejectsInvalidBound(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	addIndex := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	expression := MustOperExpr(Op("Apalache!Repeat"), Typed(IntT1), addIndex, intValue(-1), intValue(0))
	if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil {
		t.Fatal("Repeat accepted a negative bound")
	}
}

func TestStratifiedRepeatValidatesOperatorTypeAtZeroIterations(t *testing.T) {
	wrongType := mustOperator([]Type{BoolT1, IntT1}, BoolT1)
	declaration := NewOperDecl("Wrong", []OperatorParam{ValueParam("acc"), ValueParam("index")},
		NewNameExpr("acc", Typed(BoolT1)), Typed(wrongType))
	embedded := NewLetInExpr(NewNameExpr("Wrong", Typed(wrongType)), Typed(wrongType), declaration)
	expression := MustOperExpr(Op("Apalache!Repeat"), Typed(IntT1), embedded, intLiteral(0), intLiteral(7))
	_, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope())
	var typing *TypingException
	if !errors.As(err, &typing) || typing.CauseExprID() != declaration.ID() {
		t.Fatalf("zero-iteration operator type error = %T %v", err, err)
	}
}
