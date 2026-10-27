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

func TestStratifiedIntegerRangeRejectsSymbolicBounds(t *testing.T) {
	expression := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)),
		NewNameExpr("a", Typed(IntT1)), NewValueExpr(Int64Value(3), Typed(IntT1)))
	scope := InitialRewriterScope()
	newArena, bound := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
	scope.Arena, scope.Binding = newArena, scope.Binding.With("a", bound)
	if _, _, err := NewStratifiedRewriter().Rewrite(expression, scope); err == nil {
		t.Fatal("symbolic integer range bound was accepted")
	}
}
