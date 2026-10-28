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

func TestStratifiedSetOfSetsInclusionInPowerSetThroughG3(t *testing.T) {
	intSetType := mustSet(IntT1)
	setOfSetsType := mustSet(intSetType)
	set := func(values ...int64) Expr {
		elements := make([]Expr, len(values))
		for index, value := range values {
			elements[index] = NewValueExpr(Int64Value(value), Typed(IntT1))
		}
		return MustOperExpr(Op("SET_ENUM"), Typed(intSetType), elements...)
	}
	base := set(1, 2)
	power := MustOperExpr(Op("SET_POWERSET"), Typed(setOfSetsType), base)
	for _, test := range []struct {
		name string
		left Expr
		want bool
	}{
		{"all subsets", MustOperExpr(Op("SET_ENUM"), Typed(setOfSetsType), set(), set(1), set(1, 2)), true},
		{"contains non-subset", MustOperExpr(Op("SET_ENUM"), Typed(setOfSetsType), set(1), set(3)), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(
				MustOperExpr(Op("SET_SUBSET_EQ"), Typed(BoolT1), test.left, power), InitialRewriterScope())
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

func TestStratifiedPowerSetConstructionAllowsInfiniteDomainButMembershipRejectsIt(t *testing.T) {
	intSetType := mustSet(IntT1)
	powerSetType := mustSet(intSetType)
	expression := MustOperExpr(Op("SET_POWERSET"), Typed(powerSetType), NewValueExpr(NatSet, Typed(intSetType)))
	rewriter := NewStratifiedRewriter()
	scope, power, err := rewriter.Rewrite(expression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	domain, err := scope.Arena.Domain(power)
	if err != nil || !domain.Equal(scope.Arena.NatSetCell()) {
		t.Fatalf("SUBSET Nat domain = %v, %v", domain, err)
	}
	empty := MustOperExpr(Op("SET_ENUM"), Typed(intSetType))
	membership := MustOperExpr(Op("SET_IN"), Typed(BoolT1), empty, expression)
	if _, _, err := NewStratifiedRewriter().Rewrite(membership, InitialRewriterScope()); err == nil {
		t.Fatal("membership in SUBSET Nat was accepted")
	}
}

func TestStratifiedPowerSetRejectsNonSetOperandAndWrongResultType(t *testing.T) {
	intSetType := mustSet(IntT1)
	invalid := []Expr{
		MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1))),
		MustOperExpr(Op("SET_POWERSET"), Typed(intSetType),
			MustOperExpr(Op("SET_ENUM"), Typed(intSetType), NewValueExpr(Int64Value(1), Typed(IntT1)))),
	}
	for _, expression := range invalid {
		if _, _, err := NewStratifiedRewriter().Rewrite(expression, InitialRewriterScope()); err == nil {
			t.Fatalf("invalid powerset was accepted: %v", expression)
		}
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
