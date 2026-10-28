package apalache

import "testing"

func TestUTFPrinterScalaOracleExpressions(t *testing.T) {
	n := func(name string) Expr { return NewNameExpr(name, Untyped()) }
	i := func(value int64) Expr { return NewValueExpr(Int64Value(value), Untyped()) }
	s := func(value string) Expr { return NewValueExpr(StringValue(value), Untyped()) }
	b := func(value bool) Expr { return NewValueExpr(BoolValue(value), Untyped()) }
	o := func(name string, arguments ...Expr) Expr { return MustOperExpr(Op(name), Untyped(), arguments...) }
	local := NewOperDecl("c", nil, i(1), Untyped())
	let := NewLetInExpr(o("OPER_APP", n("c")), Untyped(), local)
	decimal, _ := NewDecimalValue("3.14")
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{"null", NullExpr(), "<[NULL]>"}, {"name", n("x"), "x"}, {"integer", i(42), "42"},
		{"decimal", NewValueExpr(decimal, Untyped()), "3.14"}, {"string", s("hello"), `"hello"`},
		{"true", b(true), "TRUE"}, {"false", b(false), "FALSE"},
		{"boolean set", NewValueExpr(BoolSet, Untyped()), "BOOLEAN"}, {"real", NewValueExpr(RealSet, Untyped()), "Real"},
		{"infinity", NewValueExpr(RealInfinity(), Untyped()), "Infinity"}, {"let", let, "LET c ≜ 1 IN c()"},
		{"equality", o("EQ", o("PLUS", n("a"), n("b")), o("MINUS", n("c"), n("d"))), "(a + b) = (c - d)"},
		{"inequality", o("NE", n("a"), n("b")), "a ≠ b"}, {"nullary apply", o("OPER_APP", n("x")), "x()"},
		{"nested apply", o("OPER_APP", n("a"), o("OPER_APP", n("b"), n("c"))), "a(b(c))"},
		{"choose", o("CHOOSE3", n("x"), o("SET_TIMES", n("S"), n("T")), o("AND", n("p"), n("q"))), "CHOOSE x ∈ (S × T) : (p ∧ q)"},
		{"empty and", o("AND"), ""}, {"and", o("AND", n("a"), o("AND", n("b"), n("c"))), "a ∧ (b ∧ c)"},
		{"or", o("OR", n("a"), n("b")), "a ∨ b"}, {"not", o("NOT", o("AND", n("a"), n("b"))), "¬(a ∧ b)"},
		{"implies", o("IMPLIES", o("AND", n("p"), n("q")), o("OR", n("a"), n("b"))), "(p ∧ q) ⇒ (a ∨ b)"},
		{"equivalence", o("EQUIV", n("p"), n("q")), "p ⇔ q"},
		{"forall", o("FORALL3", n("a"), o("SET_TIMES", n("b"), n("c")), o("AND", n("d"), n("e"))), "∀a ∈ (b × c): (d ∧ e)"},
		{"unbounded exists", o("EXISTS2", n("a"), o("OR", n("b"), n("c"))), "∃a: (b ∨ c)"},
		{"arithmetic", o("PLUS", o("MULT", n("a"), n("b")), n("c")), "(a * b) + c"},
		{"unary minus", o("UNARY_MINUS", o("PLUS", n("a"), n("b"))), "-(a + b)"},
		{"division", o("DIV", n("a"), n("b")), "a ÷ b"}, {"comparison", o("LE", n("a"), n("b")), "a ≤ b"},
		{"prime", o("PRIME", o("PLUS", n("x"), i(1))), "(x + 1)'"}, {"stutter", o("STUTTER", n("p"), n("x")), "[p]_x"},
		{"nonstutter", o("NO_STUTTER", n("p"), n("x")), "<p>_x"}, {"unchanged", o("UNCHANGED", n("x")), "UNCHANGED x"},
		{"enabled", o("ENABLED", o("AND", n("p"), n("q"))), "ENABLED (p ∧ q)"},
		{"composition", o("COMPOSE", o("AND", n("p"), n("q")), o("OR", n("p"), n("q"))), "(p ∧ q) ⋅ (p ∨ q)"},
		{"case", o("CASE", n("p"), n("a"), n("q"), n("b")), "CASE p → a □ q → b"},
		{"case other let", o("CASE_OTHER", let, n("p"), n("a")), "CASE p → a □ OTHER → (LET c ≜ 1 IN c())"},
		{"if", o("IF_THEN_ELSE", o("AND", n("p"), n("q")), o("PLUS", n("a"), n("b")), o("MINUS", n("a"), n("b"))), "IF (p ∧ q) THEN (a + b) ELSE (a - b)"},
		{"always", o("GLOBALLY", o("AND", n("p"), n("q"))), "□(p ∧ q)"}, {"eventually", o("EVENTUALLY", n("p")), "◇p"},
		{"temporal quantifier", o("TEMPORAL_FORALL", n("x"), n("p")), "[∀]x . p"},
		{"temporal exists", o("TEMPORAL_EXISTS", n("x"), n("p")), "[∃]x . p"},
		{"guarantees", o("GUARANTEES", n("p"), n("q")), "p ⇸ q"}, {"leads to", o("LEADS_TO", n("p"), n("q")), "p ↝ q"},
		{"fairness", o("WEAK_FAIRNESS", n("x"), n("p")), "WF_x(p)"}, {"strong fairness", o("STRONG_FAIRNESS", n("x"), n("p")), "SF_x(p)"},
		{"cardinality", o("FiniteSets!Cardinality", o("SET_ENUM", i(1), i(2))), "Cardinality({1, 2})"},
		{"finite", o("FiniteSets!IsFiniteSet", o("SET_POWERSET", n("S"))), "IsFiniteSet(SUBSET S)"},
		{"function app", o("FUN_APP", o("FUN_APP", n("f"), n("i")), n("j")), "f[i][j]"},
		{"domain", o("DOMAIN", o("FUN_APP", n("f"), n("i"))), "DOMAIN f[i]"},
		{"record", o("RECORD", s("foo"), n("a"), s("bar"), n("b")), `["foo" ↦ a, "bar" ↦ b]`},
		{"except", o("EXCEPT", n("f"), n("i"), n("v"), n("j"), n("a")), "[f EXCEPT ![i] = v, ![j] = a]"},
		{"function constructor", o("FUN_CTOR", o("PLUS", n("x"), n("y")), n("x"), n("S"), n("y"), n("T")), "[x ∈ S, y ∈ T ↦ x + y]"},
		{"tuple", o("TUPLE", n("a"), n("b")), "⟨a, b⟩"}, {"empty tuple", o("TUPLE"), "⟨⟩"}, {"recursive ref", o("FUN_REC_REF"), "FUN_REC_REF"},
		{"recursive constructor", o("FUN_REC_CTOR", n("x"), n("x"), n("S")), "FUN_REC_CTOR(x, x, S)"},
		{"append", o("Sequences!Append", o("TUPLE", n("a")), n("b")), "Append(⟨a⟩, b)"},
		{"concat", o("Sequences!Concat", o("Sequences!Append", n("S"), n("a")), n("T")), "(Append(S, a)) ∘ T"},
		{"head", o("Sequences!Head", n("s")), "Head(s)"}, {"tail", o("Sequences!Tail", n("s")), "Tail(s)"}, {"length", o("Sequences!Len", n("s")), "Len(s)"},
		{"subseq default", o("Sequences!SubSeq", n("s"), i(1), i(2)), "Sequences!SubSeq(s, 1, 2)"},
		{"empty set", o("SET_ENUM"), "{}"}, {"membership", o("SET_IN", o("PLUS", n("a"), n("b")), n("S")), "(a + b) ∈ S"},
		{"nonmembership", o("SET_NOT_IN", n("a"), n("S")), "a ∉ S"}, {"intersection", o("SET_INTERSECT", n("S"), n("T")), "S ∩ T"},
		{"union", o("SET_UNION2", n("S"), n("T")), "S ∪ T"}, {"minus", o("SET_MINUS", n("S"), n("T")), "S ∖ T"},
		{"subseteq", o("SET_SUBSET_EQ", n("S"), n("T")), "S ⊆ T"}, {"powerset", o("SET_POWERSET", n("S")), "SUBSET S"},
		{"filter", o("SET_FILTER", n("x"), o("SET_UNION2", n("S"), n("T")), o("AND", n("p"), o("SET_IN", n("x"), n("S")))), "{x ∈ (S ∪ T): (p ∧ x ∈ S)}"},
		{"function set", o("FUN_SET", n("S"), n("T")), "[S → T]"},
		{"set map", o("SET_MAP", o("PLUS", n("x"), n("y")), n("x"), n("S"), n("y"), n("T")), "{x + y : x ∈ S, y ∈ T}"},
		{"record set", o("RECORD_SET", s("foo"), n("S"), s("bar"), n("T")), `["foo": S, "bar": T]`},
		{"sequence set", o("Sequences!Seq", n("S")), "Seq(S)"}, {"unary union", o("SET_UNARY_UNION", n("S")), "UNION S"},
		{"product", o("SET_TIMES", n("S"), n("T"), o("SET_ENUM", n("a"), n("b"))), "S × T × {a, b}"},
		{"label", o("LABEL", o("EQ", n("a"), n("b")), s("lab"), s("i"), s("j")), `lab("i", "j")∷ a = b`},
		{"default", o("Apalache!Gen", n("S")), "Apalache!Gen(S)"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := FormatUTFExpression(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
	if got, err := FormatUTFExpression(o("PLUS", n("a"), n("b")), true); err != nil || got != "a+b" {
		t.Fatalf("remove spaces = %q, %v", got, err)
	}
}

func TestUTFPrinterScalaOracleDeclarations(t *testing.T) {
	n := func(name string) Expr { return NewNameExpr(name, Untyped()) }
	o := func(name string, arguments ...Expr) Expr { return MustOperExpr(Op(name), Untyped(), arguments...) }
	declarations := []struct {
		declaration Decl
		want        string
	}{
		{NewConstDecl("C", Untyped()), "CONSTANT C"},
		{NewVarDecl("x", Untyped()), "VARIABLE x"},
		{NewOperDecl("A", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped()), "A ≜ 1"},
		{NewOperDecl("A", []OperatorParam{ValueParam("x"), ValueParam("y")}, o("PLUS", n("x"), n("y")), Untyped()), "A(x, y) ≜ x + y"},
		{NewOperDecl("Apply", []OperatorParam{{Name: "F", Arity: 2}, ValueParam("x")}, NewValueExpr(Int64Value(1), Untyped()), Untyped()), "Apply(F(_, _), x) ≜ 1"},
		{NewNamedAssumeDecl("myAssume", o("EQ", n("x"), NewValueExpr(BoolValue(true), Untyped())), Untyped()), "ASSUME myAssume ≜ x = TRUE"},
		{NewUnnamedAssumeDecl(o("EQ", n("x"), NewValueExpr(BoolValue(true), Untyped())), Untyped()), "ASSUME x = TRUE"},
	}
	for _, test := range declarations {
		got, err := FormatUTFDeclaration(test.declaration)
		if err != nil || got != test.want {
			t.Errorf("%T = %q, %v; want %q", test.declaration, got, err, test.want)
		}
	}
	if _, err := FormatUTFDeclaration(NewTheoremDecl("Thm", NewValueExpr(BoolValue(true), Untyped()), Untyped())); err == nil {
		t.Fatal("UTF printer accepted theorem declaration")
	}
}

func TestSimplePrinterTransliteratesScalaUTFMarkers(t *testing.T) {
	expression := MustOperExpr(Op("AND"), Untyped(),
		MustOperExpr(Op("SET_IN"), Untyped(), NewNameExpr("x", Untyped()), NewNameExpr("S", Untyped())),
		MustOperExpr(Op("LEADS_TO"), Untyped(), NewNameExpr("p", Untyped()), NewNameExpr("q", Untyped())),
	)
	got, err := FormatSimpleExpression(expression)
	if err != nil {
		t.Fatal(err)
	}
	if got != `x \in S /\ (p ~> q)` {
		t.Fatalf("simple output = %q", got)
	}
}
