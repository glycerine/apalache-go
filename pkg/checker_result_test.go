package apalache

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestCheckerResultKindsMatchScalaNamesAndSuccessClassification(t *testing.T) {
	tests := []struct {
		kind CheckerResultKind
		name string
		ok   bool
	}{
		{CheckerNoError, "NoError", true},
		{CheckerError, "Error", false},
		{CheckerDeadlock, "Deadlock", false},
		{CheckerExecutionsTooShort, "ExecutionsTooShort", true},
		{CheckerSMTTimeout, "SmtTimeout", true},
		{CheckerInterrupted, "Interrupted", false},
		{CheckerRuntimeError, "RuntimeError", false},
	}
	for _, test := range tests {
		result := CheckerResult{Kind: test.kind}
		if result.String() != test.name || result.IsOK() != test.ok {
			t.Fatalf("%d => name=%q ok=%v", test.kind, result.String(), result.IsOK())
		}
	}
}

func TestCheckerResultJSONMatchesScalaVariants(t *testing.T) {
	module := NewModule("Root", []Decl{NewVarDecl("x", Typed(IntT1))})
	trace := BoundedCounterexample{
		Module: module, Depth: 0, Invariant: "Inv",
		States: []DecodedState{{Values: map[string]Expr{"x": intLiteral(1)}}},
	}
	for _, test := range []struct {
		name   string
		result CheckerResult
		want   map[string]any
	}{
		{"no error", CheckerResult{Kind: CheckerNoError}, map[string]any{"checking_result": "NoError"}},
		{"error", CheckerResult{Kind: CheckerError, NErrors: 2, Counterexamples: []BoundedCounterexample{trace}},
			map[string]any{"checking_result": "Error", "nerrors": float64(2)}},
		{"deadlock", CheckerResult{Kind: CheckerDeadlock, Deadlock: &trace},
			map[string]any{"checking_result": "Deadlock"}},
		{"deadlock without model", CheckerResult{Kind: CheckerDeadlock},
			map[string]any{"checking_result": "Deadlock"}},
		{"timeout", CheckerResult{Kind: CheckerSMTTimeout, NTimeouts: 3},
			map[string]any{"checking_result": "SmtTimeout", "ntimeouts": float64(3)}},
		{"interrupted", CheckerResult{Kind: CheckerInterrupted}, map[string]any{"checking_result": "Interrupted"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(test.result)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err = json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatal(err)
			}
			for key, want := range test.want {
				if !reflect.DeepEqual(decoded[key], want) {
					t.Fatalf("field %s = %#v, want %#v; JSON=%s", key, decoded[key], want, encoded)
				}
			}
			counterexamples, hasCounterexamples := decoded["counterexamples"].([]any)
			switch test.result.Kind {
			case CheckerError:
				if !hasCounterexamples || len(counterexamples) != 1 || counterexamples[0].(map[string]any)[ITFMetaField] == nil {
					t.Fatalf("error counterexamples = %#v", decoded["counterexamples"])
				}
			case CheckerDeadlock:
				wantLength := 0
				if test.result.Deadlock != nil {
					wantLength = 1
				}
				if !hasCounterexamples || len(counterexamples) != wantLength {
					t.Fatalf("deadlock counterexamples = %#v, want length %d", decoded["counterexamples"], wantLength)
				}
			default:
				if _, found := decoded["counterexamples"]; found {
					t.Fatalf("unexpected counterexamples: %s", encoded)
				}
			}
		})
	}

	if _, err := json.Marshal(CheckerResult{
		Kind: CheckerError, NErrors: 1, Counterexamples: []BoundedCounterexample{{States: trace.States}},
	}); err == nil {
		t.Fatal("counterexample without module was encoded")
	}
}

func TestBoundedModelCheckerImplementsStandaloneCheckerBoundary(t *testing.T) {
	module, configuration := counterSafetyModule(0)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	var checker Checker = BoundedModelChecker{
		Module: prepared.Module, Predicates: prepared.Predicates,
		Options: BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false},
	}
	result, err := checker.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Kind != CheckerError || result.NErrors != 1 || len(result.Counterexamples) != 1 ||
		result.Counterexamples[0].Module != prepared.Module {
		t.Fatalf("standalone checker result = %#v", result)
	}
	if _, err = json.Marshal(result); err != nil {
		t.Fatalf("standalone result JSON: %v", err)
	}
}

func TestBoundedSafetyResultMapsEveryCheckerOutcome(t *testing.T) {
	module := NewModule("Root", []Decl{NewVarDecl("x", Typed(IntT1))})
	state := []DecodedState{{Values: map[string]Expr{"x": intLiteral(1)}}}
	tests := []struct {
		safety BoundedSafetyResult
		kind   CheckerResultKind
	}{
		{BoundedSafetyResult{Outcome: SafetyNoCounterexample}, CheckerNoError},
		{BoundedSafetyResult{Outcome: SafetyExecutionsTooShort}, CheckerExecutionsTooShort},
		{BoundedSafetyResult{Outcome: SafetyCounterexample, Counterexamples: []BoundedCounterexample{{States: state}}}, CheckerError},
		{BoundedSafetyResult{Outcome: SafetyDeadlock, Depth: 2, States: state}, CheckerDeadlock},
		{BoundedSafetyResult{Outcome: SafetyUnknown}, CheckerSMTTimeout},
	}
	for _, test := range tests {
		result := test.safety.AsCheckerResult(module)
		if result.Kind != test.kind {
			t.Fatalf("outcome %d => %#v", test.safety.Outcome, result)
		}
		if result.Kind == CheckerError && (result.NErrors != 1 || result.Counterexamples[0].Module != module) {
			t.Fatalf("error mapping = %#v", result)
		}
		if result.Kind == CheckerDeadlock && (result.Deadlock == nil || !result.Deadlock.Deadlock || result.Deadlock.Module != module) {
			t.Fatalf("deadlock mapping = %#v", result)
		}
	}
}
