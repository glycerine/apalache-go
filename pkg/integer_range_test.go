package apalache

import "testing"

func TestStratifiedIntegerRangesThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	setType := mustSet(IntT1)
	rangeExpression := func(left, right Expr) Expr {
		return MustOperExpr(Op("INT_RANGE"), Typed(setType), left, right)
	}
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intValue(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(setType), elements...)
	}
	tests := []struct {
		name         string
		actual, want Expr
	}{
		{"inclusive", rangeExpression(intValue(-1), intValue(2)), set(-1, 0, 1, 2)},
		{"empty", rangeExpression(intValue(3), intValue(2)), set()},
		{"constant arithmetic", rangeExpression(
			MustOperExpr(Op("MINUS"), Typed(IntT1), intValue(2), intValue(1)),
			MustOperExpr(Op("PLUS"), Typed(IntT1), intValue(2), intValue(2))), set(1, 2, 3, 4)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				MustOperExpr(Op("EQ"), Typed(BoolT1), test.actual, test.want), InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedSymbolicIntegerRangeChoiceThroughG3(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	var err error
	scope, bound, err := rewriter.Rewrite(NewValueExpr(Int64Value(2), Typed(IntT1)), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("a", bound)
	rangeExpression := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
		NewNameExpr("a", Typed(IntT1)), NewValueExpr(Int64Value(3), Typed(IntT1)))
	choice := MustOperExpr(Op("Apalache!Guess"), Typed(IntT1), rangeExpression)
	inRange := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("LE"), Typed(BoolT1), NewValueExpr(Int64Value(2), Typed(IntT1)), choice),
		MustOperExpr(Op("LE"), Typed(BoolT1), choice, NewValueExpr(Int64Value(3), Typed(IntT1))))
	scope, result, err := rewriter.Rewrite(inRange, scope)
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedSymbolicIntegerRangeMembershipThroughG3(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	var err error
	scope, bound, err := rewriter.Rewrite(NewValueExpr(Int64Value(1), Typed(IntT1)), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("a", bound)
	rangeExpression := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
		NewNameExpr("a", Typed(IntT1)), NewValueExpr(Int64Value(3), Typed(IntT1)))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1),
		NewValueExpr(Int64Value(2), Typed(IntT1)), rangeExpression)
	scope, result, err := rewriter.Rewrite(membership, scope)
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedEmptySymbolicIntegerRangeChoiceUsesDefault(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	scope := InitialRewriterScope()
	var err error
	scope, bound, err := rewriter.Rewrite(NewValueExpr(Int64Value(3), Typed(IntT1)), scope)
	if err != nil {
		t.Fatal(err)
	}
	scope.Binding = scope.Binding.With("a", bound)
	rangeExpression := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
		NewValueExpr(Int64Value(4), Typed(IntT1)), NewNameExpr("a", Typed(IntT1)))
	choice := MustOperExpr(Op("Apalache!Guess"), Typed(IntT1), rangeExpression)
	equalZero := MustOperExpr(Op("EQ"), Typed(BoolT1), choice, NewValueExpr(Int64Value(0), Typed(IntT1)))
	scope, result, err := rewriter.Rewrite(equalZero, scope)
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}
