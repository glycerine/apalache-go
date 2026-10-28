package apalache

import (
	"errors"
	"strings"
	"testing"
)

func TestRewriteOverridesConstantsAndReferences(t *testing.T) {
	constant := NewConstDecl("N", Typed(IntT1))
	override := NewOperDecl("OVERRIDE_N", nil, NewValueExpr(Int64Value(42), Typed(IntT1)), Typed(mustOperator(nil, IntT1)))
	use := NewOperDecl("Use", nil, MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("N", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1))), Typed(mustOperator(nil, IntT1)))
	otherTag := NewOperDecl("OtherTag", nil, NewNameExpr("N", Typed(StrT1)), Typed(mustOperator(nil, StrT1)))
	module := NewModule("M", []Decl{use, otherTag, override, constant})
	module.AddProperty(PropertyTypeChecked)
	got, err := RewriteOverrides(module, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyTypeChecked) || len(got.ConstDeclarations()) != 0 || len(got.OperDeclarations()) != 3 {
		t.Fatalf("rewritten declarations = %#v", got.Declarations())
	}
	var n, rewrittenUse, rewrittenOtherTag *OperDecl
	for _, declaration := range got.OperDeclarations() {
		if declaration.Name() == "N" {
			n = declaration
		}
		if declaration.Name() == "Use" {
			rewrittenUse = declaration
		}
		if declaration.Name() == "OtherTag" {
			rewrittenOtherTag = declaration
		}
		if strings.HasPrefix(declaration.Name(), OverridePrefix) {
			t.Fatal("override declaration was retained")
		}
	}
	if n == nil || !EqualTypedExpr(n.Body(), NewValueExpr(Int64Value(42), Typed(IntT1))) {
		t.Fatalf("N override = %#v", n)
	}
	call := rewrittenUse.Body().(*OperExpr).Arguments()[0].(*OperExpr)
	if call.Operator().Name() != "OPER_APP" || call.Arguments()[0].(*NameExpr).Name() != "N" {
		t.Fatalf("constant reference = %v", call)
	}
	if _, rewritten := rewrittenOtherTag.Body().(*OperExpr); rewritten || !rewrittenOtherTag.Body().(*NameExpr).Tag().Equal(Typed(StrT1)) {
		t.Fatalf("different-tag name was rewritten: %v", rewrittenOtherTag.Body())
	}
}

func TestRewriteOverridesOperatorsAndRecursion(t *testing.T) {
	typ := mustOperator([]Type{IntT1}, IntT1)
	original := NewOperDecl("F", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(typ))
	override := NewOperDecl("OVERRIDE_F", []OperatorParam{ValueParam("y")}, MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("y", Typed(IntT1)), NewValueExpr(Int64Value(1), Typed(IntT1))), Typed(typ))
	override.SetRecursive(true)
	got, err := RewriteOverrides(NewModule("M", []Decl{original, override}), nil)
	if err != nil {
		t.Fatal(err)
	}
	rewritten := got.OperDeclarations()[0]
	if rewritten.Name() != "F" || rewritten.FormalParams()[0].Name != "y" || !rewritten.IsRecursive() {
		t.Fatalf("operator override = %#v", rewritten)
	}
}

func TestRewriteOverridesRejectsInvalidTargets(t *testing.T) {
	cases := []struct {
		declaration Decl
		override    *OperDecl
	}{
		{NewConstDecl("C", Typed(IntT1)), NewOperDecl("OVERRIDE_C", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(mustOperator([]Type{IntT1}, IntT1)))},
		{NewConstDecl("C", Typed(IntT1)), NewOperDecl("OVERRIDE_C", nil, NewValueExpr(BoolValue(true), Typed(BoolT1)), Typed(mustOperator(nil, BoolT1)))},
		{NewVarDecl("v", Typed(IntT1)), NewOperDecl("OVERRIDE_v", nil, NewValueExpr(Int64Value(1), Typed(IntT1)), Typed(mustOperator(nil, IntT1)))},
		{NewOperDecl("F", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(mustOperator([]Type{IntT1}, IntT1))), NewOperDecl("OVERRIDE_F", nil, NewValueExpr(Int64Value(1), Typed(IntT1)), Typed(mustOperator(nil, IntT1)))},
	}
	for _, test := range cases {
		_, err := RewriteOverrides(NewModule("M", []Decl{test.declaration, test.override}), nil)
		var overrideError *OverrideError
		if !errors.As(err, &overrideError) {
			t.Errorf("invalid override error = %T %v", err, err)
		}
	}
}

func TestRewriteOverridesReportsCyclesAsConfigurationErrors(t *testing.T) {
	operatorType := mustOperator(nil, IntT1)
	original := NewOperDecl("F", nil, intLiteral(0), Typed(operatorType))
	override := NewOperDecl("OVERRIDE_F", nil, untypedApp("G"), Typed(operatorType))
	g := NewOperDecl("G", nil, untypedApp("F"), Typed(operatorType))
	_, err := RewriteOverrides(NewModule("M", []Decl{original, override, g}), nil)
	var configurationError *ConfigurationError
	if !errors.As(err, &configurationError) || !strings.Contains(err.Error(), "cyclic dependency") {
		t.Fatalf("cycle error = %T %v", err, err)
	}
}
