package apalache

import (
	"errors"
	"os"
	"reflect"
	"strings"
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
	t.Chdir(t.TempDir())
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
	document, readErr := os.ReadFile("dependencies.dot")
	if readErr != nil {
		t.Fatalf("Scala-compatible cycle witness was not written: %v", readErr)
	}
	for _, fragment := range []string{
		"digraph dependencies {",
		"n" + foo.ID().String() + "[label=\"Foo\"]",
		"n" + foo.ID().String() + " -> n" + bar.ID().String(),
		"n" + bar.ID().String() + " -> n" + baz.ID().String(),
		"n" + baz.ID().String() + " -> n" + foo.ID().String(),
	} {
		if !strings.Contains(string(document), fragment) {
			t.Errorf("dependencies.dot lacks %q:\n%s", fragment, document)
		}
	}
}

func TestSortDeclarationsHiddenCallCycleRegression(t *testing.T) {
	t.Chdir(t.TempDir())
	foo := NewOperDecl("Foo", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())
	bar := NewOperDecl("Bar", nil, untypedApp("Foo", untypedApp("Baz")), Untyped())
	baz := NewOperDecl("Baz", nil, untypedApp("Foo", untypedApp("Bar")), Untyped())

	got, err := SortDeclarations(NewModule("test", []Decl{foo, bar, baz}))
	if got != nil {
		t.Fatalf("cycle unexpectedly sorted to %v", declarationNames(got))
	}
	var cycleError *CyclicDependencyError
	if !errors.As(err, &cycleError) {
		t.Fatalf("got %T %v, want CyclicDependencyError", err, err)
	}
	if want := []string{"Bar", "Baz"}; !reflect.DeepEqual(cycleError.Declarations, want) {
		t.Fatalf("got witnesses %v, want %v", cycleError.Declarations, want)
	}
	if _, err := os.Stat("dependencies.dot"); err != nil {
		t.Fatalf("Scala-compatible cycle witness was not written: %v", err)
	}
}

func TestSortDeclarationsExactScalaCases(t *testing.T) {
	boolTrue := func() Expr { return NewValueExpr(BoolValue(true), Untyped()) }
	integer := func(value int64) Expr { return NewValueExpr(Int64Value(value), Untyped()) }
	tests := []struct {
		name  string
		build func() *Module
		want  []string
	}{
		{
			name: "Bar calls Foo out of order",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, boolTrue(), Untyped())
				bar := NewOperDecl("Bar", nil, untypedApp("Foo"), Untyped())
				return NewModule("test", []Decl{bar, foo})
			},
			want: []string{"Foo", "Bar"},
		},
		{
			name: "Foo calls Bar calls Baz in order",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, untypedApp("Bar"), Untyped())
				bar := NewOperDecl("Bar", nil, untypedApp("Baz"), Untyped())
				baz := NewOperDecl("Baz", nil, boolTrue(), Untyped())
				return NewModule("test", []Decl{baz, foo, bar})
			},
			want: []string{"Baz", "Bar", "Foo"},
		},
		{
			name: "Foo calls Bar in LET-IN calls Baz in order",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, untypedApp("Bar"), Untyped())
				local := NewOperDecl("local", nil, untypedApp("Baz"), Untyped())
				bar := NewOperDecl("Bar", nil, NewLetInExpr(boolTrue(), Untyped(), local), Untyped())
				baz := NewOperDecl("Baz", nil, boolTrue(), Untyped())
				return NewModule("test", []Decl{baz, bar, foo})
			},
			want: []string{"Baz", "Bar", "Foo"},
		},
		{
			name: "regression false cycle",
			build: func() *Module {
				foo := NewOperDecl("Foo", []OperatorParam{ValueParam("pid")}, NewNameExpr("pid", Untyped()), Untyped())
				pid := NewOperDecl("pid", nil, untypedApp("Foo", integer(2)), Untyped())
				return NewModule("test", []Decl{foo, pid})
			},
			want: []string{"Foo", "pid"},
		},
		{
			name: "Foo uses VARIABLE x out of order",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, NewNameExpr("x", Untyped()), Untyped())
				return NewModule("test", []Decl{foo, NewVarDecl("x", Untyped())})
			},
			want: []string{"x", "Foo"},
		},
		{
			name: "Foo uses CONSTANT N out of order",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, NewNameExpr("N", Untyped()), Untyped())
				return NewModule("test", []Decl{foo, NewConstDecl("N", Untyped())})
			},
			want: []string{"N", "Foo"},
		},
		{
			name: "preserve order of constants and variables",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, NewNameExpr("x", Untyped()), Untyped())
				return NewModule("test", []Decl{foo, NewConstDecl("N", Untyped()), NewVarDecl("x", Untyped())})
			},
			want: []string{"N", "x", "Foo"},
		},
		{
			name: "Assume uses Foo out of order",
			build: func() *Module {
				foo := NewOperDecl("Foo", nil, integer(1), Untyped())
				assume := NewUnnamedAssumeDecl(untypedApp("Foo"), Untyped())
				return NewModule("test", []Decl{assume, foo})
			},
			want: []string{"Foo", "<anonymous>"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := test.build()
			want := append([]string(nil), test.want...)
			if len(want) > 0 && want[len(want)-1] == "<anonymous>" {
				want[len(want)-1] = input.Declarations()[0].Name()
			}
			got, err := SortDeclarations(input)
			if err != nil {
				t.Fatal(err)
			}
			if names := declarationNames(got); !reflect.DeepEqual(names, want) {
				t.Fatalf("got %v, want %v", names, want)
			}
		})
	}
}
