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
