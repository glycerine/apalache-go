package apalache

import (
	"context"
	"strings"
	"testing"
)

func TestParseCheckerTuningUsesScalaSearchWordsAndFullMatches(t *testing.T) {
	tuning, err := ParseCheckerTuning(map[string]string{
		"search.transitionFilter": "(0->1|[1-9]->0)",
		"search.invariantFilter":  "1->(state0|action1)",
		"search.invariant.mode":   "after",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tuning.allowsTransition(0, 1) || !tuning.allowsTransition(3, 0) || tuning.allowsTransition(0, 0) {
		t.Fatal("transition filter did not use step->transition full matches")
	}
	if !tuning.allowsInvariant(1, "state", 0) || !tuning.allowsInvariant(1, "action", 1) ||
		tuning.allowsInvariant(11, "state", 0) {
		t.Fatal("invariant filter did not use step->kindN full matches")
	}
}

func TestParseCheckerTuningRejectsUnsupportedAndMalformedValues(t *testing.T) {
	for _, values := range []map[string]string{
		{"search.transitionFilter": "("},
		{"search.invariant.mode": "during"},
		{"smt.randomSeed": "1"},
		{"z3.smt.random_seed": "1"},
	} {
		if _, err := ParseCheckerTuning(values); err == nil {
			t.Fatalf("ParseCheckerTuning(%v) succeeded", values)
		}
	}
	if tuning, err := ParseCheckerTuning(map[string]string{
		"search.transitionFilter": "", "search.invariantFilter": "",
	}); err != nil || !tuning.allowsTransition(42, 9) || !tuning.allowsInvariant(42, "state", 9) {
		t.Fatalf("empty filters = %#v, %v", tuning, err)
	}
}

func TestBoundedSafetyHonorsInvariantFilter(t *testing.T) {
	module, configuration := counterSafetyModule(0)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	tuning, err := ParseCheckerTuning(map[string]string{"search.invariantFilter": "1->state0"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, Tuning: tuning})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 1 || result.Invariant != "Inv" {
		t.Fatalf("filtered invariant result = %#v", result)
	}
}

func TestParseCheckerTuningErrorsNameTheField(t *testing.T) {
	_, err := ParseCheckerTuning(map[string]string{"unsupported": "true"})
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("unsupported field error = %v", err)
	}
}
