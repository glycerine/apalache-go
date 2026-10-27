package apalache

import "testing"

func TestStratifiedSetMapFiniteProductsThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	domain := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intValue(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(intSetType), elements...)
	}
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	tests := []struct {
		name       string
		mapped     Expr
		expected   Expr
		resultType Type
	}{
		{
			"one domain",
			MustOperExpr(Op("SET_MAP"), Typed(intSetType),
				MustOperExpr(Op("MULT"), Typed(IntT1), x, intValue(2)), x, domain(1, 2)),
			domain(2, 4), intSetType,
		},
		{
			"cartesian product",
			MustOperExpr(Op("SET_MAP"), Typed(intSetType),
				MustOperExpr(Op("PLUS"), Typed(IntT1), x, y), x, domain(1, 2), y, domain(10, 20)),
			domain(11, 21, 12, 22), intSetType,
		},
		{
			"empty domain",
			MustOperExpr(Op("SET_MAP"), Typed(intSetType), x, x, domain()),
			domain(), intSetType,
		},
		{
			"duplicate images",
			MustOperExpr(Op("SET_MAP"), Typed(mustSet(BoolT1)),
				NewValueExpr(BoolValue(true), Typed(BoolT1)), x, domain(1, 2)),
			MustOperExpr(Op("SET_ENUM"), Typed(mustSet(BoolT1)), NewValueExpr(BoolValue(true), Typed(BoolT1))),
			mustSet(BoolT1),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), test.mapped, test.expected)
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			if scope.Binding.Contains("x") || scope.Binding.Contains("y") {
				t.Fatalf("set map leaked binding: %#v", scope.Binding.Map())
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedSetMapPreservesSymbolicMembership(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	source := MustOperExpr(Op("SET_FILTER"), Typed(intSetType),
		NewNameExpr("n", Typed(IntT1)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(1), intValue(2)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("n", Typed(IntT1)), intValue(1)))
	x := NewNameExpr("x", Typed(IntT1))
	mapped := MustOperExpr(Op("SET_MAP"), Typed(intSetType),
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intValue(10)), x, source)
	expected := MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(11))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), mapped, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedSetMapRejectsInfiniteDomains(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	expression := MustOperExpr(Op("SET_MAP"), Typed(mustSet(IntT1)), x, x,
		NewValueExpr(IntSet, Typed(mustSet(IntT1))))
	if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil {
		t.Fatal("set map over Int was accepted")
	}
}
