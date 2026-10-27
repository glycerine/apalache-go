package apalache

import "testing"

func TestInlineConfiguredModuleFiltersOperatorsAndConstants(t *testing.T) {
	intOperator := mustOperator(nil, IntT1)
	boolOperator := mustOperator(nil, BoolT1)
	constant := NewConstDecl("N", Typed(IntT1))
	variable := NewVarDecl("x", Typed(IntT1))
	cinitBody := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("N", Typed(IntT1)), intLiteral(3))
	cinit := NewOperDecl("CInit", nil, cinitBody, Typed(boolOperator))
	cinitPrimedBody := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("N", Typed(IntT1))), intLiteral(3))
	cinitPrimed := NewOperDecl("CInitPrimed", nil, cinitPrimedBody, Typed(boolOperator))
	helper := NewOperDecl("Helper", nil, MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("N", Typed(IntT1)), intLiteral(1)), Typed(intOperator))
	helperCall := func() Expr {
		return MustOperExpr(Op("OPER_APP"), Typed(IntT1), NewNameExpr("Helper", Typed(intOperator)))
	}
	init := NewOperDecl("Init", nil, MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), helperCall()), Typed(boolOperator))
	initPrimed := NewOperDecl("InitPrimed", nil, MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), helperCall()), Typed(boolOperator))
	next := NewOperDecl("Next", nil, MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(IntT1), NewNameExpr("x", Typed(IntT1))), helperCall()), Typed(boolOperator))
	unrelated := NewOperDecl("Unused", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(boolOperator))
	module := NewModule("M", []Decl{constant, variable, cinit, cinitPrimed, helper, init, initPrimed, next, unrelated})
	module.AddProperty(PropertyPrimed)

	got, err := InlineConfiguredModule(module, InlinePassOptions{Init: "Init", Next: "Next", ConstantInit: "CInit"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyPrimed) || !got.HasProperty(PropertyInlined) || module.HasProperty(PropertyInlined) {
		t.Fatal("inline pass properties were not preserved/added immutably")
	}
	for _, removed := range []string{"Helper", "Unused"} {
		if _, found := OperatorBody(got, removed); found {
			t.Errorf("irrelevant operator %s was retained", removed)
		}
	}
	for _, retained := range []string{"CInit", "CInitPrimed", "Init", "InitPrimed", "Next"} {
		if _, found := OperatorBody(got, retained); !found {
			t.Errorf("relevant operator %s was removed", retained)
		}
	}
	for _, name := range []string{"Init", "InitPrimed", "Next"} {
		body, _ := OperatorBody(got, name)
		if countOperator(body, "OPER_APP") != 0 || countNamed(body, "N") != 0 || countInteger(body, 3) != 1 {
			t.Errorf("%s was not operator/constant inlined: %v", name, body)
		}
	}
	for _, name := range []string{"CInit", "CInitPrimed"} {
		body, _ := OperatorBody(got, name)
		if countNamed(body, "N") != 1 {
			t.Errorf("%s constant reference was replaced: %v", name, body)
		}
	}
}

func TestInlineConfiguredModuleRetainsExplicitRelevantOperators(t *testing.T) {
	boolOperator := mustOperator(nil, BoolT1)
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	module := NewModule("M", []Decl{
		NewOperDecl("CInit", nil, truth, Typed(boolOperator)),
		NewOperDecl("Init", nil, truth, Typed(boolOperator)),
		NewOperDecl("InitPrimed", nil, truth, Typed(boolOperator)),
		NewOperDecl("Next", nil, truth, Typed(boolOperator)),
		NewOperDecl("Inv", nil, truth, Typed(boolOperator)),
	})
	got, err := InlineConfiguredModule(module, InlinePassOptions{Init: "Init", Next: "Next", Relevant: []string{"Inv"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, found := OperatorBody(got, "Inv"); !found {
		t.Fatal("explicitly relevant invariant was removed")
	}
}

func TestInlineConfiguredModuleRejectsMissingConfiguration(t *testing.T) {
	module := NewModule("M", nil)
	if _, err := InlineConfiguredModule(module, InlinePassOptions{}); err == nil {
		t.Fatal("missing Init/Next names were accepted")
	}
	if _, err := InlineConfiguredModule(module, InlinePassOptions{Init: "Init", Next: "Next"}); err == nil {
		t.Fatal("missing CInit declaration was accepted")
	}
}

func countNamed(expression Expr, name string) int {
	count := 0
	visitExpression(expression, func(current Expr) {
		if named, ok := current.(*NameExpr); ok && named.Name() == name {
			count++
		}
	})
	return count
}

func countInteger(expression Expr, expected int64) int {
	count := 0
	visitExpression(expression, func(current Expr) {
		if value, ok := current.(*ValueExpr); ok {
			if integer, ok := value.Value().(IntValue); ok && integer.BigInt().Int64() == expected {
				count++
			}
		}
	})
	return count
}

func visitExpression(expression Expr, visit func(Expr)) {
	visit(expression)
	switch typed := expression.(type) {
	case *OperExpr:
		for _, argument := range typed.Arguments() {
			visitExpression(argument, visit)
		}
	case *LetInExpr:
		visitExpression(typed.Body(), visit)
		for _, declaration := range typed.Declarations() {
			visitExpression(declaration.Body(), visit)
		}
	}
}
