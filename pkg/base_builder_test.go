package apalache

import "testing"

func TestTypedBaseBuilderEqualityExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			left, right := builder.Name("lhs", typ), builder.Name("rhs", typ)
			for _, test := range []struct {
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{{"EQ", builder.Eql}, {"NE", builder.Neql}} {
				result, err := test.build(left, right)
				if err != nil {
					t.Fatal(err)
				}
				requireActionOperation(t, result, test.operator, BoolT1, left, right)
				if _, err = test.build(builder.Name("bad", differentConcreteType(typ)), right); err == nil {
					t.Errorf("%s accepted an ill-typed left operand", test.operator)
				}
				if _, err = test.build(left, builder.Name("bad", differentConcreteType(typ))); err == nil {
					t.Errorf("%s accepted an ill-typed right operand", test.operator)
				}
			}
		})
	}
}

func TestTypedBaseBuilderOperatorApplicationExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	model := mustNamedConst(t, "MODEL")
	tests := []struct {
		name       string
		arguments  []Type
		resultType Type
	}{
		{"nullary", nil, BoolT1},
		{"unary", []Type{IntT1}, StrT1},
		{"binary", []Type{IntT1, StrT1}, model},
		{"nested", []Type{mustSeqType(t, BoolT1), mustSetType(t, model)}, mustTupleType(t, IntT1, model)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			operatorType := mustOperatorType(t, test.arguments, test.resultType)
			operator := builder.Name("A", operatorType)
			arguments := make([]Expr, len(test.arguments))
			for index, typ := range test.arguments {
				arguments[index] = builder.Name("x", typ)
			}
			result, err := builder.ApplyOperator(operator, arguments...)
			if err != nil {
				t.Fatal(err)
			}
			application := result.(*OperExpr)
			requireExprType(t, application, test.resultType)
			if application.Operator().Name() != "OPER_APP" || len(application.Arguments()) != len(arguments)+1 {
				t.Fatalf("application = %s/%d", application.Operator(), len(application.Arguments()))
			}
			if len(arguments) > 0 {
				badTypes := append([]Type(nil), test.arguments...)
				badTypes[0] = differentConcreteType(test.arguments[0])
				badOperator := builder.Name("Bad", mustOperatorType(t, badTypes, test.resultType))
				if _, err = builder.ApplyOperator(badOperator, arguments...); err == nil {
					t.Fatal("accepted incompatible declared argument types")
				}
				badArguments := append([]Expr(nil), arguments...)
				badArguments[0] = builder.Name("bad", differentConcreteType(test.arguments[0]))
				if _, err = builder.ApplyOperator(operator, badArguments...); err == nil {
					t.Fatal("accepted an incompatible actual argument")
				}
			}
		})
	}
	if _, err := builder.ApplyOperator(builder.Int64(1)); err == nil {
		t.Fatal("accepted a non-operator callee")
	}
}

func TestTypedBaseBuilderPolymorphicApplicationRegressions(t *testing.T) {
	builder := NewTypedBuilder()
	a := mustNumberedVar(t, 0)
	constantType := mustOperatorType(t, []Type{a}, IntT1)
	constant := builder.Name("polyConst1", constantType)
	argument, err := builder.Str("s")
	if err != nil {
		t.Fatal(err)
	}
	result, err := builder.ApplyOperator(constant, argument)
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, result, IntT1)
	application := result.(*OperExpr)
	wantSpecialized := mustOperatorType(t, []Type{StrT1}, IntT1)
	requireExprType(t, application.Arguments()[0], wantSpecialized)

	addType := mustOperatorType(t, []Type{a}, mustSetType(t, a))
	add := builder.Name("add", addType)
	nested, err := builder.ApplyOperator(add, builder.Int64(42))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, nested, mustSetType(t, IntT1))
	receiver := builder.Name("rcvd", mustFunType(t, IntT1, mustSetType(t, IntT1)))
	updated, err := builder.Except(receiver, builder.Int64(1), nested)
	if err != nil {
		t.Fatalf("specialized application was unusable in EXCEPT: %v", err)
	}
	requireExprType(t, updated, mustFunType(t, IntT1, mustSetType(t, IntT1)))

	parameter, err := NewTypedParameter("x", IntT1)
	if err != nil {
		t.Fatal(err)
	}
	lambda, err := builder.Lambda("Lambda", builder.Name("x", IntT1), parameter)
	if err != nil {
		t.Fatal(err)
	}
	appliedLambda, err := builder.ApplyOperator(lambda, builder.Int64(1))
	if err != nil {
		t.Fatal(err)
	}
	let, ok := appliedLambda.(*LetInExpr)
	if !ok || let.Body().(*OperExpr).Operator().Name() != "OPER_APP" {
		t.Fatalf("lambda application = %T %v", appliedLambda, appliedLambda)
	}
}

func TestTypedBaseBuilderChooseAndLabelExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			variable := builder.Name("x", typ)
			predicate := builder.Name("p", BoolT1)
			set := builder.Name("set", mustSetType(t, typ))
			bounded, err := builder.ChooseIn(variable, set, predicate)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, bounded, "CHOOSE3", typ, variable, set, predicate)
			if _, err = builder.ChooseIn(builder.Name("bad", differentConcreteType(typ)), set, predicate); err == nil {
				t.Fatal("bounded CHOOSE accepted an incompatible variable")
			}
			if _, err = builder.ChooseIn(variable, builder.Name("bad", mustSetType(t, differentConcreteType(typ))), predicate); err == nil {
				t.Fatal("bounded CHOOSE accepted an incompatible set")
			}
			if _, err = builder.ChooseIn(variable, builder.Name("bad", IntT1), predicate); err == nil {
				t.Fatal("bounded CHOOSE accepted a non-set")
			}
			if _, err = builder.ChooseIn(variable, set, builder.Name("bad", StrT1)); err == nil {
				t.Fatal("bounded CHOOSE accepted a non-Boolean predicate")
			}

			unbounded, err := builder.Choose(variable, predicate)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, unbounded, "CHOOSE2", typ, variable, predicate)
			if _, err = builder.Choose(variable, builder.Name("bad", StrT1)); err == nil {
				t.Fatal("CHOOSE accepted a non-Boolean predicate")
			}

			for count := 1; count <= 11; count++ {
				labels := make([]string, count)
				for index := range labels {
					labels[index] = "x" + string(rune('a'+index))
				}
				labeled, labelErr := builder.Label(variable, labels...)
				if labelErr != nil {
					t.Fatal(labelErr)
				}
				requireExprType(t, labeled, typ)
				arguments := labeled.Arguments()
				if labeled.Operator().Name() != "LABEL" || len(arguments) != count+1 || arguments[0] != variable {
					t.Fatalf("label = %s/%d", labeled.Operator(), len(arguments))
				}
			}
		})
	}
	if _, err := builder.Label(builder.Int64(1)); err == nil {
		t.Fatal("Label accepted zero labels")
	}
	if _, err := builder.Label(builder.Int64(1), "1_OF_A"); err == nil {
		t.Fatal("Label accepted a model value as a string label")
	}
	tuple, _ := builder.Tuple(builder.Name("x", IntT1), builder.Name("y", StrT1))
	if _, err := builder.Choose(tuple, builder.Bool(true)); err == nil {
		t.Fatal("CHOOSE accepted a tuple variable")
	}
}

func TestScopedBaseBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			for _, build := range []func(ExprInstruction, ExprInstruction) ExprInstruction{builder.Eql, builder.Neql} {
				result, err := build(builder.Name("lhs", typ), builder.Name("rhs", typ)).Build()
				if err != nil {
					t.Fatal(err)
				}
				requireExprType(t, result, BoolT1)
				if _, err = build(builder.Name("bad", differentConcreteType(typ)), builder.Name("rhs", typ)).Build(); err == nil {
					t.Fatal("equality accepted an ill-typed left operand")
				}
			}

			bounded, err := builder.ChooseIn(builder.Name("x", typ), builder.Name("S", mustSetType(t, typ)), builder.Name("p", BoolT1)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, bounded, typ)
			unbounded, err := builder.Choose(builder.Name("x", typ), builder.Name("p", BoolT1)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, unbounded, typ)
			labeled, err := builder.Label(builder.Name("ex", typ), "x").Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, labeled, typ)
		})
	}

	a := mustNumberedVar(t, 0)
	polymorphic := builder.Name("poly", mustOperatorType(t, []Type{a}, mustSetType(t, a)))
	application, err := builder.ApplyOperator(polymorphic, builder.Int64(42)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, application, mustSetType(t, IntT1))
	if _, err = builder.Label(builder.Int64(1)).Build(); err == nil {
		t.Fatal("Label accepted zero labels")
	}
}

func TestScopedChooseRejectsInvalidBindings(t *testing.T) {
	builder := NewScopedBuilder()
	if _, err := builder.Choose(builder.String("x"), builder.Bool(true)).Build(); err == nil {
		t.Fatal("CHOOSE accepted a non-name variable")
	}
	tuple := builder.Tuple(builder.Name("x", StrT1), builder.Name("y", IntT1))
	if _, err := builder.Choose(tuple, builder.Bool(true)).Build(); err == nil {
		t.Fatal("CHOOSE accepted a tuple variable")
	}
	conflicting := builder.Eql(builder.Name("x", IntT1), builder.Name("x", IntT1))
	if _, err := builder.Choose(builder.Name("x", StrT1), conflicting).Build(); err == nil {
		t.Fatal("CHOOSE accepted a conflicting free-use type")
	}
	inner := builder.ExistsIn(builder.Name("x", StrT1), builder.Name("S", mustSetType(t, StrT1)), builder.Bool(true))
	if _, err := builder.Choose(builder.Name("x", StrT1), inner).Build(); err == nil {
		t.Fatal("CHOOSE accepted nested variable shadowing")
	}

	if _, err := builder.ChooseIn(builder.String("x"), builder.Name("S", mustSetType(t, StrT1)), builder.Bool(true)).Build(); err == nil {
		t.Fatal("bounded CHOOSE accepted a non-name variable")
	}
	if _, err := builder.ChooseIn(builder.Name("x", StrT1), builder.Apply(Op("SET_ENUM"), builder.Name("x", StrT1)), builder.Bool(true)).Build(); err == nil {
		t.Fatal("bounded CHOOSE accepted a variable used in its own domain")
	}
	if _, err := builder.ChooseIn(builder.Name("x", StrT1), builder.Name("S", mustSetType(t, StrT1)), inner).Build(); err == nil {
		t.Fatal("bounded CHOOSE accepted nested variable shadowing")
	}
}
