package apalache

import (
	"errors"
	"strings"
	"testing"
)

func temporalFormulaModule(operator string) (*Module, *OperDecl) {
	module := temporalTestModule()
	x := NewNameExpr("x", Typed(IntT1))
	atom := MustOperExpr(Op("EQ"), Typed(BoolT1), x, x)
	formula := boolDefinition("Live", MustOperExpr(Op(operator), Typed(BoolT1), atom))
	declarations := append(module.Declarations(), formula)
	result := NewModule(module.Name(), declarations)
	result.copyMetadataFrom(module)
	return result, formula
}

func TestTemporalTableauEncodesGloballySyntaxTree(t *testing.T) {
	module, formula := temporalFormulaModule("GLOBALLY")
	loopEncoder := NewTemporalLoopEncoder(nil)
	loop, err := loopEncoder.AddLoopLogic(module, "Init", "Next")
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := NewTemporalTableauEncoder(loop.Module, NewUniqueNameGenerator(), loopEncoder, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encoder.EncodeSyntaxTree(formula.Body())
	if err != nil {
		t.Fatal(err)
	}
	if len(encoded.Variables) != 4 || encoded.Variables[0].Name() != "__temporal_t_1" ||
		encoded.Variables[1].Name() != "__saved___temporal_t_1" ||
		encoded.Variables[2].Name() != "__temporal_t_1_unroll" ||
		encoded.Variables[3].Name() != "__temporal_t_1_unroll_prev" {
		t.Fatalf("tableau variables = %v", declarationNameSlice(encoded.Variables))
	}
	if len(encoded.Predicates.Init) != 4 || len(encoded.Predicates.Next) != 6 || len(encoded.Predicates.LoopOK) != 3 {
		t.Fatalf("tableau predicate sizes = init %d next %d loop %d", len(encoded.Predicates.Init), len(encoded.Predicates.Next), len(encoded.Predicates.LoopOK))
	}
	if countOperator(temporalAnd(encoded.Predicates.Next...), "EQUIV") != 2 ||
		countOperator(temporalAnd(encoded.Predicates.Next...), "GLOBALLY") != 0 {
		t.Fatalf("globally recurrence = %v", encoded.Predicates.Next)
	}
}

func TestTemporalTableauReplacesPropertiesAndPreprocesses(t *testing.T) {
	module, formula := temporalFormulaModule("EVENTUALLY")
	loopEncoder := NewTemporalLoopEncoder(nil)
	loop, err := loopEncoder.AddLoopLogic(module, "Init", "Next")
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := NewTemporalTableauEncoder(loop.Module, NewUniqueNameGenerator(), loopEncoder, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encoder.TemporalsToInvariants(loop, formula)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatTLAExpression(formula.Body())
	if err != nil {
		t.Fatal(err)
	}
	replacements := encoded.NameReplacements()
	if replacements["__temporal_t_1"] != formatted ||
		replacements["__saved___temporal_t_1"] != TemporalSavedPrefix+formatted {
		t.Fatalf("tableau name replacements = %#v, want expression %q", replacements, formatted)
	}
	replacements["__temporal_t_1"] = "mutated"
	if encoded.NameReplacements()["__temporal_t_1"] != formatted {
		t.Fatal("module exposed name replacements mutably")
	}
	property, found := declarationNamed(encoded, "Live")
	if !found {
		t.Fatal("encoded property is missing")
	}
	body := property.(*OperDecl).Body().(*OperExpr)
	if body.Operator().Name() != "IMPLIES" || countOperator(body, "EVENTUALLY") != 0 ||
		countOperator(body, "OPER_APP") != 1 {
		t.Fatalf("encoded property = %v", body)
	}
	if _, found = declarationNamed(encoded, "__Live_init"); !found {
		t.Fatal("initial formula value variable is missing")
	}
	desugared, err := DesugarModule(encoded, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	inlined, err := InlineConfiguredModule(desugared, InlinePassOptions{
		Init: "Init", Next: "Next", Relevant: []string{"Live"},
	})
	if err != nil {
		t.Fatal(err)
	}
	preprocessed, err := PreprocessModule(inlined, PreprocessOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got := preprocessed.NameReplacements()["__temporal_t_1"]; got != formatted {
		t.Fatalf("preprocessing lost tableau name replacement: %q", got)
	}
	for _, declaration := range preprocessed.OperDeclarations() {
		if countOperator(declaration.Body(), "GLOBALLY") != 0 || countOperator(declaration.Body(), "EVENTUALLY") != 0 ||
			countOperator(declaration.Body(), "UNCHANGED") != 0 {
			t.Fatalf("temporal operator survived preprocessing in %s", declaration.Name())
		}
	}
}

func TestTemporalTableauRejectsUnsupportedTemporalOperators(t *testing.T) {
	module, _ := temporalFormulaModule("GLOBALLY")
	loopEncoder := NewTemporalLoopEncoder(nil)
	loop, err := loopEncoder.AddLoopLogic(module, "Init", "Next")
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := NewTemporalTableauEncoder(loop.Module, nil, loopEncoder, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, operator := range []string{"LEADS_TO", "GUARANTEES", "WEAK_FAIRNESS", "TEMPORAL_EXISTS"} {
		arguments := []Expr{temporalBool(true), temporalBool(false)}
		if operator == "GUARANTEES" || operator == "LEADS_TO" {
			// two arguments
		} else if operator == "WEAK_FAIRNESS" || operator == "TEMPORAL_EXISTS" {
			// both also have two arguments in the IR
		}
		_, encodeErr := encoder.EncodeSyntaxTree(MustOperExpr(Op(operator), Typed(BoolT1), arguments...))
		if encodeErr == nil || !strings.Contains(encodeErr.Error(), "not supported") && operator != "LEADS_TO" {
			t.Errorf("%s error = %v", operator, encodeErr)
		}
		if operator == "LEADS_TO" {
			var language *LanguagePredError
			if !errors.As(encodeErr, &language) || len(language.FailedIDs()) != 1 {
				t.Errorf("LEADS_TO error type = %T, failures = %#v", encodeErr, language)
			}
		}
	}
}

func TestTemporalTableauNestedAndMultipleFormulas(t *testing.T) {
	module := temporalTestModule()
	x := NewNameExpr("x", Typed(IntT1))
	atom := MustOperExpr(Op("EQ"), Typed(BoolT1), x, intLiteral(0))
	global := MustOperExpr(Op("GLOBALLY"), Typed(BoolT1), atom)
	eventual := MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1), atom)
	first := boolDefinition("Both", MustOperExpr(Op("AND"), Typed(BoolT1), global, eventual))
	second := boolDefinition("Eventually", eventual)
	module = NewModule(module.Name(), append(module.Declarations(), first, second))
	module.AddProperty(PropertyTypeChecked)
	loopEncoder := NewTemporalLoopEncoder(nil)
	loop, err := loopEncoder.AddLoopLogic(module, "Init", "Next")
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := NewTemporalTableauEncoder(loop.Module, nil, loopEncoder, nil)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := encoder.TemporalsToInvariants(loop, first, second)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Both", "Eventually", "__Both_init", "__Eventually_init"} {
		if _, found := declarationNamed(encoded, name); !found {
			t.Errorf("encoded declaration %s is missing", name)
		}
	}
	for _, declaration := range encoded.OperDeclarations() {
		if countOperator(declaration.Body(), "GLOBALLY") != 0 || countOperator(declaration.Body(), "EVENTUALLY") != 0 {
			t.Fatalf("temporal operator survived nested tableau encoding in %s", declaration.Name())
		}
	}
	if len(encoded.NameReplacements()) != 4*2 {
		t.Fatalf("nested tableau name replacements = %#v", encoded.NameReplacements())
	}
}

func TestTemporalTableauPassesThroughNonTemporalExpressions(t *testing.T) {
	module := temporalTestModule()
	loopEncoder := NewTemporalLoopEncoder(nil)
	encoder, err := NewTemporalTableauEncoder(module, nil, loopEncoder, nil)
	if err != nil {
		t.Fatal(err)
	}
	state := MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("x", Typed(IntT1)), intLiteral(0))
	encoded, err := encoder.EncodeSyntaxTree(state)
	if err != nil || encoded.Expression != state || len(encoded.Variables) != 0 ||
		len(encoded.Predicates.Init)+len(encoded.Predicates.Next)+len(encoded.Predicates.LoopOK) != 0 {
		t.Fatalf("non-temporal encoding = %#v, %v", encoded, err)
	}
}

func TestTemporalTableauClassifiesUnexpectedTemporalLet(t *testing.T) {
	module := temporalTestModule()
	loopEncoder := NewTemporalLoopEncoder(nil)
	encoder, err := NewTemporalTableauEncoder(module, nil, loopEncoder, nil)
	if err != nil {
		t.Fatal(err)
	}
	temporal := MustOperExpr(Op("EVENTUALLY"), Typed(BoolT1), temporalBool(true))
	let := NewLetInExpr(temporal, Typed(BoolT1))
	_, err = encoder.EncodeSyntaxTree(let)
	var irrecoverable *IrrecoverablePreprocessingError
	if !errors.As(err, &irrecoverable) || !strings.Contains(err.Error(), "LET-IN") {
		t.Fatalf("unexpected temporal LET error = %T: %v", err, err)
	}
}

func TestTemporalTableauRewritesAssignmentsAsEquality(t *testing.T) {
	assignment := MustOperExpr(Op("Apalache!"+":="), Typed(BoolT1),
		NewNameExpr("x", Typed(IntT1)), intLiteral(1))
	rewritten := rewriteTemporalAssignments(assignment).(*OperExpr)
	if rewritten.Operator().Name() != "EQ" {
		t.Fatalf("assignment rewrite = %v", rewritten)
	}
}
