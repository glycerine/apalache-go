package apalache

import (
	"errors"
	"reflect"
	"testing"
)

func traceeFixtureModule() (*Module, DerivedPredicates) {
	primeN := MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("N", Typed(IntT1)))
	cinit := MustOperExpr(Op("EQ"), Typed(BoolT1), primeN, intLiteral(1))
	expression := MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewNameExpr("x", Typed(IntT1)), NewNameExpr("N", Typed(IntT1)))
	module := NewModule("Tracee", []Decl{
		NewConstDecl("N", Typed(IntT1)),
		NewVarDecl("x", Typed(IntT1)),
		NewUnnamedAssumeDecl(boolExpr(true), Typed(BoolT1)),
		NewOperDecl("Init", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("Next", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("Inv", nil, boolExpr(true), Typed(BoolT1)),
		NewOperDecl("CInit", nil, cinit, Typed(BoolT1)),
		NewOperDecl("Value", nil, expression, Typed(IntT1)),
		NewOperDecl("Helper", nil, intLiteral(7), Typed(IntT1)),
		NewTheoremDecl("DroppedTheorem", boolExpr(true), Typed(BoolT1)),
	})
	module.AddProperty(PropertyConfigured)
	return module, DerivedPredicates{
		Init: "Init", Next: "Next", ConstantInit: "CInit", Invariants: []string{"Inv"},
		Persistent: []string{"Helper"},
	}
}

func TestTraceeEncodingPassRewritesConfiguredModule(t *testing.T) {
	module, predicates := traceeFixtureModule()
	writes := 0
	pass := NewTraceeEncodingPass(TraceePassOptions{
		Predicates: predicates,
		Trace: TraceeStateSeq{
			{"x": intLiteral(2), "N": intLiteral(10)},
			{"x": intLiteral(4), "N": intLiteral(10)},
		},
		Expressions: []string{"Value"},
		WriteOutput: func(name string, output *Module) error {
			writes++
			if name != "TraceePass" || output.Name() != "Tracee" {
				t.Fatalf("output callback = %q, %q", name, output.Name())
			}
			return nil
		},
	})
	result, err := pass.Execute(module)
	if err != nil {
		t.Fatal(err)
	}
	if writes != 1 || !result.HasProperty(PropertyConfigured) || module == result {
		t.Fatalf("tracee output writes=%d configured=%v identity=%v", writes,
			result.HasProperty(PropertyConfigured), module == result)
	}
	if got := declarationNameSlice(result.VarDeclarations()); !reflect.DeepEqual(got, []string{"Value"}) {
		t.Fatalf("tracee variables = %#v", got)
	}
	if got := declarationNameSlice(result.ConstDeclarations()); !reflect.DeepEqual(got, []string{"N"}) {
		t.Fatalf("tracee constants = %#v", got)
	}
	for _, removed := range []string{"Init", "Next", "Inv", "CInit", "DroppedTheorem"} {
		if _, found := declarationNamed(result, removed); found {
			t.Fatalf("declaration %s was retained", removed)
		}
	}
	if _, found := declarationNamed(result, "Helper"); !found {
		t.Fatal("unrelated helper was dropped")
	}
	init, found := OperatorBody(result, NormalizedInitPrefix)
	if !found {
		t.Fatal("generated Init transition is missing")
	}
	next := TransitionsFromModule(result, NormalizedNextPrefix)
	if len(next) != 1 {
		t.Fatalf("generated Next transitions = %#v", next)
	}
	for index, transition := range []Expr{init, next[0]} {
		text, formatErr := FormatTLAExpression(transition)
		if formatErr != nil {
			t.Fatal(formatErr)
		}
		want := "Value' := 2 + 10"
		if index == 1 {
			want = "Value' := 4 + 10"
		}
		if text != want {
			t.Fatalf("transition %d = %q, want %q", index, text, want)
		}
	}
	cinit, found := OperatorBody(result, NormalizedConstInit)
	if !found || cinit.(*OperExpr).Operator().Name() != "Apalache!:=" {
		t.Fatalf("generated CInit = %#v, %v", cinit, found)
	}
	wantPersistent := []string{"Helper", NormalizedInitPrefix, NormalizedNextPrefix + "0000"}
	if got := pass.Predicates().Persistent; !reflect.DeepEqual(got, wantPersistent) {
		t.Fatalf("persistent predicates = %#v", got)
	}
	if module.VarDeclarations()[0].Name() != "x" || len(module.Declarations()) != 10 {
		t.Fatal("tracee pass mutated its input module")
	}
}

func TestTraceeEncodingPassValidatesInputsAndOutput(t *testing.T) {
	module, predicates := traceeFixtureModule()
	base := TraceePassOptions{Predicates: predicates, Trace: TraceeStateSeq{{"x": intLiteral(1)}}, Expressions: []string{"Value"}}
	for _, test := range []struct {
		name    string
		options TraceePassOptions
	}{
		{name: "empty trace", options: func() TraceePassOptions { o := base; o.Trace = nil; return o }()},
		{name: "empty expressions", options: func() TraceePassOptions { o := base; o.Expressions = nil; return o }()},
		{name: "missing expression", options: func() TraceePassOptions { o := base; o.Expressions = []string{"Missing"}; return o }()},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewTraceeEncodingPass(test.options).Execute(module); err == nil {
				t.Fatal("invalid tracee input was accepted")
			}
		})
	}
	unconfigured, _ := traceeFixtureModule()
	unconfigured.properties = make(map[ModuleProperty]struct{})
	if _, err := NewTraceeEncodingPass(base).Execute(unconfigured); err == nil {
		t.Fatal("unconfigured module was accepted")
	}
	sentinel := errors.New("write failed")
	base.WriteOutput = func(string, *Module) error { return sentinel }
	if _, err := NewTraceeEncodingPass(base).Execute(module); !errors.Is(err, sentinel) {
		t.Fatalf("output error = %v", err)
	}
}

func TestTraceeEncodingPassMetadataAndDefensiveOptions(t *testing.T) {
	module, predicates := traceeFixtureModule()
	state := TraceeState{"x": intLiteral(1), "N": intLiteral(1)}
	trace := TraceeStateSeq{state}
	expressions := []string{"Value", "Value"}
	pass := NewTraceeEncodingPass(TraceePassOptions{Predicates: predicates, Trace: trace, Expressions: expressions})
	trace[0] = TraceeState{}
	state["x"] = intLiteral(99)
	expressions[0] = "Missing"
	if pass.Name() != "TraceePass" ||
		!reflect.DeepEqual(pass.Dependencies(), []ModuleProperty{PropertyConfigured}) || len(pass.Transformations()) != 0 {
		t.Fatalf("tracee pass metadata = %q, %#v, %#v", pass.Name(), pass.Dependencies(), pass.Transformations())
	}
	result, err := pass.Execute(module)
	if err != nil {
		t.Fatalf("defensively copied options failed: %v", err)
	}
	if len(result.VarDeclarations()) != 1 {
		t.Fatalf("duplicate expression produced variables %#v", result.VarDeclarations())
	}
	init, _ := OperatorBody(result, NormalizedInitPrefix)
	text, _ := FormatTLAExpression(init)
	if text != "Value' := 1 + 1" {
		t.Fatalf("mutated caller trace changed transition to %q", text)
	}
}
