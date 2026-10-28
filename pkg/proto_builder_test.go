package apalache

import "testing"

func fooOperator() Operator {
	return Operator{definition: &operatorDefinition{
		name:           "Foo",
		interpretation: InterpretationPredefined,
		category:       CategoryApalache,
		arity:          FixedArity(2),
		precedence:     Precedence{Low: 160, High: 160},
		constructible:  true,
	}}
}

func fooInference(arguments []Type) (Type, error) {
	operator := fooOperator()
	if len(arguments) == 2 {
		if function, ok := arguments[1].(FunType); ok && EqualTypes(arguments[0], function.arg) {
			return function.result, nil
		}
	}
	return nil, signatureError(operator, arguments, "expected (a, a -> b)")
}

func TestHowToFooBuilderScalaExample(t *testing.T) {
	typed := NewTypedBuilder()
	scoped := NewScopedBuilder()
	for _, argumentType := range concreteActionBuilderTypes(t) {
		for _, resultType := range concreteActionBuilderTypes(t)[:5] {
			functionType := mustFunType(t, argumentType, resultType)
			argument := typed.Name("x", argumentType)
			function := typed.Name("f", functionType)
			result, err := typed.ApplyWithInference(fooOperator(), fooInference, argument, function)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, result, "Foo", resultType, argument, function)
			if _, err = typed.ApplyWithInference(fooOperator(), fooInference,
				typed.Name("bad", differentConcreteType(argumentType)), function); err == nil {
				t.Fatal("Foo accepted an incompatible argument")
			}
			if _, err = typed.ApplyWithInference(fooOperator(), fooInference, argument, typed.Name("bad", IntT1)); err == nil {
				t.Fatal("Foo accepted a non-function")
			}

			scopedResult, err := scoped.ApplyWithInference(fooOperator(), fooInference,
				scoped.Name("x", argumentType), scoped.Name("f", functionType)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, scopedResult, resultType)
			if scopedResult.(*OperExpr).Operator().Name() != "Foo" {
				t.Fatalf("scoped operator = %s", scopedResult.(*OperExpr).Operator())
			}
		}
	}
	if _, err := typed.ApplyWithInference(fooOperator(), nil, typed.Int64(1), typed.Int64(2)); err == nil {
		t.Fatal("ApplyWithInference accepted a nil inference function")
	}
}
