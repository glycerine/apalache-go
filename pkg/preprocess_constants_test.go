package apalache

import (
	"errors"
	"strings"
	"testing"
)

func TestTLAConstantInlinerBuildsMapAndReplaces(t *testing.T) {
	inliner := NewTLAConstantInliner(nil, []string{"N", "Flag"})
	eq := func(name string, value Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr(name, value.Tag()), value)
	}
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	truth := NewValueExpr(BoolValue(true), Typed(BoolT1))
	init := MustOperExpr(Op("AND"), Typed(BoolT1), eq("N", one), eq("Flag", truth), eq("Ignored", one))
	values, err := inliner.BuildConstantMap(nil, init)
	if err != nil || len(values) != 2 {
		t.Fatalf("constant map = %#v, %v", values, err)
	}
	expression := MustOperExpr(Op("IF_THEN_ELSE"), Typed(IntT1), NewNameExpr("Flag", Typed(BoolT1)), NewNameExpr("N", Typed(IntT1)), NewNameExpr("Other", Typed(IntT1)))
	got := inliner.Replace(expression, values).(*OperExpr).Arguments()
	if !EqualTypedExpr(got[0], truth) || !EqualTypedExpr(got[1], one) {
		t.Fatalf("replacement = %v", got)
	}
	if got[0].ID() == truth.ID() || got[1].ID() == one.ID() {
		t.Fatal("replacement reused configured value identity")
	}
	if got[2].(*NameExpr).Name() != "Other" {
		t.Fatalf("unconfigured name changed: %v", got[2])
	}
}

func TestTLAConstantInlinerTraversesLetsAndRejectsConflicts(t *testing.T) {
	inliner := NewTLAConstantInliner(nil, []string{"N"})
	one := NewValueExpr(Int64Value(1), Typed(IntT1))
	two := NewValueExpr(Int64Value(2), Typed(IntT1))
	eq := func(value Expr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr("N", Typed(IntT1)), value)
	}
	local := NewOperDecl("Local", nil, eq(one), Typed(mustOperator(nil, BoolT1)))
	root := NewLetInExpr(eq(one), Typed(BoolT1), local)
	values, err := inliner.BuildConstantMap(nil, root)
	if err != nil || !EqualTypedExpr(values["N"], one) {
		t.Fatalf("LET constant map = %#v, %v", values, err)
	}
	duplicate, err := inliner.BuildConstantMap(values, eq(one))
	if err != nil || !EqualTypedExpr(duplicate["N"], one) {
		t.Fatalf("identical duplicate initialization = %#v, %v", duplicate, err)
	}
	_, err = inliner.BuildConstantMap(values, eq(two))
	if err == nil || !strings.Contains(err.Error(), "multiple initializations") {
		t.Fatalf("conflict error = %v", err)
	}
	var inputError *TlaInputError
	if !errors.As(err, &inputError) {
		t.Fatalf("conflict error type = %T, want *TlaInputError", err)
	}

	replaced := inliner.Replace(root, values).(*LetInExpr)
	if _, ok := replaced.Body().(*OperExpr).Arguments()[0].(*ValueExpr); !ok {
		t.Fatalf("LET body constant was not replaced: %v", replaced.Body())
	}
	if _, ok := replaced.Declarations()[0].Body().(*OperExpr).Arguments()[0].(*ValueExpr); !ok {
		t.Fatalf("LET declaration constant was not replaced: %v", replaced.Declarations()[0].Body())
	}
}
