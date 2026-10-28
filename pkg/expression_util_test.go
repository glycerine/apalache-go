package apalache

import (
	"reflect"
	"sort"
	"strings"
	"testing"
)

func sortedUsedNames(t *testing.T, expression Expr) []string {
	t.Helper()
	used, err := FindUsedNames(expression)
	if err != nil {
		t.Fatal(err)
	}
	names := make([]string, 0, len(used))
	for name := range used {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func TestFindUsedNamesPrimesOperationsAndNullaryLets(t *testing.T) {
	x := NewNameExpr("x", Untyped())
	y := NewNameExpr("y", Untyped())
	primeX := MustOperExpr(Op("PRIME"), Untyped(), x)
	local := NewOperDecl("Local", nil, NewNameExpr("definitionName", Untyped()), Untyped())
	let := NewLetInExpr(NewNameExpr("bodyName", Untyped()), Untyped(), local)
	expression := MustOperExpr(Op("AND"), Untyped(), primeX,
		MustOperExpr(Op("EQ"), Untyped(), x, y), let)
	want := []string{"bodyName", "definitionName", "x", "x'", "y"}
	if got := sortedUsedNames(t, expression); !reflect.DeepEqual(got, want) {
		t.Fatalf("used names = %#v, want %#v", got, want)
	}

	parameterized := NewLetInExpr(NewNameExpr("body", Untyped()), Untyped(),
		NewOperDecl("Parameterized", []OperatorParam{ValueParam("p")}, NewNameExpr("p", Untyped()), Untyped()))
	if _, err := FindUsedNames(parameterized); err == nil ||
		!strings.Contains(err.Error(), "operator Parameterized: expected 0 parameters, found 1 parameters") {
		t.Fatalf("parameterized LET error = %v", err)
	}
}

func embeddedNameBody(body Expr, hiddenBodyName string) *LetInExpr {
	declaration := NewOperDecl("Lambda", []OperatorParam{ValueParam("p")}, body, Untyped())
	return NewLetInExpr(NewNameExpr(hiddenBodyName, Untyped()), Untyped(), declaration)
}

func TestFindUsedNamesSpecialEmbeddedOperatorForms(t *testing.T) {
	for _, test := range []struct {
		name       string
		expression Expr
		want       []string
	}{
		{
			name: "fold set",
			expression: MustOperExpr(Op("Apalache!ApaFoldSet"), Untyped(),
				embeddedNameBody(NewNameExpr("foldBody", Untyped()), "hiddenFoldLetBody"),
				NewNameExpr("base", Untyped()), NewNameExpr("set", Untyped())),
			want: []string{"base", "foldBody", "set"},
		},
		{
			name: "fold sequence",
			expression: MustOperExpr(Op("Apalache!ApaFoldSeqLeft"), Untyped(),
				embeddedNameBody(NewNameExpr("foldBody", Untyped()), "hiddenFoldLetBody"),
				NewNameExpr("base", Untyped()), NewNameExpr("sequence", Untyped())),
			want: []string{"base", "foldBody", "sequence"},
		},
		{
			name: "repeat",
			expression: MustOperExpr(Op("Apalache!Repeat"), Untyped(),
				embeddedNameBody(NewNameExpr("repeatBody", Untyped()), "hiddenRepeatLetBody"),
				NewNameExpr("bound", Untyped()), NewNameExpr("base", Untyped())),
			want: []string{"base", "bound", "repeatBody"},
		},
		{
			name: "make sequence",
			expression: MustOperExpr(Op("Apalache!MkSeq"), Untyped(), NewNameExpr("length", Untyped()),
				embeddedNameBody(NewNameExpr("sequenceBody", Untyped()), "hiddenSequenceLetBody")),
			want: []string{"length", "sequenceBody"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := sortedUsedNames(t, test.expression); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("used names = %#v, want %#v", got, test.want)
			}
		})
	}
}

func labelExpression(name string, body Expr, parameters ...string) Expr {
	arguments := []Expr{body, NewValueExpr(StringValue(name), Typed(StrT1))}
	for _, parameter := range parameters {
		arguments = append(arguments, NewValueExpr(StringValue(parameter), Typed(StrT1)))
	}
	return MustOperExpr(Op("LABEL"), body.Tag(), arguments...)
}

func TestFindLabelsPreorderOperationsAndLets(t *testing.T) {
	nested := labelExpression("outer", labelExpression("inner", boolExpr(true), "ignored-label-parameter"))
	definition := NewOperDecl("Local", nil, labelExpression("definition", boolExpr(true)), Untyped())
	let := NewLetInExpr(labelExpression("body", boolExpr(true)), Untyped(), definition)
	expression := MustOperExpr(Op("AND"), Untyped(), nested, let)
	want := []string{"outer", "inner", "definition", "body"}
	if got := FindLabels(expression); !reflect.DeepEqual(got, want) {
		t.Fatalf("labels = %#v, want %#v", got, want)
	}
}

func TestLabelsCacheKeysByKindAndReturnsDefensiveCopies(t *testing.T) {
	cache := NewLabelsCache()
	first := labelExpression("first", boolExpr(true))
	second := labelExpression("second", boolExpr(true))
	labels := cache.GetLabels(InitTransKind{Index: 0}, first)
	labels[0] = "mutated"
	if got := cache.GetLabels(InitTransKind{Index: 0}, second); !reflect.DeepEqual(got, []string{"first"}) {
		t.Fatalf("cached labels = %#v", got)
	}
	if got := cache.GetLabels(InitTransKind{Index: 1}, second); !reflect.DeepEqual(got, []string{"second"}) {
		t.Fatalf("different-index labels = %#v", got)
	}
	if got := cache.GetLabels(NextTransKind{Index: 0}, second); !reflect.DeepEqual(got, []string{"second"}) {
		t.Fatalf("different-kind labels = %#v", got)
	}
	if got := cache.GetLabels(VCKind{Index: 0}, nil); len(got) != 0 {
		t.Fatalf("nil-expression labels = %#v", got)
	}
}
