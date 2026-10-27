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
		{"map set", tlaOper("SET_MAP", tlaName("x"), tlaName("x"), tlaName("S")), "{(x): x \\in S}"},
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
