package apalache

import "testing"

func TestTypeAliasSubstitutionEveryTypeConstructor(t *testing.T) {
	a := mustNamedConst(t, "A")
	tail := mustNumberedVar(t, 7)
	row := mustRowType(t, map[string]Type{"value": a}, &tail)

	cases := []struct {
		name string
		typ  Type
		want Type
	}{
		{"constant", a, BoolT1},
		{"set", mustSetType(t, a), mustSetType(t, BoolT1)},
		{"sequence", mustSeqType(t, a), mustSeqType(t, BoolT1)},
		{"function", mustFunType(t, a, a), mustFunType(t, BoolT1, BoolT1)},
		{"tuple", mustTupleType(t, a, IntT1), mustTupleType(t, BoolT1, IntT1)},
		{"sparse tuple", mustSparseTupleType(t, map[int]Type{3: a}), mustSparseTupleType(t, map[int]Type{3: BoolT1})},
		{"record", mustRecordType(t, map[string]Type{"f": a}), mustRecordType(t, map[string]Type{"f": BoolT1})},
		{"operator", mustOperatorType(t, []Type{a, IntT1}, a), mustOperatorType(t, []Type{BoolT1, IntT1}, BoolT1)},
		{"row", row, mustRowType(t, map[string]Type{"value": BoolT1}, &tail)},
		{"row record", NewRecordRowType(row), NewRecordRowType(mustRowType(t, map[string]Type{"value": BoolT1}, &tail))},
		{"variant", NewVariantType(row), NewVariantType(mustRowType(t, map[string]Type{"value": BoolT1}, &tail))},
	}
	substitution := NewTypeAliasSubstitution(map[string]Type{"A": BoolT1})
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, changed, err := substitution.Apply(test.typ)
			if err != nil || !changed || !EqualTypes(got, test.want) {
				t.Fatalf("Apply(%v) = %v, %v, %v; want %v, true, nil", test.typ, got, changed, err, test.want)
			}
		})
	}

	for _, typ := range []Type{IntT1, RealT1, BoolT1, StrT1, tail, mustNamedConst(t, "UNINTERPRETED")} {
		got, changed, err := substitution.Apply(typ)
		if err != nil || changed || !EqualTypes(got, typ) {
			t.Errorf("unchanged Apply(%v) = %v, %v, %v", typ, got, changed, err)
		}
	}
}

func TestTypeAliasSubstitutionValueAndOwnershipContracts(t *testing.T) {
	context := map[string]Type{"B": BoolT1, "A": IntT1}
	substitution := NewTypeAliasSubstitution(context)
	context["A"] = StrT1
	delete(context, "B")
	if got := substitution.String(); got != "ConstSub{A -> Int, B -> Bool}" {
		t.Fatalf("String() = %q", got)
	}

	exposed := substitution.Context()
	exposed["A"] = StrT1
	delete(exposed, "B")
	if !substitution.Equal(NewTypeAliasSubstitution(map[string]Type{"A": IntT1, "B": BoolT1})) {
		t.Fatal("constructor or Context exposed mutable map ownership")
	}
	if substitution.Equal(NewTypeAliasSubstitution(map[string]Type{"A": IntT1})) || substitution.Equal(nil) {
		t.Fatal("Equal accepted unequal context or nil")
	}
	var nilSubstitution *TypeAliasSubstitution
	if !nilSubstitution.Equal(nil) {
		t.Fatal("nil substitutions should compare equal")
	}
	if got := EmptyTypeAliasSubstitution().String(); got != "ConstSub{}" {
		t.Fatalf("empty String() = %q", got)
	}
}

func TestTypeAliasSubstitutionNestedMissingAliasAndCycles(t *testing.T) {
	missing := mustNamedConst(t, "$missingAlias")
	nested := mustOperatorType(t, []Type{mustSetType(t, missing)}, NewVariantType(mustRowType(t, map[string]Type{"Tag": missing}, nil)))
	if _, _, err := EmptyTypeAliasSubstitution().Apply(nested); err == nil {
		t.Fatal("nested missing camel-case alias was accepted")
	}

	a := mustNamedConst(t, "A")
	b := mustNamedConst(t, "B")
	cycle := NewTypeAliasSubstitution(map[string]Type{
		"A": NewRecordRowType(mustRowType(t, map[string]Type{"next": b}, nil)),
		"B": NewVariantType(mustRowType(t, map[string]Type{"Next": a}, nil)),
	})
	if _, err := cycle.Closure(); err == nil {
		t.Fatal("nested row/variant alias cycle was accepted")
	}
}
