package apalache

import "testing"

func TestPrimeInitializersAppendsFreshPrimedDefinitions(t *testing.T) {
	variable := NewVarDecl("x", Typed(IntT1))
	constant := NewConstDecl("N", Typed(IntT1))
	initBody := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), NewNameExpr("N", Typed(IntT1)))
	cinitBody := MustOperExpr(Op("GT"), Typed(BoolT1), NewNameExpr("N", Typed(IntT1)), intLiteral(0))
	init := NewOperDecl("Init", nil, initBody, Typed(mustOperator(nil, BoolT1)))
	cinit := NewOperDecl("CInit", nil, cinitBody, Typed(mustOperator(nil, BoolT1)))
	module := NewModule("M", []Decl{constant, variable, cinit, init})
	module.AddProperty(PropertyConfigured)

	got, err := PrimeInitializers(module, PrimingOptions{Init: "Init", ConstantInit: "CInit"})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyConfigured) || !got.HasProperty(PropertyPrimed) || module.HasProperty(PropertyPrimed) {
		t.Fatal("priming properties were not preserved/added immutably")
	}
	primedInit, found := OperatorBody(got, "InitPrimed")
	if !found || countOperator(primedInit, "PRIME") != 1 {
		t.Fatalf("InitPrimed = %v, %v", primedInit, found)
	}
	initEquality := primedInit.(*OperExpr)
	if initEquality.Arguments()[0].(*OperExpr).Arguments()[0].(*NameExpr).Name() != "x" {
		t.Fatalf("state variable was not primed: %v", primedInit)
	}
	if _, primedConstant := initEquality.Arguments()[1].(*OperExpr); primedConstant {
		t.Fatalf("constant was primed in Init: %v", primedInit)
	}
	primedCInit, found := OperatorBody(got, "CInitPrimed")
	if !found || countOperator(primedCInit, "PRIME") != 1 || primedCInit.(*OperExpr).Arguments()[0].(*OperExpr).Arguments()[0].(*NameExpr).Name() != "N" {
		t.Fatalf("CInitPrimed = %v, %v", primedCInit, found)
	}
	if primedInit.ID() == initBody.ID() || primedCInit.ID() == cinitBody.ID() {
		t.Fatal("primed initializer reused the original body identity")
	}
	if len(got.OperDeclarations()) != 4 {
		t.Fatalf("original initializers were not retained: %v", declarationNames(got))
	}
}

func TestPrimeInitializersWithoutConstantInit(t *testing.T) {
	init := NewOperDecl("Init", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(nil, BoolT1)))
	module := NewModule("M", []Decl{init})
	module.AddProperty(PropertyConfigured)
	got, err := PrimeInitializers(module, PrimingOptions{Init: "Init"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got.OperDeclarations()) != 2 {
		t.Fatalf("primed declarations = %v", declarationNames(got))
	}
	if _, found := OperatorBody(got, "InitPrimed"); !found {
		t.Fatal("InitPrimed is missing")
	}
}

func TestPrimeInitializersAvoidsDoublePrime(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	body := MustOperExpr(Op("EQ"), Typed(BoolT1), MustOperExpr(Op("PRIME"), Typed(IntT1), x), x)
	init := NewOperDecl("Init", nil, body, Typed(mustOperator(nil, BoolT1)))
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), init})
	module.AddProperty(PropertyConfigured)
	got, err := PrimeInitializers(module, PrimingOptions{Init: "Init"})
	if err != nil {
		t.Fatal(err)
	}
	primed, _ := OperatorBody(got, "InitPrimed")
	if countOperator(primed, "PRIME") != 2 {
		t.Fatalf("expected one prime on each occurrence: %v", primed)
	}
	for _, argument := range primed.(*OperExpr).Arguments() {
		prime := argument.(*OperExpr)
		if _, nested := prime.Arguments()[0].(*OperExpr); nested {
			t.Fatalf("double prime was introduced: %v", argument)
		}
	}
}

func TestPrimeInitializersChecksConfigurationAndNames(t *testing.T) {
	module := NewModule("M", nil)
	if _, err := PrimeInitializers(module, PrimingOptions{Init: "Init"}); err == nil {
		t.Fatal("unconfigured module was accepted")
	}
	module.AddProperty(PropertyConfigured)
	if _, err := PrimeInitializers(module, PrimingOptions{Init: "Init"}); err == nil {
		t.Fatal("missing Init declaration was accepted")
	}
}
