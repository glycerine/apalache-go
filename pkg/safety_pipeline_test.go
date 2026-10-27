package apalache

import (
	"strings"
	"testing"
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

func TestPrepareSafetyTransitionsRejectsTemporalPropertiesExplicitly(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	module := configuredTestModule(boolDefinition("Init", truth), boolDefinition("Next", truth), boolDefinition("Live", truth))
	_, err := PrepareSafetyTransitions(module, SafetyPipelineOptions{Specification: SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, TemporalProperties: []string{"Live"},
	}})
	if err == nil || !strings.Contains(err.Error(), "temporal encoding") {
		t.Fatalf("temporal pipeline error = %v", err)
	}
}
