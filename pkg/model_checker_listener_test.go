package apalache

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type modelCheckerListenerFuncs struct {
	counterexample func(BoundedCounterexample, int) error
	example        func([]DecodedState, int) error
}

func (listener modelCheckerListenerFuncs) OnCounterexample(counterexample BoundedCounterexample, index int) error {
	if listener.counterexample == nil {
		return nil
	}
	return listener.counterexample(counterexample, index)
}

func (listener modelCheckerListenerFuncs) OnExample(example []DecodedState, index int) error {
	if listener.example == nil {
		return nil
	}
	return listener.example(example, index)
}

func TestCheckBoundedSafetyNotifiesCounterexampleCollector(t *testing.T) {
	module, configuration := counterSafetyModule(0)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	collector := &CollectCounterexamplesModelCheckerListener{}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false, Listeners: []ModelCheckerListener{collector}})
	if err != nil {
		t.Fatal(err)
	}
	collected := collector.Counterexamples()
	if len(collected) != 1 || collected[0].Invariant != "Inv" || collected[0].Depth != 0 {
		t.Fatalf("collected counterexamples = %#v", collected)
	}
	if !reflect.DeepEqual(collected, result.Counterexamples) {
		t.Fatalf("collector = %#v, result = %#v", collected, result.Counterexamples)
	}

	collected[0].Invariant = "changed"
	collected[0].States[0].Values["x"] = intLiteral(99)
	again := collector.Counterexamples()
	if again[0].Invariant != "Inv" {
		t.Fatalf("collector exposed counterexample mutation: %#v", again[0])
	}
	value, formatErr := FormatTLAExpression(again[0].States[0].Values["x"])
	if formatErr != nil || value != "0" {
		t.Fatalf("collector exposed state mutation: %s, %v", value, formatErr)
	}
}

func TestCheckBoundedSafetyNotifiesExamplesAndPropagatesListenerErrors(t *testing.T) {
	module, configuration := counterSafetyModule(10)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	var exampleIndex int
	var exampleStates []DecodedState
	listener := modelCheckerListenerFuncs{example: func(states []DecodedState, index int) error {
		exampleIndex = index
		exampleStates = cloneDecodedStates(states)
		return nil
	}}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{
			MaxSteps: 0, CheckDeadlocks: false, Tuning: CheckerTuning{}.WithOutputTraces(true),
			Listeners: []ModelCheckerListener{listener},
		})
	if err != nil {
		t.Fatal(err)
	}
	if exampleIndex != 1 || !reflect.DeepEqual(exampleStates, result.ExampleStates) {
		t.Fatalf("example callback index=%d states=%#v, result=%#v", exampleIndex, exampleStates, result.ExampleStates)
	}

	want := errors.New("listener stopped")
	failing := modelCheckerListenerFuncs{counterexample: func(BoundedCounterexample, int) error { return want }}
	unsafeModule, unsafeConfiguration := counterSafetyModule(0)
	unsafe, err := PrepareSafetyTransitions(unsafeModule, SafetyPipelineOptions{Specification: unsafeConfiguration})
	if err != nil {
		t.Fatal(err)
	}
	_, err = CheckBoundedSafetyWithOptions(context.Background(), unsafe.Module, unsafe.Predicates,
		BoundedSafetyOptions{MaxSteps: 0, CheckDeadlocks: false, Listeners: []ModelCheckerListener{failing}})
	if !errors.Is(err, want) || !strings.Contains(err.Error(), "notify counterexample listener") {
		t.Fatalf("listener error = %v", err)
	}
}

func TestCheckBoundedSafetyNotifiesDeadlockWithScalaIndex(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	module := configuredTestModule(
		NewVarDecl("x", Typed(IntT1)),
		boolDefinition("CInit", boolExpr(true)),
		boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))),
		boolDefinition("Next", MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1),
				MustOperExpr(Op("PRIME"), Typed(IntT1), x), x),
			boolExpr(false))),
	)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit",
	}})
	if err != nil {
		t.Fatal(err)
	}
	var seen BoundedCounterexample
	index := -1
	listener := modelCheckerListenerFuncs{counterexample: func(counterexample BoundedCounterexample, errorIndex int) error {
		seen, index = counterexample, errorIndex
		return nil
	}}
	result, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: true, Listeners: []ModelCheckerListener{listener}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyDeadlock || !seen.Deadlock || seen.Depth != result.Depth || index != 0 {
		t.Fatalf("deadlock result=%#v callback=%#v index=%d", result, seen, index)
	}
}

func TestDumpFilesModelCheckerListenerWritesLatestAndIndexedFormats(t *testing.T) {
	module := NewModule("Root", []Decl{
		NewVarDecl("x", Typed(IntT1)),
		NewOperDecl("Inv", nil,
			MustOperExpr(Op("GT"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(0)),
			Typed(mustOperator(nil, BoolT1))),
	})
	states := []DecodedState{{Values: map[string]Expr{"x": intLiteral(0)}}}
	directory := t.TempDir()
	listener := NewDumpFilesModelCheckerListener(module, directory)
	if err := listener.OnCounterexample(BoundedCounterexample{Invariant: "Inv", States: states}, 2); err != nil {
		t.Fatal(err)
	}
	if err := listener.OnExample(states, 3); err != nil {
		t.Fatal(err)
	}

	for _, pair := range [][2]string{
		{"violation.tla", "violation2.tla"},
		{"MCviolation.out", "MCviolation2.out"},
		{"violation.json", "violation2.json"},
		{"violation.itf.json", "violation2.itf.json"},
		{"example.tla", "example3.tla"},
		{"MCexample.out", "MCexample3.out"},
		{"example.json", "example3.json"},
		{"example.itf.json", "example3.itf.json"},
	} {
		latest, err := os.ReadFile(filepath.Join(directory, pair[0]))
		if err != nil {
			t.Fatal(err)
		}
		indexed, err := os.ReadFile(filepath.Join(directory, pair[1]))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(latest, indexed) {
			t.Fatalf("latest %s differs from indexed %s", pair[0], pair[1])
		}
		if strings.HasSuffix(pair[0], ".json") && !json.Valid(latest) {
			t.Fatalf("%s is not valid JSON: %s", pair[0], latest)
		}
	}

	violationTLA, err := os.ReadFile(filepath.Join(directory, "violation.tla"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(violationTLA), "InvariantViolation == ~") {
		t.Fatalf("violation trace lacks negated invariant: %s", violationTLA)
	}
	exampleTLA, err := os.ReadFile(filepath.Join(directory, "example.tla"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(exampleTLA), "InvariantViolation == TRUE") {
		t.Fatalf("example trace lacks TRUE data: %s", exampleTLA)
	}
}
