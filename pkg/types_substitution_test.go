package apalache

import (
	"reflect"
	"testing"
)

func mustNumberedVar(t *testing.T, number int) VarType {
	t.Helper()
	variable, err := NewVarType(number)
	if err != nil {
		t.Fatal(err)
	}
	return variable
}

func mustNamedConst(t *testing.T, name string) ConstType {
	t.Helper()
	typ, err := NewConstType(name)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func mustSubstitution(t *testing.T, entries ...SubstitutionEntry) *Substitution {
	t.Helper()
	substitution, err := NewSubstitution(entries...)
	if err != nil {
		t.Fatal(err)
	}
	return substitution
}

func TestEquivalenceClassIdentityAndStructure(t *testing.T) {
	class := NewEqClass(4, 2, 4)
	copy := class.Copy()
	if class.ID() == copy.ID() || !class.DeepEqual(copy) || class.Representative() != 2 {
		t.Fatal("equivalence-class copy, identity, or representative is incorrect")
	}
	variables := class.Variables()
	variables[0] = 99
	if class.Representative() != 2 {
		t.Fatal("equivalence class exposed its variable storage")
	}
	class.SetVariables(7, 3)
	if class.Representative() != 3 || class.DeepEqual(copy) {
		t.Fatal("equivalence-class mutation did not recompute structure")
	}
}

func TestEquivalenceClassRequiresContentAndCopiesIndependently(t *testing.T) {
	assertPanics := func(name string, body func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s did not panic", name)
			}
		}()
		body()
	}
	assertPanics("empty constructor", func() { NewEqClass() })
	class := NewEqClass(5, 1, 5, 3)
	copy := class.Copy()
	if class == copy || class.ID() == copy.ID() || !class.DeepEqual(copy) ||
		!reflect.DeepEqual(class.Variables(), []int{1, 3, 5}) || class.Representative() != 1 {
		t.Fatalf("class/copy = %#v / %#v", class, copy)
	}
	class.SetVariables(8, 2)
	if class.Representative() != 2 || !reflect.DeepEqual(copy.Variables(), []int{1, 3, 5}) || class.DeepEqual(copy) {
		t.Fatalf("mutated class/copy = %#v / %#v", class, copy)
	}
	assertPanics("empty setter", func() { class.SetVariables() })
	if class.DeepEqual(nil) || (*EqClass)(nil).DeepEqual(class) || !(*EqClass)(nil).DeepEqual(nil) {
		t.Fatal("nil structural equality is inconsistent")
	}
}

func TestSubstitutionAppliesOnceAndRecursively(t *testing.T) {
	a := mustNumberedVar(t, 0)
	b := mustNumberedVar(t, 1)
	substitution := mustSubstitution(t,
		SubstitutionEntry{Class: NewEqClass(0), Type: b},
		SubstitutionEntry{Class: NewEqClass(1), Type: IntT1},
	)
	oneStep, changed, err := substitution.ApplyOnce(a)
	if err != nil || !changed || !EqualTypes(oneStep, b) {
		t.Fatalf("one-step substitution = %v, %v, %v", oneStep, changed, err)
	}
	recursive, err := substitution.ApplyRecursive(a)
	if err != nil || !EqualTypes(recursive, IntT1) {
		t.Fatalf("recursive substitution = %v, %v", recursive, err)
	}

	setOfA, _ := NewSetType(a)
	tuple, _ := NewTupleType(setOfA, b, StrT1)
	got, err := substitution.ApplyRecursive(tuple)
	wantSet, _ := NewSetType(IntT1)
	want, _ := NewTupleType(wantSet, IntT1, StrT1)
	if err != nil || !EqualTypes(got, want) {
		t.Fatalf("recursive composite substitution = %v, want %v: %v", got, want, err)
	}
}

func TestSubstitutionMergesRowTail(t *testing.T) {
	tail := mustNumberedVar(t, 3)
	nestedTail := mustNumberedVar(t, 4)
	row, _ := NewRowType(map[string]Type{"own": IntT1}, &tail)
	replacement, _ := NewRowType(map[string]Type{"inherited": BoolT1}, &nestedTail)
	substitution := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(3), Type: replacement})

	got, changed, err := substitution.ApplyOnce(row)
	want, _ := NewRowType(map[string]Type{"own": IntT1, "inherited": BoolT1}, &nestedTail)
	if err != nil || !changed || !EqualTypes(got, want) {
		t.Fatalf("row substitution = %v, %v, %v; want %v", got, changed, err, want)
	}

	bad := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(3), Type: StrT1})
	if _, _, err := bad.ApplyOnce(row); err == nil {
		t.Fatal("row tail accepted a non-row, non-variable replacement")
	}
}

func TestSubstitutionRejectsOverlappingClasses(t *testing.T) {
	_, err := NewSubstitution(
		SubstitutionEntry{Class: NewEqClass(0, 1), Type: IntT1},
		SubstitutionEntry{Class: NewEqClass(1, 2), Type: BoolT1},
	)
	if err == nil {
		t.Fatal("overlapping equivalence classes were accepted")
	}

	left := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(0, 2), Type: IntT1})
	right := mustSubstitution(t, SubstitutionEntry{Class: NewEqClass(2, 0), Type: IntT1})
	if !EqualSubstitutions(left, right) {
		t.Fatal("substitution equality depends on equivalence-class identity or order")
	}
}

func TestTypeVariablePool(t *testing.T) {
	pool := NewTypeVarPool(10)
	if got := pool.Fresh(); got.Number() != 10 {
		t.Fatalf("first variable = %v", got)
	}
	batch := pool.FreshN(3)
	if batch[0].Number() != 11 || batch[2].Number() != 13 || pool.Size() != 14 {
		t.Fatalf("fresh batch = %v, size = %d", batch, pool.Size())
	}
}

func TestModelValueHandling(t *testing.T) {
	tests := []struct {
		text    string
		isModel bool
		typ     Type
		name    string
	}{
		{"1_OF_A", true, mustNamedConst(t, "A"), "1"},
		{"2_OF_A", true, mustNamedConst(t, "A"), "2"},
		{"one_OF_B", true, mustNamedConst(t, "B"), "one"},
		{"two_OF_B", true, mustNamedConst(t, "B"), "two"},
		{"one_two_OF_PERSON_ID", true, mustNamedConst(t, "PERSON_ID"), "one_two"},
		{"1_OF_lowercase", false, StrT1, ""},
		{"ordinary", false, StrT1, ""},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			typ, name, ok := ParseModelValue(test.text)
			if ok != test.isModel || name != test.name || !EqualTypes(ModelValueOrStringType(test.text), test.typ) {
				t.Fatalf("parse = %v, %q, %v; selected type %v", typ, name, ok, ModelValueOrStringType(test.text))
			}
			if ok && ConstructModelValue(typ.Name(), name) != test.text {
				t.Fatal("model-value construction did not invert parsing")
			}
		})
	}
}
