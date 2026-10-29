package apalache

import "testing"

func TestTransitionEnabledProjectionEliminatesSymbolicRangeAssignment(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	assignment := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, i)
	domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), x, intLiteral(0))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, assignment)
	skolem := MustOperExpr(Op("Apalache!Skolem"), Typed(BoolT1), exists)
	projected, exact := transitionEnabledProjection(skolem)
	if !exact {
		t.Fatal("symbolic range assignment was not projected")
	}
	want := MustOperExpr(Op("LE"), Typed(BoolT1), x, intLiteral(0))
	if !EqualTypedExpr(projected, want) {
		t.Fatalf("enabled projection = %v, want %v", projected, want)
	}
}

func TestTransitionEnabledProjectionSubstitutesAssignedPrimeInGuard(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	right := MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))
	assignment := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, right)
	guard := MustOperExpr(Op("LT"), Typed(BoolT1), prime, intLiteral(4))
	projected, exact := transitionEnabledProjection(MustOperExpr(Op("AND"), Typed(BoolT1), assignment, guard))
	if !exact || expressionContainsPrime(projected) || countOperator(projected, "PLUS") != 1 {
		t.Fatalf("guard projection = %v, exact=%t", projected, exact)
	}
}

func TestTransitionEnabledProjectionRefusesUneliminatedWitnessPredicate(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), NewNameExpr("a", Typed(IntT1)), NewNameExpr("b", Typed(IntT1)))
	body := MustOperExpr(Op("GT"), Typed(BoolT1), i, intLiteral(0))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, body)
	if projected, exact := transitionEnabledProjection(exists); exact || projected != nil {
		t.Fatalf("nontrivial witness projection = %v, exact=%t", projected, exact)
	}
}
