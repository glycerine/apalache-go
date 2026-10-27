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
	cases := []struct {
		expression Expr
		want       string
	}{
		{NewValueExpr(Int64Value(-3), Typed(IntT1)), "-3"},
		{NewValueExpr(BoolValue(true), Typed(BoolT1)), "true"},
		{NewValueExpr(StringValue("plain"), Typed(StrT1)), "plain_Str"},
		{NewValueExpr(StringValue("alice_OF_SSORT"), Typed(typ)), "alice_SSORT"},
		{NewNameExpr("x", Typed(typ)), "x"},
		{MustOperExpr(Op("PRIME"), Typed(typ), NewNameExpr("x", Typed(typ))), "x^"},
	}
	for _, test := range cases {
		if got := rewriteVMTText(t, rewriter, test.expression); got != test.want {
			t.Errorf("rewrite %T = %q, want %q", test.expression, got, test.want)
		}
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
	and := MustOperExpr(Op("AND"), Typed(BoolT1), p,
		MustOperExpr(Op("NOT"), Typed(BoolT1), q))
	if got := rewriteVMTText(t, rewriter, and); got != "(and p (not q))" {
		t.Fatalf("Boolean rewrite = %q", got)
	}

	x := NewNameExpr("x", Typed(typ))
	set := NewNameExpr("S", Typed(mustSet(typ)))
	predicate := MustOperExpr(Op("EQ"), Typed(BoolT1), x, x)
	forall := MustOperExpr(Op("FORALL3"), Typed(BoolT1), x, set, predicate)
	if got := rewriteVMTText(t, rewriter, forall); got != "(forall ((x SSORT)) (= x x))" {
		t.Fatalf("quantifier rewrite = %q", got)
	}

	intX := NewNameExpr("i", Typed(IntT1))
	exists := MustOperExpr(Op("EXISTS3"), Typed(BoolT1), intX,
		NewValueExpr(NatSet, Typed(mustSet(IntT1))),
		MustOperExpr(Op("EQ"), Typed(BoolT1), intX, intX))
	if got := rewriteVMTText(t, rewriter, exists); got != "(exists ((i Int)) (= i i))" {
		t.Fatalf("integer quantifier rewrite = %q", got)
	}
	bad := MustOperExpr(Op("FORALL3"), Typed(BoolT1), intX,
		NewValueExpr(StringSet, Typed(mustSet(StrT1))), predicate)
	if _, err := rewriter.Rewrite(bad); err == nil {
		t.Fatal("unrestricted STRING quantifier was accepted")
	}
}

func TestVMTRewriterFunctionConstructionApplicationAndEquality(t *testing.T) {
	rewriter, typ, _ := newTestVMTRewriter(t)
	tupleType := mustTuple(typ, IntT1)
	functionType := mustFun(tupleType, typ)
	x := NewNameExpr("x", Typed(typ))
	y := NewNameExpr("y", Typed(IntT1))
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
