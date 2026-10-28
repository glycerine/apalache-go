package apalache

import (
	"strings"
	"testing"
)

func TestBMCInvariantKindsAndLimitsMatchScala(t *testing.T) {
	if StateInvariant.String() != "state" || ActionInvariant.String() != "action" {
		t.Fatal("invariant kind text differs from Scala")
	}
	if MaxProductSize != 1_000_000 || PowSetMaxBaseSize != 20 || RewritingRecursionLimit != 10_000 {
		t.Fatal("rewriting limits differ from Scala")
	}
}

func TestCheckerVerificationConditionsDefaultsAndDefensiveSlices(t *testing.T) {
	var empty CheckerVerificationConditions
	if len(empty.StateInvariantsAndNegations()) != 0 || len(empty.ActionInvariantsAndNegations()) != 0 || len(empty.TraceInvariantsAndNegations()) != 0 {
		t.Fatal("zero verification conditions are not Scala's empty defaults")
	}
	if _, found := empty.StateView(); found {
		t.Fatal("zero verification conditions unexpectedly have a view")
	}

	inv := NewNameExpr("Inv", Typed(BoolT1))
	neg := MustOperExpr(Op("NOT"), Typed(BoolT1), inv)
	traceInv := NewOperDecl("TraceInv", nil, inv, Typed(mustOperatorType(t, nil, BoolT1)))
	traceNeg := NewOperDecl("NotTraceInv", nil, neg, Typed(mustOperatorType(t, nil, BoolT1)))
	state := []ExpressionVerificationCondition{{Invariant: inv, Negation: neg}}
	trace := []TraceVerificationCondition{{Invariant: traceInv, Negation: traceNeg}}
	conditions := NewCheckerVerificationConditions(state, nil, trace, inv)
	state[0].Invariant = neg
	trace[0].Invariant = traceNeg
	gotState := conditions.StateInvariantsAndNegations()
	gotTrace := conditions.TraceInvariantsAndNegations()
	if gotState[0].Invariant != inv || gotTrace[0].Invariant != traceInv {
		t.Fatal("constructor aliased input slices")
	}
	gotState[0].Invariant = neg
	if conditions.StateInvariantsAndNegations()[0].Invariant != inv {
		t.Fatal("accessor exposed mutable slice storage")
	}
	if view, found := conditions.StateView(); !found || view != inv {
		t.Fatal("state view was not retained")
	}
}

func TestCheckerInputCapturesScalaBoundaryDefensively(t *testing.T) {
	init := NewNameExpr("Init", Typed(BoolT1))
	next := NewNameExpr("Next", Typed(BoolT1))
	constInit := NewNameExpr("CInit", Typed(BoolT1))
	module := NewModule("M", nil)
	decl := NewOperDecl("Persistent", nil, init, Typed(mustOperatorType(t, nil, BoolT1)))
	inits := []Expr{init}
	nexts := []Expr{next}
	declarations := map[string]*OperDecl{"Persistent": decl}
	input := NewCheckerInput(module, inits, nexts, constInit, CheckerVerificationConditions{}, declarations)
	inits[0], nexts[0] = next, init
	delete(declarations, "Persistent")
	if input.RootModule() != module || input.InitTransitions()[0] != init || input.NextTransitions()[0] != next {
		t.Fatal("checker input lost its boundary values")
	}
	if got, found := input.ConstInitPrimed(); !found || got != constInit {
		t.Fatal("checker input lost constant initialization")
	}
	copy := input.PersistentDeclarations()
	if copy["Persistent"] != decl {
		t.Fatal("checker input lost persistent declaration")
	}
	delete(copy, "Persistent")
	if len(input.PersistentDeclarations()) != 1 {
		t.Fatal("persistent declaration accessor exposed map storage")
	}

	withoutOptional := NewCheckerInput(module, nil, nil, nil, CheckerVerificationConditions{}, nil)
	if _, found := withoutOptional.ConstInitPrimed(); found || len(withoutOptional.PersistentDeclarations()) != 0 {
		t.Fatal("Scala optional/default values are not empty")
	}
}

func TestReTLACombinedPredicatePreservesPredicateOrder(t *testing.T) {
	variable, err := NewVarType(0)
	if err != nil {
		t.Fatal(err)
	}
	left := NewNameExpr("x", Typed(IntT1))
	right := NewNameExpr("y", Typed(IntT1))
	expression := MustOperExpr(Op("SET_INTERSECT"), Typed(variable), left, right)
	result := (ReTLACombinedPredicate{}).CheckExpression(expression)
	if len(result.Failures) != 2 || result.Failures[0].ID != expression.ID() || result.Failures[1].ID != expression.ID() {
		t.Fatalf("combined expression failures = %+v", result.Failures)
	}
	if !strings.HasPrefix(result.Failures[0].Message, "Expected ") || result.Failures[1].Message != "SET_INTERSECT" {
		t.Fatalf("combined predicate order differs from Scala: %+v", result.Failures)
	}
	decl := NewOperDecl("Bad", nil, expression, Typed(mustOperatorType(t, nil, variable)))
	moduleResult := (ReTLACombinedPredicate{}).CheckModule(NewModule("M", []Decl{decl}))
	if len(moduleResult.Failures) < 2 || moduleResult.Failures[len(moduleResult.Failures)-1].Message != "SET_INTERSECT" {
		t.Fatalf("combined module failures = %+v", moduleResult.Failures)
	}
}
