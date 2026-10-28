package apalache

import (
	"context"
	"testing"
)

func TestSimulateBehaviorSelectsFirstSatisfiablePath(t *testing.T) {
	module, configuration := counterSafetyModule(100)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	result, err := SimulateBehavior(context.Background(), prepared.Module, prepared.Predicates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SimulationCompleted || result.Depth != 2 || result.InitTransition != 0 ||
		len(result.NextTransitions) != 2 || len(result.States) != 3 {
		t.Fatalf("simulation = %+v", result)
	}
	for index, state := range result.States {
		formatted, formatErr := FormatTLAExpression(state.Values["x"])
		if formatErr != nil || formatted != string(rune('0'+index)) {
			t.Fatalf("state %d = %#v (%v)", index, state, formatErr)
		}
	}
}

func TestSimulateBehaviorReportsDeadlock(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x),
		MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(0))))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit",
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := SimulateBehavior(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SimulationDeadlock || result.Depth != 0 || len(result.States) != 1 {
		t.Fatalf("simulation = %+v", result)
	}
}

func TestSimulateBehaviorRejectsUnsatisfiableInitialState(t *testing.T) {
	module, configuration := counterSafetyModule(100)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	init, _ := declarationNamed(prepared.Module, NormalizedInitPrefix+"0000")
	init.(*OperDecl).SetBody(NewValueExpr(BoolValue(false), Typed(BoolT1)))
	if _, err := SimulateBehavior(context.Background(), prepared.Module, prepared.Predicates, 1); err == nil {
		t.Fatal("unsatisfiable initial state was accepted")
	}
}

func TestSimulateBehaviorsRandomizesIndependentRuns(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), prime, intLiteral(1)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), prime, intLiteral(2))))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit",
	}})
	if err != nil {
		t.Fatal(err)
	}
	results, err := SimulateBehaviorsWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		SimulationOptions{MaxSteps: 1, MaxRuns: 20, RandomSeed: 17})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 20 {
		t.Fatalf("simulation run count = %d", len(results))
	}
	selected := make(map[int]struct{})
	for _, result := range results {
		if result.Outcome != SimulationCompleted || len(result.NextTransitions) != 1 {
			t.Fatalf("simulation result = %#v", result)
		}
		selected[result.NextTransitions[0]] = struct{}{}
	}
	if len(selected) != 2 {
		t.Fatalf("randomized transitions = %v", selected)
	}
}

func TestSimulateBehaviorsStopsRunsAfterDeadlock(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x),
		NewValueExpr(BoolValue(false), Typed(BoolT1))))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit",
	}})
	if err != nil {
		t.Fatal(err)
	}
	results, err := SimulateBehaviorsWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		SimulationOptions{MaxSteps: 2, MaxRuns: 10, RandomSeed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Outcome != SimulationDeadlock {
		t.Fatalf("deadlocking runs = %#v", results)
	}
}

func TestSimulateAndCheckStopsAtInvariantErrorBudget(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "SimInv.tla", `---- MODULE SimInv ----
EXTENDS Integers, FiniteSets
\* @type: Int;
VARIABLE x
Init == x \in {1, 2, 3}
Next == x' = x
Inv == x = 0
View == x
====
`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics = %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{
		Specification: SpecificationConfiguration{
			Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"}, View: "View",
		}, Sources: frontend.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	report, err := SimulateAndCheckWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		SimulationOptions{MaxSteps: 0, MaxRuns: 20, MaxErrors: 2, RandomSeed: 17})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Runs) != 1 || len(report.Counterexamples) != 2 {
		t.Fatalf("simulation report = %#v", report)
	}
	views := make(map[string]struct{})
	for _, counterexample := range report.Counterexamples {
		if counterexample.Invariant != "Inv" || len(counterexample.View) != 1 {
			t.Fatalf("simulation counterexample = %#v", counterexample)
		}
		formatted, formatErr := FormatTLAExpression(counterexample.View[0])
		if formatErr != nil {
			t.Fatal(formatErr)
		}
		views[formatted] = struct{}{}
	}
	if len(views) != 2 {
		t.Fatalf("simulation distinct views = %v", views)
	}
}
