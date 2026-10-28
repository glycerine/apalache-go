package apalache

import "testing"

func concreteActionBuilderTypes(t *testing.T) []Type {
	t.Helper()
	model := mustNamedConst(t, "MODEL")
	recordRow := mustRowType(t, map[string]Type{"field": IntT1}, nil)
	variantRow := mustRowType(t, map[string]Type{
		"Number": NewRecordRowType(mustRowType(t, map[string]Type{"value": IntT1}, nil)),
	}, nil)
	return []Type{
		BoolT1,
		IntT1,
		StrT1,
		model,
		mustSetType(t, BoolT1),
		mustSeqType(t, IntT1),
		mustFunType(t, model, BoolT1),
		mustTupleType(t, IntT1, StrT1),
		NewRecordRowType(recordRow),
		NewVariantType(variantRow),
		mustSetType(t, mustSeqType(t, mustTupleType(t, model, BoolT1))),
	}
}

func requireActionOperation(t *testing.T, result *OperExpr, operator string, want Type, arguments ...Expr) {
	t.Helper()
	requireExprType(t, result, want)
	got := result.Arguments()
	if result.Operator().Name() != operator || len(got) != len(arguments) {
		t.Fatalf("result = %s/%d, want %s/%d", result.Operator(), len(got), operator, len(arguments))
	}
	for index := range arguments {
		if got[index] != arguments[index] {
			t.Fatalf("argument %d was not preserved", index)
		}
	}
}

func TestTypedActionBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			expression := builder.Name("e", typ)
			primed, err := builder.Prime(expression)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, primed, "PRIME", typ, expression)

			unchanged, err := builder.Unchanged(expression)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, unchanged, "UNCHANGED", BoolT1, expression)

			action := builder.Name("A", BoolT1)
			for _, test := range []struct {
				name     string
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"stutt", "STUTTER", builder.Stutt},
				{"nostutt", "NO_STUTTER", builder.NoStutt},
			} {
				result, buildErr := test.build(action, expression)
				if buildErr != nil {
					t.Fatalf("%s: %v", test.name, buildErr)
				}
				requireActionOperation(t, result, test.operator, BoolT1, action, expression)
				if _, buildErr = test.build(builder.Name("bad", StrT1), expression); buildErr == nil {
					t.Errorf("%s accepted a non-Boolean action", test.name)
				}
			}
		})
	}

	action := builder.Name("A", BoolT1)
	enabled, err := builder.Enabled(action)
	if err != nil {
		t.Fatal(err)
	}
	requireActionOperation(t, enabled, "ENABLED", BoolT1, action)
	if _, err = builder.Enabled(builder.Name("bad", StrT1)); err == nil {
		t.Fatal("Enabled accepted a non-Boolean action")
	}

	left, right := builder.Name("A", BoolT1), builder.Name("B", BoolT1)
	composition, err := builder.Comp(left, right)
	if err != nil {
		t.Fatal(err)
	}
	requireActionOperation(t, composition, "COMPOSE", BoolT1, left, right)
	if _, err = builder.Comp(builder.Name("bad", StrT1), right); err == nil {
		t.Fatal("Comp accepted an ill-typed left operand")
	}
	if _, err = builder.Comp(left, builder.Name("bad", StrT1)); err == nil {
		t.Fatal("Comp accepted an ill-typed right operand")
	}
}

func TestScopedActionBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			primed, err := builder.Prime(builder.Name("e", typ)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, primed, typ)
			if primed.(*OperExpr).Operator().Name() != "PRIME" {
				t.Fatalf("prime operator = %s", primed.(*OperExpr).Operator())
			}

			unchanged, err := builder.Unchanged(builder.Name("e", typ)).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, unchanged, BoolT1)
			if unchanged.(*OperExpr).Operator().Name() != "UNCHANGED" {
				t.Fatalf("unchanged operator = %s", unchanged.(*OperExpr).Operator())
			}

			for _, test := range []struct {
				name     string
				operator string
				build    func(ExprInstruction, ExprInstruction) ExprInstruction
			}{
				{"stutt", "STUTTER", builder.Stutt},
				{"nostutt", "NO_STUTTER", builder.NoStutt},
			} {
				result, buildErr := test.build(builder.Name("A", BoolT1), builder.Name("e", typ)).Build()
				if buildErr != nil {
					t.Fatalf("%s: %v", test.name, buildErr)
				}
				requireExprType(t, result, BoolT1)
				if result.(*OperExpr).Operator().Name() != test.operator {
					t.Fatalf("%s operator = %s", test.name, result.(*OperExpr).Operator())
				}
				if _, buildErr = test.build(builder.Name("bad", StrT1), builder.Name("e", typ)).Build(); buildErr == nil {
					t.Errorf("%s accepted a non-Boolean action", test.name)
				}
			}
		})
	}

	enabled, err := builder.Enabled(builder.Name("A", BoolT1)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, enabled, BoolT1)
	if enabled.(*OperExpr).Operator().Name() != "ENABLED" {
		t.Fatalf("enabled operator = %s", enabled.(*OperExpr).Operator())
	}
	if _, err = builder.Enabled(builder.Name("bad", StrT1)).Build(); err == nil {
		t.Fatal("Enabled accepted a non-Boolean action")
	}

	composition, err := builder.Comp(builder.Name("A", BoolT1), builder.Name("B", BoolT1)).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, composition, BoolT1)
	if composition.(*OperExpr).Operator().Name() != "COMPOSE" {
		t.Fatalf("composition operator = %s", composition.(*OperExpr).Operator())
	}
	if _, err = builder.Comp(builder.Name("bad", StrT1), builder.Name("B", BoolT1)).Build(); err == nil {
		t.Fatal("Comp accepted an ill-typed left operand")
	}
	if _, err = builder.Comp(builder.Name("A", BoolT1), builder.Name("bad", StrT1)).Build(); err == nil {
		t.Fatal("Comp accepted an ill-typed right operand")
	}
}
