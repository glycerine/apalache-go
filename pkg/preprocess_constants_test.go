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

func TestTLAConstantInlinerScalaReplacementMatrix(t *testing.T) {
	inliner := NewTLAConstantInliner(nil, []string{"Primitive", "NonPrimitive"})
	primitive := NewNameExpr("Primitive", Typed(IntT1))
	nonPrimitive := NewNameExpr("NonPrimitive", Typed(IntT1))
	nonConstant := NewNameExpr("x", Typed(IntT1))
	value := NewValueExpr(Int64Value(42), Typed(IntT1))
	values := map[string]*ValueExpr{
		"Primitive": value,
		"x":         value,
	}
	plus := func(left, right Expr) Expr {
		return MustOperExpr(Op("PLUS"), Typed(IntT1), left, right)
	}
	minus := func(left, right Expr) Expr {
		return MustOperExpr(Op("MINUS"), Typed(IntT1), left, right)
	}
	nested := func(replaceable bool) Expr {
		left := Expr(nonConstant)
		if replaceable {
			left = primitive
		}
		return plus(nonConstant, minus(left, nonPrimitive))
	}

	for _, test := range []struct {
		name string
		in   Expr
		want Expr
	}{
		{"mapped constant", primitive, value},
		{"unmapped constant", nonPrimitive, nonPrimitive},
		{"mapped non-constant", nonConstant, nonConstant},
		{"nested replacement", nested(true), plus(nonConstant, minus(value, nonPrimitive))},
		{"nested unchanged", nested(false), nested(false)},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := inliner.Replace(test.in, values)
			if !EqualTypedExpr(got, test.want) {
				t.Fatalf("replacement = %v, want %v", got, test.want)
			}
		})
	}

	for _, test := range []struct {
		name               string
		replaceBody        bool
		replaceDeclaration bool
	}{
		{"body and declaration", true, true},
		{"body only", true, false},
		{"declaration only", false, true},
		{"neither", false, false},
	} {
		t.Run("LET "+test.name, func(t *testing.T) {
			body := nested(test.replaceBody)
			declarationBody := nested(test.replaceDeclaration)
			local := NewOperDecl("A", nil, declarationBody, Typed(mustOperator(nil, IntT1)))
			input := NewLetInExpr(body, Typed(IntT1), local)
			got := inliner.Replace(input, values)

			wantBody := nested(false)
			if test.replaceBody {
				wantBody = plus(nonConstant, minus(value, nonPrimitive))
			}
			wantDeclarationBody := nested(false)
			if test.replaceDeclaration {
				wantDeclarationBody = plus(nonConstant, minus(value, nonPrimitive))
			}
			wantLocal := NewOperDecl("A", nil, wantDeclarationBody, Typed(mustOperator(nil, IntT1)))
			want := NewLetInExpr(wantBody, Typed(IntT1), wantLocal)
			if !EqualTypedExpr(got, want) {
				t.Fatalf("replacement = %v, want %v", got, want)
			}
		})
	}
}

func TestTLAConstantInlinerScalaExtractionMatrix(t *testing.T) {
	inliner := NewTLAConstantInliner(nil, []string{"Primitive", "NonPrimitive"})
	value := NewValueExpr(Int64Value(42), Typed(IntT1))
	other := NewValueExpr(Int64Value(5), Typed(IntT1))
	equality := func(name string, literal *ValueExpr) Expr {
		return MustOperExpr(Op("EQ"), Typed(BoolT1), NewNameExpr(name, Typed(IntT1)), literal)
	}

	for _, test := range []struct {
		name       string
		expression Expr
		want       map[string]*ValueExpr
	}{
		{"non-module constant", equality("x", value), nil},
		{"module constant", equality("Primitive", value), map[string]*ValueExpr{"Primitive": value}},
		{"nested equalities", MustOperExpr(Op("AND"), Typed(BoolT1), equality("Primitive", value), equality("x", other)), map[string]*ValueExpr{"Primitive": value}},
		{"LET body", NewLetInExpr(equality("Primitive", value), Typed(BoolT1), NewOperDecl("A", nil, equality("x", other), Typed(mustOperator(nil, BoolT1)))), map[string]*ValueExpr{"Primitive": value}},
		{"name", NewNameExpr("Primitive", Typed(IntT1)), nil},
		{"value", value, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := inliner.BuildConstantMap(nil, test.expression)
			if err != nil || len(got) != len(test.want) {
				t.Fatalf("constant map = %#v, want %#v, error = %v", got, test.want, err)
			}
			for name, want := range test.want {
				if !EqualTypedExpr(got[name], want) {
					t.Fatalf("constant %s = %v, want %v", name, got[name], want)
				}
			}
		})
	}
}
