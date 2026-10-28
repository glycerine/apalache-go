package apalache

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestPredicateResultAccumulatesFailures(t *testing.T) {
	left := PredicateFail(NewUID(), "left")
	right := PredicateFail(NewUID(), "right")
	result := left.And(PredicateOK()).And(right)
	if result.OK() || len(result.Failures) != 2 || result.Failures[0].Message != "left" || result.Failures[1].Message != "right" {
		t.Fatalf("unexpected combined result: %+v", result)
	}
}

func TestPredicateResultScalaAndTruthTableAndCopies(t *testing.T) {
	oneID := NewUID()
	twoID := NewUID()
	one := PredicateFail(oneID, "one")
	two := PredicateFail(twoID, "two")
	cases := []struct {
		left, right PredicateResult
		want        []LanguageFailure
	}{
		{PredicateOK(), PredicateOK(), nil},
		{PredicateOK(), one, one.Failures},
		{one, PredicateOK(), one.Failures},
		{one, two, []LanguageFailure{{ID: oneID, Message: "one"}, {ID: twoID, Message: "two"}}},
	}
	for _, test := range cases {
		got := test.left.And(test.right)
		if len(got.Failures) != len(test.want) {
			t.Fatalf("%+v AND %+v: got %+v, want %+v", test.left, test.right, got.Failures, test.want)
		}
		for index := range test.want {
			if got.Failures[index] != test.want[index] {
				t.Fatalf("%+v AND %+v: got %+v, want %+v", test.left, test.right, got.Failures, test.want)
			}
		}
		if len(got.Failures) > 0 {
			got.Failures[0].Message = "mutated"
			if len(test.left.Failures) > 0 && test.left.Failures[0].Message == "mutated" {
				t.Fatal("And aliased its left input")
			}
			if len(test.left.Failures) == 0 && test.right.Failures[0].Message == "mutated" {
				t.Fatal("And aliased its right input")
			}
		}
	}
}

func TestFlatLanguagePredicate(t *testing.T) {
	predicate := FlatLanguagePredicate{}
	flat := MustOperExpr(Op("SET_ENUM"), Untyped(), NewValueExpr(Int64Value(1), Untyped()),
		NewValueExpr(StringValue("abc"), Untyped()), NewValueExpr(BoolValue(false), Untyped()))
	if result := predicate.CheckExpression(flat); !result.OK() {
		t.Fatalf("rejected flat expression: %+v", result)
	}
	undeclared := untypedApp("TopLevel")
	if result := predicate.CheckExpression(undeclared); result.OK() || result.Failures[0].ID != undeclared.ID() {
		t.Fatalf("accepted undeclared application: %+v", result)
	}
	local := NewOperDecl("Local", nil, NewValueExpr(BoolValue(true), Untyped()), Untyped())
	valid := NewLetInExpr(untypedApp("Local"), Untyped(), local)
	if result := predicate.CheckExpression(valid); !result.OK() {
		t.Fatalf("rejected nullary local call: %+v", result)
	}
	parameterized := NewOperDecl("Parameterized", []OperatorParam{ValueParam("x")},
		MustOperExpr(Op("PLUS"), Untyped(), NewValueExpr(Int64Value(1), Untyped()), NewNameExpr("x", Untyped())), Untyped())
	invalid := NewLetInExpr(untypedApp("Parameterized", NewValueExpr(Int64Value(1), Untyped())), Untyped(), parameterized)
	if result := predicate.CheckExpression(invalid); result.OK() {
		t.Fatal("accepted non-nullary local call")
	}
	a := NewOperDecl("A", nil, MustOperExpr(Op("PLUS"), Untyped(), NewValueExpr(Int64Value(1), Untyped()), NewValueExpr(Int64Value(2), Untyped())), Untyped())
	b := NewOperDecl("B", nil, NewValueExpr(Int64Value(3), Untyped()), Untyped())
	nested := NewLetInExpr(NewLetInExpr(MustOperExpr(Op("PLUS"), Untyped(), untypedApp("A"), untypedApp("B")), Untyped(), a), Untyped(), b)
	if result := predicate.CheckExpression(nested); !result.OK() {
		t.Fatalf("rejected nested nullary LET-IN: %+v", result)
	}
	badModule := NewModule("Bad", []Decl{NewOperDecl("A", nil,
		untypedApp("B", NewValueExpr(Int64Value(1), Untyped())), Untyped())})
	if predicate.CheckModule(badModule).OK() {
		t.Fatal("accepted module with a user-operator call")
	}
	goodModule := NewModule("Good", []Decl{NewOperDecl("A", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())})
	if result := predicate.CheckModule(goodModule); !result.OK() {
		t.Fatalf("rejected module without calls: %+v", result)
	}
}

func TestFlatLanguagePredicateSequentialLetScope(t *testing.T) {
	first := NewOperDecl("First", nil, untypedApp("Second"), Untyped())
	second := NewOperDecl("Second", nil, NewValueExpr(BoolValue(true), Untyped()), Untyped())
	forward := NewLetInExpr(untypedApp("First"), Untyped(), first, second)
	if (FlatLanguagePredicate{}).CheckExpression(forward).OK() {
		t.Fatal("accepted a forward LET-IN reference")
	}
	backward := NewLetInExpr(untypedApp("Second"), Untyped(), second,
		NewOperDecl("First", nil, untypedApp("Second"), Untyped()))
	if result := (FlatLanguagePredicate{}).CheckExpression(backward); !result.OK() {
		t.Fatalf("rejected a backward LET-IN reference: %+v", result)
	}
}

func TestMonotypeLanguagePredicate(t *testing.T) {
	mono := NewValueExpr(Int64Value(1), Typed(IntT1))
	if result := (MonotypeLanguagePredicate{}).CheckExpression(mono); !result.OK() {
		t.Fatalf("rejected monotype: %+v", result)
	}
	variable, err := NewVarType(7)
	if err != nil {
		t.Fatal(err)
	}
	poly := NewNameExpr("empty", Typed(variable))
	if result := (MonotypeLanguagePredicate{}).CheckExpression(poly); result.OK() || result.Failures[0].ID != poly.ID() {
		t.Fatalf("accepted polymorphic type: %+v", result)
	} else if want := fmt.Sprintf("Expected a non-polymorphic type, found: %s\nYou may need to annotate an empty collection.", variable); result.Failures[0].Message != want {
		t.Fatalf("wrong Scala diagnostic:\n got %q\nwant %q", result.Failures[0].Message, want)
	}
	if result := (MonotypeLanguagePredicate{}).CheckExpression(NewNameExpr("x", Untyped())); result.OK() {
		t.Fatal("accepted untyped expression")
	}
}

func TestTypedLanguagePredicateAllowsPolymorphismButRejectsUntypedNodes(t *testing.T) {
	a, err := NewVarType(1)
	if err != nil {
		t.Fatal(err)
	}
	polymorphic := NewOperDecl("Identity", []OperatorParam{ValueParam("x")},
		NewNameExpr("x", Typed(a)), Typed(mustOperator([]Type{a}, a)))
	module := NewModule("Typed", []Decl{polymorphic})
	if result := (TypedLanguagePredicate{}).CheckModule(module); !result.OK() {
		t.Fatalf("typed polymorphic module was rejected: %+v", result.Failures)
	}
	polymorphic.SetBody(NewNameExpr("x", Untyped()))
	if result := (TypedLanguagePredicate{}).CheckModule(module); result.OK() {
		t.Fatal("untyped expression was accepted")
	}
}

func TestNonrecursiveLanguagePredicate(t *testing.T) {
	predicate := NonrecursiveLanguagePredicate{}
	recursive := NewOperDecl("Rec", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Untyped()), Untyped())
	recursive.SetRecursive(true)
	module := NewModule("M", []Decl{recursive})
	if result := predicate.CheckModule(module); result.OK() || result.Failures[0].ID != recursive.ID() {
		t.Fatalf("accepted recursive declaration: %+v", result)
	}
	recursiveFunction := MustOperExpr(Op("FUN_REC_CTOR"), Untyped(),
		NewNameExpr("body", Untyped()), NewNameExpr("x", Untyped()), NewNameExpr("S", Untyped()))
	if result := predicate.CheckExpression(recursiveFunction); result.OK() || result.Failures[0].ID != recursiveFunction.ID() {
		t.Fatalf("accepted recursive function: %+v", result)
	}
	if got, want := predicate.CheckModule(module).Failures[0].Message,
		"RECURSIVE Rec(_). Apalache does not support recursive operators."; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestLanguageWatchdogReturnsStructuredError(t *testing.T) {
	err := WatchExpression(FlatLanguagePredicate{}, untypedApp("Missing"))
	var predicateError *LanguagePredicateError
	if !errors.As(err, &predicateError) || len(predicateError.Failures) != 1 {
		t.Fatalf("got %T %v", err, err)
	}
	if got, want := err.Error(), "Some expressions do not fit in the fragment FlatLanguagePred"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
	canonical := new(LanguagePredError)
	if !errors.As(err, &canonical) || canonical.Error() != err.Error() || !reflect.DeepEqual(canonical.FailedIDs(), predicateError.Failures) {
		t.Fatalf("canonical unwrap mismatch: %#v", canonical)
	}

	module := NewModule("M", []Decl{NewOperDecl("A", nil, untypedApp("Missing"), Untyped())})
	err = WatchModule(FlatLanguagePredicate{}, module)
	if got, want := err.Error(), "Some declarations do not fit in the fragment FlatLanguagePred"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

type fallbackNamedPredicate struct{}

func (fallbackNamedPredicate) CheckExpression(expression Expr) PredicateResult {
	return (FlatLanguagePredicate{}).CheckExpression(expression)
}

func (fallbackNamedPredicate) CheckModule(module *Module) PredicateResult {
	return (FlatLanguagePredicate{}).CheckModule(module)
}

func TestLanguageWatchdogFallsBackToConcretePredicateName(t *testing.T) {
	err := WatchExpression(fallbackNamedPredicate{}, untypedApp("Missing"))
	if got, want := err.Error(), "Some expressions do not fit in the fragment fallbackNamedPredicate"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
