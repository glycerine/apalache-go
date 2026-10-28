package apalache

import "testing"

func newTestVMTRewriter(t *testing.T) (*VMTRewriter, ConstType, VMTSort) {
	t.Helper()
	typ, err := NewConstType("SSORT")
	if err != nil {
		t.Fatal(err)
	}
	sort, err := UninterpretedVMTSort("SSORT")
	if err != nil {
		t.Fatal(err)
	}
	rewriter, err := NewVMTRewriter(map[string]VMTSort{"S": sort})
	if err != nil {
		t.Fatal(err)
	}
	return rewriter, typ, sort
}

func rewriteVMTText(t *testing.T, rewriter *VMTRewriter, expression Expr) string {
	t.Helper()
	term, err := rewriter.Rewrite(expression)
	if err != nil {
		t.Fatal(err)
	}
	text, err := FormatVMTTerm(term)
	if err != nil {
		t.Fatal(err)
	}
	return text
}

func TestVMTRewriterValuesAndDeclarations(t *testing.T) {
	rewriter, typ, _ := newTestVMTRewriter(t)
	aType := mustNamedConst(t, "A")
	bType := mustNamedConst(t, "B")
	cases := []struct {
		name       string
		expression Expr
		want       string
	}{
		{"integer-literal", NewValueExpr(Int64Value(1), Typed(IntT1)), "1"},
		{"integer-variable", NewNameExpr("x", Typed(IntT1)), "x"},
		{"Boolean-literal", NewValueExpr(BoolValue(true), Typed(BoolT1)), "true"},
		{"Boolean-variable", NewNameExpr("p", Typed(BoolT1)), "p"},
		{"model-value", NewValueExpr(StringValue("1_OF_A"), Typed(aType)), "1_A"},
		{"uninterpreted-variable", NewNameExpr("v", Typed(bType)), "v"},
		{"string-literal", NewValueExpr(StringValue("string"), Typed(StrT1)), "string_Str"},
		{"negative-integer", NewValueExpr(Int64Value(-3), Typed(IntT1)), "-3"},
		{"model-value-ssort", NewValueExpr(StringValue("alice_OF_SSORT"), Typed(typ)), "alice_SSORT"},
		{"prime", MustOperExpr(Op("PRIME"), Typed(typ), NewNameExpr("x", Typed(typ))), "x^"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteVMTText(t, rewriter, test.expression); got != test.want {
				t.Errorf("rewrite %T = %q, want %q", test.expression, got, test.want)
			}
		})
	}
	declarations := rewriter.Declarations()
	if _, ok := declarations.UninterpretedSorts["Str"]; !ok {
		t.Fatal("string sort was not retained")
	}
	if _, ok := declarations.UninterpretedSorts["SSORT"]; !ok {
		t.Fatal("model-value sort was not retained")
	}
	if got := declarations.UninterpretedLiterals["alice_SSORT"]; got.Name != "alice" {
		t.Fatalf("model value declaration = %#v", got)
	}
}

func TestVMTRewriterBooleanAndQuantifierRules(t *testing.T) {
	rewriter, typ, _ := newTestVMTRewriter(t)
	p := NewNameExpr("p", Typed(BoolT1))
	q := NewNameExpr("q", Typed(BoolT1))
	booleanCases := []struct {
		name       string
		expression Expr
		want       string
	}{
		{"and", MustOperExpr(Op("AND"), Typed(BoolT1), p, q), "(and p q)"},
		{"not", MustOperExpr(Op("NOT"), Typed(BoolT1), p), "(not p)"},
		{"or-implies", MustOperExpr(Op("OR"), Typed(BoolT1),
			MustOperExpr(Op("IMPLIES"), Typed(BoolT1), p, q), p), "(or (=> p q) p)"},
	}
	for _, test := range booleanCases {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteVMTText(t, rewriter, test.expression); got != test.want {
				t.Fatalf("Boolean rewrite = %q, want %q", got, test.want)
			}
		})
	}

	x := NewNameExpr("x", Typed(typ))
	set := NewNameExpr("S", Typed(mustSet(typ)))
	y := NewNameExpr("y", Typed(IntT1))
	quantifierCases := []struct {
		name       string
		expression Expr
		want       string
	}{
		{"exists-constant-set", MustOperExpr(Op("EXISTS3"), Typed(BoolT1), x, set, p),
			"(exists ((x SSORT)) p)"},
		{"forall-integers", MustOperExpr(Op("FORALL3"), Typed(BoolT1), y,
			NewValueExpr(IntSet, Typed(mustSet(IntT1))), q), "(forall ((y Int)) q)"},
	}
	for _, test := range quantifierCases {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteVMTText(t, rewriter, test.expression); got != test.want {
				t.Fatalf("quantifier rewrite = %q, want %q", got, test.want)
			}
		})
	}

	unknownType := mustNamedConst(t, "TSORT")
	unknownSet := MustOperExpr(Op("EXISTS3"), Typed(BoolT1),
		NewNameExpr("t", Typed(unknownType)), NewNameExpr("T", Typed(mustSet(unknownType))), p)
	if _, err := rewriter.Rewrite(unknownSet); err == nil {
		t.Fatal("quantifier over undeclared constant set T was accepted")
	}
}

func TestVMTRewriterFunctionConstructionApplicationAndEquality(t *testing.T) {
	rewriter, typ, _ := newTestVMTRewriter(t)
	tupleType := mustTuple(typ, IntT1)
	functionType := mustFun(tupleType, typ)
	x := NewNameExpr("x", Typed(typ))
	y := NewNameExpr("y", Typed(IntT1))
	p := NewNameExpr("p", Typed(BoolT1))
	q := NewNameExpr("q", Typed(BoolT1))
	directCases := []struct {
		name       string
		expression Expr
		want       string
	}{
		{"assignment", MustOperExpr(Op("Apalache!:="), Typed(BoolT1),
			MustOperExpr(Op("PRIME"), Typed(typ), x), x), "(= x^ x)"},
		{"equality", MustOperExpr(Op("EQ"), Typed(BoolT1), x, x), "(= x x)"},
		{"ite", MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), p, p, q), "(ite p p q)"},
	}
	for _, test := range directCases {
		t.Run(test.name, func(t *testing.T) {
			if got := rewriteVMTText(t, rewriter, test.expression); got != test.want {
				t.Fatalf("EUF rewrite = %q, want %q", got, test.want)
			}
		})
	}

	constructor := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), x,
		x, NewNameExpr("S", Typed(mustSet(typ))),
		y, NewValueExpr(IntSet, Typed(mustSet(IntT1))))
	if got := rewriteVMTText(t, rewriter, constructor); got != "t_1" {
		t.Fatalf("function constructor = %q", got)
	}
	definition := rewriter.Declarations().DefinedFunctions["t_1"]
	if got, err := FormatVMTFunctionDefinition(definition); err != nil || got != "(define-fun t_1 ((x SSORT) (y Int)) SSORT x)" {
		t.Fatalf("function definition = %q, %v", got, err)
	}
	if order := rewriter.Declarations().DefinitionOrder; len(order) != 1 || order[0] != "t_1" {
		t.Fatalf("function definition order = %v", order)
	}

	f := NewNameExpr("f", Typed(functionType))
	tuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), x, y)
	application := MustOperExpr(Op("FUN_APP"), Typed(typ), f, tuple)
	if got := rewriteVMTText(t, rewriter, application); got != "(f x y)" {
		t.Fatalf("function application = %q", got)
	}

	g := NewNameExpr("g", Typed(functionType))
	equality := MustOperExpr(Op("EQ"), Typed(BoolT1), f, g)
	if got := rewriteVMTText(t, rewriter, equality); got != "(forall ((t_2 SSORT) (t_3 Int)) (= (f t_2 t_3) (g t_2 t_3)))" {
		t.Fatalf("function equality = %q", got)
	}
}

func TestVMTRewriterExceptDefinesPointUpdate(t *testing.T) {
	rewriter, typ, _ := newTestVMTRewriter(t)
	tupleType := mustTuple(typ, IntT1)
	functionType := mustFun(tupleType, typ)
	f := NewNameExpr("f", Typed(functionType))
	x := NewNameExpr("x", Typed(typ))
	y := NewNameExpr("y", Typed(IntT1))
	pathTuple := MustOperExpr(Op("TUPLE"), Typed(tupleType), x, y)
	outerPath := MustOperExpr(Op("TUPLE"), Typed(mustTuple(tupleType)), pathTuple)
	newValue := NewValueExpr(StringValue("new_OF_SSORT"), Typed(typ))
	except := MustOperExpr(Op("EXCEPT"), Typed(functionType), f, outerPath, newValue)
	if got := rewriteVMTText(t, rewriter, except); got != "t_3" {
		t.Fatalf("EXCEPT rewrite = %q", got)
	}
	definition := rewriter.Declarations().DefinedFunctions["t_3"]
	want := "(define-fun t_3 ((t_1 SSORT) (t_2 Int)) SSORT (ite (and (= t_1 x) (= t_2 y)) new_SSORT (f t_1 t_2)))"
	if got, err := FormatVMTFunctionDefinition(definition); err != nil || got != want {
		t.Fatalf("EXCEPT definition = %q, %v; want %q", got, err, want)
	}
}

func TestVMTRewriterRejectsUnsupportedExpressions(t *testing.T) {
	rewriter, _, _ := newTestVMTRewriter(t)
	if _, err := rewriter.Rewrite(NewValueExpr(Real(), Typed(RealT1))); err == nil {
		t.Fatal("real literal was accepted")
	}
	if _, err := rewriter.Rewrite(MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))); err == nil {
		t.Fatal("arithmetic expression was accepted")
	}
}

func TestVMTRestrictedSetJudgementScalaOracleCases(t *testing.T) {
	s1, _ := UninterpretedVMTSort("S1")
	s2, _ := UninterpretedVMTSort("S2")
	zSort, _ := UninterpretedVMTSort("ZSORT")
	rewriter, err := NewVMTRewriter(map[string]VMTSort{"x": s1, "y": s2, "z": zSort})
	if err != nil {
		t.Fatal(err)
	}
	constType := mustNamedConst(t, "X")
	allowed := []struct {
		name       string
		expression Expr
		want       VMTSort
	}{
		{"Int", NewValueExpr(IntSet, Typed(mustSet(IntT1))), IntVMTSort()},
		{"Nat", NewValueExpr(NatSet, Typed(mustSet(IntT1))), IntVMTSort()},
		{"BOOLEAN", NewValueExpr(BoolSet, Typed(mustSet(BoolT1))), BoolVMTSort()},
		{"x", NewNameExpr("x", Typed(constType)), s1},
		{"y", NewNameExpr("y", Typed(constType)), s2},
		{"z", NewNameExpr("z", Typed(constType)), zSort},
	}
	for _, test := range allowed {
		t.Run("allowed-"+test.name, func(t *testing.T) {
			got, err := rewriter.restrictedSetSort(test.expression)
			if err != nil || !got.Equal(test.want) {
				t.Fatalf("restricted-set sort = %v, %v; want %v", got, err, test.want)
			}
		})
	}

	intSetType := mustSet(IntT1)
	disallowed := []Expr{
		NewValueExpr(RealSet, Typed(mustSet(RealT1))),
		NewValueExpr(StringSet, Typed(mustSet(StrT1))),
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType),
			NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1))),
		MustOperExpr(Op("INT_RANGE"), Typed(intSetType),
			NewValueExpr(Int64Value(0), Typed(IntT1)), NewValueExpr(Int64Value(42), Typed(IntT1))),
		NewNameExpr("potato", Typed(intSetType)),
	}
	for index, expression := range disallowed {
		if _, err := rewriter.restrictedSetSort(expression); err == nil {
			t.Errorf("disallowed restricted-set oracle case %d was accepted", index)
		}
	}
}
