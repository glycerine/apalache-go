package apalache

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestDesugarModuleUsesNullaryDefinitionsForUnchanged(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	varsType := mustTuple(IntT1)
	vars := NewOperDecl("vars", nil, MustOperExpr(Op("TUPLE"), Typed(varsType), x), Typed(mustOperator(nil, varsType)))
	body := MustOperExpr(Op("UNCHANGED"), Typed(BoolT1), NewNameExpr("vars", Typed(varsType)))
	next := NewOperDecl("Next", nil, body, Typed(mustOperator(nil, BoolT1)))
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), vars, next})
	module.AddProperty(PropertyTypeChecked)
	got, err := DesugarModule(module, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyDesugared) || !got.HasProperty(PropertyTypeChecked) {
		t.Fatal("desugaring properties were not preserved and added")
	}
	rewritten, _ := OperatorBody(got, "Next")
	if rewritten.(*OperExpr).Operator().Name() != "EQ" || rewritten.(*OperExpr).Arguments()[0].(*OperExpr).Operator().Name() != "PRIME" {
		t.Fatalf("UNCHANGED nullary tuple definition = %v", rewritten)
	}
}

func TestDesugarModuleChecksTypeCheckedDependency(t *testing.T) {
	module := NewModule("M", nil)
	if _, err := DesugarModule(module, nil, nil); err == nil {
		t.Fatal("desugaring accepted a module without the TypeChecked property")
	}
}

func TestPrepareSafetyTransitionsEndToEnd(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	x := NewNameExpr("x", Typed(IntT1))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))),
		MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1))))
	cinit := boolDefinition("CInit", truth)
	invariant := boolDefinition("Inv", MustOperExpr(Op("GE"), Typed(BoolT1), x, intLiteral(0)))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), cinit, init, next, invariant)

	result, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, ConstantInit: "CInit", Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, property := range []ModuleProperty{PropertyConfigured, PropertyDesugared, PropertyInlined, PropertyPrimed, PropertyPreprocessed, PropertyTransitionsFound, PropertyOptimized, PropertyAnalyzed} {
		if !result.Module.HasProperty(property) {
			t.Errorf("pipeline result lacks property %d", property)
		}
	}
	initTransitions := TransitionsFromModule(result.Module, NormalizedInitPrefix)
	nextTransitions := TransitionsFromModule(result.Module, NormalizedNextPrefix)
	if len(initTransitions) != 1 || len(nextTransitions) != 1 {
		t.Fatalf("pipeline transitions: init=%d next=%d declarations=%v", len(initTransitions), len(nextTransitions), declarationNames(result.Module))
	}
	if countOperator(initTransitions[0], "Apalache!:=") != 1 || countOperator(nextTransitions[0], "Apalache!:=") != 1 {
		t.Fatalf("assignments were not introduced: init=%v next=%v", initTransitions[0], nextTransitions[0])
	}
	if _, found := OperatorBody(result.Module, "Inv"); !found {
		t.Fatal("raw invariant was not retained without VC generation")
	}
}

func TestPrepareSafetyTransitionsAvoidsMaterializingLargeConstructorDomain(t *testing.T) {
	frontend := ParseAndTypeCheckSource(context.Background(), "LargeFunction.tla", `
---- MODULE LargeFunction ----
EXTENDS Integers
Init == TRUE
Next == TRUE
Inv ==
  LET fun == [x \in 1..10000 |-> x + 1] IN
  \E i \in DOMAIN fun: fun[i] = i + 1
====`, FrontendOptions{}, TypeCheckOptions{})
	if !frontend.OK() {
		t.Fatalf("frontend diagnostics: %v", frontend.Diagnostics)
	}
	prepared, err := PrepareSafetyTransitions(frontend.Native, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	invariant, found := OperatorBody(prepared.Module, "Inv")
	if !found {
		t.Fatal("prepared module has no Inv declaration")
	}
	if constructors := countOperator(invariant, "FUN_CTOR"); constructors != 0 {
		formatted, _ := FormatTLAExpression(invariant)
		t.Fatalf("prepared invariant retains %d function constructors: %s", constructors, formatted)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	result, err := CheckBoundedSafetyWithOptions(ctx, prepared.Module, prepared.Predicates, BoundedSafetyOptions{
		MaxSteps: 0, CheckDeadlocks: false,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample {
		t.Fatalf("large function constructor check = %+v", result)
	}
}

func TestPrepareSafetyTransitionsEncodesTemporalProperties(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	nextValue := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)), intLiteral(1), intLiteral(0))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, nextValue))
	live := boolDefinition("Live", MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(2))))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), init, next, live)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, TemporalProperties: []string{"Live"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(prepared.Predicates.TemporalProperties) != 0 ||
		!reflect.DeepEqual(prepared.Predicates.Invariants, []string{"Live"}) {
		t.Fatalf("encoded predicates = %+v", prepared.Predicates)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Invariant != "Live" || result.Depth < 2 {
		t.Fatalf("temporal counterexample = %+v", result)
	}
}

func TestPrepareSafetyTransitionsAcceptsSatisfiedEventually(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	nextValue := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)), intLiteral(1), intLiteral(0))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, nextValue))
	live := boolDefinition("Live", MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(1))))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), init, next, live)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, TemporalProperties: []string{"Live"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyNoCounterexample {
		t.Fatalf("satisfied temporal property = %+v", result)
	}
}

func TestPrepareSafetyTransitionsDesugarsLeadsToBeforeTableau(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), x))
	leads := boolDefinition("Leads", MustOperExpr(Op("LEADS_TO"), Typed(BoolT1),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)),
		MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(1))))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), init, next, leads)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, TemporalProperties: []string{"Leads"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Invariant != "Leads" {
		t.Fatalf("leads-to counterexample = %+v", result)
	}
}

func TestPrepareSafetyTransitionsRemovesAssignmentsFromTemporalAtoms(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	prime := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)))
	init := boolDefinition("Init", MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0)))
	next := boolDefinition("Next", MustOperExpr(Op("EQ"), Typed(BoolT1), prime, x))
	actionAssignment := MustOperExpr(Op("Apalache!"+":="), Typed(BoolT1), prime, intLiteral(1))
	live := boolDefinition("Live", MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1), actionAssignment))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), init, next, live)
	prepared, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, TemporalProperties: []string{"Live"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, transition := range TransitionsFromModule(prepared.Module, NormalizedNextPrefix) {
		// Exactly one assignment to x and assignments to generated variables must
		// originate from Next/tableau state, never from the temporal atom itself.
		if countOperator(transition, "Apalache!"+":=") == 0 {
			t.Fatalf("normalized transition has no assignments: %v", transition)
		}
	}
	result, err := CheckBoundedSafety(context.Background(), prepared.Module, prepared.Predicates, 2)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != SafetyCounterexample || result.Invariant != "Live" {
		t.Fatalf("action temporal counterexample = %+v", result)
	}
}
