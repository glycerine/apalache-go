package apalache

import "testing"

func TestStratifiedSetAsFunctionThroughG3(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	pairType, _ := NewTupleType(IntT1, IntT1)
	relationType := mustSet(pairType)
	functionType, _ := NewFunType(IntT1, IntT1)
	pair := func(key, value int64) Expr {
		return MustOperExpr(Op("TUPLE"), Typed(pairType), intValue(key), intValue(value))
	}
	tests := []struct {
		name      string
		pairs     []Expr
		key, want int64
	}{
		{"ordinary relation", []Expr{pair(1, 10), pair(2, 20)}, 2, 20},
		{"first duplicate key wins", []Expr{pair(1, 10), pair(1, 99)}, 1, 10},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			relation := MustOperExpr(Op("SET_ENUM"), Typed(relationType), test.pairs...)
			function := MustOperExpr(Op("Apalache!SetAsFun"), Typed(functionType), relation)
			application := MustOperExpr(Op("FUN_APP"), Typed(IntT1), function, intValue(test.key))
			claim := MustOperExpr(Op("EQ"), Typed(BoolT1), application, intValue(test.want))
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(claim, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, true)
		})
	}
}

func TestStratifiedSetAsFunctionPreservesDomain(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	pairType, _ := NewTupleType(IntT1, BoolT1)
	relationType := mustSet(pairType)
	functionType, _ := NewFunType(IntT1, BoolT1)
	pair := func(key int64, value bool) Expr {
		return MustOperExpr(Op("TUPLE"), Typed(pairType), intValue(key), NewValueExpr(BoolValue(value), Typed(BoolT1)))
	}
	relation := MustOperExpr(Op("SET_ENUM"), Typed(relationType), pair(3, true), pair(5, false))
	function := MustOperExpr(Op("Apalache!SetAsFun"), Typed(functionType), relation)
	domain := MustOperExpr(Op("DOMAIN"), Typed(mustSet(IntT1)), function)
	expected := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intValue(3), intValue(5))
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(
		MustOperExpr(Op("EQ"), Typed(BoolT1), domain, expected), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)
}
