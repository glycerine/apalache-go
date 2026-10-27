package apalache

import "testing"

func TestStratifiedFoldSetThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	setType := mustSet(IntT1)
	add := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), intValue(1), intValue(2), intValue(3))
	fold := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), add, intValue(0), set)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), fold, intValue(6)), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedFoldSetHonorsMembershipAndDeduplicates(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	setType := mustSet(IntT1)
	add := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	oneViaArithmetic := MustOperExpr(Op("PLUS"), Typed(IntT1), intValue(1), intValue(0))
	base := MustOperExpr(Op("SET_ENUM"), Typed(setType), intValue(1), oneViaArithmetic, intValue(2))
	filtered := MustOperExpr(Op("SET_FILTER"), Typed(setType),
		NewNameExpr("x", Typed(IntT1)), base,
		MustOperExpr(Op("LE"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intValue(1)))
	fold := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), add, intValue(0), filtered)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), fold, intValue(1)), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedFoldSetRejectsInfiniteSet(t *testing.T) {
	add := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	expression := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), add,
		NewValueExpr(Int64Value(0), Typed(IntT1)), NewValueExpr(IntSet, Typed(mustSet(IntT1))))
	if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil {
		t.Fatal("FoldSet over Int was accepted")
	}
}
