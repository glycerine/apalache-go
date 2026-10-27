package apalache

import (
	"context"
	"reflect"
	"testing"
)

func counterSafetyModule(limit int64) (*Module, SpecificationConfiguration) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))),
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(limit)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	configuration := SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}
	return module, configuration
}

func TestCheckBoundedSafetyFindsFirstBadDepth(t *testing.T) {
	module, configuration := counterSafetyModule(2)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}

	safe, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if safe.Outcome != SafetyNoCounterexample || safe.Depth != -1 || safe.ExploredExecutions != 2 {
		t.Fatalf("one-step result = %#v", safe)
	}

	unsafe, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if unsafe.Outcome != SafetyCounterexample || unsafe.Depth != 2 || unsafe.Invariant != "Inv" ||
		unsafe.InitTransition != 0 || !reflect.DeepEqual(unsafe.NextTransitions, []int{0, 0}) {
		t.Fatalf("counterexample result = %#v", unsafe)
	}
	if len(unsafe.States) != 3 {
		t.Fatalf("decoded counterexample states = %#v", unsafe.States)
	}
	for index, expected := range []string{"0", "1", "2"} {
		formatted, formatErr := FormatTLAExpression(unsafe.States[index].Values["x"])
		if formatErr != nil || formatted != expected {
			t.Fatalf("state %d x = %q, %v", index, formatted, formatErr)
		}
	}
}

func TestCheckBoundedSafetyReportsInitialViolation(t *testing.T) {
	module, configuration := counterSafetyModule(0)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: configuration})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 0 || len(result.NextTransitions) != 0 {
		t.Fatalf("initial counterexample = %#v", result)
	}
}

func TestCheckBoundedSafetyEnumeratesTransitionAlternatives(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("OR"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(3))))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(2)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if count := len(TransitionsFromModule(prepared.Module, NormalizedInitPrefix)); count != 2 {
		t.Fatalf("normalized initial transition count = %d", count)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 0)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 0 || result.ExploredExecutions != 2 {
		t.Fatalf("alternative initial paths = %#v", result)
	}
}

func TestCheckBoundedSafetySkipsUnsatisfiablePaths(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	assignment := MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))
	init := boolDefinition("Init", MustOperExpr(Op("AND"), Typed(BoolT1), assignment,
		NewValueExpr(BoolValue(false), Typed(BoolT1))))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x))
	invariant := boolDefinition("Inv", NewValueExpr(BoolValue(false), Typed(BoolT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample || result.ExploredExecutions != 0 {
		t.Fatalf("disabled initial path = %#v", result)
	}
}

func TestCheckBoundedSafetyDetectsAndCanIgnoreDeadlocks(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))),
			MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))),
		MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(0))))
	invariant := boolDefinition("Inv", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	deadlock, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if deadlock.Outcome != SafetyDeadlock || deadlock.Depth != 0 || len(deadlock.States) != 1 {
		t.Fatalf("deadlock result = %#v", deadlock)
	}
	ignored, err := CheckBoundedSafetyWithOptions(context.Background(), prepared.Module, prepared.Predicates,
		BoundedSafetyOptions{MaxSteps: 1, CheckDeadlocks: false})
	if err != nil {
		t.Fatal(err)
	}
	if ignored.Outcome != SafetyNoCounterexample {
		t.Fatalf("ignored deadlock result = %#v", ignored)
	}
}

func TestCheckBoundedSafetyEnforcesConfiguredConstraints(t *testing.T) {
	for _, test := range []struct {
		name              string
		stateConstraints  []string
		actionConstraints []string
	}{
		{name: "state", stateConstraints: []string{"StateOK"}},
		{name: "action", actionConstraints: []string{"ActionOK"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			x := NewNameExpr("x", Typed(IntT1))
			prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
			constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
			init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
			next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
				MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
			stateOK := boolDefinition("StateOK", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(1)))
			actionOK := boolDefinition("ActionOK", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x))
			invariant := boolDefinition("Inv", NewValueExpr(BoolValue(true), Typed(BoolT1)))
			module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, stateOK, actionOK, invariant)
			prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
				Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
				StateConstraints: test.stateConstraints, ActionConstraints: test.actionConstraints,
			}})
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range append(append([]string(nil), test.stateConstraints...), test.actionConstraints...) {
				if _, found := OperatorBody(prepared.Module, name); !found {
					t.Fatalf("constraint %s was not retained", name)
				}
			}
			result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
			if err != nil {
				t.Fatal(err)
			}
			if result.Outcome != SafetyDeadlock || result.Depth != 0 {
				t.Fatalf("constrained result = %#v", result)
			}
		})
	}
}

func TestCheckBoundedSafetyChecksActionInvariantsOnPendingState(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	actionInvariant := boolDefinition("ActionInv", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, actionInvariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"ActionInv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || !result.ActionInvariant || result.Invariant != "ActionInv" ||
		result.Depth != 1 || !reflect.DeepEqual(result.NextTransitions, []int{0}) || len(result.States) != 2 {
		t.Fatalf("action invariant result = %#v", result)
	}
	for index, expected := range []string{"0", "1"} {
		formatted, formatErr := FormatTLAExpression(result.States[index].Values["x"])
		if formatErr != nil || formatted != expected {
			t.Fatalf("action state %d x = %q, %v", index, formatted, formatErr)
		}
	}
}

func TestCheckBoundedSafetyChecksTraceInvariantsOverStateHistory(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime,
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	recordType, _ := NewRecordType(map[string]Type{"x": IntT1})
	historyType, _ := NewSeqType(recordType)
	history := NewNameExpr("hist", Typed(historyType))
	traceBody := MustOperExpr(Op("LE"), Typed(BoolT1),
		MustOperExpr(Op("Sequences!Len"), Typed(IntT1), history), intLiteral(2))
	traceInvariant := NewOperDecl("TraceInv", []OperatorParam{ValueParam("hist")}, traceBody,
		Typed(mustOperator([]Type{historyType}, BoolT1)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, traceInvariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"TraceInv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || !result.TraceInvariant || result.ActionInvariant ||
		result.Invariant != "TraceInv" || result.Depth != 2 || len(result.States) != 3 {
		t.Fatalf("trace invariant result = %#v", result)
	}
	for index, expected := range []string{"0", "1", "2"} {
		formatted, formatErr := FormatTLAExpression(result.States[index].Values["x"])
		if formatErr != nil || formatted != expected {
			t.Fatalf("trace state %d x = %q, %v", index, formatted, formatErr)
		}
	}
}

func TestCheckBoundedSafetySkolemizesExistentialAssignments(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	binder := NewNameExpr("i", Typed(IntT1))
	setType := mustSet(IntT1)
	constantInit := boolDefinition("CInit", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EXISTS3"), Typed(BoolT1), binder,
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("AND"), Typed(BoolT1),
			MustOperExpr(Op("EQ"), Typed(BoolT1), prime, binder),
			MustOperExpr(Op("EQ"), Typed(BoolT1), binder, intLiteral(2)))))
	invariant := boolDefinition("Inv", MustOperExpr(Op("LT"), Typed(BoolT1), x, intLiteral(2)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), constantInit, init, next, invariant)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	nextTransitions := TransitionsFromModule(prepared.Module, NormalizedNextPrefix)
	if len(nextTransitions) != 1 || countOperator(nextTransitions[0], "Apalache!Skolem") != 1 {
		t.Fatalf("analyzed Next transitions = %v", nextTransitions)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 1)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Depth != 1 || len(result.States) != 2 {
		t.Fatalf("existential assignment result = %#v", result)
	}
	value, formatErr := FormatTLAExpression(result.States[1].Values["x"])
	if formatErr != nil || value != "2" {
		t.Fatalf("existentially assigned x = %q, %v", value, formatErr)
	}
}

func TestCheckBoundedSafetyValidatesInputs(t *testing.T) {
	if _, err := CheckBoundedSafety(context.Background(), nil, DerivedPredicates{}, 0); err == nil {
		t.Fatal("nil module was accepted")
	}
	module := NewModule("M", nil)
	if _, err := CheckBoundedSafety(context.Background(), module, DerivedPredicates{}, 0); err == nil {
		t.Fatal("unprepared module was accepted")
	}
}
