package apalache

import "testing"

func scalaUnifier() *TypeUnifier { return NewTypeUnifier(NewTypeVarPool(100)) }

func TestTypeUnifierScalaMonotypeMatrix(t *testing.T) {
	for _, text := range []string{"F", "Int", "Bool", "Str", "Real", "Int -> Int", "(Int, Bool) => Int", "<<Int, Bool>>"} {
		typ := parsedType(t, text)
		assertUnifies(t, scalaUnifier(), EmptySubstitution(), typ, typ, EmptySubstitution(), typ)
	}
	for _, pair := range [][2]string{
		{"F", "Bool"}, {"F", "G"}, {"Int", "Bool"}, {"Bool", "Str"}, {"Str", "Int"}, {"Real", "Int"},
		{"Int -> Int", "Bool -> Int"}, {"(Int, Bool) => Int", "(Bool, Int) => Int"},
		{"<<Int, Bool>>", "<<Int, Str>>"}, {"<<Int, Str>>", "<<Str, Int>>"},
		{"<| 2: Int, 4: Bool |>", "<| 2: Int, 4: Int |>"},
		{"<<Str, Int>>", "<| 2: Int, 4: Bool |>"}, {"[foo: Bool, bar: Int]", "[foo: Bool, bar: Bool]"},
	} {
		assertDoesNotUnify(t, scalaUnifier(), parsedType(t, pair[0]), parsedType(t, pair[1]))
	}

	assertUnifies(t, scalaUnifier(), EmptySubstitution(),
		parsedType(t, "<| 2: Int, 4: Bool |>"), parsedType(t, "<| 3: Str |>"), EmptySubstitution(),
		parsedType(t, "<| 2: Int, 3: Str, 4: Bool |>"))
	for _, pair := range [][2]string{{"<<Int, Bool>>", "<| 1: Int |>"}, {"<| 1: Int |>", "<<Int, Bool>>"}} {
		assertUnifies(t, scalaUnifier(), EmptySubstitution(), parsedType(t, pair[0]), parsedType(t, pair[1]), EmptySubstitution(), parsedType(t, "<<Int, Bool>>"))
	}
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), parsedType(t, "[foo: Bool, bar: Int]"), parsedType(t, "[baz: Str]"), EmptySubstitution(), parsedType(t, "[foo: Bool, bar: Int, baz: Str]"))
}

func TestTypeUnifierScalaPolytypeMatrix(t *testing.T) {
	a, b, c := mustNumberedVar(t, 0), mustNumberedVar(t, 1), mustNumberedVar(t, 2)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), a, IntT1,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: IntT1}), IntT1)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), a, mustNamedConst(t, "ID"),
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: mustNamedConst(t, "ID")}), mustNamedConst(t, "ID"))
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), mustNamedConst(t, "ID"), a,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: mustNamedConst(t, "ID")}), mustNamedConst(t, "ID"))

	assertUnifies(t, scalaUnifier(), EmptySubstitution(), a, a,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: a}), a)
	initialA := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: IntT1})
	assertUnifies(t, scalaUnifier(), initialA, a, a, initialA, IntT1)
	merged := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 1), Type: a})
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), a, b, merged, a)
	cycleMerged := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 1, 2), Type: a})
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), mustTupleType(t, a, b, c), mustTupleType(t, b, c, a), cycleMerged, mustTupleType(t, a, a, a))

	wantTransitive := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(1, 2), Type: mustNamedConst(t, "PERSON")})
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), parsedType(t, "Set(b) -> Set(b)"), parsedType(t, "Set(c) -> Set(PERSON)"), wantTransitive, parsedType(t, "Set(PERSON) -> Set(PERSON)"))
	assertUnifies(t, scalaUnifier(), EmptySubstitution(),
		mustOperatorType(t, []Type{a}, mustSetType(t, a)), mustOperatorType(t, []Type{IntT1}, b),
		mustSubstitution(t,
			SubstitutionEntry{Class: NewEqClass(0), Type: IntT1},
			SubstitutionEntry{Class: NewEqClass(1), Type: mustSetType(t, a)},
		), mustOperatorType(t, []Type{IntT1}, mustSetType(t, IntT1)))
	assertDoesNotUnify(t, scalaUnifier(), a, mustSetType(t, a))
}

func TestTypeUnifierScalaInitialSubstitutionRegressions(t *testing.T) {
	v0 := mustNumberedVar(t, 0)
	v1000 := mustNumberedVar(t, 1000)
	v1001 := mustNumberedVar(t, 1001)
	v1003 := mustNumberedVar(t, 1003)
	v1004 := mustNumberedVar(t, 1004)
	v1005 := mustNumberedVar(t, 1005)
	v1006 := mustNumberedVar(t, 1006)

	viaSets := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(1003), Type: mustSetType(t, v0)},
		SubstitutionEntry{Class: NewEqClass(1004), Type: mustSetType(t, v1005)},
	)
	wantSets := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(1003, 1004), Type: mustSetType(t, v0)},
		SubstitutionEntry{Class: NewEqClass(0, 1005), Type: v0},
	)
	assertUnifies(t, scalaUnifier(), viaSets, v1004, v1003, wantSets, mustSetType(t, v0))

	viaOperators := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(1000, 1004), Type: v1000},
		SubstitutionEntry{Class: NewEqClass(1005), Type: mustOperatorType(t, []Type{v1000}, v1001)},
		SubstitutionEntry{Class: NewEqClass(1006), Type: v1000},
	)
	wantOperators := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(1000, 1001, 1004, 1006), Type: v1000},
		SubstitutionEntry{Class: NewEqClass(1005), Type: mustOperatorType(t, []Type{v1000}, v1000)},
	)
	right := mustOperatorType(t, []Type{v1004}, v1006)
	wantType := mustOperatorType(t, []Type{v1000}, v1000)
	assertUnifies(t, scalaUnifier(), viaOperators, v1005, right, wantOperators, wantType)

	mutual := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(0), Type: mustNumberedVar(t, 1)},
		SubstitutionEntry{Class: NewEqClass(1), Type: v0},
	)
	assertUnifies(t, scalaUnifier(), mutual, v0, mustNumberedVar(t, 1),
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 1), Type: v0}), v0)
}

func TestTypeUnifierScalaRowMatrix(t *testing.T) {
	c, d := mustNumberedVar(t, 2), mustNumberedVar(t, 3)
	empty := mustRowType(t, nil, nil)
	rowD := mustRowType(t, nil, &d)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), empty, rowD,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(3), Type: empty}), empty)
	assertDoesNotUnify(t, scalaUnifier(), empty, mustRowType(t, map[string]Type{"field1": IntT1}, nil))

	fieldD := mustRowType(t, map[string]Type{"field1": IntT1}, &d)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), mustRowType(t, nil, &c), fieldD,
		mustSubstitution(t,
			SubstitutionEntry{Class: NewEqClass(2), Type: fieldD},
			SubstitutionEntry{Class: NewEqClass(3), Type: d},
		), fieldD)

	assertUnifies(t, scalaUnifier(), EmptySubstitution(), mustRowType(t, nil, &c), rowD,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(2, 3), Type: c}), mustRowType(t, nil, &c))
	incompatibleInitial := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(2), Type: mustRowType(t, map[string]Type{"field1": IntT1}, nil)},
		SubstitutionEntry{Class: NewEqClass(3), Type: mustRowType(t, map[string]Type{"field1": BoolT1}, nil)},
	)
	if _, _, ok, err := scalaUnifier().Unify(incompatibleInitial, mustRowType(t, nil, &c), rowD); err != nil || ok {
		t.Fatalf("incompatible row variables = ok %v, err %v", ok, err)
	}

	closed := mustRowType(t, map[string]Type{"field1": IntT1, "field2": StrT1}, nil)
	partial := mustRowType(t, map[string]Type{"field2": StrT1}, &d)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), closed, partial,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(3), Type: mustRowType(t, map[string]Type{"field1": IntT1}, nil)}), closed)
	assertDoesNotUnify(t, scalaUnifier(),
		mustRowType(t, map[string]Type{"field1": IntT1}, nil), mustRowType(t, map[string]Type{"field1": StrT1}, nil))
	assertDoesNotUnify(t, scalaUnifier(),
		mustRowType(t, map[string]Type{"shared": BoolT1, "field1": IntT1}, nil),
		mustRowType(t, map[string]Type{"shared": BoolT1, "field2": StrT1}, nil))
}

func TestTypeUnifierScalaRecordAndVariantMatrix(t *testing.T) {
	c, d, fresh := mustNumberedVar(t, 2), mustNumberedVar(t, 3), mustNumberedVar(t, 100)
	leftRecord := NewRecordRowType(mustRowType(t, map[string]Type{"field1": IntT1}, &c))
	rightRecord := NewRecordRowType(mustRowType(t, map[string]Type{"field2": StrT1}, &d))
	wantRecord := NewRecordRowType(mustRowType(t, map[string]Type{"field1": IntT1, "field2": StrT1}, &fresh))
	wantRecordSub := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(2), Type: mustRowType(t, map[string]Type{"field2": StrT1}, &fresh)},
		SubstitutionEntry{Class: NewEqClass(3), Type: mustRowType(t, map[string]Type{"field1": IntT1}, &fresh)},
		SubstitutionEntry{Class: NewEqClass(100), Type: fresh},
	)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), leftRecord, rightRecord, wantRecordSub, wantRecord)

	partialRecord := NewRecordRowType(mustRowType(t, map[string]Type{"field2": c}, &d))
	completeRecord := NewRecordRowType(mustRowType(t, map[string]Type{"field2": StrT1, "field3": BoolT1}, nil))
	wantPartialSub := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(2), Type: StrT1},
		SubstitutionEntry{Class: NewEqClass(3), Type: mustRowType(t, map[string]Type{"field3": BoolT1}, nil)},
	)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), partialRecord, completeRecord, wantPartialSub, completeRecord)
	assertDoesNotUnify(t, scalaUnifier(), NewRecordRowType(mustRowType(t, map[string]Type{"field1": IntT1}, &c)), completeRecord)
	assertDoesNotUnify(t, scalaUnifier(),
		NewRecordRowType(mustRowType(t, map[string]Type{"field1": IntT1}, nil)),
		NewRecordRowType(mustRowType(t, map[string]Type{"field1": StrT1}, nil)))

	leftVariant := NewVariantType(mustRowType(t, map[string]Type{"Tag1": NewRecordRowType(mustRowType(t, map[string]Type{"field1": IntT1}, nil))}, &c))
	rightVariant := NewVariantType(mustRowType(t, map[string]Type{"Tag2": NewRecordRowType(mustRowType(t, map[string]Type{"field2": StrT1}, nil))}, &d))
	wantVariant := NewVariantType(mustRowType(t, map[string]Type{
		"Tag1": NewRecordRowType(mustRowType(t, map[string]Type{"field1": IntT1}, nil)),
		"Tag2": NewRecordRowType(mustRowType(t, map[string]Type{"field2": StrT1}, nil)),
	}, &fresh))
	wantVariantSub := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(2), Type: mustRowType(t, map[string]Type{"Tag2": rightVariant.Row().Fields()["Tag2"]}, &fresh)},
		SubstitutionEntry{Class: NewEqClass(3), Type: mustRowType(t, map[string]Type{"Tag1": leftVariant.Row().Fields()["Tag1"]}, &fresh)},
		SubstitutionEntry{Class: NewEqClass(100), Type: fresh},
	)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), leftVariant, rightVariant, wantVariantSub, wantVariant)
	differentSameField := NewVariantType(mustRowType(t, map[string]Type{
		"Tag2": NewRecordRowType(mustRowType(t, map[string]Type{"field1": StrT1}, nil)),
	}, &d))
	wantDifferentSameField := NewVariantType(mustRowType(t, map[string]Type{
		"Tag1": leftVariant.Row().Fields()["Tag1"],
		"Tag2": differentSameField.Row().Fields()["Tag2"],
	}, &fresh))
	wantDifferentSameFieldSub := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(2), Type: mustRowType(t, map[string]Type{"Tag2": differentSameField.Row().Fields()["Tag2"]}, &fresh)},
		SubstitutionEntry{Class: NewEqClass(3), Type: mustRowType(t, map[string]Type{"Tag1": leftVariant.Row().Fields()["Tag1"]}, &fresh)},
		SubstitutionEntry{Class: NewEqClass(100), Type: fresh},
	)
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), leftVariant, differentSameField, wantDifferentSameFieldSub, wantDifferentSameField)

	unknown := NewVariantType(mustRowType(t, nil, &c))
	assertUnifies(t, scalaUnifier(), EmptySubstitution(), unknown, rightVariant,
		mustSubstitution(t,
			SubstitutionEntry{Class: NewEqClass(2), Type: rightVariant.Row()},
			SubstitutionEntry{Class: NewEqClass(3), Type: d},
		), rightVariant)
	assertDoesNotUnify(t, scalaUnifier(),
		NewVariantType(mustRowType(t, map[string]Type{"Tag1": IntT1}, nil)),
		NewVariantType(mustRowType(t, map[string]Type{"Tag2": StrT1}, nil)))
}

func TestTypeUnifierScalaNoStackOverflow925(t *testing.T) {
	a := mustNumberedVar(t, 0)
	left := mustOperatorType(t, []Type{a}, mustRecordType(t, map[string]Type{"f": a}))
	right := mustOperatorType(t, []Type{a}, mustRecordType(t, map[string]Type{"f": mustSetType(t, a)}))
	assertDoesNotUnify(t, scalaUnifier(), left, right)

	// A failed call must not leak its partially-mutated classes into reuse.
	reused := scalaUnifier()
	assertDoesNotUnify(t, reused, left, right)
	assertUnifies(t, reused, EmptySubstitution(), a, IntT1,
		mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: IntT1}), IntT1)
}
