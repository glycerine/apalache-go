package apalache

import "testing"

func mustSetType(t *testing.T, element Type) SetType {
	t.Helper()
	result, err := NewSetType(element)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustTupleType(t *testing.T, elements ...Type) TupleType {
	t.Helper()
	result, err := NewTupleType(elements...)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustRowType(t *testing.T, fields map[string]Type, tail *VarType) RowType {
	t.Helper()
	result, err := NewRowType(fields, tail)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func assertUnifies(t *testing.T, unifier *TypeUnifier, initial *Substitution, left, right Type, wantSub *Substitution, wantType Type) {
	t.Helper()
	gotSub, gotType, ok, err := unifier.Unify(initial, left, right)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatalf("%v and %v did not unify", left, right)
	}
	if !EqualSubstitutions(gotSub, wantSub) || !EqualTypes(gotType, wantType) {
		t.Fatalf("unify(%v, %v) = (%v, %v), want (%v, %v)", left, right, gotSub.Entries(), gotType, wantSub.Entries(), wantType)
	}
}

func assertDoesNotUnify(t *testing.T, unifier *TypeUnifier, left, right Type) {
	t.Helper()
	_, _, ok, err := unifier.Unify(EmptySubstitution(), left, right)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatalf("%v and %v unexpectedly unified", left, right)
	}
}

func TestUnifierMonotypesAndWidening(t *testing.T) {
	unifier := NewTypeUnifier(NewTypeVarPool(100))
	assertUnifies(t, unifier, EmptySubstitution(), IntT1, IntT1, EmptySubstitution(), IntT1)
	assertDoesNotUnify(t, unifier, IntT1, BoolT1)

	leftSparse, _ := NewSparseTupleType(map[int]Type{2: IntT1, 4: BoolT1})
	rightSparse, _ := NewSparseTupleType(map[int]Type{3: StrT1})
	wantSparse, _ := NewSparseTupleType(map[int]Type{2: IntT1, 3: StrT1, 4: BoolT1})
	assertUnifies(t, unifier, EmptySubstitution(), leftSparse, rightSparse, EmptySubstitution(), wantSparse)

	tuple := mustTupleType(t, IntT1, BoolT1)
	sparseFirst, _ := NewSparseTupleType(map[int]Type{1: IntT1})
	assertUnifies(t, unifier, EmptySubstitution(), sparseFirst, tuple, EmptySubstitution(), tuple)
	outOfBounds, _ := NewSparseTupleType(map[int]Type{3: IntT1})
	assertDoesNotUnify(t, unifier, tuple, outOfBounds)

	leftRecord, _ := NewRecordType(map[string]Type{"foo": BoolT1})
	rightRecord, _ := NewRecordType(map[string]Type{"bar": IntT1})
	wantRecord, _ := NewRecordType(map[string]Type{"foo": BoolT1, "bar": IntT1})
	assertUnifies(t, unifier, EmptySubstitution(), leftRecord, rightRecord, EmptySubstitution(), wantRecord)
}

func TestUnifierVariablesCanonicalizeToMinimum(t *testing.T) {
	unifier := NewTypeUnifier(NewTypeVarPool(100))
	a := mustNumberedVar(t, 0)
	b := mustNumberedVar(t, 1)
	want := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 1), Type: a})
	assertUnifies(t, unifier, EmptySubstitution(), a, b, want, a)

	left := mustTupleType(t, a, b)
	right := mustTupleType(t, b, a)
	wantTuple := mustTupleType(t, a, a)
	assertUnifies(t, unifier, EmptySubstitution(), left, right, want, wantTuple)
}

func TestUnifierRecordFieldApplication(t *testing.T) {
	unifier := NewTypeUnifier(NewTypeVarPool(100))
	result := mustNumberedVar(t, 10)
	field := mustNumberedVar(t, 11)
	full, _ := NewRecordType(map[string]Type{"ack": IntT1, "rdy": IntT1, "val": mustConst(t, "DATUM")})
	selected, _ := NewRecordType(map[string]Type{"rdy": field})
	actual, _ := NewOperatorType([]Type{full, StrT1}, result)
	signature, _ := NewOperatorType([]Type{selected, StrT1}, field)
	_, unified, ok, err := unifier.Unify(EmptySubstitution(), actual, signature)
	if err != nil || !ok {
		t.Fatalf("Unify() = %v, %v, %v", unified, ok, err)
	}
}

func TestUnifierCrossRecordFamiliesPreserveConcreteValueRepresentation(t *testing.T) {
	full, _ := NewRecordType(map[string]Type{"ack": IntT1, "rdy": IntT1})
	field := mustNumberedVar(t, 11)
	tail := mustNumberedVar(t, 12)
	row, _ := NewRowType(map[string]Type{"rdy": field}, &tail)
	selection := NewRecordRowType(row)
	for _, pair := range [][2]Type{{full, selection}, {selection, full}} {
		_, unified, ok, err := NewTypeUnifier(NewTypeVarPool(100)).Unify(EmptySubstitution(), pair[0], pair[1])
		if err != nil || !ok {
			t.Fatalf("Unify(%s, %s) = %v, %v, %v", pair[0], pair[1], unified, ok, err)
		}
		if _, legacy := unified.(RecordType); !legacy {
			t.Fatalf("Unify(%s, %s) returned %T, want RecordType", pair[0], pair[1], unified)
		}
	}
}

func TestUnifierVariablesAndOccursCheck(t *testing.T) {
	unifier := NewTypeUnifier(NewTypeVarPool(100))
	a := mustNumberedVar(t, 0)
	b := mustNumberedVar(t, 1)
	leftFun, _ := NewFunType(a, IntT1)
	rightFun, _ := NewFunType(BoolT1, b)
	wantFun, _ := NewFunType(BoolT1, IntT1)
	wantSub := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(0), Type: BoolT1},
		SubstitutionEntry{Class: NewEqClass(1), Type: IntT1},
	)
	assertUnifies(t, unifier, EmptySubstitution(), leftFun, rightFun, wantSub, wantFun)

	setOfA := mustSetType(t, a)
	assertDoesNotUnify(t, unifier, a, setOfA)
}

func TestUnifierHonorsInitialSubstitution(t *testing.T) {
	unifier := NewTypeUnifier(NewTypeVarPool(100))
	a := mustNumberedVar(t, 0)
	b := mustNumberedVar(t, 1)
	initial := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(1), Type: IntT1})
	want := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 1), Type: IntT1})
	assertUnifies(t, unifier, initial, a, b, want, IntT1)
}

func TestUnifierRows(t *testing.T) {
	c := mustNumberedVar(t, 2)
	d := mustNumberedVar(t, 3)
	fresh := mustNumberedVar(t, 100)
	left := mustRowType(t, map[string]Type{"field1": IntT1, "field2": StrT1}, &c)
	right := mustRowType(t, map[string]Type{"field3": BoolT1}, &d)
	wantLeftTail := mustRowType(t, map[string]Type{"field3": BoolT1}, &fresh)
	wantRightTail := mustRowType(t, map[string]Type{"field1": IntT1, "field2": StrT1}, &fresh)
	wantSub := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(2), Type: wantLeftTail},
		SubstitutionEntry{Class: NewEqClass(3), Type: wantRightTail},
		SubstitutionEntry{Class: NewEqClass(100), Type: fresh},
	)
	wantRow := mustRowType(t, map[string]Type{"field1": IntT1, "field2": StrT1, "field3": BoolT1}, &fresh)
	assertUnifies(t, NewTypeUnifier(NewTypeVarPool(100)), EmptySubstitution(), left, right, wantSub, wantRow)

	closedLeft := mustRowType(t, map[string]Type{"shared": BoolT1, "field1": IntT1}, nil)
	closedRight := mustRowType(t, map[string]Type{"shared": BoolT1, "field2": StrT1}, nil)
	assertDoesNotUnify(t, NewTypeUnifier(NewTypeVarPool(100)), closedLeft, closedRight)

	partial := mustRowType(t, map[string]Type{"field2": StrT1}, &d)
	closed := mustRowType(t, map[string]Type{"field1": IntT1, "field2": StrT1, "field3": BoolT1}, nil)
	wantTail := mustRowType(t, map[string]Type{"field1": IntT1, "field3": BoolT1}, nil)
	wantPartialSub := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(3), Type: wantTail})
	assertUnifies(t, NewTypeUnifier(NewTypeVarPool(100)), EmptySubstitution(), partial, closed, wantPartialSub, closed)
}

func TestUnifierOperatorArityAndVariants(t *testing.T) {
	leftOperator, _ := NewOperatorType([]Type{IntT1}, BoolT1)
	rightOperator, _ := NewOperatorType([]Type{IntT1, IntT1}, BoolT1)
	assertDoesNotUnify(t, NewTypeUnifier(NewTypeVarPool(100)), leftOperator, rightOperator)

	closed := mustRowType(t, map[string]Type{"A": IntT1}, nil)
	leftVariant := NewVariantType(closed)
	rightVariant := NewVariantType(mustRowType(t, map[string]Type{"A": BoolT1}, nil))
	assertDoesNotUnify(t, NewTypeUnifier(NewTypeVarPool(100)), leftVariant, rightVariant)
}
