package apalache

import (
	"context"
	"math/big"
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

func newTestG3SolverWithLogic(t *testing.T, logic string) *G3SolverContext {
	t.Helper()
	solver, err := NewG3SolverContextWithLogic(logic)
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

func TestG3SolverEvaluatesBooleanAndIntegerModels(t *testing.T) {
	solver := newTestG3Solver(t)
	x, err := solver.Declare("model_x", SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	want := new(big.Int).Neg(new(big.Int).Exp(big.NewInt(2), big.NewInt(90), nil))
	value, err := solver.BigInt(want)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := solver.Equal(x, value)
	if err != nil {
		t.Fatal(err)
	}
	if err := solver.Assert(equal); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTSatisfiable {
		t.Fatalf("model check = %v, %v", status, err)
	}
	got, err := solver.EvaluateBigInt(x)
	if err != nil || got.Cmp(want) != 0 {
		t.Fatalf("integer model value = %v, %v", got, err)
	}
	truth, err := solver.EvaluateBool(equal)
	if err != nil || !truth {
		t.Fatalf("Boolean model value = %v, %v", truth, err)
	}
}

func TestG3SolverArrayStoreSelect(t *testing.T) {
	solver := newTestG3Solver(t)
	array, err := solver.DeclareArray("array_a", SMTInt, SMTInt)
	if err != nil {
		t.Fatal(err)
	}
	index, err := solver.Int(1)
	if err != nil {
		t.Fatal(err)
	}
	value, err := solver.Int(42)
	if err != nil {
		t.Fatal(err)
	}
	stored, err := solver.Store(array, index, value)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := solver.Select(stored, index)
	if err != nil {
		t.Fatal(err)
	}
	equal, err := solver.Equal(selected, value)
	if err != nil {
		t.Fatal(err)
	}
	contradiction, err := solver.Not(equal)
	if err != nil {
		t.Fatal(err)
	}
	if err = solver.Assert(contradiction); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("array store/select contradiction = %v, %v", status, err)
	}
	truth, err := solver.Bool(true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = solver.Store(array, truth, value); err == nil {
		t.Fatal("Boolean array index was accepted for an integer-indexed array")
	}
}

func TestG3SolverQuantifiers(t *testing.T) {
	for _, test := range []struct {
		name       string
		forall     bool
		wantStatus SMTStatus
	}{
		{name: "forall", forall: true, wantStatus: SMTUnsatisfiable},
		{name: "exists", forall: false, wantStatus: SMTSatisfiable},
	} {
		t.Run(test.name, func(t *testing.T) {
			solver := newTestG3SolverWithLogic(t, "AUFNIA")
			bound, err := solver.Bound(0, SMTInt)
			if err != nil {
				t.Fatal(err)
			}
			zero, err := solver.Int(0)
			if err != nil {
				t.Fatal(err)
			}
			body, err := solver.GreaterEqual(bound, zero)
			if !test.forall {
				body, err = solver.Greater(bound, zero)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = solver.Forall("not_bound", zero, body); err == nil {
				t.Fatal("ordinary integer term was accepted as a quantified binder")
			}
			var quantified SMTTerm
			if test.forall {
				quantified, err = solver.Forall("x", bound, body)
			} else {
				quantified, err = solver.Exists("x", bound, body)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err = solver.Assert(quantified); err != nil {
				t.Fatal(err)
			}
			if status, err := solver.Check(context.Background()); err != nil || status != test.wantStatus {
				t.Fatalf("quantified integer formula = %v, %v", status, err)
			}
		})
	}
}
