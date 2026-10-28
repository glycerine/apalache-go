package apalache

import (
	"strings"
	"testing"
)

func TestPreprocessReTLAModuleExactSequenceAndContracts(t *testing.T) {
	x := NewVarDecl("x", Typed(IntT1))
	unchanged := tlaOper("UNCHANGED", tlaName("x"))
	next := NewOperDecl("Next", nil, unchanged, Untyped())
	module := NewModule("M", []Decl{x, next})
	module.AddProperty(PropertyInlined)

	got, err := PreprocessReTLAModule(module, PreprocessOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyPreprocessed) || module.HasProperty(PropertyPreprocessed) {
		t.Fatal("Preprocessed property was not added exclusively to the output")
	}
	body, found := OperatorBody(got, "Next")
	if !found || countOperator(body, "UNCHANGED") != 0 || countOperator(body, "PRIME") != 1 || countOperator(body, "EQ") != 1 {
		t.Fatalf("Next after ReTLA preprocessing = %v", body)
	}
	if result := (ReTLALanguagePredicate{}).CheckModule(got); !result.OK() {
		t.Fatalf("output is outside ReTLA: %+v", result.Failures)
	}
}

func TestPreprocessReTLAModuleExcludesPrimedConstantInitializer(t *testing.T) {
	x := NewVarDecl("x", Typed(IntT1))
	// PRIME over an equality would become an accepted equality of primed names
	// if propagation ran. The configured CInitPrimed exclusion leaves it intact.
	primed := tlaOper("PRIME", tlaOper("EQ", tlaName("x"), tlaName("x")))
	cinit := NewOperDecl("BootPrimed", nil, primed, Untyped())
	module := NewModule("M", []Decl{x, cinit})
	module.AddProperty(PropertyInlined)

	_, err := PreprocessReTLAModule(module, PreprocessOptions{ConstantInitName: "Boot"})
	if err == nil || !strings.Contains(err.Error(), "ReTLALanguagePred") {
		t.Fatalf("excluded initializer language check error = %v", err)
	}
}

func TestPreprocessReTLAModuleRejectsInvalidInputsAndLanguage(t *testing.T) {
	if _, err := PreprocessReTLAModule(nil, PreprocessOptions{}); err == nil {
		t.Fatal("nil module accepted")
	}
	if _, err := PreprocessReTLAModule(NewModule("M", nil), PreprocessOptions{}); err == nil {
		t.Fatal("module without Inlined accepted")
	}
	bad := NewModule("M", []Decl{NewOperDecl("Bad", nil, tlaOper("PLUS", tlaInt(1), tlaInt(2)), Untyped())})
	bad.AddProperty(PropertyInlined)
	if _, err := PreprocessReTLAModule(bad, PreprocessOptions{}); err == nil || !strings.Contains(err.Error(), "ReTLALanguagePred") {
		t.Fatalf("unsupported ReTLA expression error = %v", err)
	}
}
