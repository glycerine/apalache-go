package apalache

import (
	"fmt"
	"testing"
)

func searchStateParams(t *testing.T, maxErrors, runs int) *ModelCheckerParams {
	t.Helper()
	input := NewCheckerInput(NewModule("Root", nil), nil, nil, nil, CheckerVerificationConditions{}, nil)
	options := map[string]string{}
	if runs > 0 {
		options["search.simulation"] = "true"
		options["search.simulation.maxRun"] = intString(runs)
	}
	params, err := NewModelCheckerParams(input, 0, options)
	if err != nil {
		t.Fatal(err)
	}
	params.MaxErrors = maxErrors
	return params
}

func intString(value int) string {
	return fmt.Sprintf("%d", value)
}

func TestSearchStateAccumulatesErrorsUntilBudget(t *testing.T) {
	state := NewSearchState(searchStateParams(t, 2, 0))
	first := BoundedCounterexample{Invariant: "Inv1", States: []DecodedState{{Values: map[string]Expr{"x": intLiteral(1)}}}}
	second := BoundedCounterexample{Invariant: "Inv2"}
	state.OnResult(NewCheckerError(first))
	if state.FoundErrors() != 1 || !state.CanContinue() || state.FinalResult().Kind != CheckerError {
		t.Fatalf("after first error: result=%#v continue=%v", state.FinalResult(), state.CanContinue())
	}
	state.OnResult(NewCheckerError(second))
	result := state.FinalResult()
	if result.Kind != CheckerError || result.NErrors != 2 || len(result.Counterexamples) != 2 || state.CanContinue() {
		t.Fatalf("after error budget: %#v continue=%v", result, state.CanContinue())
	}
	result.Counterexamples[0].Invariant = "changed"
	result.Counterexamples[0].States[0].Values["x"] = intLiteral(99)
	if again := state.FinalResult(); again.Counterexamples[0].Invariant != "Inv1" {
		t.Fatalf("FinalResult exposed mutable counterexamples: %#v", again)
	}
}

func TestSearchStateAccumulatesTimeoutsWithoutStopping(t *testing.T) {
	state := NewSearchState(searchStateParams(t, 1, 0))
	state.OnResult(CheckerResult{Kind: CheckerSMTTimeout, NTimeouts: 2})
	state.OnResult(CheckerResult{Kind: CheckerSMTTimeout, NTimeouts: 3})
	if state.Timeouts() != 5 || !state.CanContinue() {
		t.Fatalf("timeouts=%d continue=%v", state.Timeouts(), state.CanContinue())
	}
	if result := state.FinalResult(); result.Kind != CheckerSMTTimeout || result.NTimeouts != 5 || !result.Kind.IsOK() {
		t.Fatalf("timeout final result = %#v", result)
	}
}

func TestSearchStateDeadlockAndTerminalResults(t *testing.T) {
	deadlock := BoundedCounterexample{Deadlock: true}
	state := NewSearchState(searchStateParams(t, 2, 0))
	state.OnResult(CheckerResult{Kind: CheckerDeadlock, Deadlock: &deadlock})
	if result := state.FinalResult(); result.Kind != CheckerDeadlock || result.Deadlock == nil || state.CanContinue() {
		t.Fatalf("deadlock result = %#v continue=%v", result, state.CanContinue())
	}

	for _, kind := range []CheckerResultKind{CheckerInterrupted, CheckerRuntimeError} {
		state = NewSearchState(searchStateParams(t, 1, 0))
		state.OnResult(CheckerResult{Kind: kind})
		if state.FinalResult().Kind != kind || state.CanContinue() || kind.IsOK() {
			t.Fatalf("terminal %s: result=%#v continue=%v", kind, state.FinalResult(), state.CanContinue())
		}
	}

	state = NewSearchState(searchStateParams(t, 2, 0))
	state.OnResult(NewCheckerError(BoundedCounterexample{Invariant: "Inv"}))
	state.OnResult(CheckerResult{Kind: CheckerDeadlock, Deadlock: &deadlock})
	if result := state.FinalResult(); result.Kind != CheckerError || result.NErrors != 1 || result.Deadlock != nil {
		t.Fatalf("deadlock after excluded counterexample = %#v", result)
	}
}

func TestSearchStateSimulationRunBudgetAndShortExecutions(t *testing.T) {
	state := NewSearchState(searchStateParams(t, 1, 3))
	if state.RunsLeft() != 3 || !state.CanContinue() {
		t.Fatalf("initial runs=%d continue=%v", state.RunsLeft(), state.CanContinue())
	}
	state.OnResult(CheckerResult{Kind: CheckerExecutionsTooShort})
	state.OnRunDone()
	if state.RunsLeft() != 2 || !state.CanContinue() || state.FinalResult().Kind != CheckerNoError {
		t.Fatalf("recoverable short run: runs=%d result=%#v", state.RunsLeft(), state.FinalResult())
	}
	state.OnResult(CheckerResult{Kind: CheckerExecutionsTooShort})
	state.OnRunDone()
	state.OnResult(CheckerResult{Kind: CheckerExecutionsTooShort})
	state.OnRunDone()
	if state.RunsLeft() != 0 || state.CanContinue() || state.FinalResult().Kind != CheckerExecutionsTooShort ||
		!CheckerExecutionsTooShort.IsOK() {
		t.Fatalf("exhausted short runs: runs=%d result=%#v", state.RunsLeft(), state.FinalResult())
	}
}
