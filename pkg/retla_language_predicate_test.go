package apalache

import "testing"

func retlaValue(value Value) Expr { return NewValueExpr(value, Untyped()) }
func retlaInt(value int64) Expr   { return retlaValue(Int64Value(value)) }
func retlaBool(value bool) Expr   { return retlaValue(BoolValue(value)) }
func retlaString(value string) Expr {
	return retlaValue(StringValue(value))
}
func retlaName(name string) Expr { return NewNameExpr(name, Untyped()) }
func retlaOper(name string, arguments ...Expr) Expr {
	return MustOperExpr(Op(name), Untyped(), arguments...)
}

func TestReTLALanguagePredicateExactScalaExpressions(t *testing.T) {
	predicate := ReTLALanguagePredicate{}
	tuple := func(arguments ...Expr) Expr { return retlaOper("TUPLE", arguments...) }
	set := func(arguments ...Expr) Expr { return retlaOper("SET_ENUM", arguments...) }

	accepted := []struct {
		name       string
		expression Expr
	}{
		{"name", retlaName("x")},
		{"primed name", retlaOper("PRIME", retlaName("x"))},
		{"integer", retlaInt(22)},
		{"boolean", retlaBool(false)},
		{"string", retlaString("foo")},
		{"BOOLEAN", retlaValue(BoolSet)},
		{"Int", retlaValue(IntSet)},
		{"Nat", retlaValue(NatSet)},
		{"function constructor", retlaOper("FUN_CTOR", retlaInt(1), retlaName("x"), retlaName("S"))},
		{"function application", retlaOper("FUN_APP", retlaName("f"), retlaInt(3))},
		{"function application tuple key", retlaOper("FUN_APP", retlaName("f"), tuple(retlaInt(3), retlaInt(3)))},
		{"EXCEPT scalar key", retlaOper("EXCEPT", retlaName("f"), tuple(retlaInt(3)), retlaBool(true))},
		{"EXCEPT tuple key", retlaOper("EXCEPT", retlaName("f"), tuple(tuple(retlaInt(3), retlaInt(3))), retlaBool(true))},
		{"if then else", retlaOper("IF_THEN_ELSE", retlaName("p"), retlaInt(1), retlaInt(2))},
		{"or", retlaOper("OR", retlaBool(false), retlaName("b"))},
		{"and", retlaOper("AND", retlaBool(false), retlaName("b"))},
		{"equivalence", retlaOper("EQUIV", retlaBool(false), retlaName("b"))},
		{"implication", retlaOper("IMPLIES", retlaBool(false), retlaName("b"))},
		{"inequality", retlaOper("NE", retlaBool(false), retlaName("b"))},
	}
	for _, test := range accepted {
		t.Run("accept "+test.name, func(t *testing.T) {
			if result := predicate.CheckExpression(test.expression); !result.OK() {
				t.Fatalf("rejected expression: %+v", result.Failures)
			}
		})
	}

	rejected := []struct {
		name       string
		expression Expr
	}{
		{"set union", retlaOper("SET_UNION2", set(retlaInt(1)), set(retlaInt(2)))},
		{"membership", retlaOper("SET_IN", retlaInt(1), set(retlaInt(2)))},
		{"set enumeration", set(retlaInt(1))},
		{"subset equality", retlaOper("SET_SUBSET_EQ", set(retlaInt(1)), set(retlaInt(2)))},
		{"set filter", retlaOper("SET_FILTER", retlaName("x"), set(retlaInt(1)), retlaBool(false))},
		{"set map", retlaOper("SET_MAP", retlaInt(2), retlaName("x"), set(retlaInt(1)))},
		{"powerset", retlaOper("SET_POWERSET", set(retlaInt(1), retlaInt(2)))},
		{"unary union", retlaOper("SET_UNARY_UNION", set(set(retlaInt(1), retlaInt(2))))},
		{"integer range", retlaOper("INT_RANGE", retlaInt(10), retlaInt(20))},
		{"set intersection", retlaOper("SET_INTERSECT", set(retlaInt(1)), set(retlaInt(2)))},
		{"set minus", retlaOper("SET_MINUS", set(retlaInt(1)), set(retlaInt(2)))},
		{"non-membership", retlaOper("SET_NOT_IN", retlaInt(1), set(retlaInt(2)))},
		{"Cartesian product", retlaOper("SET_TIMES", retlaName("X"), retlaName("Y"))},
		{"finite set test", retlaOper("FiniteSets!IsFiniteSet", set(retlaInt(1)))},
		{"cardinality", retlaOper("FiniteSets!Cardinality", set(retlaInt(1)))},
		{"function set", retlaOper("FUN_SET", retlaName("X"), retlaName("Y"))},
		{"domain", retlaOper("DOMAIN", retlaName("X"))},
		{"tuple", tuple(retlaInt(1), retlaBool(true))},
		{"record", retlaOper("RECORD", retlaString("a"), retlaBool(true))},
		{"sequence head", retlaOper("Sequences!Head", tuple(retlaInt(1), retlaInt(2)))},
		{"sequence tail", retlaOper("Sequences!Tail", tuple(retlaInt(1), retlaInt(2)))},
		{"subsequence", retlaOper("Sequences!SubSeq", tuple(retlaInt(1), retlaInt(2)), retlaInt(3), retlaInt(4))},
		{"sequence length", retlaOper("Sequences!Len", tuple(retlaInt(1), retlaInt(2)))},
		{"sequence append", retlaOper("Sequences!Append", tuple(retlaInt(1), retlaInt(2)), tuple(retlaInt(2), retlaInt(3)))},
		{"label", retlaOper("LABEL", retlaInt(2), retlaString("a"))},
		{"label with parameter", retlaOper("LABEL", retlaInt(2), retlaString("a"), retlaString("b"))},
		{"less than", retlaOper("LT", retlaInt(1), retlaInt(3))},
		{"less equal", retlaOper("LE", retlaInt(1), retlaInt(3))},
		{"greater than", retlaOper("GT", retlaInt(1), retlaInt(3))},
		{"greater equal", retlaOper("GE", retlaInt(1), retlaInt(3))},
		{"addition", retlaOper("PLUS", retlaInt(1), retlaInt(3))},
		{"subtraction", retlaOper("MINUS", retlaInt(1), retlaInt(3))},
		{"multiplication", retlaOper("MULT", retlaInt(1), retlaInt(3))},
		{"division", retlaOper("DIV", retlaInt(1), retlaInt(3))},
		{"modulo", retlaOper("MOD", retlaInt(1), retlaInt(3))},
		{"exponentiation", retlaOper("POW", retlaInt(2), retlaInt(3))},
		{"unary minus", retlaOper("UNARY_MINUS", retlaInt(2))},
		{"case other", retlaOper("CASE_OTHER", retlaInt(1), retlaBool(false), retlaInt(2))},
		{"case split", retlaOper("CASE", retlaBool(false), retlaInt(2))},
		{"record set", retlaOper("RECORD_SET", retlaName("a"), retlaName("A"))},
	}
	for _, test := range rejected {
		t.Run("reject "+test.name, func(t *testing.T) {
			if result := predicate.CheckExpression(test.expression); result.OK() {
				t.Fatal("accepted expression outside reTLA")
			}
		})
	}
}

func TestReTLALanguagePredicateOperatorsAndSpecialForms(t *testing.T) {
	predicate := ReTLALanguagePredicate{}
	x := retlaName("x")
	for _, expression := range []Expr{
		retlaOper("Apalache!:=", x, retlaInt(1)),
		retlaOper("Apalache!Skolem", retlaOper("EXISTS3", x, retlaName("S"), retlaBool(true))),
		retlaOper("FORALL3", x, retlaName("S"), retlaOper("NOT", retlaBool(false))),
		retlaOper("EQ", retlaInt(1), retlaInt(1)),
	} {
		if result := predicate.CheckExpression(expression); !result.OK() {
			t.Errorf("rejected source-listed operator %s: %+v", expression.(*OperExpr).Operator(), result.Failures)
		}
	}
	for _, expression := range []Expr{
		retlaOper("PRIME", retlaInt(1)),
		retlaValue(StringSet),
		retlaValue(RealSet),
		retlaOper("FORALL2", x, retlaBool(true)),
		retlaOper("EXISTS2", x, retlaBool(true)),
		retlaOper("EXCEPT", retlaName("f"), retlaOper("TUPLE", retlaInt(1), retlaInt(2)), retlaBool(true)),
		retlaOper("EXCEPT", retlaName("f"), retlaOper("TUPLE", retlaInt(1)), retlaBool(true), retlaOper("TUPLE", retlaInt(2)), retlaBool(false)),
	} {
		if result := predicate.CheckExpression(expression); result.OK() {
			t.Errorf("accepted forbidden special form %v", expression)
		}
	}
}

func TestReTLALanguagePredicateLetAndModuleCases(t *testing.T) {
	predicate := ReTLALanguagePredicate{}
	if predicate.CheckExpression(untypedApp("UserOp", retlaString("abc"))).OK() {
		t.Fatal("accepted a call to a top-level user operator")
	}
	parameterized := NewOperDecl("UserOp", []OperatorParam{ValueParam("x")},
		retlaOper("AND", retlaBool(true), retlaName("x")), Untyped())
	if predicate.CheckExpression(NewLetInExpr(untypedApp("UserOp", retlaInt(3)), Untyped(), parameterized)).OK() {
		t.Fatal("accepted a non-nullary LET-IN call")
	}
	nullary := NewOperDecl("UserOp", nil, retlaOper("AND", retlaBool(true), retlaBool(false)), Untyped())
	if result := predicate.CheckExpression(NewLetInExpr(untypedApp("UserOp"), Untyped(), nullary)); !result.OK() {
		t.Fatalf("rejected a nullary LET-IN call: %+v", result.Failures)
	}
	a := NewOperDecl("A", nil, retlaOper("AND", retlaBool(true), retlaBool(false)), Untyped())
	b := NewOperDecl("B", nil, retlaBool(true), Untyped())
	inner := NewLetInExpr(retlaOper("AND", untypedApp("A"), untypedApp("B")), Untyped(), a)
	nested := NewLetInExpr(inner, Untyped(), b)
	if result := predicate.CheckExpression(nested); !result.OK() {
		t.Fatalf("rejected nested nullary LET-INs: %+v", result.Failures)
	}

	forward := NewLetInExpr(untypedApp("A"), Untyped(),
		NewOperDecl("A", nil, untypedApp("B"), Untyped()),
		NewOperDecl("B", nil, retlaBool(true), Untyped()))
	if predicate.CheckExpression(forward).OK() {
		t.Fatal("accepted a forward LET-IN reference")
	}

	badModule := NewModule("mod", []Decl{NewOperDecl("A", nil, untypedApp("B", retlaInt(1)), Untyped())})
	if predicate.CheckModule(badModule).OK() {
		t.Fatal("accepted module with a user-operator call")
	}
	goodModule := NewModule("mod", []Decl{NewOperDecl("A", nil, retlaInt(1), Untyped())})
	if result := predicate.CheckModule(goodModule); !result.OK() {
		t.Fatalf("rejected module without calls: %+v", result.Failures)
	}
}
