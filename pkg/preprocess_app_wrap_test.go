package apalache

import "testing"

func TestOperAppToLetInDefSelectionAndNaming(t *testing.T) {
	opType := mustOperator([]Type{IntT1, IntT1}, IntT1)
	application := MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("A$7", Typed(opType)), NewNameExpr("x", Typed(IntT1)), NewNameExpr("y", Typed(IntT1)))
	wrapper := NewOperAppToLetInDef(nil, nil)
	unchanged, err := wrapper.Wrap(application, nil)
	if err != nil || unchanged != application {
		t.Fatalf("unselected application = %v, %v", unchanged, err)
	}
	wrapped, err := wrapper.Wrap(application, map[string]struct{}{"A$7": {}})
	if err != nil {
		t.Fatal(err)
	}
	let, ok := wrapped.(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 {
		t.Fatalf("wrapped = %T %v", wrapped, wrapped)
	}
	definition := let.Declarations()[0]
	if definition.Name() != "CALL_A_t_1" || !EqualTypedExpr(definition.Body(), application) {
		t.Fatalf("definition = %s / %v", definition.Name(), definition.Body())
	}
	body := let.Body().(*OperExpr)
	if body.Arguments()[0].(*NameExpr).Name() != definition.Name() {
		t.Fatalf("LET body = %v", body)
	}
}

func TestOperAppToLetInDefLeavesNonApplicationsAndMalformedCalleesAlone(t *testing.T) {
	wrapper := NewOperAppToLetInDef(nil, nil)
	x := NewNameExpr("x", Typed(IntT1))
	nonApplications := []Expr{
		MustOperExpr(Op("PLUS"), Typed(IntT1), intLiteral(1), intLiteral(2)),
		MustOperExpr(Op("TUPLE"), Typed(mustTuple(IntT1, IntT1)), x, intLiteral(2)),
		MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x,
			MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1)), temporalBool(true)),
	}
	for _, expression := range nonApplications {
		got, err := wrapper.Wrap(expression, map[string]struct{}{})
		if err != nil || got != expression {
			t.Fatalf("non-application changed: %#v, %v", got, err)
		}
	}

	malformed := MustOperExpr(Op("OPER_APP"), Typed(IntT1), intLiteral(1), intLiteral(2))
	got, err := wrapper.Wrap(malformed, map[string]struct{}{"A": {}})
	if err != nil || got != malformed {
		t.Fatalf("non-name callee = %#v, %v", got, err)
	}
}

func TestOperAppToLetInDefMixedAndNestedArguments(t *testing.T) {
	opType := mustOperator([]Type{IntT1}, IntT1)
	app := func(name string, argument Expr) Expr {
		return MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr(name, Typed(opType)), argument)
	}
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	nested := app("A", app("A", one))
	unselected := app("B", one)
	root := MustOperExpr(Op("PLUS"), Typed(IntT1), nested, unselected)
	got, err := NewOperAppToLetInDef(nil, nil).Wrap(root, map[string]struct{}{"A": {}})
	if err != nil {
		t.Fatal(err)
	}
	plus := got.(*OperExpr)
	outer := plus.Arguments()[0].(*LetInExpr)
	if _, ok := outer.Declarations()[0].Body().(*OperExpr); !ok {
		t.Fatalf("outer definition = %v", outer.Declarations()[0].Body())
	}
	if !EqualTypedExpr(plus.Arguments()[1], unselected) {
		t.Fatalf("unselected call changed: %v", plus.Arguments()[1])
	}
	if outer.Declarations()[0].Name() != "CALL_A_t_2" {
		t.Fatalf("nested naming = %s", outer.Declarations()[0].Name())
	}
}

func TestOperAppToLetInDefAddsLocalNamesInDependencyOrder(t *testing.T) {
	opType := mustOperator([]Type{IntT1}, IntT1)
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	call := func(name string) Expr {
		return MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr(name, Typed(opType)), one)
	}
	first := NewOperDecl("First", []OperatorParam{ValueParam("x")}, one, Typed(opType))
	second := NewOperDecl("Second", []OperatorParam{ValueParam("x")}, call("First"), Typed(opType))
	root := NewLetInExpr(call("Second"), Typed(IntT1), first, second)
	got, err := NewOperAppToLetInDef(nil, nil).Wrap(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	let := got.(*LetInExpr)
	if _, ok := let.Declarations()[0].Body().(*LetInExpr); ok {
		t.Fatal("first definition wrapped itself")
	}
	if _, ok := let.Declarations()[1].Body().(*LetInExpr); !ok {
		t.Fatalf("dependency call not wrapped: %v", let.Declarations()[1].Body())
	}
	if _, ok := let.Body().(*LetInExpr); !ok {
		t.Fatalf("body local call not wrapped: %v", let.Body())
	}
}

func TestOperAppToLetInDefModuleMatchesScalaIdentityAndProperties(t *testing.T) {
	opType := mustOperator([]Type{IntT1}, IntT1)
	application := MustOperExpr(Op("OPER_APP"), Typed(IntT1),
		NewNameExpr("A", Typed(opType)), intLiteral(1))
	operator := NewOperDecl("Use", nil, application, Typed(mustOperator(nil, IntT1)))
	variable := NewVarDecl("x", Typed(IntT1))
	module := NewModule("M", []Decl{variable, operator})
	module.AddProperty(PropertyTypeChecked)
	got, err := NewOperAppToLetInDef(nil, nil).Module(module, map[string]struct{}{"A": {}})
	if err != nil {
		t.Fatal(err)
	}
	if got == module || got.VarDeclarations()[0] != variable || got.OperDeclarations()[0] == operator ||
		got.HasProperty(PropertyTypeChecked) {
		t.Fatalf("module identity/properties = %#v", got)
	}
	if _, wrapped := got.OperDeclarations()[0].Body().(*LetInExpr); !wrapped {
		t.Fatalf("module operator application was not wrapped: %#v", got.OperDeclarations()[0].Body())
	}
}
