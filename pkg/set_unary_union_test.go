package apalache

import "testing"

func TestStratifiedUnaryUnionThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	setOfSetsType := mustSet(intSetType)
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = intValue(value)
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(intSetType), elements...)
	}
	tests := []struct {
		name     string
		outer    Expr
		expected Expr
	}{
		{"overlap", MustOperExpr(Op("SET_ENUM"), Typed(setOfSetsType), set(1, 2), set(2, 3)), set(1, 2, 3)},
		{"empty outer", MustOperExpr(Op("SET_ENUM"), Typed(setOfSetsType)), set()},
		{"empty inner", MustOperExpr(Op("SET_ENUM"), Typed(setOfSetsType), set(), set(4)), set(4)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			union := MustOperExpr(Op("SET_UNARY_UNION"), Typed(intSetType), test.outer)
			equal := MustOperExpr(Op("EQ"), Typed(BoolT1), union, test.expected)
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedUnaryUnionPreservesOuterAndInnerGuards(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	setOfSetsType := mustSet(intSetType)
	inner := MustOperExpr(Op("SET_FILTER"), Typed(intSetType),
		NewNameExpr("x", Typed(IntT1)),
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(1), intValue(2)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intValue(1)))
	empty := MustOperExpr(Op("SET_ENUM"), Typed(intSetType))
	outer := MustOperExpr(Op("SET_FILTER"), Typed(setOfSetsType),
		NewNameExpr("s", Typed(intSetType)),
		MustOperExpr(Op("SET_ENUM"), Typed(setOfSetsType), inner, empty),
		MustOperExpr(Op("NE"), Typed(BoolT1), NewNameExpr("s", Typed(intSetType)), empty))
	union := MustOperExpr(Op("SET_UNARY_UNION"), Typed(intSetType), outer)
	expected := MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(1))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), union, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}

func TestStratifiedUnaryUnionRejectsNonSetOperand(t *testing.T) {
	expression := MustOperExpr(Op("SET_UNARY_UNION"), Typed(mustSet(IntT1)), intLiteral(1))
	base := InitialRewriterScope()
	returned, _, err := NewStratifiedRewriter().Rewrite(expression, base)
	if err == nil || returned.Arena.CellCount() != base.Arena.CellCount()+1 {
		t.Fatalf("non-set unary union returned scope=%v, err=%v", returned, err)
	}
}
