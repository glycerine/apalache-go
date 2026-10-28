package apalache

import (
	"math/big"
	"testing"
)

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

func TestStratifiedSymbolicIntegerRangeQuantifiersThroughG3(t *testing.T) {
	tests := []struct {
		name       string
		quantifier string
		predicate  func(Expr) Expr
		want       bool
	}{
		{"exists", "EXISTS3", func(x Expr) Expr {
			return MustOperExpr(Op("EQ"), Typed(BoolT1), x, NewValueExpr(Int64Value(2), Typed(IntT1)))
		}, true},
		{"forall", "FORALL3", func(x Expr) Expr {
			return MustOperExpr(Op("GE"), Typed(BoolT1), x, NewValueExpr(Int64Value(1), Typed(IntT1)))
		}, true},
		{"false forall", "FORALL3", func(x Expr) Expr {
			return MustOperExpr(Op("LT"), Typed(BoolT1), x, NewValueExpr(Int64Value(3), Typed(IntT1)))
		}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope := InitialRewriterScope()
			var err error
			scope, bound, err := rewriter.Rewrite(NewValueExpr(Int64Value(1), Typed(IntT1)), scope)
			if err != nil {
				t.Fatal(err)
			}
			scope.Binding = scope.Binding.With("a", bound)
			x := NewNameExpr("x", Typed(IntT1))
			rangeExpression := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
				NewNameExpr("a", Typed(IntT1)), NewValueExpr(Int64Value(3), Typed(IntT1)))
			quantified := MustOperExpr(Op(test.quantifier), Typed(BoolT1), x, rangeExpression, test.predicate(x))
			scope, result, err := rewriter.Rewrite(quantified, scope)
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, test.want)
		})
	}
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

func TestStratifiedIntegerRangeRejectsInvalidTypesAndOversizedBounds(t *testing.T) {
	setType := mustSet(IntT1)
	huge := new(big.Int).Lsh(big.NewInt(1), 100)
	invalid := []Expr{
		MustOperExpr(Op("INT_RANGE"), Typed(IntT1), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("INT_RANGE"), Typed(setType), boolExpr(true), intLiteral(2)),
		MustOperExpr(Op("INT_RANGE"), Typed(setType),
			NewValueExpr(NewIntValue(huge), Typed(IntT1)), intLiteral(2)),
	}
	for _, expression := range invalid {
		base := InitialRewriterScope()
		returned, _, err := NewStratifiedRewriter().Rewrite(expression, base)
		if err == nil || returned.Arena != base.Arena {
			t.Fatalf("invalid integer range returned scope=%v, err=%v", returned, err)
		}
	}
}
