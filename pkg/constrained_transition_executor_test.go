package apalache

import (
	"context"
	"testing"
)

func TestConstrainedTransitionExecutorAssertsPathDisjunction(t *testing.T) {
	base := NewTransitionExecutor(nil, []string{"x"})
	executor, err := NewConstrainedTransitionExecutor(base)
	if err != nil {
		t.Fatal(err)
	}
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(1))
	equals := func(value int64) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(value))
	}
	if err := executor.AddPathOrConstraint([]Expr{boolExpr(true), equals(1)}); err != nil {
		t.Fatal(err)
	}
	if enabled, err := executor.PrepareTransition(0, init); err != nil || !enabled {
		t.Fatalf("prepare = %v, %v", enabled, err)
	}
	if err := executor.AssumeTransition(0); err != nil {
		t.Fatal(err)
	}
	if status, err := executor.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("matching path constraint = %v, %v", status, err)
	}
	if err := executor.AddPathOrConstraint([]Expr{boolExpr(true), equals(2)}); err != nil {
		t.Fatal(err)
	}
	if status, err := executor.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("contradictory path constraint = %v, %v", status, err)
	}
}

func TestConstrainedTransitionExecutorValidation(t *testing.T) {
	if _, err := NewConstrainedTransitionExecutor(nil); err == nil {
		t.Fatal("nil executor was accepted")
	}
	executor, _ := NewConstrainedTransitionExecutor(NewTransitionExecutor(nil, nil))
	if err := executor.AddPathOrConstraint(nil); err == nil {
		t.Fatal("empty path constraint was accepted")
	}
	if err := executor.AddPathOrConstraint([]Expr{intLiteral(1)}); err == nil {
		t.Fatal("non-Boolean path constraint was accepted")
	}
}

func TestConstrainedTransitionExecutorAppliesConstraintAfterSymbolicPick(t *testing.T) {
	base := NewTransitionExecutor(nil, []string{"x"})
	executor, _ := NewConstrainedTransitionExecutor(base)
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	assign := func(value int64) Expr {
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, intLiteral(value))
	}
	constraint := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(20))
	if err := executor.AddPathOrConstraint([]Expr{boolExpr(true), constraint}); err != nil {
		t.Fatal(err)
	}
	for number, value := range map[int]int64{2: 10, 7: 20} {
		if enabled, err := executor.PrepareTransition(number, assign(value)); err != nil || !enabled {
			t.Fatalf("prepare %d = %v, %v", number, enabled, err)
		}
	}
	if _, err := executor.PickTransition(); err != nil {
		t.Fatal(err)
	}
	status, states, err := executor.CheckPendingAndDecode(context.Background(), boolExpr(true))
	if err != nil || status != SMTSatisfiable || len(states) != 1 || states[0].TransitionNumber != 7 {
		t.Fatalf("constrained symbolic pick = %v, %#v, %v", status, states, err)
	}
}
