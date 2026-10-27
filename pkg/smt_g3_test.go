package apalache

import (
	"context"
	"testing"
)

func newTestG3Solver(t *testing.T) *G3SolverContext {
	t.Helper()
	solver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := solver.Close(); err != nil {
			t.Error(err)
		}
	})
	return solver
}

func TestG3SolverIntegerArithmeticAndScopes(t *testing.T) {
	solver := newTestG3Solver(t)
	x, err := solver.Declare("x", SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	zero, err := solver.Int(0)
	if err != nil {
		t.Fatal(err)
	}
	one, err := solver.Int(1)
	if err != nil {
		t.Fatal(err)
	}
	equalZero, err := solver.Equal(x, zero)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(equalZero); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("base check = %v, %v", status, err)
	}
	if err = solver.Push(); err != nil {
		t.Fatal(err)
	}
	lowerBound, err := solver.GreaterEqual(x, one)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(lowerBound); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("scoped contradiction = %v, %v", status, err)
	}
	if err = solver.Pop(); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("post-pop check = %v, %v", status, err)
	}
}

func TestG3SolverRejectsCrossContextAndCancellation(t *testing.T) {
	left := newTestG3Solver(t)
	right := newTestG3Solver(t)
	term, err := left.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	if err = right.Assert(term); err == nil {
		t.Fatal("cross-context term was accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if status, err := left.Check(ctx); err != context.Canceled || status != SMTUnknown {
		t.Fatalf("canceled check = %v, %v", status, err)
	}
}
