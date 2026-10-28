package apalache

import (
	"reflect"
	"strings"
	"testing"
)

func TestSubstitutionCoversEveryCompositeTypeConstructor(t *testing.T) {
	a, b, c := mustNumberedVar(t, 0), mustNumberedVar(t, 1), mustNumberedVar(t, 2)
	substitution := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(0), Type: IntT1},
		SubstitutionEntry{Class: NewEqClass(1), Type: BoolT1},
	)
	set, _ := NewSetType(a)
	sequence, _ := NewSeqType(b)
	function, _ := NewFunType(a, b)
	tuple, _ := NewTupleType(a, set, sequence, function)
	sparse, _ := NewSparseTupleType(map[int]Type{1: a, 3: b, 7: c})
	record, _ := NewRecordType(map[string]Type{"a": a, "b": b, "c": c})
	operator, _ := NewOperatorType([]Type{set, function}, b)
	closedRow, _ := NewRowType(map[string]Type{"a": a, "b": b}, nil)
	recordRow := NewRecordRowType(closedRow)
	variant := NewVariantType(closedRow)

	wantSet, _ := NewSetType(IntT1)
	wantSequence, _ := NewSeqType(BoolT1)
	wantFunction, _ := NewFunType(IntT1, BoolT1)
	wantTuple, _ := NewTupleType(IntT1, wantSet, wantSequence, wantFunction)
	wantSparse, _ := NewSparseTupleType(map[int]Type{1: IntT1, 3: BoolT1, 7: c})
	wantRecord, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1, "c": c})
	wantOperator, _ := NewOperatorType([]Type{wantSet, wantFunction}, BoolT1)
	wantRow, _ := NewRowType(map[string]Type{"a": IntT1, "b": BoolT1}, nil)

	tests := []struct {
		name string
		in   Type
		want Type
	}{
		{"set", set, wantSet}, {"sequence", sequence, wantSequence}, {"function", function, wantFunction},
		{"tuple", tuple, wantTuple}, {"sparse tuple", sparse, wantSparse}, {"record", record, wantRecord},
		{"operator", operator, wantOperator}, {"row", closedRow, wantRow},
		{"record row", recordRow, NewRecordRowType(wantRow)}, {"variant", variant, NewVariantType(wantRow)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, changed, err := substitution.ApplyOnce(test.in)
			if err != nil || !changed || !EqualTypes(got, test.want) {
				t.Fatalf("substitution = %v, changed=%v, error=%v; want %v", got, changed, err, test.want)
			}
		})
	}
	for _, primitive := range []Type{IntT1, BoolT1, RealT1, StrT1, mustNamedConst(t, "PERSON"), c} {
		got, changed, err := substitution.ApplyOnce(primitive)
		if err != nil || changed || !EqualTypes(got, primitive) {
			t.Errorf("unchanged %v => %v, %v, %v", primitive, got, changed, err)
		}
	}
}

func TestSubstitutionOneStepDoesNotRevisitReplacement(t *testing.T) {
	a, b := mustNumberedVar(t, 0), mustNumberedVar(t, 1)
	substitution := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(0), Type: b},
		SubstitutionEntry{Class: NewEqClass(1), Type: IntT1},
	)
	one, changed, err := substitution.ApplyOnce(a)
	if err != nil || !changed || !EqualTypes(one, b) {
		t.Fatalf("one step = %v, %v, %v", one, changed, err)
	}
	fixed, err := substitution.ApplyRecursive(a)
	if err != nil || !EqualTypes(fixed, IntT1) {
		t.Fatalf("recursive = %v, %v", fixed, err)
	}
	self := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0), Type: a})
	if got, changed, err := self.ApplyOnce(a); err != nil || changed || !EqualTypes(got, a) {
		t.Fatalf("self substitution = %v, %v, %v", got, changed, err)
	}
}

func TestSubstitutionRowTailMergePrecedenceAndWrappers(t *testing.T) {
	tail := mustNumberedVar(t, 5)
	nestedTail := mustNumberedVar(t, 6)
	row, _ := NewRowType(map[string]Type{"shared": IntT1, "own": BoolT1}, &tail)
	replacement, _ := NewRowType(map[string]Type{"shared": StrT1, "inherited": RealT1}, &nestedTail)
	substitution := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(5), Type: replacement})
	want, _ := NewRowType(map[string]Type{
		"shared": IntT1, "own": BoolT1, "inherited": RealT1,
	}, &nestedTail)
	for _, input := range []Type{row, NewRecordRowType(row), NewVariantType(row)} {
		got, changed, err := substitution.ApplyOnce(input)
		if err != nil || !changed {
			t.Fatalf("%T substitution = %v, %v, %v", input, got, changed, err)
		}
		switch input.(type) {
		case RowType:
			if !EqualTypes(got, want) {
				t.Fatalf("row = %v, want %v", got, want)
			}
		case RecordRowType:
			if !EqualTypes(got, NewRecordRowType(want)) {
				t.Fatalf("record row = %v", got)
			}
		case VariantType:
			if !EqualTypes(got, NewVariantType(want)) {
				t.Fatalf("variant = %v", got)
			}
		}
	}
}

func TestSubstitutionOwnershipEqualityRenderingAndCycles(t *testing.T) {
	class := NewEqClass(2, 0)
	substitution := mustSubstitution(t, SubstitutionEntry{Class: class, Type: IntT1})
	class.SetVariables(9)
	entries := substitution.Entries()
	entries[0].Class.SetVariables(8)
	if !reflect.DeepEqual(substitution.Entries()[0].Class.Variables(), []int{0, 2}) {
		t.Fatal("substitution aliases constructor or returned equivalence classes")
	}
	equal := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 2), Type: IntT1})
	differentClass := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 3), Type: IntT1})
	differentType := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 2), Type: BoolT1})
	if !EqualSubstitutions(substitution, equal) || EqualSubstitutions(substitution, differentClass) ||
		EqualSubstitutions(substitution, differentType) || EqualSubstitutions(substitution, nil) || !EqualSubstitutions(nil, nil) {
		t.Fatal("substitution structural equality is incorrect")
	}
	if EmptySubstitution().String() != "Sub{}" || substitution.String() != "Sub{[a, c] -> Int}" {
		t.Fatalf("rendering = %q / %q", EmptySubstitution(), substitution)
	}
	if _, _, err := substitution.ApplyOnce(nil); err == nil {
		t.Fatal("nil type was accepted")
	}
	a, b := mustNumberedVar(t, 0), mustNumberedVar(t, 1)
	cycle := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(0), Type: b},
		SubstitutionEntry{Class: NewEqClass(1), Type: a},
	)
	if _, err := cycle.ApplyRecursive(a); err == nil || !strings.Contains(err.Error(), "more than 100000 iterations") {
		t.Fatalf("cycle error = %v", err)
	}
}
