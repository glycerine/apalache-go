package apalache

import (
	"math/big"
	"strings"
	"testing"
)

func configuredTestModule(declarations ...Decl) *Module {
	module := NewModule("M", declarations)
	module.AddProperty(PropertyTypeChecked)
	return module
}

func boolDefinition(name string, body Expr) *OperDecl {
	return NewOperDecl(name, nil, body, Typed(mustOperator(nil, BoolT1)))
}

func TestConfigureModuleWithoutTLCConfig(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	module := configuredTestModule(
		boolDefinition("Init", truth), boolDefinition("Next", truth),
		boolDefinition("Inv", truth), boolDefinition("Live", truth), boolDefinition("View", truth),
	)
	result, err := ConfigureModule(module, SpecificationConfiguration{
		Behavior: InitNextSpec{Init: "Init", Next: "Next"}, Invariants: []string{"Inv"},
		TemporalProperties: []string{"Live"}, View: "View", Persistent: []string{"Keep"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Module.HasProperty(PropertyConfigured) || !result.Module.HasProperty(PropertyTypeChecked) || module.HasProperty(PropertyConfigured) {
		t.Fatal("configuration properties were not preserved/added immutably")
	}
	if result.Predicates.Init != "Init" || result.Predicates.Next != "Next" || result.Predicates.View != "View" || len(result.Predicates.OperatorNames()) != 6 {
		t.Fatalf("derived predicates = %#v", result.Predicates)
	}
}

func TestConfigureModuleExtendsConstantInitFromTLCValues(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	constant := NewConstDecl("N", Typed(IntT1))
	module := configuredTestModule(constant, boolDefinition("Init", truth), boolDefinition("Next", truth))
	config := TLCConfig{
		ConstAssignments: map[string]ConfigConstExpr{"N": ConfigIntValue{Value: big.NewInt(3)}},
		Behavior:         InitNextSpec{Init: "Init", Next: "Next"},
	}
	result, err := ConfigureModule(module, SpecificationConfiguration{TLCConfig: &config})
	if err != nil {
		t.Fatal(err)
	}
	if result.Predicates.ConstantInit != "CInit" {
		t.Fatalf("constant initializer name = %q", result.Predicates.ConstantInit)
	}
	cinit, found := OperatorBody(result.Module, "CInit")
	if !found || cinit.(*OperExpr).Operator().Name() != "AND" || len(cinit.(*OperExpr).Arguments()) != 1 {
		t.Fatalf("generated CInit = %v, %v", cinit, found)
	}
	equality := cinit.(*OperExpr).Arguments()[0].(*OperExpr)
	if equality.Arguments()[0].(*NameExpr).Name() != "N" || equality.Arguments()[1].(*ValueExpr).Value().(IntValue).BigInt().Int64() != 3 {
		t.Fatalf("CInit override equality = %v", equality)
	}
	if len(result.Module.ConstDeclarations()) != 1 {
		t.Fatal("constant assignment incorrectly replaced the constant declaration")
	}
}

func TestConfigureModuleAppliesDefinitionReplacement(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	falsehood := NewValueExpr(BoolValue(false), Typed(BoolT1))
	module := configuredTestModule(
		boolDefinition("A", falsehood), boolDefinition("B", truth),
		boolDefinition("Init", truth), boolDefinition("Next", truth),
	)
	config := TLCConfig{
		ConstReplacements: map[string]string{"A": "B"},
		Behavior:          InitNextSpec{Init: "Init", Next: "Next"},
	}
	result, err := ConfigureModule(module, SpecificationConfiguration{TLCConfig: &config})
	if err != nil {
		t.Fatal(err)
	}
	body, found := OperatorBody(result.Module, "A")
	if !found || body.(*OperExpr).Operator().Name() != "OPER_APP" || body.(*OperExpr).Arguments()[0].(*NameExpr).Name() != "B" {
		t.Fatalf("definition replacement = %v, %v", body, found)
	}
	if _, found := OperatorBody(result.Module, "OVERRIDE_A"); found {
		t.Fatal("override declaration was retained")
	}
}

func TestConfigureModuleExtractsCanonicalSpecification(t *testing.T) {
	operatorType := mustOperator(nil, BoolT1)
	call := func(name string) Expr {
		return MustOperExpr(Op("OPER_APP"), Typed(BoolT1), NewNameExpr(name, Typed(operatorType)))
	}
	vars := MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1)), NewNameExpr("x", Typed(IntT1)))
	behavior := MustOperExpr(Op("AND"), Typed(BoolT1),
		MustOperExpr(Op("AND"), Typed(BoolT1), call("Init"),
			MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), MustOperExpr(Op("STUTTER"), Typed(BoolT1), call("Next"), vars))),
		MustOperExpr(Op("WEAK_FAIRNESS"), Typed(BoolT1), vars, call("Next")))
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	module := configuredTestModule(NewVarDecl("x", Typed(IntT1)), boolDefinition("Init", truth), boolDefinition("Next", truth), boolDefinition("Spec", behavior))
	config := TLCConfig{Behavior: TemporalSpec{Name: "Spec"}}
	result, err := ConfigureModule(module, SpecificationConfiguration{TLCConfig: &config, ContextName: "M.cfg"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Predicates.Init != "Init" || result.Predicates.Next != "Next" {
		t.Fatalf("SPECIFICATION predicates = %#v", result.Predicates)
	}
}

func TestConfigureModuleRejectsTypeMismatchMissingAndRecursion(t *testing.T) {
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	module := configuredTestModule(NewConstDecl("N", Typed(IntT1)), boolDefinition("Init", truth), boolDefinition("Next", truth))
	config := TLCConfig{ConstAssignments: map[string]ConfigConstExpr{"N": ConfigStringValue{Value: "bad"}}, Behavior: InitNextSpec{Init: "Init", Next: "Next"}}
	if _, err := ConfigureModule(module, SpecificationConfiguration{TLCConfig: &config}); err == nil || !strings.Contains(err.Error(), "has type") {
		t.Fatalf("constant type mismatch error = %v", err)
	}
	if _, err := ConfigureModule(module, SpecificationConfiguration{Behavior: InitNextSpec{Init: "Missing", Next: "Next"}}); err == nil || !strings.Contains(err.Error(), "Missing") {
		t.Fatalf("missing predicate error = %v", err)
	}
	recursive := boolDefinition("R", truth)
	recursive.SetRecursive(true)
	recursiveModule := configuredTestModule(recursive, boolDefinition("Init", truth), boolDefinition("Next", truth))
	if _, err := ConfigureModule(recursiveModule, SpecificationConfiguration{Behavior: InitNextSpec{Init: "Init", Next: "Next"}}); err == nil {
		t.Fatal("recursive module was accepted")
	}
}
