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

func TestStratifiedFoldSetScalaOracleCases(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	stringSetType := mustSet(StrT1)
	intSet := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intValue(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(intSetType), elements...)
	}
	constant := makeBinaryIntegerLambda(intValue(0))
	plusOne := makeIntegerAccumulatorLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), intValue(1)), StrT1)
	sum := makeBinaryIntegerLambda(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("acc", Typed(IntT1)), NewNameExpr("elem", Typed(IntT1))))
	strings := MustOperExpr(Op("SET_ENUM"), Typed(stringSetType),
		NewValueExpr(StringValue("x"), Typed(StrT1)), NewValueExpr(StringValue("x"), Typed(StrT1)),
		NewValueExpr(StringValue("y"), Typed(StrT1)))
	for _, test := range []struct {
		name     string
		operator Expr
		base     int64
		set      Expr
		want     int64
	}{
		{"constant", constant, 1, intSet(2, 3), 0},
		{"empty", constant, 1, intSet(), 1},
		{"cardinality", plusOne, 0, strings, 2},
		{"sum", sum, 0, intSet(2, 93, 4), 99},
	} {
		t.Run(test.name, func(t *testing.T) {
			fold := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), test.operator, intValue(test.base), test.set)
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
	for _, infiniteSet := range []PredefinedSet{NatSet, IntSet} {
		expression := MustOperExpr(Op("Apalache!ApaFoldSet"), Typed(IntT1), add,
			NewValueExpr(Int64Value(0), Typed(IntT1)), NewValueExpr(infiniteSet, Typed(mustSet(IntT1))))
		if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil {
			t.Fatalf("FoldSet over %s was accepted", infiniteSet.Name())
		}
	}
}
