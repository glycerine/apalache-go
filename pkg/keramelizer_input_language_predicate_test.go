package apalache

import (
	"strings"
	"testing"
)

func TestKeramelizerInputLanguagePredicateExactAcceptedFamilies(t *testing.T) {
	predicate := KeramelizerInputLanguagePredicate{}
	value := func() Expr { return NewValueExpr(Int64Value(1), Untyped()) }
	name := func() Expr { return NewNameExpr("x", Untyped()) }
	for operatorName := range keraUnaryOperators {
		if result := predicate.CheckExpression(MustOperExpr(Op(operatorName), Untyped(), value())); !result.OK() {
			t.Errorf("unary %s rejected: %+v", operatorName, result.Failures)
		}
	}
	for operatorName := range keraBinaryOperators {
		if result := predicate.CheckExpression(MustOperExpr(Op(operatorName), Untyped(), value(), value())); !result.OK() {
			t.Errorf("binary %s rejected: %+v", operatorName, result.Failures)
		}
	}
	nary := map[string][]Expr{
		"AND":                       {},
		"OR":                        {},
		"SET_ENUM":                  {},
		"EXCEPT":                    {name(), value(), value()},
		"TUPLE":                     {},
		"RECORD":                    {NewValueExpr(StringValue("a"), Untyped()), value()},
		"Sequences!SubSeq":          {name(), value(), value()},
		"LABEL":                     {value(), NewValueExpr(StringValue("label"), Untyped())},
		"Variants!VariantGetOrElse": {NewValueExpr(StringValue("tag"), Untyped()), name(), value()},
	}
	for operatorName, arguments := range nary {
		if result := predicate.CheckExpression(MustOperExpr(Op(operatorName), Untyped(), arguments...)); !result.OK() {
			t.Errorf("nary %s rejected: %+v", operatorName, result.Failures)
		}
	}
	for _, expression := range []Expr{
		MustOperExpr(Op("SET_SUBSET_EQ"), Untyped(), value(), value()),
		MustOperExpr(Op("SET_NOT_IN"), Untyped(), value(), value()),
		MustOperExpr(Op("EQUIV"), Untyped(), value(), value()),
		MustOperExpr(Op("IMPLIES"), Untyped(), value(), value()),
		MustOperExpr(Op("NE"), Untyped(), value(), value()),
		MustOperExpr(Op("CASE_OTHER"), Untyped(), value(), value(), value()),
		MustOperExpr(Op("CASE"), Untyped(), value(), value()),
	} {
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("desugared-only %s rejected: %+v", expression.(*OperExpr).Operator().Name(), result.Failures)
		}
	}
}

func TestKeramelizerInputLanguagePredicateNameFirstForms(t *testing.T) {
	predicate := KeramelizerInputLanguagePredicate{}
	set := MustOperExpr(Op("SET_ENUM"), Untyped(), NewValueExpr(Int64Value(1), Untyped()))
	truth := NewValueExpr(BoolValue(true), Untyped())
	for _, operatorName := range []string{"EXISTS3", "FORALL3", "CHOOSE3", "SET_FILTER"} {
		valid := MustOperExpr(Op(operatorName), Untyped(), NewNameExpr("x", Untyped()), set, truth)
		if result := predicate.CheckExpression(valid); !result.OK() {
			t.Errorf("name-first %s rejected: %+v", operatorName, result.Failures)
		}
		for _, invalidFirst := range []Expr{
			NewValueExpr(Int64Value(1), Untyped()),
			NewValueExpr(StringValue("x"), Untyped()),
		} {
			invalid := MustOperExpr(Op(operatorName), Untyped(), invalidFirst, set, truth)
			result := predicate.CheckExpression(invalid)
			if result.OK() || result.Failures[0].ID != invalid.ID() || !strings.HasPrefix(result.Failures[0].Message, "first argument not a name: ") {
				t.Errorf("%s accepted invalid first argument or returned wrong failure: %+v", operatorName, result.Failures)
			}
		}
	}

	invalidApplication := MustOperExpr(Op("OPER_APP"), Untyped(), NewValueExpr(Int64Value(1), Untyped()))
	if result := predicate.CheckExpression(invalidApplication); result.OK() || result.Failures[0].ID != invalidApplication.ID() {
		t.Fatalf("accepted non-name operator application: %+v", result.Failures)
	}
	local := NewOperDecl("Local", nil, truth, Untyped())
	validApplication := NewLetInExpr(untypedApp("Local"), Untyped(), local)
	if result := predicate.CheckExpression(validApplication); !result.OK() {
		t.Fatalf("rejected flat nullary local application: %+v", result.Failures)
	}
	nonNullary := NewLetInExpr(untypedApp("Local", truth), Untyped(), local)
	result := predicate.CheckExpression(nonNullary)
	if result.OK() || result.Failures[0].Message != "non-nullary operator Local [FlatLanguagePredicate]" {
		t.Fatalf("flat predicate was not conjoined first: %+v", result.Failures)
	}
}

func TestKeramelizerInputLanguagePredicateSetTypeRequirements(t *testing.T) {
	predicate := KeramelizerInputLanguagePredicate{}
	setInt := newSetTypeUnchecked(IntT1)
	typedSet := MustOperExpr(Op("SET_ENUM"), Typed(setInt), NewValueExpr(Int64Value(1), Typed(IntT1)))
	untypedSet := MustOperExpr(Op("SET_ENUM"), Untyped(), NewValueExpr(Int64Value(1), Untyped()))
	typedInt := NewNameExpr("a", Typed(IntT1))
	key := NewValueExpr(StringValue("a"), Untyped())

	succeed := []Expr{
		MustOperExpr(Op("SET_INTERSECT"), Untyped(), typedSet, untypedSet),
		MustOperExpr(Op("SET_MINUS"), Untyped(), typedSet, untypedSet),
		MustOperExpr(Op("RECORD_SET"), Typed(setInt), key, typedSet),
		MustOperExpr(Op("SET_TIMES"), Untyped(), typedSet, typedSet),
		MustOperExpr(Op("SET_IN"), Typed(BoolT1), MustOperExpr(Op("PRIME"), Typed(IntT1), typedInt), typedSet),
	}
	for _, expression := range succeed {
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("%s rejected: %+v", expression.(*OperExpr).Operator().Name(), result.Failures)
		}
	}

	fail := []Expr{
		MustOperExpr(Op("SET_INTERSECT"), Untyped(), typedInt, typedSet),
		MustOperExpr(Op("SET_MINUS"), Untyped(), untypedSet, typedSet),
		MustOperExpr(Op("RECORD_SET"), Typed(setInt), key, typedInt),
		MustOperExpr(Op("RECORD_SET"), Untyped(), key, typedSet),
		MustOperExpr(Op("SET_TIMES"), Untyped(), typedSet, typedInt),
		MustOperExpr(Op("SET_IN"), Typed(BoolT1), MustOperExpr(Op("PRIME"), Typed(IntT1), typedInt), untypedSet),
	}
	for _, expression := range fail {
		result := predicate.CheckExpression(expression)
		if result.OK() || result.Failures[0].ID != expression.ID() || !strings.HasPrefix(result.Failures[0].Message, "Expected a set, found: ") {
			t.Errorf("wrong set-type result for %s: %+v", expression.(*OperExpr).Operator().Name(), result.Failures)
		}
	}

	// Ordinary membership has no Keramelizer-specific type restriction.
	ordinary := MustOperExpr(Op("SET_IN"), Typed(BoolT1), typedInt, untypedSet)
	if result := predicate.CheckExpression(ordinary); !result.OK() {
		t.Fatalf("ordinary membership was type-restricted: %+v", result.Failures)
	}
}

func TestKeramelizerInputLanguagePredicateFailureOrderSeqAndModules(t *testing.T) {
	predicate := KeramelizerInputLanguagePredicate{}
	seq := MustOperExpr(Op("Sequences!Seq"), Untyped(), NewNameExpr("S", Untyped()))
	result := predicate.CheckExpression(seq)
	want := "Seq(_) produces an infinite set of unbounded sequences. See: " + keraSequenceManualLink
	if result.OK() || result.Failures[0].Message != want {
		t.Fatalf("wrong Seq failure: %+v", result.Failures)
	}

	// The parent set-type failure precedes the recursively discovered Seq failure.
	combined := MustOperExpr(Op("SET_INTERSECT"), Untyped(), seq, NewNameExpr("T", Untyped()))
	result = predicate.CheckExpression(combined)
	if len(result.Failures) != 2 || !strings.HasPrefix(result.Failures[0].Message, "Expected a set") || result.Failures[1].Message != want {
		t.Fatalf("wrong failure accumulation order: %+v", result.Failures)
	}

	bad := NewOperDecl("Bad", nil, MustOperExpr(Op("OPER_APP"), Untyped(), NewValueExpr(Int64Value(1), Untyped())), Untyped())
	module := NewModule("M", []Decl{NewConstDecl("C", Untyped()), bad})
	if result := predicate.CheckModule(module); result.OK() || result.Failures[0].ID != bad.Body().ID() {
		t.Fatalf("module predicate did not check operator bodies: %+v", result.Failures)
	}
	validModule := NewModule("Valid", []Decl{NewOperDecl("Value", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped())})
	if result := predicate.CheckModule(validModule); !result.OK() {
		t.Fatalf("module without calls was rejected: %+v", result.Failures)
	}

	inner := NewLetInExpr(untypedApp("A"), Untyped(), NewOperDecl("A", nil, NewValueExpr(Int64Value(1), Untyped()), Untyped()))
	nested := NewLetInExpr(inner, Untyped(), NewOperDecl("B", nil, NewValueExpr(Int64Value(2), Untyped()), Untyped()))
	if result := predicate.CheckExpression(nested); !result.OK() {
		t.Fatalf("nested nullary LET was rejected: %+v", result.Failures)
	}
}
