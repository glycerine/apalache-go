package apalache

import "testing"

func TestTypedBooleanBuilderExactScalaLogicCases(t *testing.T) {
	builder := NewTypedBuilder()
	for count := 0; count <= 10; count++ {
		arguments := make([]Expr, count)
		for index := range arguments {
			arguments[index] = builder.Name("x", BoolT1)
		}
		for _, test := range []struct {
			name     string
			operator string
			build    func(...Expr) (*OperExpr, error)
		}{
			{"and", "AND", builder.And},
			{"or", "OR", builder.Or},
		} {
			result, err := test.build(arguments...)
			if err != nil {
				t.Fatalf("%s/%d: %v", test.name, count, err)
			}
			requireActionOperation(t, result, test.operator, BoolT1, arguments...)
			for badIndex := range arguments {
				bad := append([]Expr(nil), arguments...)
				bad[badIndex] = builder.Name("bad", StrT1)
				if _, err = test.build(bad...); err == nil {
					t.Errorf("%s/%d accepted Str at %d", test.name, count, badIndex)
				}
			}
		}
	}

	predicate := builder.Name("p", BoolT1)
	negated, err := builder.Not(predicate)
	if err != nil {
		t.Fatal(err)
	}
	requireActionOperation(t, negated, "NOT", BoolT1, predicate)
	if _, err = builder.Not(builder.Name("bad", StrT1)); err == nil {
		t.Fatal("Not accepted a non-Boolean predicate")
	}

	left, right := builder.Name("p", BoolT1), builder.Name("q", BoolT1)
	for _, test := range []struct {
		name     string
		operator string
		build    func(Expr, Expr) (*OperExpr, error)
	}{
		{"impl", "IMPLIES", builder.Impl},
		{"equiv", "EQUIV", builder.Equiv},
	} {
		result, buildErr := test.build(left, right)
		if buildErr != nil {
			t.Fatalf("%s: %v", test.name, buildErr)
		}
		requireActionOperation(t, result, test.operator, BoolT1, left, right)
		if _, buildErr = test.build(builder.Name("bad", StrT1), right); buildErr == nil {
			t.Errorf("%s accepted an ill-typed left predicate", test.name)
		}
		if _, buildErr = test.build(left, builder.Name("bad", StrT1)); buildErr == nil {
			t.Errorf("%s accepted an ill-typed right predicate", test.name)
		}
	}
}

func booleanQuantifierTypes(t *testing.T) []Type {
	t.Helper()
	types := append([]Type(nil), concreteActionBuilderTypes(t)...)
	types = append(types,
		mustTupleType(t, IntT1, StrT1),
		mustTupleType(t, mustSetType(t, BoolT1), mustSeqType(t, IntT1), mustNamedConst(t, "A")),
	)
	return types
}

func typedBoundVariable(builder *TypedBuilder, typ Type) Expr {
	if tuple, ok := typ.(TupleType); ok {
		elements := tuple.Elements()
		arguments := make([]Expr, len(elements))
		for index, element := range elements {
			arguments[index] = builder.Name("x"+string(rune('a'+index)), element)
		}
		result, _ := builder.Tuple(arguments...)
		return result
	}
	return builder.Name("x", typ)
}

func scopedBoundVariable(builder *ScopedBuilder, typ Type) ExprInstruction {
	if tuple, ok := typ.(TupleType); ok {
		elements := tuple.Elements()
		arguments := make([]ExprInstruction, len(elements))
		for index, element := range elements {
			arguments[index] = builder.Name("x"+string(rune('a'+index)), element)
		}
		return builder.Tuple(arguments...)
	}
	return builder.Name("x", typ)
}

func TestTypedBooleanBuilderExactScalaQuantifierCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range booleanQuantifierTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			variable := typedBoundVariable(builder, typ)
			set := builder.Name("set", mustSetType(t, typ))
			predicate := builder.Name("p", BoolT1)
			for _, test := range []struct {
				name     string
				operator string
				build    func(Expr, Expr, Expr) (*OperExpr, error)
			}{
				{"forall", "FORALL3", builder.ForallIn},
				{"exists", "EXISTS3", builder.ExistsIn},
			} {
				result, err := test.build(variable, set, predicate)
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				requireActionOperation(t, result, test.operator, BoolT1, variable, set, predicate)
				if _, err = test.build(builder.Name("bad", differentConcreteType(typ)), set, predicate); err == nil {
					t.Errorf("%s accepted an incompatible variable", test.name)
				}
				if _, err = test.build(variable, builder.Name("bad", mustSetType(t, differentConcreteType(typ))), predicate); err == nil {
					t.Errorf("%s accepted an incompatible set", test.name)
				}
				if _, err = test.build(variable, builder.Name("bad", IntT1), predicate); err == nil {
					t.Errorf("%s accepted a non-set", test.name)
				}
				if _, err = test.build(variable, set, builder.Name("bad", StrT1)); err == nil {
					t.Errorf("%s accepted a non-Boolean predicate", test.name)
				}
			}
			for _, test := range []struct {
				name     string
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"forall", "FORALL2", builder.Forall},
				{"exists", "EXISTS2", builder.Exists},
			} {
				result, err := test.build(variable, predicate)
				if err != nil {
					t.Fatalf("%s: %v", test.name, err)
				}
				requireActionOperation(t, result, test.operator, BoolT1, variable, predicate)
				if _, err = test.build(variable, builder.Name("bad", StrT1)); err == nil {
					t.Errorf("%s accepted a non-Boolean predicate", test.name)
				}
			}
		})
	}

	badTuple, _ := builder.Tuple(builder.Name("x", IntT1), builder.Name("x", IntT1))
	for _, build := range []func(Expr, Expr) (*OperExpr, error){builder.Forall, builder.Exists} {
		if _, err := build(builder.Int64(1), builder.Bool(true)); err == nil {
			t.Fatal("unbounded quantifier accepted a non-name variable")
		}
		if _, err := build(badTuple, builder.Bool(true)); err == nil {
			t.Fatal("unbounded quantifier accepted duplicate tuple variables")
		}
	}
}

func TestScopedBooleanBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	for count := 0; count <= 10; count++ {
		arguments := make([]ExprInstruction, count)
		for index := range arguments {
			arguments[index] = builder.Name("x", BoolT1)
		}
		for _, test := range []struct {
			operator string
			build    func(...ExprInstruction) ExprInstruction
		}{{"AND", builder.And}, {"OR", builder.Or}} {
			result, err := test.build(arguments...).Build()
			if err != nil {
				t.Fatal(err)
			}
			requireExprType(t, result, BoolT1)
			if result.(*OperExpr).Operator().Name() != test.operator {
				t.Fatalf("operator = %s", result.(*OperExpr).Operator())
			}
			for badIndex := range arguments {
				bad := append([]ExprInstruction(nil), arguments...)
				bad[badIndex] = builder.Name("bad", StrT1)
				if _, err = test.build(bad...).Build(); err == nil {
					t.Errorf("%s/%d accepted Str at %d", test.operator, count, badIndex)
				}
			}
		}
	}
	if _, err := builder.Not(builder.Name("p", BoolT1)).Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Not(builder.Name("bad", StrT1)).Build(); err == nil {
		t.Fatal("Not accepted a non-Boolean predicate")
	}
	for _, build := range []func(ExprInstruction, ExprInstruction) ExprInstruction{builder.Impl, builder.Equiv} {
		if _, err := build(builder.Name("p", BoolT1), builder.Name("q", BoolT1)).Build(); err != nil {
			t.Fatal(err)
		}
		if _, err := build(builder.Name("bad", StrT1), builder.Name("q", BoolT1)).Build(); err == nil {
			t.Fatal("binary Boolean builder accepted an ill-typed left predicate")
		}
		if _, err := build(builder.Name("p", BoolT1), builder.Name("bad", StrT1)).Build(); err == nil {
			t.Fatal("binary Boolean builder accepted an ill-typed right predicate")
		}
	}

	for _, typ := range booleanQuantifierTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			for _, build := range []func(ExprInstruction, ExprInstruction, ExprInstruction) ExprInstruction{
				builder.ForallIn, builder.ExistsIn,
			} {
				if _, err := build(scopedBoundVariable(builder, typ), builder.Name("S", mustSetType(t, typ)), builder.Name("p", BoolT1)).Build(); err != nil {
					t.Fatal(err)
				}
				if _, err := build(scopedBoundVariable(builder, typ), builder.Name("bad", IntT1), builder.Name("p", BoolT1)).Build(); err == nil {
					t.Fatal("bounded quantifier accepted a non-set")
				}
				if _, err := build(scopedBoundVariable(builder, typ), builder.Name("S", mustSetType(t, typ)), builder.Name("bad", StrT1)).Build(); err == nil {
					t.Fatal("bounded quantifier accepted a non-Boolean predicate")
				}
			}
			for _, build := range []func(ExprInstruction, ExprInstruction) ExprInstruction{builder.Forall, builder.Exists} {
				if _, err := build(scopedBoundVariable(builder, typ), builder.Name("p", BoolT1)).Build(); err != nil {
					t.Fatal(err)
				}
				if _, err := build(scopedBoundVariable(builder, typ), builder.Name("bad", StrT1)).Build(); err == nil {
					t.Fatal("unbounded quantifier accepted a non-Boolean predicate")
				}
			}
		})
	}
}

func TestScopedBooleanQuantifiersRejectInvalidBindings(t *testing.T) {
	builder := NewScopedBuilder()
	bounded := []func(ExprInstruction, ExprInstruction, ExprInstruction) ExprInstruction{builder.ForallIn, builder.ExistsIn}
	for _, build := range bounded {
		if _, err := build(builder.String("x"), builder.Name("S", mustSetType(t, StrT1)), builder.Bool(true)).Build(); err == nil {
			t.Fatal("accepted a non-name variable")
		}
		duplicate := builder.Tuple(builder.Name("x", StrT1), builder.Name("x", StrT1))
		if _, err := build(duplicate, builder.Name("S", mustSetType(t, mustTupleType(t, StrT1, StrT1))), builder.Bool(true)).Build(); err == nil {
			t.Fatal("accepted duplicate tuple variables")
		}
		if _, err := build(builder.Name("x", StrT1), builder.Name("S", mustSetType(t, StrT1)), builder.Apply(Op("EQ"), builder.Name("x", IntT1), builder.Name("x", IntT1))).Build(); err == nil {
			t.Fatal("accepted a conflicting free-use type")
		}
		if _, err := build(builder.Name("x", StrT1), builder.Apply(Op("SET_ENUM"), builder.Name("x", StrT1)), builder.Bool(true)).Build(); err == nil {
			t.Fatal("accepted a variable used in its own domain")
		}
		inner := builder.ExistsIn(builder.Name("x", StrT1), builder.Name("T", mustSetType(t, StrT1)), builder.Bool(true))
		if _, err := build(builder.Name("x", StrT1), builder.Name("S", mustSetType(t, StrT1)), inner).Build(); err == nil {
			t.Fatal("accepted nested variable shadowing")
		}
	}

	unbounded := []func(ExprInstruction, ExprInstruction) ExprInstruction{builder.Forall, builder.Exists}
	for _, build := range unbounded {
		if _, err := build(builder.String("x"), builder.Bool(true)).Build(); err == nil {
			t.Fatal("accepted a non-name variable")
		}
		duplicate := builder.Tuple(builder.Name("x", StrT1), builder.Name("x", StrT1))
		if _, err := build(duplicate, builder.Bool(true)).Build(); err == nil {
			t.Fatal("accepted duplicate tuple variables")
		}
		if _, err := build(builder.Name("x", StrT1), builder.Apply(Op("EQ"), builder.Name("x", IntT1), builder.Name("x", IntT1))).Build(); err == nil {
			t.Fatal("accepted a conflicting free-use type")
		}
		inner := builder.ExistsIn(builder.Name("x", StrT1), builder.Name("S", mustSetType(t, StrT1)), builder.Bool(true))
		if _, err := build(builder.Name("x", StrT1), inner).Build(); err == nil {
			t.Fatal("accepted nested variable shadowing")
		}
	}
}
