package apalache

import (
	"math/big"
	"testing"
)

func TestVMTSortFromTypeMatchesScalaFragment(t *testing.T) {
	a, err := NewConstType("A")
	if err != nil {
		t.Fatal(err)
	}
	tuple := mustTuple(IntT1, BoolT1)
	function := mustFun(tuple, a)
	sort, err := VMTSortFromType(function)
	if err != nil {
		t.Fatal(err)
	}
	wantA, _ := UninterpretedVMTSort("A")
	want := FunctionVMTSort(wantA, IntVMTSort(), BoolVMTSort())
	if !sort.Equal(want) {
		t.Fatalf("VMT function sort = %#v, want %#v", sort, want)
	}
	for _, unsupported := range []Type{mustSet(IntT1), mustSeq(IntT1), mustTuple(IntT1)} {
		if _, err := VMTSortFromType(unsupported); err == nil {
			t.Errorf("unsupported VMT type %s was accepted", unsupported)
		}
	}
}

func TestVMTTermConstructorsEnforceSorts(t *testing.T) {
	a, _ := UninterpretedVMTSort("A")
	b, _ := UninterpretedVMTSort("B")
	one, _ := VMTInt(big.NewInt(1))
	two, _ := VMTInt(big.NewInt(2))
	x, _ := VMTVar("x", IntVMTSort())
	av, _ := VMTVar("a", a)
	al, _ := VMTLiteral("one", a)
	bl, _ := VMTLiteral("one", b)
	if _, err := VMTEqualTerm(one, two); err != nil {
		t.Fatal(err)
	}
	if _, err := VMTEqualTerm(av, al); err != nil {
		t.Fatal(err)
	}
	if _, err := VMTEqualTerm(one, al); err == nil {
		t.Fatal("cross-sort equality was accepted")
	}
	if _, err := VMTIfThenElseTerm(VMTBool(true), one, x); err != nil {
		t.Fatal(err)
	}
	if _, err := VMTIfThenElseTerm(VMTBool(true), al, bl); err == nil {
		t.Fatal("cross-sort conditional was accepted")
	}
	function, _ := VMTVar("f", FunctionVMTSort(a, IntVMTSort(), BoolVMTSort()))
	if _, err := VMTApplyTerm(function, one, VMTBool(false)); err != nil {
		t.Fatal(err)
	}
	if _, err := VMTApplyTerm(function, VMTBool(false), one); err == nil {
		t.Fatal("wrong function argument sorts were accepted")
	}
}

func TestFormatVMTTermsAndAnnotations(t *testing.T) {
	x, _ := VMTVar("x", IntVMTSort())
	y, _ := VMTVar("y", IntVMTSort())
	equal, _ := VMTEqualTerm(x, y)
	negated, _ := VMTNotTerm(equal)
	quantified, _ := VMTForallTerm([]VMTBinding{{Name: "x", Sort: IntVMTSort()}}, negated)
	if got, err := FormatVMTTerm(quantified); err != nil || got != "(forall ((x Int)) (not (= x y)))" {
		t.Fatalf("quantified VMT = %q, %v", got, err)
	}
	emptyAnd, _ := VMTAndTerm()
	singleOr, _ := VMTOrTerm(VMTBool(true))
	if got, _ := FormatVMTTerm(emptyAnd); got != "true" {
		t.Fatalf("empty and = %q", got)
	}
	if got, _ := FormatVMTTerm(singleOr); got != "true" {
		t.Fatalf("unary or = %q", got)
	}

	init, err := FormatVMTAnnotated(VMTAnnotatedExpression{Kind: VMTInit, Name: "Init$0", Term: equal})
	if err != nil || init != "(define-fun Init$0 () Bool (! (= x y) :init true))" {
		t.Fatalf("init annotation = %q, %v", init, err)
	}
	current, _ := VMTVar("f", FunctionVMTSort(IntVMTSort(), BoolVMTSort()))
	next, _ := VMTVar("f^", FunctionVMTSort(IntVMTSort(), BoolVMTSort()))
	binding, err := FormatVMTAnnotated(VMTAnnotatedExpression{Kind: VMTNext, Name: ".f", Current: current, Next: next})
	want := "(define-fun .f ((_p0@ Bool)) Int (! (f _p0@) :next f^))"
	if err != nil || binding != want {
		t.Fatalf("next annotation = %q, %v; want %q", binding, err, want)
	}
}

func TestFormatVMTDeclarations(t *testing.T) {
	declaration, err := FormatVMTVariableDeclaration("x", IntVMTSort())
	if err != nil || declaration != "(declare-fun x () Int)\n(declare-fun x^ () Int)" {
		t.Fatalf("variable declaration = %q, %v", declaration, err)
	}
	a, _ := UninterpretedVMTSort("A")
	literal, _ := VMTLiteral("one", a)
	declaration, err = FormatVMTLiteralDeclaration(literal)
	want := "(declare-fun one_A () A)\n(define-fun .one_A () A (! one_A :global true))"
	if err != nil || declaration != want {
		t.Fatalf("literal declaration = %q, %v", declaration, err)
	}
	distinct, err := FormatVMTDistinct([]VMTTerm{literal})
	if err != nil || distinct != "(define-fun .axiom () Bool (! (distinct one_A) :axiom true))" {
		t.Fatalf("distinct = %q, %v", distinct, err)
	}
}
