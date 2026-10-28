package apalache

import (
	"strings"
	"testing"
)

func TestStratifiedPowerSetExpansionThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	powerSetType := mustSet(intSetType)
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intValue(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(intSetType), elements...)
	}
	base := set(1, 2)
	expanded := MustOperExpr(Op("Apalache!Expand"), Typed(powerSetType),
		MustOperExpr(Op("SET_POWERSET"), Typed(powerSetType), base))
	expected := MustOperExpr(Op("SET_ENUM"), Typed(powerSetType), set(), set(1), set(2), set(1, 2))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), expanded, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedPowerSetExpansionPreservesBaseGuards(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	powerSetType := mustSet(intSetType)
	base := MustOperExpr(Op("SET_FILTER"), Typed(intSetType),
		NewNameExpr("x", Typed(IntT1)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(1), intValue(2)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intValue(1)))
	expanded := MustOperExpr(Op("Apalache!Expand"), Typed(powerSetType),
		MustOperExpr(Op("SET_POWERSET"), Typed(powerSetType), base))
	two := MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(2))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), two, expanded)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(membership, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, false)
}

func TestStratifiedFunctionSetExpansionIsRejected(t *testing.T) {
	functionType, _ := NewFunType(IntT1, BoolT1)
	functionSet := MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)),
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1))),
		MustOperExpr(Op("SET_ENUM"), Typed(mustSet(BoolT1)), NewValueExpr(BoolValue(true), Typed(BoolT1))))
	expression := MustOperExpr(Op("Apalache!Expand"), Typed(mustSet(functionType)), functionSet)
	if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil ||
		!strings.Contains(err.Error(), "blow up the solver") {
		t.Fatalf("function-set expansion error = %v", err)
	}
}
