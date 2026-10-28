package apalache

import "testing"

func TestKeraLanguagePredicateExactOperatorFamilies(t *testing.T) {
	predicate := KeraLanguagePredicate{}
	value := func() Expr { return NewValueExpr(Int64Value(1), Untyped()) }
	name := func() Expr { return NewNameExpr("x", Untyped()) }

	for operatorName := range keraUnaryOperators {
		expression := MustOperExpr(Op(operatorName), Untyped(), value())
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("unary %s rejected: %+v", operatorName, result.Failures)
		}
	}
	for operatorName := range keraBinaryOperators {
		expression := MustOperExpr(Op(operatorName), Untyped(), value(), value())
		if result := predicate.CheckExpression(expression); !result.OK() {
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
		expression := MustOperExpr(Op(operatorName), Untyped(), arguments...)
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("nary %s rejected: %+v", operatorName, result.Failures)
		}
	}
}

func TestKeraLanguagePredicateSpecialFormsAndFailures(t *testing.T) {
	predicate := KeraLanguagePredicate{}
	one := NewValueExpr(Int64Value(1), Untyped())
	set := MustOperExpr(Op("SET_ENUM"), Untyped(), one)
	for _, operatorName := range []string{"EXISTS3", "FORALL3", "CHOOSE3", "SET_FILTER"} {
		expression := MustOperExpr(Op(operatorName), Untyped(), NewNameExpr("x", Untyped()), set, NewValueExpr(BoolValue(true), Untyped()))
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("binding %s rejected: %+v", operatorName, result.Failures)
		}
	}
	for _, operatorName := range []string{"SET_MAP", "FUN_CTOR", "FUN_REC_CTOR"} {
		expression := MustOperExpr(Op(operatorName), Untyped(), one, NewNameExpr("x", Untyped()), set)
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("binder %s rejected: %+v", operatorName, result.Failures)
		}
	}
	for _, operatorName := range []string{"Apalache!ApaFoldSet", "Apalache!ApaFoldSeqLeft", "Apalache!Repeat"} {
		expression := MustOperExpr(Op(operatorName), Untyped(), NewNameExpr("Op", Untyped()), one, set)
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("higher-order %s rejected: %+v", operatorName, result.Failures)
		}
	}
	mkSeq := MustOperExpr(Op("Apalache!MkSeq"), Untyped(), one, NewNameExpr("Op", Untyped()))
	if result := predicate.CheckExpression(mkSeq); !result.OK() {
		t.Fatalf("MkSeq rejected: %+v", result.Failures)
	}

	seq := MustOperExpr(Op("Sequences!Seq"), Untyped(), set)
	result := predicate.CheckExpression(seq)
	want := "Seq(_) produces an infinite set of unbounded sequences. See: " + keraSequenceManualLink
	if result.OK() || result.Failures[0].ID != seq.ID() || result.Failures[0].Message != want {
		t.Fatalf("wrong Seq failure: %+v", result.Failures)
	}
	unsupported := MustOperExpr(Op("ApalacheInternal!__NotSupportedByModelChecker"), Untyped(), NewValueExpr(StringValue("foo"), Untyped()))
	if result := predicate.CheckExpression(unsupported); result.OK() || result.Failures[0].Message != "Not supported: foo" {
		t.Fatalf("wrong unsupported failure: %+v", result.Failures)
	}
	for _, rejected := range []*OperExpr{
		MustOperExpr(Op("SET_INTERSECT"), Untyped(), set, set),
		MustOperExpr(Op("SET_MINUS"), Untyped(), set, set),
		MustOperExpr(Op("EQUIV"), Untyped(), NewValueExpr(BoolValue(true), Untyped()), NewValueExpr(BoolValue(false), Untyped())),
	} {
		if predicate.CheckExpression(rejected).OK() {
			t.Errorf("accepted non-KerA operator %s", rejected.Operator().Name())
		}
	}
}

func TestKeraLanguagePredicateSequentialLetAndModule(t *testing.T) {
	predicate := KeraLanguagePredicate{}
	first := NewOperDecl("First", nil, untypedApp("Second"), Untyped())
	second := NewOperDecl("Second", nil, NewValueExpr(BoolValue(true), Untyped()), Untyped())
	if predicate.CheckExpression(NewLetInExpr(untypedApp("First"), Untyped(), first, second)).OK() {
		t.Fatal("accepted forward LET reference")
	}
	valid := NewLetInExpr(untypedApp("First"), Untyped(), second,
		NewOperDecl("First", nil, untypedApp("Second"), Untyped()))
	if result := predicate.CheckExpression(valid); !result.OK() {
		t.Fatalf("rejected sequential LET: %+v", result.Failures)
	}
	module := NewModule("M", []Decl{NewOperDecl("Bad", nil, untypedApp("Top"), Untyped())})
	if predicate.CheckModule(module).OK() {
		t.Fatal("accepted module-level user operator application")
	}
}
