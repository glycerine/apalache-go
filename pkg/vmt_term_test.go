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

func TestVMTFormulaBaseMatchesScala(t *testing.T) {
	a, err := UninterpretedVMTSort("A")
	if err != nil {
		t.Fatal(err)
	}
	function := FunctionVMTSort(a, IntVMTSort(), BoolVMTSort())
	for _, test := range []struct {
		sort VMTSort
		name string
	}{
		{BoolVMTSort(), "Boolean"}, {IntVMTSort(), "Integer"},
		{UntypedVMTSort(), "Untyped"}, {a, "A"}, {function, "Function"},
	} {
		if test.sort.SortName() != test.name {
			t.Errorf("sort name = %q, want %q", test.sort.SortName(), test.name)
		}
	}
	if function.Arity() != 2 || IntVMTSort().Arity() != 0 {
		t.Fatalf("sort arities = %d, %d", function.Arity(), IntVMTSort().Arity())
	}

	constant := VMTConstDeclaration{Name: "x", Sort: a}
	declaration := VMTFunctionDeclaration{Name: "f", Sort: function}
	variable, err := declaration.AsVar()
	if err != nil || variable.Name != "f" || !variable.Sort.Equal(function) {
		t.Fatalf("declared function variable = %#v, %v", variable, err)
	}
	body, _ := VMTVar("result", a)
	definition := VMTFunctionDefinition{
		Name: "g", Arguments: []VMTBinding{{Name: "i", Sort: IntVMTSort()}}, Body: body,
	}
	defined, err := definition.AsVar()
	wantDefinedSort := FunctionVMTSort(a, IntVMTSort())
	if err != nil || defined.Name != "g" || !defined.Sort.Equal(wantDefinedSort) {
		t.Fatalf("defined function variable = %#v, %v", defined, err)
	}
	var declarations []VMTDeclaration = []VMTDeclaration{constant, declaration, definition}
	if len(declarations) != 3 {
		t.Fatal("formula declaration family is incomplete")
	}
}

func TestVMTBooleanFormulaFamilyMatchesScala(t *testing.T) {
	truth, falsehood := VMTBool(true), VMTBool(false)
	integer, _ := VMTInt(big.NewInt(1))
	for _, build := range []func() error{
		func() error { _, err := VMTAndTerm(truth, falsehood); return err },
		func() error { _, err := VMTOrTerm(truth, falsehood); return err },
		func() error { _, err := VMTNotTerm(truth); return err },
		func() error { _, err := VMTImpliesTerm(truth, falsehood); return err },
		func() error { _, err := VMTEquivTerm(truth, falsehood); return err },
		func() error {
			_, err := VMTForallTerm([]VMTBinding{{Name: "p", Sort: BoolVMTSort()}}, truth)
			return err
		},
		func() error { _, err := VMTExistsTerm(nil, falsehood); return err },
	} {
		if err := build(); err != nil {
			t.Errorf("valid Boolean formula rejected: %v", err)
		}
	}
	for _, build := range []func() error{
		func() error { _, err := VMTAndTerm(truth, integer); return err },
		func() error { _, err := VMTOrTerm(integer); return err },
		func() error { _, err := VMTNotTerm(integer); return err },
		func() error { _, err := VMTImpliesTerm(truth, integer); return err },
		func() error { _, err := VMTEquivTerm(integer, falsehood); return err },
		func() error { _, err := VMTForallTerm(nil, integer); return err },
		func() error { _, err := VMTExistsTerm(nil, integer); return err },
	} {
		if err := build(); err == nil {
			t.Error("invalid Boolean formula was accepted")
		}
	}
}

func TestVMTIntegerFormulaFamilyMatchesScala(t *testing.T) {
	one, _ := VMTInt(big.NewInt(1))
	two, _ := VMTInt(big.NewInt(2))
	boolean := VMTBool(true)
	tests := []struct {
		name string
		kind VMTTermKind
		sort VMTSort
		make func(VMTTerm, VMTTerm) (VMTTerm, error)
	}{
		{"plus", VMTPlus, IntVMTSort(), VMTPlusTerm},
		{"minus", VMTMinus, IntVMTSort(), VMTMinusTerm},
		{"multiply", VMTMultiply, IntVMTSort(), VMTMultiplyTerm},
		{"divide", VMTDivide, IntVMTSort(), VMTDivideTerm},
		{"modulo", VMTModulo, IntVMTSort(), VMTModuloTerm},
		{"less-than", VMTLessThan, BoolVMTSort(), VMTLessThanTerm},
		{"less-equal", VMTLessEqual, BoolVMTSort(), VMTLessEqualTerm},
		{"greater-than", VMTGreaterThan, BoolVMTSort(), VMTGreaterThanTerm},
		{"greater-equal", VMTGreaterEqual, BoolVMTSort(), VMTGreaterEqualTerm},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			term, err := test.make(one, two)
			if err != nil || term.Kind != test.kind || !term.Sort.Equal(test.sort) {
				t.Fatalf("term = %#v, %v", term, err)
			}
			if _, err := test.make(one, boolean); err == nil {
				t.Fatal("non-Integer right argument was accepted")
			}
			if _, err := test.make(boolean, two); err == nil {
				t.Fatal("non-Integer left argument was accepted")
			}
		})
	}
	for _, test := range []struct {
		name string
		kind VMTTermKind
		make func(VMTTerm) (VMTTerm, error)
	}{
		{"unary-minus", VMTUnaryMinus, VMTUnaryMinusTerm},
		{"absolute", VMTAbsolute, VMTAbsoluteTerm},
	} {
		term, err := test.make(one)
		if err != nil || term.Kind != test.kind || !term.Sort.Equal(IntVMTSort()) {
			t.Fatalf("%s term = %#v, %v", test.name, term, err)
		}
		if _, err := test.make(boolean); err == nil {
			t.Errorf("%s accepted a Boolean", test.name)
		}
	}
}

func TestFormatVMTIntegerFormulaFamily(t *testing.T) {
	one, _ := VMTInt(big.NewInt(1))
	two, _ := VMTInt(big.NewInt(2))
	binary := []struct {
		make func(VMTTerm, VMTTerm) (VMTTerm, error)
		want string
	}{
		{VMTPlusTerm, "(+ 1 2)"}, {VMTMinusTerm, "(- 1 2)"},
		{VMTMultiplyTerm, "(* 1 2)"}, {VMTDivideTerm, "(div 1 2)"},
		{VMTModuloTerm, "(mod 1 2)"}, {VMTLessThanTerm, "(< 1 2)"},
		{VMTLessEqualTerm, "(<= 1 2)"}, {VMTGreaterThanTerm, "(> 1 2)"},
		{VMTGreaterEqualTerm, "(>= 1 2)"},
	}
	for _, test := range binary {
		term, _ := test.make(one, two)
		if got, err := FormatVMTTerm(term); err != nil || got != test.want {
			t.Errorf("formatted term = %q, %v; want %q", got, err, test.want)
		}
	}
	for _, test := range []struct {
		make func(VMTTerm) (VMTTerm, error)
		want string
	}{{VMTUnaryMinusTerm, "(- 1)"}, {VMTAbsoluteTerm, "(abs 1)"}} {
		term, _ := test.make(one)
		if got, err := FormatVMTTerm(term); err != nil || got != test.want {
			t.Errorf("formatted term = %q, %v; want %q", got, err, test.want)
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

func TestVMTTermConstructionScalaOracleCases(t *testing.T) {
	a, _ := UninterpretedVMTSort("A")
	b, _ := UninterpretedVMTSort("B")
	one, _ := VMTInt(big.NewInt(1))
	two, _ := VMTInt(big.NewInt(2))
	intA, _ := VMTVar("a", IntVMTSort())
	intB, _ := VMTVar("b", IntVMTSort())
	uvalA, _ := VMTLiteral("x", a)
	uvalB, _ := VMTLiteral("y", b)
	uvarA, _ := VMTVar("c", a)
	uvarB, _ := VMTVar("d", b)

	accept := func(name string, err error) {
		t.Helper()
		if err != nil {
			t.Errorf("%s rejected: %v", name, err)
		}
	}
	reject := func(name string, err error) {
		t.Helper()
		if err == nil {
			t.Errorf("%s was accepted", name)
		}
	}

	_, err := VMTEqualTerm(one, two)
	accept("equal integer literals", err)
	_, err = VMTEqualTerm(uvalA, uvalA)
	accept("equal same-sort literals", err)
	_, err = VMTEqualTerm(one, intA)
	accept("equal integer literal and variable", err)
	_, err = VMTEqualTerm(uvarA, uvalA)
	accept("equal uninterpreted variable and literal", err)
	_, err = VMTEqualTerm(one, uvalA)
	reject("equal integer and uninterpreted", err)
	_, err = VMTEqualTerm(uvalA, uvalB)
	reject("equal different uninterpreted sorts", err)

	_, err = VMTIfThenElseTerm(VMTBool(true), one, intB)
	accept("ITE integers", err)
	_, err = VMTIfThenElseTerm(VMTBool(false), uvalB, uvalB)
	accept("ITE same literal", err)
	condition, _ := VMTEqualTerm(one, two)
	_, err = VMTIfThenElseTerm(condition, uvalA, uvarA)
	accept("ITE same uninterpreted sort", err)
	boolVar, _ := VMTVar("x", BoolVMTSort())
	boolCondition, _ := VMTAndTerm(VMTBool(true), VMTBool(false), boolVar)
	_, err = VMTIfThenElseTerm(boolCondition, VMTBool(true), VMTBool(false))
	accept("ITE Booleans", err)
	_, err = VMTIfThenElseTerm(VMTBool(true), one, uvalA)
	reject("ITE integer and uninterpreted", err)
	_, err = VMTIfThenElseTerm(VMTBool(true), uvalA, uvarB)
	reject("ITE different uninterpreted sorts", err)

	nullary, _ := VMTVar("f", FunctionVMTSort(IntVMTSort()))
	unaryBool, _ := VMTVar("f", FunctionVMTSort(IntVMTSort(), BoolVMTSort()))
	unaryB, _ := VMTVar("f", FunctionVMTSort(a, b))
	nary, _ := VMTVar("f", FunctionVMTSort(b, IntVMTSort(), IntVMTSort(), BoolVMTSort()))
	_, err = VMTApplyTerm(nullary)
	accept("nullary application", err)
	_, err = VMTApplyTerm(unaryBool, VMTBool(true))
	accept("unary Boolean literal application", err)
	_, err = VMTApplyTerm(unaryBool, boolVar)
	accept("unary Boolean variable application", err)
	application, err := VMTApplyTerm(unaryB, uvalB)
	accept("unary uninterpreted application", err)
	_, err = VMTEqualTerm(application, uvarA)
	accept("unary application range", err)
	_, err = VMTApplyTerm(nary, one, intB, VMTBool(false))
	accept("n-ary application", err)
	_, err = VMTApplyTerm(unaryBool, intA)
	reject("unary wrong primitive sort", err)
	_, err = VMTApplyTerm(unaryB, uvarA)
	reject("unary wrong uninterpreted sort", err)
	_, err = VMTApplyTerm(nary, VMTBool(false), uvalA, intA)
	reject("n-ary wrong argument sorts", err)
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
	namedDistinct, err := FormatVMTDistinctNamed(".axiom.1", []VMTTerm{literal})
	if err != nil || namedDistinct != "(define-fun .axiom.1 () Bool (! (distinct one_A) :axiom true))" {
		t.Fatalf("named distinct = %q, %v", namedDistinct, err)
	}
	x, _ := VMTVar("x", IntVMTSort())
	definition, err := FormatVMTFunctionDefinition(VMTFunctionDefinition{
		Name: "f", Arguments: []VMTBinding{{Name: "x", Sort: IntVMTSort()}}, Body: x,
	})
	if err != nil || definition != "(define-fun f ((x Int)) Int x)" {
		t.Fatalf("function definition = %q, %v", definition, err)
	}
}
