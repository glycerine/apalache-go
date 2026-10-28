package apalache

import "testing"

func requireTemporalOperation(t *testing.T, result *OperExpr, operator string, arguments ...Expr) {
	t.Helper()
	requireActionOperation(t, result, operator, BoolT1, arguments...)
}

func TestTypedTemporalBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	predicate := builder.Name("P", BoolT1)
	for _, test := range []struct {
		name     string
		operator string
		build    func(Expr) (*OperExpr, error)
	}{
		{"box", "GLOBALLY", builder.Box},
		{"diamond", "EVENTUALLY", builder.Diamond},
	} {
		result, err := test.build(predicate)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		requireTemporalOperation(t, result, test.operator, predicate)
		if _, err = test.build(builder.Name("bad", StrT1)); err == nil {
			t.Errorf("%s accepted a non-Boolean predicate", test.name)
		}
	}

	left, right := builder.Name("P", BoolT1), builder.Name("Q", BoolT1)
	for _, test := range []struct {
		name     string
		operator string
		build    func(Expr, Expr) (*OperExpr, error)
	}{
		{"leadsTo", "LEADS_TO", builder.LeadsTo},
		{"guarantees", "GUARANTEES", builder.Guarantees},
	} {
		result, err := test.build(left, right)
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		requireTemporalOperation(t, result, test.operator, left, right)
		if _, err = test.build(builder.Name("bad", StrT1), right); err == nil {
			t.Errorf("%s accepted an ill-typed left predicate", test.name)
		}
		if _, err = test.build(left, builder.Name("bad", StrT1)); err == nil {
			t.Errorf("%s accepted an ill-typed right predicate", test.name)
		}
	}

	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			expression := builder.Name("x", typ)
			action := builder.Name("A", BoolT1)
			for _, test := range []struct {
				name     string
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"WF", "WEAK_FAIRNESS", builder.WF},
				{"SF", "STRONG_FAIRNESS", builder.SF},
			} {
				result, err := test.build(expression, action)
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				requireTemporalOperation(t, result, test.operator, expression, action)
				if _, err = test.build(expression, builder.Name("bad", StrT1)); err == nil {
					t.Errorf("%s accepted a non-Boolean action", test.name)
				}
			}
			for _, test := range []struct {
				name     string
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"AA", "TEMPORAL_FORALL", builder.AA},
				{"EE", "TEMPORAL_EXISTS", builder.EE},
			} {
				result, err := test.build(expression, action)
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				requireTemporalOperation(t, result, test.operator, expression, action)
				if _, err = test.build(expression, builder.Name("bad", StrT1)); err == nil {
					t.Errorf("%s accepted a non-Boolean predicate", test.name)
				}
				if _, err = test.build(builder.Int64(1), action); err == nil {
					t.Errorf("%s accepted a non-name variable", test.name)
				}
			}
		})
	}
}

func TestScopedTemporalBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	for _, test := range []struct {
		name     string
		operator string
		build    func(ExprInstruction) ExprInstruction
	}{
		{"box", "GLOBALLY", builder.Box},
		{"diamond", "EVENTUALLY", builder.Diamond},
	} {
		result, err := test.build(builder.Name("P", BoolT1)).Build()
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		requireExprType(t, result, BoolT1)
		if result.(*OperExpr).Operator().Name() != test.operator {
			t.Fatalf("%s operator = %s", test.name, result.(*OperExpr).Operator())
		}
		if _, err = test.build(builder.Name("bad", StrT1)).Build(); err == nil {
			t.Errorf("%s accepted a non-Boolean predicate", test.name)
		}
	}

	for _, test := range []struct {
		name     string
		operator string
		build    func(ExprInstruction, ExprInstruction) ExprInstruction
	}{
		{"leadsTo", "LEADS_TO", builder.LeadsTo},
		{"guarantees", "GUARANTEES", builder.Guarantees},
	} {
		result, err := test.build(builder.Name("P", BoolT1), builder.Name("Q", BoolT1)).Build()
		if err != nil {
			t.Fatalf("%s: %v", test.name, err)
		}
		requireExprType(t, result, BoolT1)
		if result.(*OperExpr).Operator().Name() != test.operator {
			t.Fatalf("%s operator = %s", test.name, result.(*OperExpr).Operator())
		}
		if _, err = test.build(builder.Name("bad", StrT1), builder.Name("Q", BoolT1)).Build(); err == nil {
			t.Errorf("%s accepted an ill-typed left predicate", test.name)
		}
		if _, err = test.build(builder.Name("P", BoolT1), builder.Name("bad", StrT1)).Build(); err == nil {
			t.Errorf("%s accepted an ill-typed right predicate", test.name)
		}
	}

	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			for _, test := range []struct {
				name     string
				operator string
				build    func(ExprInstruction, ExprInstruction) ExprInstruction
			}{
				{"WF", "WEAK_FAIRNESS", builder.WF},
				{"SF", "STRONG_FAIRNESS", builder.SF},
				{"AA", "TEMPORAL_FORALL", builder.AA},
				{"EE", "TEMPORAL_EXISTS", builder.EE},
			} {
				result, err := test.build(builder.Name("x", typ), builder.Name("P", BoolT1)).Build()
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				requireExprType(t, result, BoolT1)
				if result.(*OperExpr).Operator().Name() != test.operator {
					t.Fatalf("%s operator = %s", test.name, result.(*OperExpr).Operator())
				}
				if _, err = test.build(builder.Name("x", typ), builder.Name("bad", StrT1)).Build(); err == nil {
					t.Errorf("%s accepted a non-Boolean predicate/action", test.name)
				}
			}
		})
	}
}

func TestScopedTemporalQuantifiersRejectInvalidBinding(t *testing.T) {
	builder := NewScopedBuilder()
	for _, test := range []struct {
		name  string
		build func(ExprInstruction, ExprInstruction) ExprInstruction
	}{
		{"AA", builder.AA},
		{"EE", builder.EE},
	} {
		t.Run(test.name, func(t *testing.T) {
			stringValue := builder.String("x")
			if _, err := test.build(stringValue, builder.Bool(true)).Build(); err == nil {
				t.Fatal("accepted a non-name variable")
			}
			tuple := builder.Tuple(builder.Name("x", StrT1), builder.Name("y", IntT1))
			if _, err := test.build(tuple, builder.Bool(true)).Build(); err == nil {
				t.Fatal("accepted a tuple variable")
			}
			conflictingUse := builder.Apply(Op("EQ"), builder.Name("x", IntT1), builder.Name("x", IntT1))
			if _, err := test.build(builder.Name("x", StrT1), conflictingUse).Build(); err == nil {
				t.Fatal("accepted a variable with conflicting free-use type")
			}
			inner := builder.EE(builder.Name("x", StrT1), builder.Bool(true))
			if _, err := test.build(builder.Name("x", StrT1), inner).Build(); err == nil {
				t.Fatal("accepted nested variable shadowing")
			}
		})
	}
}
