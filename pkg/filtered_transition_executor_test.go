package apalache

import "testing"

func TestFilteredTransitionExecutorMatchesScala(t *testing.T) {
	assign := func(name string, value Expr) Expr {
		prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr(name, Typed(IntT1)))
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, value)
	}
	and := func(expressions ...Expr) Expr { return MustOperExpr(Op("AND"), Typed(BoolT1), expressions...) }
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	init := and(assign("y", intLiteral(1)), assign("x", intLiteral(1)))
	next1 := and(assign("x", x), assign("y", MustOperExpr(Op("PLUS"), Typed(IntT1), x, y)))
	next2 := and(assign("x", y), assign("y", x))

	base := NewTransitionExecutor(nil, []string{"x", "y"})
	filtered, err := NewFilteredTransitionExecutor("(0->.*|1->2|2->1)", "", base)
	if err != nil {
		t.Fatal(err)
	}
	if enabled, prepareErr := filtered.PrepareTransition(3, init); prepareErr != nil || !enabled {
		t.Fatalf("step 0 transition 3 = %v, %v", enabled, prepareErr)
	}
	if err = filtered.AssumeTransition(3); err != nil {
		t.Fatal(err)
	}
	if err = filtered.NextState(); err != nil {
		t.Fatal(err)
	}
	if enabled, prepareErr := filtered.PrepareTransition(1, next1); prepareErr != nil || enabled {
		t.Fatalf("step 1 transition 1 = %v, %v", enabled, prepareErr)
	}
	if enabled, prepareErr := filtered.PrepareTransition(2, next2); prepareErr != nil || !enabled {
		t.Fatalf("step 1 transition 2 = %v, %v", enabled, prepareErr)
	}
	if err = filtered.AssumeTransition(2); err != nil {
		t.Fatal(err)
	}
	if err = filtered.NextState(); err != nil {
		t.Fatal(err)
	}
	if enabled, prepareErr := filtered.PrepareTransition(1, next1); prepareErr != nil || !enabled {
		t.Fatalf("step 2 transition 1 = %v, %v", enabled, prepareErr)
	}
	if enabled, prepareErr := filtered.PrepareTransition(2, next2); prepareErr != nil || enabled {
		t.Fatalf("step 2 transition 2 = %v, %v", enabled, prepareErr)
	}
}

func TestFilteredTransitionExecutorMayChangeScalaMatrix(t *testing.T) {
	assign := func(name string, value Expr) Expr {
		prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr(name, Typed(IntT1)))
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, value)
	}
	and := func(expressions ...Expr) Expr { return MustOperExpr(Op("AND"), Typed(BoolT1), expressions...) }
	x := NewNameExpr("x", Typed(IntT1))
	y := NewNameExpr("y", Typed(IntT1))
	init := and(assign("y", intLiteral(1)), assign("x", intLiteral(1)))
	next := and(assign("x", x), assign("y", MustOperExpr(Op("PLUS"), Typed(IntT1), x, y)))
	inv0 := MustOperExpr(Op("GE"), Typed(BoolT1), x, intLiteral(3))
	inv1 := MustOperExpr(Op("GE"), Typed(BoolT1), y, x)

	base := NewTransitionExecutor(nil, []string{"x", "y"})
	filter := "(0->.*|2->.*|3->state0|4->.*|5->state1|6->action1|7->action1)"
	filtered, err := NewFilteredTransitionExecutor("", filter, base)
	if err != nil {
		t.Fatal(err)
	}
	if enabled, prepareErr := filtered.PrepareTransition(1, init); prepareErr != nil || !enabled {
		t.Fatalf("prepare init = %v, %v", enabled, prepareErr)
	}
	if err = filtered.AssumeTransition(1); err != nil {
		t.Fatal(err)
	}
	if err = filtered.NextState(); err != nil {
		t.Fatal(err)
	}

	steps := []struct {
		kind         InvariantKind
		want0, want1 bool
	}{
		{kind: StateInvariant, want0: false, want1: false},
		{kind: StateInvariant, want0: true, want1: true},
		{kind: StateInvariant, want0: false, want1: false},
		{kind: StateInvariant, want0: false, want1: true},
		{kind: StateInvariant, want0: false, want1: true},
		{kind: StateInvariant, want0: false, want1: false},
		{kind: ActionInvariant, want0: false, want1: true},
	}
	for index, step := range steps {
		stepNumber := index + 1
		if enabled, prepareErr := filtered.PrepareTransition(1, next); prepareErr != nil || !enabled {
			t.Fatalf("step %d prepare = %v, %v", stepNumber, enabled, prepareErr)
		}
		got0, err0 := filtered.MayChangeAssertion(1, step.kind, 0, inv0)
		got1, err1 := filtered.MayChangeAssertion(1, step.kind, 1, inv1)
		if err0 != nil || err1 != nil || got0 != step.want0 || got1 != step.want1 {
			t.Fatalf("step %d may-change = (%v, %v), want (%v, %v), errors (%v, %v)",
				stepNumber, got0, got1, step.want0, step.want1, err0, err1)
		}
		if stepNumber < len(steps) {
			if err = filtered.AssumeTransition(1); err != nil {
				t.Fatal(err)
			}
			if err = filtered.NextState(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestFilteredTransitionExecutorScalaRegression108(t *testing.T) {
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("y", Typed(IntT1)))
	assign := func(value Expr) Expr {
		return MustOperExpr(Op("Apalache!:="), Typed(BoolT1), prime, value)
	}
	y := NewNameExpr("y", Typed(IntT1))
	base := NewTransitionExecutor(nil, []string{"y"})
	filtered, err := NewFilteredTransitionExecutor("", "(1->.*|2->.*)", base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = filtered.PrepareTransition(1, assign(intLiteral(1))); err != nil {
		t.Fatal(err)
	}
	if changed, changeErr := filtered.MayChangeAssertion(1, StateInvariant, 0, boolExpr(false)); changeErr != nil || changed {
		t.Fatalf("state 0 skipped invariant = %v, %v", changed, changeErr)
	}
	if err = filtered.AssumeTransition(1); err != nil {
		t.Fatal(err)
	}
	if err = filtered.NextState(); err != nil {
		t.Fatal(err)
	}
	next := assign(MustOperExpr(Op("PLUS"), Typed(IntT1), y, intLiteral(1)))
	if _, err = filtered.PrepareTransition(1, next); err != nil {
		t.Fatal(err)
	}
	if changed, changeErr := filtered.MayChangeAssertion(1, StateInvariant, 0, boolExpr(false)); changeErr != nil || !changed {
		t.Fatalf("state 1 rechecked invariant = %v, %v", changed, changeErr)
	}
	if err = filtered.AssumeTransition(1); err != nil {
		t.Fatal(err)
	}
	if err = filtered.NextState(); err != nil {
		t.Fatal(err)
	}
	if _, err = filtered.PrepareTransition(1, next); err != nil {
		t.Fatal(err)
	}
	if changed, changeErr := filtered.MayChangeAssertion(1, StateInvariant, 0, boolExpr(false)); changeErr != nil || changed {
		t.Fatalf("state 2 stable invariant = %v, %v", changed, changeErr)
	}
}

func TestFilteredTransitionExecutorValidationAndEmptyFilters(t *testing.T) {
	if _, err := NewFilteredTransitionExecutor("(", "", NewTransitionExecutor(nil, nil)); err == nil {
		t.Fatal("invalid transition filter was accepted")
	}
	if _, err := NewFilteredTransitionExecutor("", "(", NewTransitionExecutor(nil, nil)); err == nil {
		t.Fatal("invalid invariant filter was accepted")
	}
	if _, err := NewFilteredTransitionExecutor("", "", nil); err == nil {
		t.Fatal("nil executor was accepted")
	}
	base := NewTransitionExecutor(nil, nil)
	filtered, err := NewFilteredTransitionExecutor("", "", base)
	if err != nil {
		t.Fatal(err)
	}
	if enabled, err := filtered.PrepareTransition(3, boolExpr(true)); err != nil || !enabled {
		t.Fatalf("empty transition filter = %v, %v", enabled, err)
	}
	if changed, err := filtered.MayChangeAssertion(3, StateInvariant, 0, boolExpr(true)); err != nil || !changed {
		t.Fatalf("empty initial invariant filter = %v, %v", changed, err)
	}
}
