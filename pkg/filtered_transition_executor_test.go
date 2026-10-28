package apalache

import "testing"

func TestFilteredTransitionExecutorMatchesScala(t *testing.T) {
	base := NewTransitionExecutor(nil, nil)
	filtered, err := NewFilteredTransitionExecutor("1->2", "1->state0", base)
	if err != nil {
		t.Fatal(err)
	}
	truth := boolExpr(true)
	if enabled, err := filtered.PrepareTransition(2, truth); err != nil || enabled || len(base.prepared) != 0 {
		t.Fatalf("excluded transition = %v, prepared %v, error %v", enabled, base.PreparedTransitionNumbers(), err)
	}
	base.step = 1
	if enabled, err := filtered.PrepareTransition(2, truth); err != nil || !enabled {
		t.Fatalf("included transition = %v, %v", enabled, err)
	}
	// The invariant is included now but was excluded at step zero, so Scala
	// requires checking it even though the transition changes no variables.
	if changed, err := filtered.MayChangeAssertion(2, StateInvariant, 0, truth); err != nil || !changed {
		t.Fatalf("newly included invariant = %v, %v", changed, err)
	}
	if changed, err := filtered.MayChangeAssertion(2, ActionInvariant, 0, truth); err != nil || changed {
		t.Fatalf("excluded action invariant = %v, %v", changed, err)
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
