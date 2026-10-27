package apalache

import (
	"errors"
	"reflect"
	"testing"
)

func untypedApp(name string, arguments ...Expr) Expr {
	all := append([]Expr{NewNameExpr(name, Untyped())}, arguments...)
	return MustOperExpr(Op("OPER_APP"), Untyped(), all...)
}

func declarationNames(module *Module) []string {
	declarations := module.Declarations()
	result := make([]string, len(declarations))
	for index, declaration := range declarations {
		result[index] = declaration.Name()
	}
	return result
}

func TestSortDeclarationsOrdersCallsAndPreservesLayers(t *testing.T) {
	foo := NewOperDecl("Foo", nil, untypedApp("Bar"), Untyped())
	bar := NewOperDecl("Bar", nil, untypedApp("Baz"), Untyped())
	baz := NewOperDecl("Baz", nil, NewValueExpr(BoolValue(true), Untyped()), Untyped())
	unrelated := NewConstDecl("N", Untyped())
	input := NewModule("test", []Decl{foo, unrelated, bar, baz})
	input.AddProperty(PropertyTypeChecked)

	got, err := SortDeclarations(input)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"N", "Baz", "Bar", "Foo"}; !reflect.DeepEqual(declarationNames(got), want) {
		t.Fatalf("got %v, want %v", declarationNames(got), want)
	}
	if !got.HasProperty(PropertyTypeChecked) {
		t.Fatal("module properties were not preserved")
	}
	if reflect.DeepEqual(declarationNames(input), declarationNames(got)) {
		t.Fatal("sort mutated its input")
	}
}

func TestSortDeclarationsFindsVariablesConstantsAndAssumptions(t *testing.T) {
	foo := NewOperDecl("Foo", nil, MustOperExpr(Op("EQ"), Untyped(),
		NewNameExpr("x", Untyped()), NewNameExpr("N", Untyped())), Untyped())
	assume := NewUnnamedAssumeDecl(untypedApp("Foo"), Untyped())
	n := NewConstDecl("N", Untyped())
	x := NewVarDecl("x", Untyped())
	got, err := SortDeclarations(NewModule("test", []Decl{assume, foo, n, x}))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"N", "x", "Foo", assume.Name()}; !reflect.DeepEqual(declarationNames(got), want) {
		t.Fatalf("got %v, want %v", declarationNames(got), want)
	}
}

func TestSortDeclarationsRespectsParameterShadowingAndLetIn(t *testing.T) {
	foo := NewOperDecl("Foo", []OperatorParam{ValueParam("pid")}, NewNameExpr("pid", Untyped()), Untyped())
	pid := NewOperDecl("pid", nil, untypedApp("Foo", NewValueExpr(Int64Value(2), Untyped())), Untyped())
	baz := NewOperDecl("Baz", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	local := NewOperDecl("local", nil, untypedApp("Baz"), Untyped())
	bar := NewOperDecl("Bar", nil, NewLetInExpr(NewValueExpr(BoolValue(true), Untyped()), Untyped(), local), Untyped())

	got, err := SortDeclarations(NewModule("test", []Decl{bar, foo, pid, baz}))
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Foo", "Baz", "Bar", "pid"}; !reflect.DeepEqual(declarationNames(got), want) {
		t.Fatalf("got %v, want %v", declarationNames(got), want)
	}
}

func TestSortDeclarationsReportsCycle(t *testing.T) {
	foo := NewOperDecl("Foo", nil, untypedApp("Bar"), Untyped())
	bar := NewOperDecl("Bar", nil, untypedApp("Baz"), Untyped())
	baz := NewOperDecl("Baz", nil, untypedApp("Foo"), Untyped())
	got, err := SortDeclarations(NewModule("test", []Decl{foo, bar, baz}))
	if got != nil {
		t.Fatalf("cycle unexpectedly sorted to %v", declarationNames(got))
	}
	var cycleError *CyclicDependencyError
	if !errors.As(err, &cycleError) {
		t.Fatalf("got %T %v, want CyclicDependencyError", err, err)
	}
	if want := []string{"Foo", "Bar", "Baz"}; !reflect.DeepEqual(cycleError.Declarations, want) {
		t.Fatalf("got witnesses %v, want %v", cycleError.Declarations, want)
	}
}
