package apalache

import (
	"reflect"
	"testing"
)

func TestModelCheckerParamsMatchesScalaDefaultsAndDerivedFields(t *testing.T) {
	module := NewModule("Root", []Decl{
		NewConstDecl("N", Typed(IntT1)), NewVarDecl("x", Typed(IntT1)),
	})
	input := NewCheckerInput(module, nil, nil, nil, CheckerVerificationConditions{}, nil)
	params, err := NewModelCheckerParams(input, 7, nil)
	if err != nil {
		t.Fatal(err)
	}
	if params.StepsBound != 7 || !params.DiscardDisabled || !params.CheckForDeadlocks ||
		params.InvariantMode != InvariantBeforeJoin || params.MaxErrors != 1 || params.TimeoutSMTSeconds != 0 ||
		params.JailTimeoutMinSeconds != 1800 || params.JailTimeoutFactor != 200 ||
		params.IdleTimeoutSeconds != 1800 || params.IdleTimeoutMilliseconds() != 1_800_000 ||
		params.RandomSimulation || params.SimulationRuns != DefaultSimulationRuns || params.SaveRuns != DefaultOutputTraces ||
		params.SMTEncoding != "OOPSLA19" {
		t.Fatalf("default params = %#v", params)
	}
	if !reflect.DeepEqual(params.Constants(), map[string]struct{}{"N": {}}) ||
		!reflect.DeepEqual(params.Variables(), map[string]struct{}{"x": {}}) {
		t.Fatalf("derived names: constants=%v variables=%v", params.Constants(), params.Variables())
	}
	constants := params.Constants()
	delete(constants, "N")
	if _, found := params.Constants()["N"]; !found {
		t.Fatal("Constants exposed mutable state")
	}
}

func TestModelCheckerParamsParsesEveryOwnedTuningField(t *testing.T) {
	module := NewModule("Root", []Decl{NewVarDecl("x", Typed(IntT1))})
	input := NewCheckerInput(module, nil, nil, nil, CheckerVerificationConditions{}, nil)
	params, err := NewModelCheckerParams(input, 3, map[string]string{
		"search.invariant.mode":        "after",
		"search.transitionFilter":      "0->1",
		"search.invariantFilter":       "2->state0",
		"search.split.timeout.minimum": "11",
		"search.split.timeout.factor":  "12",
		"search.idle.timeout":          "13",
		"search.simulation":            "true",
		"search.simulation.maxRun":     "14",
		"search.outputTraces":          "true",
	})
	if err != nil {
		t.Fatal(err)
	}
	if params.InvariantMode != InvariantAfterJoin || params.TransitionFilter != "0->1" ||
		params.InvariantFilter != "2->state0" || params.JailTimeoutMinSeconds != 11 ||
		params.JailTimeoutFactor != 12 || params.IdleTimeoutSeconds != 13 ||
		!params.RandomSimulation || params.SimulationRuns != 14 || !params.SaveRuns {
		t.Fatalf("configured params = %#v", params)
	}
	for _, values := range []map[string]string{
		{"search.split.timeout.minimum": "no"},
		{"search.split.timeout.factor": "2147483648"},
		{"search.idle.timeout": "no"},
	} {
		if _, err = NewModelCheckerParams(input, 1, values); err == nil {
			t.Fatalf("invalid params accepted: %v", values)
		}
	}
}

func TestModelCheckerContextRetainsComponentsAndCopiesListeners(t *testing.T) {
	module := NewModule("Root", nil)
	input := NewCheckerInput(module, nil, nil, nil, CheckerVerificationConditions{}, nil)
	params, err := NewModelCheckerParams(input, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	executor := NewTransitionExecutor(nil, nil)
	collector := &CollectCounterexamplesModelCheckerListener{}
	listeners := []ModelCheckerListener{collector}
	context := NewModelCheckerContext(params, input, executor, listeners)
	listeners[0] = nil
	if context.Params() != params || context.CheckerInput() != input || context.TransitionExecutor() != executor ||
		len(context.Listeners()) != 1 || context.Listeners()[0] != collector {
		t.Fatalf("context components were not retained")
	}
	copy := context.Listeners()
	copy[0] = nil
	if context.Listeners()[0] != collector {
		t.Fatal("Listeners exposed mutable state")
	}
}
