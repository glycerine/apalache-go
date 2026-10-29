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
	body := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), i, intLiteral(1)), intLiteral(0))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, body)
	if projected, exact := transitionEnabledProjection(exists); exact || projected != nil {
		t.Fatalf("nontrivial witness projection = %v, exact=%t", projected, exact)
	}
}

func TestProjectedTransitionEnablednessIncludesActionAndTargetConstraints(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	transition := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, x)
	action := namedExpression{expression: MustOperExpr(Op("LE"), Typed(BoolT1), x, intLiteral(1))}
	target := namedExpression{expression: MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))}
	projected := projectedTransitionEnabledness([]Expr{transition}, []namedExpression{action}, []namedExpression{target},
		[]*VarDecl{NewVarDecl("x", Typed(IntT1))})
	if len(projected) != 1 || expressionContainsPrime(projected[0]) ||
		countOperator(projected[0], "LE") != 1 || countOperator(projected[0], "EQ") != 1 {
		t.Fatalf("constrained enabledness = %v", projected)
	}
}

func TestTransitionEnabledProjectionEliminatesIntegerWitnessBounds(t *testing.T) {
	i := NewNameExpr("i", Typed(IntT1))
	x := NewNameExpr("x", Typed(IntT1))
	domain := MustOperExpr(Op("INT_RANGE"), Typed(mustSet(IntT1)), intLiteral(0), intLiteral(3))
	predicate := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("GT"), Typed(BoolT1), i, x),
		MustOperExpr(Op("LE"), Typed(BoolT1), i, intLiteral(2)),
		MustOperExpr(Op("GE"), Typed(BoolT1), x, intLiteral(0)))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, predicate)
	projected, exact := transitionEnabledProjection(exists)
	if !exact || expressionContainsName(projected, "i") || countOperator(projected, "LE") != 4 {
		t.Fatalf("integer witness bounds projection = %v, exact=%t", projected, exact)
	}

	nonlinear := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PLUS"), Typed(IntT1), i, intLiteral(1)), x)
	if projected, exact = transitionEnabledProjection(MustOperExpr(Op("EXISTS3"), Typed(BoolT1), i, domain, nonlinear)); exact || projected != nil {
		t.Fatalf("nonlinear binder projection = %v, exact=%t", projected, exact)
	}
}
