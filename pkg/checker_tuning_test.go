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
		{"smt.randomSeed": "-1"},
		{"z3.smt.random_seed": "2147483648"},
		{"z3.smt.mbqi": "maybe"},
		{"z3.not_a_parameter": "true"},
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

func TestParseCheckerTuningUsesPinnedG3TypedCatalog(t *testing.T) {
	tuning, err := ParseCheckerTuning(map[string]string{
		"z3.smt.mbqi":              "true",
		"z3.smt.array.extensional": "false",
		"z3.smt.arith.solver":      "2",
		"z3.smt.restart_factor":    "1.25",
		"z3.sat.random_freq":       "0.25",
		"z3.sat.phase":             "random",
		"z3.pp.max_width":          "120",
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]string{
		"smt.mbqi": "true", "smt.array.extensional": "false", "smt.arith.solver": "2",
		"smt.restart_factor": "1.25", "sat.random_freq": "0.25", "sat.phase": "random",
		"pp.max_width": "120",
	} {
		if got := tuning.solverTextOptions[key]; got != want {
			t.Fatalf("g3 text option %s = %q, want %q", key, got, want)
		}
	}
}

func TestParseCheckerTuningMatchesFineTuningParserOptions(t *testing.T) {
	tuning, err := ParseCheckerTuning(map[string]string{
		"rewriter.shortCircuit":       "true",
		"z3.memory_high_watermark_mb": "123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tuning.shortCircuit {
		t.Fatal("rewriter.shortCircuit was not enabled")
	}
	if got := tuning.solverUIntOptions["memory_high_watermark"]; got != 123 {
		t.Fatalf("memory_high_watermark = %d, want 123", got)
	}
	if solver, err := newG3SolverContextWithUIntOptions("QF_AUFNIA", tuning.solverUIntOptions); err != nil {
		t.Fatalf("g3 rejected memory watermark: %v", err)
	} else if err = solver.Close(); err != nil {
		t.Fatal(err)
	}
	for _, values := range []map[string]string{
		{"rewriter.shortCircuit": "12333"},
		{"z3.memory_high_watermark_mb": "aaaa"},
	} {
		if _, err = ParseCheckerTuning(values); err == nil {
			t.Fatalf("invalid fine tuning accepted: %v", values)
		}
	}
}

func TestParseCheckerTuningMapsIntegerSolverOptionsToG3(t *testing.T) {
	tuning, err := ParseCheckerTuning(map[string]string{
		"smt.randomSeed":     "17",
		"z3.smt.random_seed": "19",
		"z3.sat.random_seed": "23",
		"z3.timeout":         "1000",
		"z3.rlimit":          "500",
	})
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range map[string]uint32{
		"smt.random_seed": 19, "sat.random_seed": 23,
		"timeout": 1000, "rlimit": 500,
	} {
		if got := tuning.solverUIntOptions[key]; got != want {
			t.Fatalf("solver option %s = %d, want %d", key, got, want)
		}
	}
	if solver, err := newG3SolverContextWithUIntOptions("QF_AUFNIA", tuning.solverUIntOptions); err != nil {
		t.Fatalf("g3 rejected validated tuning: %v", err)
	} else if err = solver.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestCheckerTuningWithSMTRandomSeedIsImmutable(t *testing.T) {
	original, err := ParseCheckerTuning(map[string]string{"z3.rlimit": "11"})
	if err != nil {
		t.Fatal(err)
	}
	seeded := original.WithSMTRandomSeed(17)
	if _, found := original.solverUIntOptions["smt.random_seed"]; found {
		t.Fatal("WithSMTRandomSeed mutated the original tuning")
	}
	if seeded.solverUIntOptions["smt.random_seed"] != 17 || seeded.solverUIntOptions["sat.random_seed"] != 17 ||
		seeded.solverUIntOptions["rlimit"] != 11 {
		t.Fatalf("seeded solver options = %#v", seeded.solverUIntOptions)
	}
}

func TestParseCheckerTuningSimulationControls(t *testing.T) {
	tuning, err := ParseCheckerTuning(map[string]string{
		"search.simulation": "true", "search.simulation.maxRun": "17", "search.outputTraces": "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !tuning.HasSimulationControls() || tuning.SimulationRuns() != 17 || !tuning.simulation ||
		!tuning.HasOutputTraces() || !tuning.OutputTraces() {
		t.Fatalf("simulation tuning = %#v", tuning)
	}
	for _, values := range []map[string]string{
		{"search.simulation": "yes"}, {"search.simulation.maxRun": "0"}, {"search.outputTraces": "yes"},
	} {
		if _, err = ParseCheckerTuning(values); err == nil {
			t.Fatalf("invalid simulation tuning accepted: %v", values)
		}
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
