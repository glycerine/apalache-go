package apalache

import (
	"context"
	"strings"
	"testing"
)

func tlaName(name string) Expr { return NewNameExpr(name, Untyped()) }
func tlaInt(value int64) Expr  { return NewValueExpr(Int64Value(value), Untyped()) }
func tlaString(value string) Expr {
	return NewValueExpr(StringValue(value), Untyped())
}
func tlaOper(name string, arguments ...Expr) Expr {
	return MustOperExpr(Op(name), Untyped(), arguments...)
}

func TestSanitizeTLAIdentifierMatchesScalaWriter(t *testing.T) {
	if got := SanitizeTLAIdentifier("__123awesome::possom::pie-for-all"); got != "id__123awesome_possom_pie_for_all" {
		t.Fatalf("got %q", got)
	}
	if got := ParseableTLAName("A!B$C"); got != "A_i_B_si_C" {
		t.Fatalf("got %q", got)
	}
}

func TestFormatTLAExpressionScalaOracleCases(t *testing.T) {
	prime := tlaOper("PRIME", tlaName("x"))
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{"apply name", tlaOper("OPER_APP", tlaName("A!B")), "A_i_B"},
		{"apply arguments", tlaOper("OPER_APP", tlaName("A"), tlaInt(1), tlaInt(2)), "A(1, 2)"},
		{"assignment", tlaOper("Apalache!:=", prime, tlaInt(2)), "x' := 2"},
		{"enabled prime", tlaOper("ENABLED", prime), "ENABLED (x')"},
		{"unchanged", tlaOper("UNCHANGED", tlaName("x")), "UNCHANGED x"},
		{"stutter", tlaOper("STUTTER", tlaName("A"), tlaName("vars")), "[A]_vars"},
		{"nonstutter", tlaOper("NO_STUTTER", tlaName("A"), NewValueExpr(BoolValue(false), Untyped())), "<<A>>_FALSE"},
		{"weak fairness", tlaOper("WEAK_FAIRNESS", tlaName("vars"), tlaName("A")), "WF_vars(A)"},
		{"precedence", tlaOper("MULT", tlaOper("PLUS", tlaName("x"), tlaName("y")), tlaName("z")), "(x + y) * z"},
		{"empty conjunction", tlaOper("AND"), "TRUE"},
		{"set", tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)), "{ 1, 2 }"},
		{"tuple", tlaOper("TUPLE", tlaInt(1), tlaInt(2)), "<<1, 2>>"},
		{"record", tlaOper("RECORD", tlaString("a"), tlaInt(1)), "[a |-> 1]"},
		{"function", tlaOper("FUN_CTOR", tlaName("x"), tlaName("x"), tlaName("S")), "[x \\in S |-> x]"},
		{"map set", tlaOper("SET_MAP", tlaName("x"), tlaName("x"), tlaName("S")), "{ (x): x \\in S }"},
		{"filter", tlaOper("SET_FILTER", tlaName("x"), tlaName("S"), tlaOper("GT", tlaName("x"), tlaInt(0))), "{x \\in S: x > 0}"},
		{"bounded exists", tlaOper("EXISTS3", tlaName("x"), tlaName("S"), tlaOper("GT", tlaName("x"), tlaInt(0))), "\\E x \\in S: x > 0"},
		{"temporal forall", tlaOper("TEMPORAL_FORALL", tlaName("x"), tlaName("A")), "\\AA x: A"},
		{"if", tlaOper("IF_THEN_ELSE", tlaName("A"), tlaInt(1), tlaInt(2)), "IF A THEN 1 ELSE 2"},
		{"case other", tlaOper("CASE_OTHER", tlaInt(0), tlaName("A"), tlaInt(1)), "CASE A -> 1 [] OTHER -> 0"},
		{"except", tlaOper("EXCEPT", tlaName("f"), tlaOper("TUPLE", tlaString("a"), tlaInt(1)), tlaInt(2)), "[f EXCEPT ![\"a\", 1] = 2]"},
		{"label", tlaOper("LABEL", tlaName("A"), tlaString("L"), tlaString("x")), "L(x) :: A"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := FormatTLAExpression(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestFormatTLAPrecedenceAndBinderOracleCases(t *testing.T) {
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{"composition left of equality", tlaOper("EQ", tlaOper("COMPOSE", tlaName("a"), tlaName("b")), tlaName("c")), "(a \\cdot b) = c"},
		{"composition right of equality", tlaOper("EQ", tlaName("a"), tlaOper("COMPOSE", tlaName("b"), tlaName("c"))), "a = (b \\cdot c)"},
		{"UNION left of product", tlaOper("SET_TIMES", tlaOper("SET_UNARY_UNION", tlaName("S")), tlaName("T")), "(UNION S) \\X T"},
		{"SUBSET right of product", tlaOper("SET_TIMES", tlaName("S"), tlaOper("SET_POWERSET", tlaName("T"))), "S \\X (SUBSET T)"},
		{"DOMAIN left of product", tlaOper("SET_TIMES", tlaOper("DOMAIN", tlaName("S")), tlaName("T")), "(DOMAIN S) \\X T"},
		{"unbounded choose", tlaOper("CHOOSE2", tlaName("z"), NewValueExpr(BoolValue(true), Untyped())), "CHOOSE z : TRUE"},
		{"unbounded forall", tlaOper("FORALL2", tlaName("z"), NewValueExpr(BoolValue(false), Untyped())), "\\A z : FALSE"},
		{"unbounded exists", tlaOper("EXISTS2", tlaName("z"), NewValueExpr(BoolValue(false), Untyped())), "\\E z : FALSE"},
		{"choose under equality", tlaOper("EQ", tlaOper("CHOOSE2", tlaName("z"), NewValueExpr(BoolValue(true), Untyped())), tlaName("y")), "(CHOOSE z : TRUE) = y"},
		{"negative exponent", tlaOper("POW", tlaInt(2), tlaInt(-1)), "2 ^ (-1)"},
		{"negative multiplier", tlaOper("MULT", tlaInt(2), tlaInt(-1)), "2 * (-1)"},
		{"negative divisor", tlaOper("DIV", tlaInt(2), tlaInt(-1)), "2 \\div (-1)"},
		{"minus negative", tlaOper("UNARY_MINUS", tlaInt(-106)), "-(-106)"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := FormatTLAExpression(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			module := "---- MODULE PrettyRoundTrip ----\nEXTENDS Integers, Sequences\nVARIABLES a, b, c, S, T, y\nTest == " + got + "\n====\n"
			parsed := ParseSource(context.Background(), "PrettyRoundTrip.tla", module, FrontendOptions{})
			if !parsed.OK() {
				t.Fatalf("formatted expression did not parse: %v\n%s", parsed.Diagnostics, module)
			}
		})
	}
}

func TestFormatTLAActionLabelAndPrefixOracleCases(t *testing.T) {
	falseValue := NewValueExpr(BoolValue(false), Untyped())
	emptyTuple := tlaOper("TUPLE")
	head := tlaOper("Sequences!Head", emptyTuple)
	labelledTuple := tlaOper("LABEL", emptyTuple, tlaString("label0"))
	labelledFalse := tlaOper("LABEL", falseValue, tlaString("label0"))
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{"stutter bare", tlaOper("STUTTER", falseValue, tlaName("x")), "[FALSE]_x"},
		{"nonstutter grouped", tlaOper("NO_STUTTER", falseValue, head), "<<FALSE>>_(Head(<<>>))"},
		{"weak fairness grouped", tlaOper("WEAK_FAIRNESS", tlaInt(0), falseValue), "WF_(0)(FALSE)"},
		{"strong fairness bare", tlaOper("STRONG_FAIRNESS", tlaName("x"), falseValue), "SF_x(FALSE)"},
		{"label left concat", tlaOper("Sequences!Concat", labelledTuple, emptyTuple), "(label0 :: <<>>) \\o <<>>"},
		{"label right concat", tlaOper("Sequences!Concat", emptyTuple, labelledTuple), "<<>> \\o (label0 :: <<>>)"},
		{"label membership", tlaOper("SET_IN", tlaOper("LABEL", tlaName("x"), tlaString("label0")), tlaName("S")), "(label0 :: x) \\in S"},
		{"label function", tlaOper("FUN_APP", tlaOper("LABEL", tlaName("a"), tlaString("label0")), tlaInt(1)), "(label0 :: a)[1]"},
		{"label argument", tlaOper("FUN_APP", tlaName("a"), tlaOper("LABEL", tlaInt(1), tlaString("label0"))), "a[(label0 :: 1)]"},
		{"label enabled", tlaOper("ENABLED", labelledFalse), "ENABLED (label0 :: FALSE)"},
		{"enabled equality", tlaOper("ENABLED", tlaOper("EQ", tlaName("x"), falseValue)), "ENABLED (x = FALSE)"},
		{"unchanged membership", tlaOper("UNCHANGED", tlaOper("SET_IN", tlaName("x"), tlaName("S"))), "UNCHANGED (x \\in S)"},
		{"eventually inequality", tlaOper("EVENTUALLY", tlaOper("NE", tlaName("x"), falseValue)), "<>(x /= FALSE)"},
		{"always nonmembership", tlaOper("GLOBALLY", tlaOper("SET_NOT_IN", tlaName("x"), tlaName("S"))), "[](x \\notin S)"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := FormatTLAExpression(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			module := "---- MODULE PrettyActionRoundTrip ----\nEXTENDS Integers, Sequences\nVARIABLES a, x, S\nTest == " + got + "\n====\n"
			parsed := ParseSource(context.Background(), "PrettyActionRoundTrip.tla", module, FrontendOptions{})
			if !parsed.OK() {
				t.Fatalf("formatted expression did not parse: %v\n%s", parsed.Diagnostics, module)
			}
		})
	}
}

func TestFormatTLASetMapBodyOracleCases(t *testing.T) {
	setMap := func(body Expr, bindings ...Expr) Expr {
		return tlaOper("SET_MAP", append([]Expr{body}, bindings...)...)
	}
	membership := tlaOper("SET_IN", tlaOper("NOT", NewValueExpr(BoolValue(false), Untyped())), tlaName("S"))
	tupleMembership := tlaOper("SET_IN", tlaOper("TUPLE", tlaName("x"), tlaName("y")), tlaName("S"))
	cases := []struct {
		name string
		expr Expr
		want string
	}{
		{"ordinary multiple binders", setMap(tlaOper("PLUS", tlaName("u"), tlaName("v")), tlaName("u"), tlaName("S"), tlaName("v"), tlaName("T")), "{ (u + v): u \\in S, v \\in T }"},
		{"scalar membership body", setMap(membership, tlaName("z"), tlaName("T")), "{ ((~FALSE) \\in S): z \\in T }"},
		{"tuple membership body", setMap(tupleMembership, tlaName("z"), tlaName("T")), "{ (<<x, y>> \\in S): z \\in T }"},
		{"nested map body", setMap(setMap(tlaOper("SET_IN", tlaName("x"), tlaName("S")), tlaName("u"), tlaName("T")), tlaName("v"), tlaName("S")), "{ ({ (x \\in S): u \\in T }): v \\in S }"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := FormatTLAExpression(test.expr)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("got %q, want %q", got, test.want)
			}
			module := "---- MODULE PrettySetMapRoundTrip ----\nEXTENDS Integers\nVARIABLES S, T, x, y\nTest == " + got + "\n====\n"
			parsed := ParseSource(context.Background(), "PrettySetMapRoundTrip.tla", module, FrontendOptions{})
			if !parsed.OK() {
				t.Fatalf("formatted expression did not parse: %v\n%s", parsed.Diagnostics, module)
			}
		})
	}
}

func TestFormatTLASynthesizedLetsStayInsideApplicationScope(t *testing.T) {
	for _, named := range []bool{false, true} {
		for _, multipleDeclarations := range []bool{false, true} {
			caseName := "native"
			if named {
				caseName = "named"
			}
			if multipleDeclarations {
				caseName += " multiple"
			}
			t.Run(caseName, func(t *testing.T) {
				lambdaDeclaration := NewOperDecl("Lambda3",
					[]OperatorParam{ValueParam("p"), ValueParam("q")}, tlaName("p"), Untyped())
				lambda := NewLetInExpr(tlaName("Lambda3"), Untyped(), lambdaDeclaration)
				seed := Expr(NewValueExpr(BoolValue(false), Untyped()))
				if multipleDeclarations {
					seedDeclaration := NewOperDecl("Seed", nil, NewValueExpr(BoolValue(false), Untyped()), Untyped())
					seed = NewLetInExpr(tlaOper("OPER_APP", tlaName("Seed")), Untyped(), seedDeclaration)
				}
				call := func() Expr {
					arguments := []Expr{lambda, seed, tlaOper("TUPLE")}
					if named {
						return tlaOper("OPER_APP", append([]Expr{tlaName("Fold")}, arguments...)...)
					}
					return tlaOper("Apalache!ApaFoldSeqLeft", arguments...)
				}
				stepIsZero := func() Expr { return tlaOper("EQ", tlaName("step"), tlaInt(0)) }
				contexts := []struct {
					name string
					expr Expr
				}{
					{"top level", call()},
					{"issue 88", tlaOper("AND", tlaOper("EQ", tlaName("var0"), call()), stepIsZero())},
					{"conjunction", tlaOper("AND", call(), stepIsZero())},
					{"implication", tlaOper("IMPLIES", tlaOper("EQ", tlaName("var0"), call()), stepIsZero())},
					{"membership", tlaOper("AND", tlaOper("SET_IN", call(), tlaOper("SET_ENUM", NewValueExpr(BoolValue(false), Untyped()))), stepIsZero())},
					{"CASE OTHER", tlaOper("AND", tlaOper("CASE_OTHER", call(), NewValueExpr(BoolValue(true), Untyped()), NewValueExpr(BoolValue(false), Untyped())), stepIsZero())},
					{"IF branch", tlaOper("AND", tlaOper("IF_THEN_ELSE", NewValueExpr(BoolValue(true), Untyped()), call(), NewValueExpr(BoolValue(false), Untyped())), stepIsZero())},
				}
				callee := "ApaFoldSeqLeft"
				if named {
					callee = "Fold"
				}
				letPrefix := "(LET Lambda3(p, q) == p IN "
				if multipleDeclarations {
					letPrefix += "LET Seed == FALSE IN "
				}
				application := callee + "(Lambda3, "
				if multipleDeclarations {
					application += "Seed"
				} else {
					application += "FALSE"
				}
				application += ", <<>>))"
				for index, test := range contexts {
					t.Run(test.name, func(t *testing.T) {
						printed, err := FormatTLAExpression(test.expr)
						if err != nil {
							t.Fatal(err)
						}
						if !strings.Contains(printed, letPrefix+application) {
							t.Fatalf("application-local LET was not preserved: %q", printed)
						}
						module := "---- MODULE SynthesizedLet ----\n" +
							"EXTENDS Integers, Sequences, Apalache\n" +
							"VARIABLES var0, step\n" +
							"Fold(Op(_, _), initial, seq) == ApaFoldSeqLeft(Op, initial, seq)\n" +
							"Test == " + printed + "\n====\n"
						parsed := ParseSource(context.Background(), "SynthesizedLet"+string(rune('A'+index))+".tla", module, FrontendOptions{})
						if !parsed.OK() {
							t.Fatalf("formatted expression did not parse: %v\n%s", parsed.Diagnostics, module)
						}
					})
				}
			})
		}
	}
}

func TestFormatTLADoesNotHoistLetAcrossBinderScope(t *testing.T) {
	local := NewOperDecl("Local", nil, tlaName("x"), Untyped())
	body := NewLetInExpr(tlaName("Local"), Untyped(), local)
	expression := tlaOper("EXISTS3", tlaName("x"), tlaName("S"), body)
	printed, err := FormatTLAExpression(expression)
	if err != nil {
		t.Fatal(err)
	}
	if printed != "\\E x \\in S: LET Local == x IN Local" {
		t.Fatalf("got %q", printed)
	}
	module := "---- MODULE BinderLet ----\nVARIABLE S\nTest == " + printed + "\n====\n"
	parsed := ParseSource(context.Background(), "BinderLet.tla", module, FrontendOptions{})
	if !parsed.OK() {
		t.Fatalf("formatted expression did not parse: %v\n%s", parsed.Diagnostics, module)
	}
}

func TestFormatTLARecursiveFunctionDeclaration(t *testing.T) {
	call := tlaOper("FUN_APP", tlaOper("FUN_REC_REF"), tlaOper("MINUS", tlaName("n"), tlaInt(1)))
	body := tlaOper("IF_THEN_ELSE", tlaOper("EQ", tlaName("n"), tlaInt(0)), tlaInt(1), call)
	recursive := tlaOper("FUN_REC_CTOR", body, tlaName("n"), NewValueExpr(NatSet, Untyped()))
	declaration := NewOperDecl("Fact", nil, recursive, Untyped())

	got, err := FormatTLADeclaration(declaration, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "Fact[n \\in Nat] == IF n = 0 THEN 1 ELSE Fact[(n - 1)]"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestFormatTLAModuleIsAcceptedByFrontend(t *testing.T) {
	x := NewVarDecl("x", Untyped())
	s := NewConstDecl("S", Untyped())
	init := NewOperDecl("Init", nil, tlaOper("SET_IN", tlaName("x"), tlaName("S")), Untyped())
	next := NewOperDecl("Next", nil,
		tlaOper("SET_IN", tlaOper("PRIME", tlaName("x")), tlaName("S")), Untyped())
	spec := NewOperDecl("Spec", nil,
		tlaOper("AND", tlaName("Init"), tlaOper("STUTTER", tlaName("Next"), tlaName("x"))), Untyped())
	module := NewModule("Pretty", []Decl{s, x, init, next, spec})

	formatted, err := FormatTLAModule(module, TLAFormatOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result := ParseSource(context.Background(), "Pretty.tla", formatted, FrontendOptions{})
	if !result.OK() {
		t.Fatalf("frontend rejected formatted module:\n%s\n%+v", formatted, result.Diagnostics)
	}
	if result.Module != "Pretty" || !strings.Contains(formatted, "Spec == Init /\\ [Next]_x") {
		t.Fatalf("unexpected output:\n%s", formatted)
	}
}

func TestFormatTLAAnnotations(t *testing.T) {
	declaration := NewConstDecl("N", Untyped())
	store := NewAnnotationStore()
	store.Set(declaration.ID(), []Annotation{{Name: "type", Arguments: []AnnotationArg{AnnotationString{Text: "Int"}}}})
	got, err := FormatTLADeclaration(declaration, store)
	if err != nil {
		t.Fatal(err)
	}
	if got != "(*\n  @type: Int;\n*)\nCONSTANT N" {
		t.Fatalf("got %q", got)
	}
}
