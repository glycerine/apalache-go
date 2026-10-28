package apalache

import "testing"

func TestTypedApalacheInternalBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			unsupported, err := builder.NotSupportedByModelChecker("message", typ)
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, unsupported, typ)
			if unsupported.Operator().Name() != "ApalacheInternal!__NotSupportedByModelChecker" {
				t.Fatalf("not-supported operator = %s", unsupported.Operator())
			}
			message := unsupported.Arguments()[0].(*ValueExpr)
			if message.Value() != StringValue("message") {
				t.Fatalf("message = %v", message.Value())
			}

			for count := 0; count <= 5; count++ {
				arguments := make([]Expr, count)
				for index := range arguments {
					arguments[index] = builder.Name("x", typ)
				}
				distinct, buildErr := builder.Distinct(arguments...)
				if buildErr != nil {
					t.Fatal(buildErr)
				}
				requireActionOperation(t, distinct, "Apalache!Distinct", BoolT1, arguments...)
				if count > 1 {
					bad := append([]Expr(nil), arguments...)
					bad[0] = builder.Name("bad", differentConcreteType(typ))
					if _, buildErr = builder.Distinct(bad...); buildErr == nil {
						t.Fatal("Distinct accepted heterogeneous arguments")
					}
				}
			}

			sequence := builder.Name("seq", mustSeqType(t, typ))
			capacity, err := builder.ApalacheSeqCapacity(sequence)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, capacity, "ApalacheInternal!__ApalacheSeqCapacity", IntT1, sequence)
			if _, err = builder.ApalacheSeqCapacity(builder.Int64(1)); err == nil {
				t.Fatal("ApalacheSeqCapacity accepted a non-sequence")
			}

			element := builder.Name("element", typ)
			set := builder.Name("set", mustSetType(t, typ))
			for _, test := range []struct {
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"Apalache!SelectInSet", builder.SelectInSet},
				{"Apalache!StoreInSet", builder.StoreInSet},
				{"Apalache!storeNotInSet", builder.StoreNotInSet},
			} {
				result, buildErr := test.build(element, set)
				if buildErr != nil {
					t.Fatal(buildErr)
				}
				requireActionOperation(t, result, test.operator, BoolT1, element, set)
				if _, buildErr = test.build(builder.Name("bad", differentConcreteType(typ)), set); buildErr == nil {
					t.Errorf("%s accepted an incompatible element", test.operator)
				}
				if _, buildErr = test.build(element, builder.Int64(1)); buildErr == nil {
					t.Errorf("%s accepted a non-set", test.operator)
				}
			}

			unconstrained, err := builder.UnconstrainArray(set)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, unconstrained, "Apalache!UnconstrainArray", BoolT1, set)
			if _, err = builder.UnconstrainArray(builder.Int64(1)); err == nil {
				t.Fatal("UnconstrainArray accepted a non-set")
			}
		})
	}
	if _, err := builder.NotSupportedByModelChecker("message", nil); err == nil {
		t.Fatal("NotSupportedByModelChecker accepted a nil type")
	}
}

func TestTypedApalacheInternalFunctionAndSMTMapCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, argumentType := range concreteActionBuilderTypes(t)[:5] {
		resultType := differentConcreteType(argumentType)
		functionType := mustFunType(t, argumentType, resultType)
		function := builder.Name("f", functionType)
		argument := builder.Name("x", argumentType)
		selected, err := builder.SelectInFun(argument, function)
		if err != nil {
			t.Fatal(err)
		}
		requireActionOperation(t, selected, "Apalache!SelectInFun", resultType, argument, function)
		notStored, err := builder.StoreNotInFun(argument, function)
		if err != nil {
			t.Fatal(err)
		}
		requireActionOperation(t, notStored, "Apalache!storeNotInFun", BoolT1, argument, function)
		value := builder.Name("value", resultType)
		stored, err := builder.StoreInFun(value, function, argument)
		if err != nil {
			t.Fatal(err)
		}
		requireActionOperation(t, stored, "Apalache!StoreInSet", BoolT1, value, function, argument)
		if _, err = builder.StoreInFun(builder.Name("bad", argumentType), function, argument); err == nil {
			t.Fatal("StoreInFun accepted a wrong value type")
		}
		if _, err = builder.StoreInFun(value, function, builder.Name("bad", resultType)); err == nil {
			t.Fatal("StoreInFun accepted a wrong argument type")
		}
	}

	left := builder.Name("S", mustSetType(t, IntT1))
	right := builder.Name("T", mustSetType(t, IntT1))
	for _, operator := range []Operator{Op("AND"), Op("OR")} {
		mapped, err := builder.SMTMap(operator, left, right)
		if err != nil {
			t.Fatal(err)
		}
		requireActionOperation(t, mapped, "Apalache!SmtMap_"+operator.Name(), mustSetType(t, IntT1), left, right)
	}
	if _, err := builder.SMTMap(Op("SET_UNION2"), left, right); err == nil {
		t.Fatal("SMTMap accepted an unsupported operator")
	}
	if _, err := builder.SMTMap(Op("AND"), left, builder.Name("bad", mustSetType(t, StrT1))); err == nil {
		t.Fatal("SMTMap accepted incompatible sets")
	}
}

func TestScopedApalacheInternalBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	unsupported, err := builder.NotSupportedByModelChecker("message", IntT1).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, unsupported, IntT1)
	set := builder.Name("S", mustSetType(t, IntT1))
	function := builder.Name("f", mustFunType(t, IntT1, BoolT1))
	for _, test := range []struct {
		instruction ExprInstruction
		want        Type
	}{
		{builder.Distinct(), BoolT1},
		{builder.ApalacheSeqCapacity(builder.Name("seq", mustSeqType(t, IntT1))), IntT1},
		{builder.SelectInSet(builder.Int64(1), set), BoolT1},
		{builder.StoreInSet(builder.Int64(1), set), BoolT1},
		{builder.StoreNotInSet(builder.Int64(1), set), BoolT1},
		{builder.SelectInFun(builder.Int64(1), function), BoolT1},
		{builder.StoreNotInFun(builder.Int64(1), function), BoolT1},
		{builder.StoreInFun(builder.Bool(true), function, builder.Int64(1)), BoolT1},
		{builder.SMTMap(Op("AND"), set, set), mustSetType(t, IntT1)},
		{builder.UnconstrainArray(set), BoolT1},
	} {
		result, buildErr := test.instruction.Build()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		requireExprType(t, result, test.want)
	}
}
