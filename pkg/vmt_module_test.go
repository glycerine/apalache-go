package apalache

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/glycerine/g3/pkg/cmd_context"
	"github.com/glycerine/g3/pkg/parsers/smt2"
)

func vmtBoolDefinition(name string, body Expr) *OperDecl {
	return NewOperDecl(name, nil, body, Typed(mustOperator(nil, BoolT1)))
}

// g3 v0.0.8 intentionally diagnoses unknown expression attributes, whereas
// VMT communicates its transition-system metadata through such attributes.
// Remove only the outer VMT annotation wrappers to validate the underlying
// SMT-LIB declarations and terms with the pinned solver frontend.
func stripVMTAttributes(document string) string {
	for {
		start := strings.Index(document, "(! ")
		if start < 0 {
			return document
		}
		depth, attribute, end := 1, -1, -1
		for index := start + 3; index < len(document); index++ {
			switch document[index] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					end = index
				}
			case ':':
				if depth == 1 && index > start+3 && document[index-1] == ' ' && attribute < 0 {
					attribute = index - 1
				}
			}
			if end >= 0 {
				break
			}
		}
		if attribute < 0 || end < 0 {
			return document
		}
		term := document[start+3 : attribute]
		document = document[:start] + term + document[end+1:]
	}
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
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := context.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if err = smt2.ParseCommands(context, strings.NewReader(stripVMTAttributes(first)), io.Discard); err != nil {
		t.Fatalf("g3 rejected generated VMT: %v\n%s", err, first)
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

func TestFormatVMTModuleChecksReTLALanguageBeforeLowering(t *testing.T) {
	bad := MustOperExpr(Op("PLUS"), Typed(IntT1),
		NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(Int64Value(2), Typed(IntT1)))
	module := NewModule("Bad", []Decl{vmtBoolDefinition("Init$0000", bad)})
	_, err := FormatVMTModule(module)
	if err == nil || !strings.Contains(err.Error(), "checking reTLA language of Init$0000") {
		t.Fatalf("unsupported reTLA expression error = %v", err)
	}
}

func TestVMTDeclarationsKeepEqualLiteralIndicesAcrossSorts(t *testing.T) {
	a, _ := NewConstType("A")
	b, _ := NewConstType("B")
	rewriter, err := NewVMTRewriter(nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, expression := range []Expr{
		NewValueExpr(StringValue("same_OF_A"), Typed(a)),
		NewValueExpr(StringValue("same_OF_B"), Typed(b)),
	} {
		if _, err = rewriter.Rewrite(expression); err != nil {
			t.Fatal(err)
		}
	}
	declarations := rewriter.Declarations().UninterpretedLiterals
	if len(declarations) != 2 || declarations["same_A"].Sort.Name != "A" || declarations["same_B"].Sort.Name != "B" {
		t.Fatalf("cross-sort declarations = %#v", declarations)
	}
}

func TestFormatVMTModuleKeepsGeneratedDefinitionsInDependencyOrder(t *testing.T) {
	functionType := mustFun(IntT1, IntT1)
	binder := NewNameExpr("x", Typed(IntT1))
	domain := NewValueExpr(IntSet, Typed(mustSet(IntT1)))
	body := Expr(NewValueExpr(Int64Value(0), Typed(IntT1)))
	for range 36 {
		function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, binder, domain)
		body = MustOperExpr(Op("FUN_APP"), Typed(IntT1), function, binder)
	}
	outer := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), body, binder, domain)
	f := NewNameExpr("f", Typed(functionType))
	next := MustOperExpr(Op("Apalache!:="), Typed(BoolT1),
		MustOperExpr(Op("PRIME"), Typed(functionType), f), outer)
	module := NewModule("DefinitionOrder", []Decl{
		NewVarDecl("f", Typed(functionType)), vmtBoolDefinition("Next$0000", next),
	})
	document, err := FormatVMTModule(module)
	if err != nil {
		t.Fatal(err)
	}
	previous := strings.Index(document, "(define-fun t_1 ")
	if previous < 0 {
		t.Fatalf("first generated definition is missing:\n%s", document)
	}
	for number := 2; number <= 37; number++ {
		marker := fmt.Sprintf("(define-fun t_%s ", strconv.FormatInt(int64(number), 36))
		position := strings.Index(document, marker)
		if position <= previous {
			t.Fatalf("definition %s is not after its dependency:\n%s", marker, document)
		}
		previous = position
	}
	context, err := cmd_context.NewStandaloneContext()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if closeErr := context.Close(); closeErr != nil {
			t.Error(closeErr)
		}
	})
	if err = smt2.ParseCommands(context, strings.NewReader(stripVMTAttributes(document)), io.Discard); err != nil {
		t.Fatalf("g3 rejected dependency-ordered VMT: %v\n%s", err, document)
	}
}
