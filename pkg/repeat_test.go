package apalache

import "testing"

func TestStratifiedRepeatThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	addIndex := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	tests := []struct {
		name        string
		bound, base int64
		want        int64
	}{
		{"zero", 0, 7, 7},
		{"four iterations", 4, 0, 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repeat := MustOperExpr(Op("Apalache!Repeat"), Typed(IntT1),
				addIndex, intValue(test.bound), intValue(test.base))
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
