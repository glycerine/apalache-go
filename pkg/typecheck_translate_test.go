package apalache

import "testing"

func newTranslator() *ToEtcExpr { return NewToEtcExpr(nil, nil, NewTypeVarPool(1000), false) }

func inferIRExpression(t *testing.T, expression Expr) (Type, *collectingTypeListener, bool) {
	t.Helper()
	translator := newTranslator()
	etc, err := translator.TranslateExpression(expression)
	if err != nil {
		t.Fatal(err)
	}
	listener := &collectingTypeListener{}
	typ, ok, err := NewEtcTypeChecker(translator.pool, true).Compute(listener, EmptyTypeContext(), etc)
	if err != nil {
		t.Fatal(err)
	}
	return typ, listener, ok
}

func TestToEtcExprInfersCoreOperators(t *testing.T) {
	tests := []struct {
		name       string
		expression Expr
		want       Type
	}{
		{"arithmetic", tlaOper("PLUS", tlaInt(1), tlaInt(2)), IntT1},
		{"comparison", tlaOper("LT", tlaInt(1), tlaInt(2)), BoolT1},
		{"set", tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)), mustSet(IntT1)},
		{"tuple", tlaOper("TUPLE", tlaInt(1), NewValueExpr(StringValue("x"), Untyped())), mustTuple(IntT1, StrT1)},
		{"function set", tlaOper("FUN_SET", tlaOper("SET_ENUM", tlaInt(1)), tlaOper("SET_ENUM", NewValueExpr(BoolValue(true), Untyped()))), mustSet(mustFun(IntT1, BoolT1))},
		{"if", tlaOper("IF_THEN_ELSE", NewValueExpr(BoolValue(true), Untyped()), tlaInt(1), tlaInt(2)), IntT1},
		{"sequence head", tlaOper("Sequences!Head", tlaOper("TUPLE", tlaInt(1), tlaInt(2))), IntT1},
		{"unchanged singleton tuple", tlaOper("UNCHANGED", tlaOper("TUPLE", tlaInt(1))), BoolT1},
		{"variant unsafe", tlaOper("Variants!VariantGetUnsafe", tlaString("A"), tlaOper("Variants!Variant", tlaString("A"), tlaInt(1))), IntT1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, events, ok := inferIRExpression(t, test.expression)
			if !ok || !EqualTypes(got, test.want) {
				t.Fatalf("inferred %v, want %v; events %+v", got, test.want, events.events)
			}
		})
	}
}

func TestToEtcExprRejectsIllTypedIR(t *testing.T) {
	expression := tlaOper("PLUS", tlaInt(1), NewValueExpr(BoolValue(true), Untyped()))
	_, listener, ok := inferIRExpression(t, expression)
	if ok {
		t.Fatal("accepted Int + Bool")
	}
	if !listener.errorContains(expression.ID(), "cannot be applied") {
		t.Fatalf("missing diagnostic: %+v", listener.events)
	}
}

func TestToEtcExprTranslatesBoundedQuantifier(t *testing.T) {
	expression := tlaOper("EXISTS3", tlaName("x"), tlaOper("SET_ENUM", tlaInt(1), tlaInt(2)), tlaOper("GT", tlaName("x"), tlaInt(0)))
	got, events, ok := inferIRExpression(t, expression)
	if !ok || !EqualTypes(got, BoolT1) {
		t.Fatalf("inferred %v; events %+v", got, events.events)
	}
}

func TestToEtcExprUsesDeclarationAnnotations(t *testing.T) {
	variable := NewVarDecl("x", Untyped())
	body := tlaOper("PLUS", tlaName("x"), tlaInt(1))
	operator := NewOperDecl("Inc", nil, body, Untyped())
	call := tlaOper("OPER_APP", tlaName("Inc"))
	translator := NewToEtcExpr(map[UID]Type{variable.ID(): IntT1}, nil, NewTypeVarPool(1000), false)
	scoped, err := translator.TranslateExpression(call)
	if err != nil {
		t.Fatal(err)
	}
	scoped, err = translator.TranslateDeclaration(operator, scoped)
	if err != nil {
		t.Fatal(err)
	}
	scoped, err = translator.TranslateDeclaration(variable, scoped)
	if err != nil {
		t.Fatal(err)
	}
	typ, ok, err := NewEtcTypeChecker(translator.pool, true).Compute(&collectingTypeListener{}, EmptyTypeContext(), scoped)
	if err != nil || !ok || !EqualTypes(typ, IntT1) {
		t.Fatalf("Compute() = %v, %v, %v", typ, ok, err)
	}
}

func TestToEtcExprRecordAndFunctionApplication(t *testing.T) {
	record := tlaOper("RECORD", tlaString("answer"), tlaInt(42), tlaString("other"), tlaString("value"))
	access := tlaOper("FUN_APP", record, tlaString("answer"))
	got, events, ok := inferIRExpression(t, access)
	if !ok || !EqualTypes(got, IntT1) {
		t.Fatalf("inferred %v; events %+v", got, events.events)
	}
}
