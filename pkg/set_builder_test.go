package apalache

import "testing"

func TestTypedSetBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			setType := mustSetType(t, typ)
			for count := 1; count <= 5; count++ {
				elements := make([]Expr, count)
				for index := range elements {
					elements[index] = builder.Name("x", typ)
				}
				enumerated, err := builder.EnumSet(elements...)
				if err != nil {
					t.Fatal(err)
				}
				requireActionOperation(t, enumerated, "SET_ENUM", setType, elements...)
				if count > 1 {
					bad := append([]Expr(nil), elements...)
					bad[0] = builder.Name("bad", differentConcreteType(typ))
					if _, err = builder.EnumSet(bad...); err == nil {
						t.Fatal("EnumSet accepted heterogeneous elements")
					}
				}
			}
			requireActionOperation(t, builder.EmptySet(typ), "SET_ENUM", setType)

			element, set := builder.Name("x", typ), builder.Name("S", setType)
			for _, test := range []struct {
				name     string
				operator string
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"in", "SET_IN", builder.In},
				{"notin", "SET_NOT_IN", builder.NotIn},
			} {
				result, err := test.build(element, set)
				if err != nil {
					t.Fatal(err)
				}
				requireActionOperation(t, result, test.operator, BoolT1, element, set)
				if _, err = test.build(element, builder.Name("bad", IntT1)); err == nil {
					t.Errorf("%s accepted a non-set", test.name)
				}
				if _, err = test.build(builder.Name("bad", differentConcreteType(typ)), set); err == nil {
					t.Errorf("%s accepted an incompatible element", test.name)
				}
			}

			right := builder.Name("T", setType)
			for _, test := range []struct {
				operator string
				want     Type
				build    func(Expr, Expr) (*OperExpr, error)
			}{
				{"SET_INTERSECT", setType, builder.Cap},
				{"SET_UNION2", setType, builder.Cup},
				{"SET_MINUS", setType, builder.SetMinus},
				{"SET_SUBSET_EQ", BoolT1, builder.SubsetEq},
			} {
				result, err := test.build(set, right)
				if err != nil {
					t.Fatal(err)
				}
				requireActionOperation(t, result, test.operator, test.want, set, right)
				if _, err = test.build(builder.Name("bad", IntT1), right); err == nil {
					t.Errorf("%s accepted a non-set", test.operator)
				}
				if _, err = test.build(builder.Name("bad", mustSetType(t, differentConcreteType(typ))), right); err == nil {
					t.Errorf("%s accepted incompatible sets", test.operator)
				}
			}

			outer := builder.Name("SS", mustSetType(t, setType))
			union, err := builder.Union(outer)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, union, "SET_UNARY_UNION", setType, outer)
			if _, err = builder.Union(builder.Name("bad", IntT1)); err == nil {
				t.Fatal("Union accepted a set of non-sets")
			}

			for _, test := range []struct {
				operator string
				want     Type
				build    func(Expr) (*OperExpr, error)
			}{
				{"Sequences!Seq", mustSetType(t, mustSeqType(t, typ)), builder.SeqSet},
				{"SET_POWERSET", mustSetType(t, setType), builder.PowSet},
			} {
				result, err := test.build(set)
				if err != nil {
					t.Fatal(err)
				}
				requireActionOperation(t, result, test.operator, test.want, set)
				if _, err = test.build(builder.Int64(1)); err == nil {
					t.Errorf("%s accepted a non-set", test.operator)
				}
			}

			predicate := builder.Bool(true)
			filtered, err := builder.Filter(element, set, predicate)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, filtered, "SET_FILTER", setType, element, set, predicate)
			if _, err = builder.Filter(element, set, builder.Name("bad", StrT1)); err == nil {
				t.Fatal("Filter accepted a non-Boolean predicate")
			}
			if _, err = builder.Filter(builder.Int64(1), set, builder.Bool(true)); err == nil {
				t.Fatal("Filter accepted a non-variable binder")
			}
		})
	}

	if _, err := builder.EnumSet(); err == nil {
		t.Fatal("EnumSet accepted empty arity")
	}
	elements := make([]Expr, 10_001)
	for index := range elements {
		elements[index] = builder.Int64(int64(index - 5000))
	}
	if _, err := builder.EnumSet(elements...); err != nil {
		t.Fatalf("large EnumSet was not stack safe: %v", err)
	}
}

func TestTypedSetBuilderMapsFunctionSetsRecordsAndProducts(t *testing.T) {
	builder := NewTypedBuilder()
	x, y := builder.Name("x", IntT1), builder.Name("y", StrT1)
	intSet := builder.Name("S", mustSetType(t, IntT1))
	strSet := builder.Name("T", mustSetType(t, StrT1))
	body := builder.Name("e", BoolT1)
	domains := []SetMapDomain{{x, intSet}, {y, strSet}}
	mixed := []Expr{x, intSet, y, strSet}
	for _, build := range []func() (*OperExpr, error){
		func() (*OperExpr, error) { return builder.Map(body, domains...) },
		func() (*OperExpr, error) { return builder.MapMixed(body, mixed...) },
	} {
		mapped, err := build()
		if err != nil {
			t.Fatal(err)
		}
		requireActionOperation(t, mapped, "SET_MAP", mustSetType(t, BoolT1), append([]Expr{body}, mixed...)...)
	}
	if _, err := builder.Map(body); err == nil {
		t.Fatal("Map accepted no domains")
	}
	if _, err := builder.MapMixed(body, x); err == nil {
		t.Fatal("MapMixed accepted odd arity")
	}
	if _, err := builder.Map(body, SetMapDomain{x, intSet}, SetMapDomain{x, intSet}); err == nil {
		t.Fatal("Map accepted duplicate variables")
	}
	tuple := mustTupleExpr(t, builder, x, y)
	pairSet := builder.Name("Pairs", mustSetType(t, mustTupleType(t, IntT1, StrT1)))
	if _, err := builder.Map(body, SetMapDomain{tuple, pairSet}); err != nil {
		t.Fatalf("Map rejected a tuple binder: %v", err)
	}
	if _, err := builder.Map(body, SetMapDomain{builder.Int64(1), intSet}); err == nil {
		t.Fatal("Map accepted a non-variable binder")
	}

	funSet, err := builder.FunSet(intSet, strSet)
	if err != nil {
		t.Fatal(err)
	}
	requireActionOperation(t, funSet, "FUN_SET", mustSetType(t, mustFunType(t, IntT1, StrT1)), intSet, strSet)
	if _, err = builder.FunSet(intSet, builder.Int64(1)); err == nil {
		t.Fatal("FunSet accepted a non-set codomain")
	}

	keyA, _ := builder.Str("a")
	keyB, _ := builder.Str("b")
	recordFields := []RecordField{{"a", intSet}, {"b", strSet}}
	recordArgs := []Expr{keyA, intSet, keyB, strSet}
	wantRecord, _ := NewRecordType(map[string]Type{"a": IntT1, "b": StrT1})
	for _, build := range []func() (*OperExpr, error){
		func() (*OperExpr, error) { return builder.RecSet(recordFields...) },
		func() (*OperExpr, error) { return builder.RecSetMixed(recordArgs...) },
	} {
		recordSet, buildErr := build()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		requireExprType(t, recordSet, mustSetType(t, wantRecord))
		if recordSet.Operator().Name() != "RECORD_SET" || len(recordSet.Arguments()) != 4 {
			t.Fatal("record-set structure is wrong")
		}
	}
	if _, err = builder.RecSet(); err == nil {
		t.Fatal("RecSet accepted no fields")
	}
	if _, err = builder.RecSetMixed(); err == nil {
		t.Fatal("RecSetMixed accepted no fields")
	}
	if _, err = builder.RecSetMixed(keyA); err == nil {
		t.Fatal("RecSetMixed accepted odd arity")
	}
	if _, err = builder.RecSetMixed(builder.Name("a", StrT1), intSet); err == nil {
		t.Fatal("RecSetMixed accepted a non-literal key")
	}
	if _, err = builder.RecSetMixed(keyA, intSet, keyA, intSet); err == nil {
		t.Fatal("RecSetMixed accepted a duplicate key")
	}
	if _, err = builder.RecSetMixed(keyA, builder.Int64(1)); err == nil {
		t.Fatal("RecSetMixed accepted a non-set field")
	}

	product, err := builder.Times(intSet, strSet, builder.Name("B", mustSetType(t, BoolT1)))
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, product, mustSetType(t, mustTupleType(t, IntT1, StrT1, BoolT1)))
	if _, err = builder.Times(); err == nil {
		t.Fatal("Times accepted no sets")
	}
	if _, err = builder.Times(intSet); err == nil {
		t.Fatal("Times accepted one set")
	}
	if _, err = builder.Times(intSet, builder.Int64(1)); err == nil {
		t.Fatal("Times accepted a non-set")
	}
}

func mustTupleExpr(t *testing.T, builder *TypedBuilder, arguments ...Expr) Expr {
	t.Helper()
	result, err := builder.Tuple(arguments...)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestScopedSetBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	set := builder.EnumSet(builder.Int64(1), builder.Int64(2))
	for _, instruction := range []ExprInstruction{
		builder.In(builder.Int64(1), set),
		builder.NotIn(builder.Int64(1), set),
		builder.SubsetEq(set, set),
	} {
		result, err := instruction.Build()
		if err != nil {
			t.Fatal(err)
		}
		requireExprType(t, result, BoolT1)
	}
	for _, instruction := range []ExprInstruction{
		builder.Cap(set, set), builder.Cup(set, set), builder.SetMinus(set, set),
	} {
		result, err := instruction.Build()
		if err != nil {
			t.Fatal(err)
		}
		requireExprType(t, result, mustSetType(t, IntT1))
	}
	for _, test := range []struct {
		instruction ExprInstruction
		want        Type
	}{
		{builder.Union(builder.EnumSet(set)), mustSetType(t, IntT1)},
		{builder.SeqSet(set), mustSetType(t, mustSeqType(t, IntT1))},
		{builder.PowSet(set), mustSetType(t, mustSetType(t, IntT1))},
	} {
		result, err := test.instruction.Build()
		if err != nil {
			t.Fatal(err)
		}
		requireExprType(t, result, test.want)
	}

	filtered := builder.Filter(builder.Name("x", IntT1), set,
		builder.Apply(Op("GT"), builder.Name("x", IntT1), builder.Int64(0)))
	if _, err := filtered.Build(); err != nil {
		t.Fatal(err)
	}
	if _, err := builder.Filter(builder.Name("x", IntT1),
		builder.EnumSet(builder.Name("x", IntT1)), builder.Bool(true)).Build(); err == nil {
		t.Fatal("Filter accepted its bound variable in its domain")
	}

	mapped := builder.Map(builder.Name("x", IntT1), BoundDomain{builder.Name("x", IntT1), set})
	result, err := mapped.Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, result, mustSetType(t, IntT1))
	if _, err = builder.Map(builder.Int64(1)).Build(); err == nil {
		t.Fatal("Map accepted no domains")
	}
	if _, err = builder.MapMixed(builder.Int64(1), builder.Name("x", IntT1)).Build(); err == nil {
		t.Fatal("MapMixed accepted odd arity")
	}
	duplicate := builder.Map(builder.Int64(1),
		BoundDomain{builder.Name("x", IntT1), set}, BoundDomain{builder.Name("x", IntT1), set})
	if _, err = duplicate.Build(); err == nil {
		t.Fatal("Map accepted duplicate variables")
	}

	recordSet, err := builder.RecSet(ScopedRecordField{"a", set}).Build()
	if err != nil {
		t.Fatal(err)
	}
	wantRecord, _ := NewRecordType(map[string]Type{"a": IntT1})
	requireExprType(t, recordSet, mustSetType(t, wantRecord))
	if _, err = builder.RecSet().Build(); err == nil {
		t.Fatal("scoped RecSet accepted no fields")
	}
	if _, err = builder.Times(set).Build(); err == nil {
		t.Fatal("scoped Times accepted one set")
	}
}
