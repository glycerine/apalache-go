package apalache

import (
	"strings"
	"testing"
)

func TestPreprocessModuleRunsProductionOrderAndProperties(t *testing.T) {
	x := NewNameExpr("x", Typed(IntT1))
	body := MustOperExpr(Op("PRIME"), Typed(IntT1), MustOperExpr(Op("PLUS"), Typed(IntT1), x, intLiteral(1)))
	step := NewOperDecl("Step", nil, body, Typed(mustOperator(nil, IntT1)))
	constantInit := NewOperDecl("CInitPrimed", nil, body, Typed(mustOperator(nil, IntT1)))
	module := NewModule("M", []Decl{NewVarDecl("x", Typed(IntT1)), step, constantInit})
	module.AddProperty(PropertyTypeChecked)
	module.AddProperty(PropertyInlined)

	got, err := PreprocessModule(module, PreprocessOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyTypeChecked) || !got.HasProperty(PropertyInlined) || !got.HasProperty(PropertyPreprocessed) {
		t.Fatalf("preprocessing properties were not preserved and added")
	}
	var rewrittenStep, untouchedConstantInit *OperDecl
	for _, declaration := range got.OperDeclarations() {
		switch declaration.Name() {
		case "Step":
			rewrittenStep = declaration
		case "CInitPrimed":
			untouchedConstantInit = declaration
		}
	}
	plus := rewrittenStep.Body().(*OperExpr)
	if plus.Operator().Name() != "PLUS" || plus.Arguments()[0].(*OperExpr).Operator().Name() != "PRIME" {
		t.Fatalf("prime was not propagated to the state variable: %v", plus)
	}
	if untouchedConstantInit.Body().(*OperExpr).Operator().Name() != "PRIME" {
		t.Fatalf("constant-init declaration unexpectedly received prime propagation: %v", untouchedConstantInit.Body())
	}
	if module.HasProperty(PropertyPreprocessed) {
		t.Fatal("preprocessing mutated the input module")
	}
}

func TestOptimizeModuleRunsProductionSequenceOnAllBodyDeclarations(t *testing.T) {
	intSet := mustSet(IntT1)
	set := NewNameExpr("S", Typed(intSet))
	cardinalityZero := MustOperExpr(Op("EQ"), Typed(BoolT1),
		MustOperExpr(Op("FiniteSets!Cardinality"), Typed(IntT1), set), intLiteral(0))
	module := NewModule("M", []Decl{
		NewOperDecl("P", nil, cardinalityZero, Typed(mustOperator(nil, BoolT1))),
		NewNamedAssumeDecl("A", cardinalityZero, Typed(BoolT1)),
		NewTheoremDecl("T", cardinalityZero, Typed(BoolT1)),
	})
	module.AddProperty(PropertyPreprocessed)

	got, err := OptimizeModule(module, PreprocessOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyPreprocessed) || !got.HasProperty(PropertyOptimized) {
		t.Fatal("optimization properties were not preserved and added")
	}
	for _, declaration := range got.Declarations() {
		var body Expr
		switch typed := declaration.(type) {
		case *OperDecl:
			body = typed.Body()
		case *AssumeDecl:
			body = typed.Body()
		case *TheoremDecl:
			body = typed.Body()
		}
		equality := body.(*OperExpr)
		if equality.Operator().Name() != "EQ" || equality.Arguments()[1].(*OperExpr).Operator().Name() != "SET_ENUM" {
			t.Errorf("%s was not optimized: %v", declaration.Name(), body)
		}
	}
}

func TestPreprocessingPipelineChecksDependenciesAndQualifiesErrors(t *testing.T) {
	module := NewModule("M", nil)
	if _, err := PreprocessModule(module, PreprocessOptions{}); err == nil || !strings.Contains(err.Error(), "inlined") {
		t.Fatalf("missing preprocessing dependency error = %v", err)
	}
	if _, err := OptimizeModule(module, PreprocessOptions{}); err == nil || !strings.Contains(err.Error(), "preprocessed") {
		t.Fatalf("missing optimization dependency error = %v", err)
	}

	bad := NewModule("M", []Decl{NewOperDecl("Bad", nil,
		MustOperExpr(Op("FUN_APP"), Typed(IntT1),
			MustOperExpr(Op("RECORD"), Typed(mustSet(IntT1)), s("a"), intLiteral(1)), s("missing")),
		Typed(mustOperator(nil, IntT1)))})
	bad.AddProperty(PropertyPreprocessed)
	if _, err := OptimizeModule(bad, PreprocessOptions{}); err == nil || !strings.Contains(err.Error(), "expression optimization in declaration Bad") {
		t.Fatalf("stage-qualified error = %v", err)
	}
}
