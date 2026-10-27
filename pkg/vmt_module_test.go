package apalache

import (
	"strings"
	"testing"
)

func vmtBoolDefinition(name string, body Expr) *OperDecl {
	return NewOperDecl(name, nil, body, Typed(mustOperator(nil, BoolT1)))
}

func TestFormatVMTModuleAssemblesDeterministicDocument(t *testing.T) {
	typ, _ := NewConstType("SSORT")
	setType := mustSet(typ)
	x := NewNameExpr("x", Typed(typ))
	primeX := MustOperExpr(Op("PRIME"), Typed(typ), x)
	a := NewValueExpr(StringValue("a_OF_SSORT"), Typed(typ))
	b := NewValueExpr(StringValue("b_OF_SSORT"), Typed(typ))
	module := NewModule("M", []Decl{
		NewConstDecl("S", Typed(setType)),
		NewVarDecl("x", Typed(typ)),
		vmtBoolDefinition(NormalizedConstInit, MustOperExpr(Op("EQ"), Typed(BoolT1), x, a)),
		vmtBoolDefinition("Init$0001", MustOperExpr(Op("Apalache!:="), Typed(BoolT1), primeX, a)),
		vmtBoolDefinition("Next$0000", MustOperExpr(Op("Apalache!:="), Typed(BoolT1), primeX, b)),
		vmtBoolDefinition("VCInv$0", MustOperExpr(Op("NE"), Typed(BoolT1), a, b)),
	})

	first, err := FormatVMTModule(module)
	if err != nil {
		t.Fatal(err)
	}
	second, err := FormatVMTModule(module)
	if err != nil || first != second {
		t.Fatalf("VMT output is not deterministic: %v", err)
	}
	for _, fragment := range []string{
		";Sorts\n(declare-sort SSORT 0)",
		"(declare-fun a_SSORT () SSORT)",
		"(declare-fun b_SSORT () SSORT)",
		"(define-fun .axiom () Bool (! (distinct a_SSORT b_SSORT) :axiom true))",
		"(declare-fun x () SSORT)\n(declare-fun x^ () SSORT)",
		"(define-fun .x () SSORT (! x :next x^))",
		"(define-fun Init$0001 () Bool (! (= x a_SSORT) :init true))",
		"(define-fun Next$0000 () Bool (! (= x^ b_SSORT) :action Next$0000))",
		"(define-fun VCInv$0 () Bool (! (not (= a_SSORT b_SSORT)) :invar-property 0))",
	} {
		if !strings.Contains(first, fragment) {
			t.Errorf("VMT output does not contain %q:\n%s", fragment, first)
		}
	}
	if strings.Contains(first, NormalizedConstInit) {
		t.Fatalf("constant initialization leaked into VMT output:\n%s", first)
	}
	if strings.Index(first, ";Initial states") > strings.Index(first, ";Transitions") ||
		strings.Index(first, ";Transitions") > strings.Index(first, ";Invariants") {
		t.Fatalf("VMT sections are out of order:\n%s", first)
	}
}

func TestFormatVMTModuleEmitsGeneratedFunctionDefinitions(t *testing.T) {
	typ, _ := NewConstType("SSORT")
	functionType := mustFun(typ, typ)
	f := NewNameExpr("f", Typed(functionType))
	primeF := MustOperExpr(Op("PRIME"), Typed(functionType), f)
	x := NewNameExpr("x", Typed(typ))
	a := NewValueExpr(StringValue("a_OF_SSORT"), Typed(typ))
	constructor := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), a,
		x, NewNameExpr("S", Typed(mustSet(typ))))
	next := MustOperExpr(Op("Apalache!:="), Typed(BoolT1), primeF, constructor)
	module := NewModule("Functions", []Decl{
		NewConstDecl("S", Typed(mustSet(typ))),
		NewVarDecl("f", Typed(functionType)),
		vmtBoolDefinition("Next$0000", next),
	})
	output, err := FormatVMTModule(module)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "(define-fun t_1 ((x SSORT)) SSORT a_SSORT)") {
		t.Fatalf("generated function definition is missing:\n%s", output)
	}
	if !strings.Contains(output, "(forall ((t_2 SSORT)) (= (f^ t_2) (t_1 t_2)))") {
		t.Fatalf("function assignment was not lowered extensionally:\n%s", output)
	}
}

func TestFormatVMTModuleRejectsUnsupportedVariableType(t *testing.T) {
	module := NewModule("Bad", []Decl{NewVarDecl("s", Typed(mustSet(IntT1)))})
	if _, err := FormatVMTModule(module); err == nil {
		t.Fatal("set-valued VMT variable was accepted")
	}
}
