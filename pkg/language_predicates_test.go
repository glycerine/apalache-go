package apalache

import (
	"errors"
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
}

func TestLanguageWatchdogReturnsStructuredError(t *testing.T) {
	err := WatchExpression(FlatLanguagePredicate{}, untypedApp("Missing"))
	var predicateError *LanguagePredicateError
	if !errors.As(err, &predicateError) || len(predicateError.Failures) != 1 {
		t.Fatalf("got %T %v", err, err)
	}
}
