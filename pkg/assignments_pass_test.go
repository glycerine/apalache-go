package apalache

import (
	"reflect"
	"testing"
)

func TestNormalizedDeclarationPredicates(t *testing.T) {
	boolOperator := func(name string, parameters ...OperatorParam) *OperDecl {
		argumentTypes := make([]Type, len(parameters))
		for index := range argumentTypes {
			argumentTypes[index] = BoolT1
		}
		return NewOperDecl(name, parameters, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(argumentTypes, BoolT1)))
	}
	if !IsVerificationCondition(boolOperator("VCInv$0")) || !IsVerificationCondition(boolOperator(NormalizedVCView)) {
		t.Fatal("verification condition was not recognized")
	}
	if IsVerificationCondition(boolOperator("VCTemporal$0")) || IsVerificationCondition(boolOperator("Other")) {
		t.Fatal("non-checker verification condition was recognized")
	}
	if IsVerificationCondition(NewConstDecl("VCInv$0", Typed(BoolT1))) {
		t.Fatal("constant was recognized as a verification condition")
	}
	if !IsTemporalVerificationCondition(boolOperator("VCTemporal$0")) || IsTemporalVerificationCondition(boolOperator("VCTemporal$0", ValueParam("p"))) {
		t.Fatal("temporal verification condition arity was ignored")
	}
	if !IsNormalizedConstantInit(boolOperator(NormalizedConstInit)) || !IsNormalizedInit(boolOperator("Init$0000")) || !IsNormalizedNext(boolOperator("Next$0000")) {
		t.Fatal("normalized transition declaration was not recognized")
	}
	if IsNormalizedConstantInit(boolOperator(NormalizedConstInit, ValueParam("p"))) ||
		IsNormalizedInit(boolOperator("Init$0000", ValueParam("p"))) ||
		IsNormalizedNext(boolOperator("Next$0000", ValueParam("p"))) {
		t.Fatal("normalized transition declaration arity was ignored")
	}
	if got, want := NormalizedStandardOptionNames(), []string{ConfigInit, ConfigCInit, ConfigNext, ConfigInvariant, ConfigTemporal}; !reflect.DeepEqual(got, want) {
		t.Fatalf("standard option names = %v, want %v", got, want)
	}
	got := NormalizedStandardOptionNames()
	got[0] = "changed"
	if NormalizedStandardOptionNames()[0] != ConfigInit {
		t.Fatal("standard option names leaked mutable shared state")
	}
}

func TestFindTransitionsBuildsProductionModule(t *testing.T) {
	x := NewVarDecl("x", Typed(IntT1))
	initOne, initTwo := assignmentCandidate("x", intLiteral(0)), assignmentCandidate("x", intLiteral(1))
	initBody := MustOperExpr(Op("OR"), Typed(BoolT1), initOne, initTwo)
	nextBody := assignmentCandidate("x", MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1)))
	init := NewOperDecl("InitPrimed", nil, initBody, Typed(mustOperator(nil, BoolT1)))
	next := NewOperDecl("Next", nil, nextBody, Typed(mustOperator(nil, BoolT1)))
	invariant := NewOperDecl("Inv", nil, MustOperExpr(Op("GE"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(0)), Typed(mustOperator(nil, BoolT1)))
	persistent := NewOperDecl("Persist", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(nil, BoolT1)))
	dropped := NewOperDecl("Helper", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(nil, BoolT1)))
	assumption := NewUnnamedAssumeDecl(NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(BoolT1))
	module := NewModule("M", []Decl{x, init, next, invariant, persistent, dropped, assumption})
	module.AddProperty(PropertyPrimed)
	module.AddProperty(PropertyPreprocessed)

	got, err := FindTransitions(module, TransitionPassOptions{
		Init: "Init", Next: "Next", Invariants: []string{"Inv"}, Persistent: map[string]struct{}{"Persist": {}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyTransitionsFound) || !got.HasProperty(PropertyPreprocessed) {
		t.Fatal("transition properties were not preserved and added")
	}
	if len(TransitionsFromModule(got, NormalizedInitPrefix)) != 2 || len(TransitionsFromModule(got, NormalizedNextPrefix)) != 1 {
		t.Fatalf("transition declarations = %v", declarationNames(got))
	}
	for _, expected := range []string{"Inv", "Persist"} {
		if _, found := OperatorBody(got, expected); !found {
			t.Errorf("retained declaration %s is missing", expected)
		}
	}
	if _, found := OperatorBody(got, "Helper"); found {
		t.Fatal("preprocessing-only helper was retained")
	}
	if len(got.AssumeDeclarations()) != 1 || module.HasProperty(PropertyTransitionsFound) {
		t.Fatal("assumptions were lost or input module was mutated")
	}
}

func TestFindTransitionsPrefersVCsAndBuildsConstantInit(t *testing.T) {
	constant := NewConstDecl("N", Typed(IntT1))
	cinit := NewOperDecl("CInitPrimed", nil, assignmentCandidate("N", intLiteral(3)), Typed(mustOperator(nil, BoolT1)))
	stateless := NewValueExpr(BoolValue(true), Typed(BoolT1))
	init := NewOperDecl("InitPrimed", nil, stateless, Typed(mustOperator(nil, BoolT1)))
	next := NewOperDecl("Next", nil, stateless, Typed(mustOperator(nil, BoolT1)))
	invariant := NewOperDecl("Inv", nil, stateless, Typed(mustOperator(nil, BoolT1)))
	vc := NewOperDecl("VCInv$0", nil, stateless, Typed(mustOperator(nil, BoolT1)))
	module := NewModule("M", []Decl{constant, cinit, init, next, invariant, vc})
	module.AddProperty(PropertyPrimed)
	module.AddProperty(PropertyPreprocessed)
	got, err := FindTransitions(module, TransitionPassOptions{Init: "Init", Next: "Next", ConstantInit: "CInit", Invariants: []string{"Inv"}})
	if err != nil {
		t.Fatal(err)
	}
	if body, found := OperatorBody(got, NormalizedConstInit); !found || countOperator(body, "Apalache!:=") != 1 {
		t.Fatalf("constant initializer = %v, %v", body, found)
	}
	if _, found := OperatorBody(got, "VCInv$0"); !found {
		t.Fatal("verification condition was not retained")
	}
	if _, found := OperatorBody(got, "Inv"); found {
		t.Fatal("raw invariant was retained alongside verification conditions")
	}
}

func TestFindTransitionsChecksDependenciesAndNames(t *testing.T) {
	module := NewModule("M", nil)
	if _, err := FindTransitions(module, TransitionPassOptions{}); err == nil {
		t.Fatal("missing pass dependencies were accepted")
	}
	module.AddProperty(PropertyPrimed)
	module.AddProperty(PropertyPreprocessed)
	if _, err := FindTransitions(module, TransitionPassOptions{Init: "Init", Next: "Next"}); err == nil {
		t.Fatal("missing InitPrimed/Next declarations were accepted")
	}
}
