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
