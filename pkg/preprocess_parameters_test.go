package apalache

import (
	"strings"
	"testing"
)

func TestParameterNormalizerNullaryIsStructuralNoOp(t *testing.T) {
	typ := mustOperator(nil, IntT1)
	declaration := NewOperDecl("A", nil, intLiteral(1), Typed(typ))
	normalized, err := NewParameterNormalizer(nil, nil, NormalizeAllParameters).Declaration(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if normalized == declaration || !EqualTypedDecl(normalized, declaration) || normalized.IsRecursive() != declaration.IsRecursive() {
		t.Fatalf("nullary normalization = %#v", normalized)
	}
}

func TestParameterNormalizerValueParameter(t *testing.T) {
	operatorType := mustOperator([]Type{IntT1}, IntT1)
	body := MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1)))
	declaration := NewOperDecl("Inc", []OperatorParam{ValueParam("x")}, body, Typed(operatorType))
	normalized, err := NewParameterNormalizer(nil, nil, NormalizeAllParameters).Declaration(declaration)
	if err != nil {
		t.Fatal(err)
	}
	let, ok := normalized.Body().(*LetInExpr)
	if !ok || len(let.Declarations()) != 1 {
		t.Fatalf("normalized body = %T %v", normalized.Body(), normalized.Body())
	}
	definition := let.Declarations()[0]
	if definition.Name() != "t_1" || len(definition.FormalParams()) != 0 {
		t.Fatalf("definition = %#v", definition)
	}
	plus := let.Body().(*OperExpr)
	application := plus.Arguments()[0].(*OperExpr)
	if application.Operator().Name() != "OPER_APP" || application.Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("replacement = %v", application)
	}
	if definition.Body().(*NameExpr).Name() != "x" {
		t.Fatalf("definition body = %v", definition.Body())
	}
}

func TestParameterNormalizerHigherOrderParameterAndRecursion(t *testing.T) {
	parameterType := mustOperator([]Type{IntT1, BoolT1}, StrT1)
	declarationType := mustOperator([]Type{parameterType}, StrT1)
	body := MustOperExpr(Op("OPER_APP"), Typed(StrT1), NewNameExpr("f", Typed(parameterType)), NewValueExpr(Int64Value(1), Typed(IntT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	declaration := NewOperDecl("Apply", []OperatorParam{{Name: "f", Arity: 2}}, body, Typed(declarationType))
	declaration.SetRecursive(true)
	normalized, err := NewParameterNormalizer(nil, nil, nil).Declaration(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if !normalized.IsRecursive() {
		t.Fatal("recursive flag was lost")
	}
	let := normalized.Body().(*LetInExpr)
	definition := let.Declarations()[0]
	if definition.Name() != "t_1" || len(definition.FormalParams()) != 2 || definition.FormalParams()[0].Name != "t_2" || definition.FormalParams()[1].Name != "t_3" {
		t.Fatalf("higher-order definition = %#v", definition)
	}
	definitionApplication := definition.Body().(*OperExpr)
	if definitionApplication.Arguments()[0].(*NameExpr).Name() != "f" {
		t.Fatalf("definition application = %v", definitionApplication)
	}
	newApplication := let.Body().(*OperExpr)
	if newApplication.Arguments()[0].(*NameExpr).Name() != "t_1" {
		t.Fatalf("body application = %v", newApplication)
	}
}

func TestParameterNormalizerRecursesIntoLocalDeclarations(t *testing.T) {
	localType := mustOperator([]Type{IntT1}, IntT1)
	local := NewOperDecl("Local", []OperatorParam{ValueParam("y")}, NewNameExpr("y", Typed(IntT1)), Typed(localType))
	outerType := mustOperator(nil, IntT1)
	outer := NewOperDecl("Outer", nil, NewLetInExpr(NewValueExpr(Int64Value(1), Typed(IntT1)), Typed(IntT1), local), Typed(outerType))
	normalized, err := NewParameterNormalizer(nil, nil, NormalizeAllParameters).Declaration(outer)
	if err != nil {
		t.Fatal(err)
	}
	localBody := normalized.Body().(*LetInExpr).Declarations()[0].Body()
	if _, ok := localBody.(*LetInExpr); !ok {
		t.Fatalf("local declaration was not normalized: %v", localBody)
	}
}

func TestParameterNormalizerRejectsBadSignatures(t *testing.T) {
	bad := NewOperDecl("Bad", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(mustOperator(nil, IntT1)))
	_, err := NewParameterNormalizer(nil, nil, NormalizeAllParameters).Declaration(bad)
	if err == nil || !strings.Contains(err.Error(), "has 1 parameters") {
		t.Fatalf("signature error = %v", err)
	}
	badHigherOrder := NewOperDecl("BadHO", []OperatorParam{{Name: "f", Arity: 1}}, NewNameExpr("f", Typed(IntT1)), Typed(mustOperator([]Type{IntT1}, IntT1)))
	_, err = NewParameterNormalizer(nil, nil, NormalizeAllParameters).Declaration(badHigherOrder)
	if err == nil || !strings.Contains(err.Error(), "higher-order") {
		t.Fatalf("higher-order error = %v", err)
	}
}

func TestParameterNormalizerDefaultOnlyNormalizesRecursiveDeclarations(t *testing.T) {
	typ := mustOperator([]Type{IntT1}, IntT1)
	declaration := NewOperDecl("Identity", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(typ))
	normalized, err := NewParameterNormalizer(nil, nil, nil).Declaration(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if _, changed := normalized.Body().(*LetInExpr); changed {
		t.Fatal("normalized a nonrecursive declaration by default")
	}
}

func TestParameterNormalizerTracksIntroducedLet(t *testing.T) {
	typ := mustOperator([]Type{IntT1}, IntT1)
	body := NewNameExpr("x", Typed(IntT1))
	declaration := NewOperDecl("Identity", []OperatorParam{ValueParam("x")}, body, Typed(typ))
	store := NewSourceStore()
	location := SourceLocation{Filename: "M", Region: NewSourceRegion(2, 1, 2, 20)}
	if err := store.Add(body.ID(), location); err != nil {
		t.Fatal(err)
	}
	normalized, err := NewParameterNormalizer(nil, NewListenerTracker(store), NormalizeAllParameters).Declaration(declaration)
	if err != nil {
		t.Fatal(err)
	}
	if !store.Contains(normalized.Body().ID()) {
		t.Fatal("introduced LET lost the original body source")
	}
}

func TestParameterNormalizerModuleMatchesScalaIdentityAndProperties(t *testing.T) {
	operator := NewOperDecl("A", nil, intLiteral(1), Typed(mustOperator(nil, IntT1)))
	variable := NewVarDecl("x", Typed(IntT1))
	module := NewModule("M", []Decl{variable, operator})
	module.AddProperty(PropertyTypeChecked)
	normalized, err := NewParameterNormalizer(nil, nil, NormalizeAllParameters).Module(module)
	if err != nil {
		t.Fatal(err)
	}
	if normalized == module || normalized.VarDeclarations()[0] != variable ||
		normalized.OperDeclarations()[0] == operator || normalized.HasProperty(PropertyTypeChecked) {
		t.Fatalf("normalized module identity/properties = %#v", normalized)
	}
}
