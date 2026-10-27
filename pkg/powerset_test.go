package apalache

import "testing"

func TestStratifiedPowerSetMembershipThroughG3(t *testing.T) {
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
	domain := set(1, 2)
	powerSet := MustOperExpr(Op("SET_POWERSET"), Typed(powerSetType), domain)
	tests := []struct {
		name    string
		element Expr
		want    bool
	}{
		{"empty subset", set(), true},
		{"proper subset", set(1), true},
		{"whole set", set(1, 2), true},
		{"not a subset", set(1, 3), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), test.element, powerSet)
			scope, result, err := rewriter.Rewrite(membership, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, test.want)
		})
	}
}

func TestStratifiedPowerSetStoresItsDomain(t *testing.T) {
	intSetType := mustSet(IntT1)
	domainExpression := MustOperExpr(Op("SET_ENUM"), Typed(intSetType),
		NewValueExpr(Int64Value(1), Typed(IntT1)))
	expression := MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSetType)), domainExpression)
	rewriter := NewStratifiedRewriter()
	scope, powerSet, err := rewriter.Rewrite(expression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := powerSet.Type.(PowerSetCellType); !ok {
		t.Fatalf("powerset arena type = %T", powerSet.Type)
	}
	domain, err := scope.Arena.Domain(powerSet)
	if err != nil || len(scope.Arena.GetHas(domain)) != 1 {
		t.Fatalf("powerset domain = %v, %v", domain, err)
	}
}
