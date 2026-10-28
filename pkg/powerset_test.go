package apalache

import (
	"fmt"
	"testing"
)

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

func TestStratifiedPowerSetRejectsInfiniteSubsetComparisons(t *testing.T) {
	intValue := func(value int64) Expr { return NewValueExpr(Int64Value(value), Typed(IntT1)) }
	intSetType := mustSet(IntT1)
	powerSetType := mustSet(intSetType)
	finite := MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intValue(1), intValue(2), intValue(3), intValue(4))
	nat := NewValueExpr(NatSet, Typed(intSetType))
	integers := NewValueExpr(IntSet, Typed(intSetType))
	for _, test := range []struct {
		name, left string
		element    Expr
		base       Expr
	}{
		{"finite in subset Nat", "finite", finite, nat},
		{"Nat in subset finite", "Nat", nat, finite},
		{"Nat in subset Nat", "Nat", nat, nat},
		{"Nat in subset Int", "Nat", nat, integers},
		{"Int in subset Nat", "Int", integers, nat},
	} {
		t.Run(test.name, func(t *testing.T) {
			powerSet := MustOperExpr(Op("SET_POWERSET"), Typed(powerSetType), test.base)
			membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), test.element, powerSet)
			if _, _, err := NewStratifiedRewriter().Rewrite(membership, InitialRewriterScope()); err == nil {
				t.Fatalf("accepted unsupported %s subset comparison", test.left)
			}
		})
	}
}

func TestStratifiedPowerSetExistentialAndSkolemOracleCases(t *testing.T) {
	intSetType := mustSet(IntT1)
	powerSetType := mustSet(intSetType)
	base := MustOperExpr(Op("SET_ENUM"), Typed(intSetType),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	powerSet := MustOperExpr(Op("SET_POWERSET"), Typed(powerSetType), base)
	binder := NewNameExpr("X", Typed(intSetType))
	exists := func(predicate bool) Expr {
		return MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder, powerSet,
			NewValueExpr(BoolValue(predicate), Typed(BoolT1)))
	}
	if _, _, err := NewStratifiedRewriter().Rewrite(exists(true), InitialRewriterScope()); err == nil {
		t.Fatal("non-Skolemized existential over a compact powerset was accepted")
	}
	for _, want := range []bool{true, false} {
		t.Run(fmt.Sprintf("Skolem predicate %v", want), func(t *testing.T) {
			skolem := MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists(want))
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(skolem, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, want)
		})
	}
}
