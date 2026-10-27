package apalache

import (
	"strings"
	"testing"
)

func TestTypeAliasSubstitutionScalaCases(t *testing.T) {
	a, _ := NewConstType("A")
	setA, _ := NewSetType(a)
	substitution := NewTypeAliasSubstitution(map[string]Type{"A": IntT1})
	result, changed, err := substitution.Apply(setA)
	if err != nil || !changed || !EqualTypes(result, mustSetType(t, IntT1)) {
		t.Fatalf("Apply() = %v, %v, %v", result, changed, err)
	}

	camel, _ := NewConstType("$camlCase")
	result, changed, err = NewTypeAliasSubstitution(map[string]Type{"$camlCase": IntT1}).Apply(camel)
	if err != nil || !changed || !EqualTypes(result, IntT1) {
		t.Fatalf("camel-case Apply() = %v, %v, %v", result, changed, err)
	}
	missing, _ := NewConstType("$camlCaseMissing")
	if _, _, err = substitution.Apply(missing); err == nil || !strings.Contains(err.Error(), "missing @typeAlias") {
		t.Fatalf("missing alias error = %v", err)
	}
}

func TestTypeAliasSubstitutionClosure(t *testing.T) {
	a, _ := NewConstType("A")
	b, _ := NewConstType("B")
	c, _ := NewConstType("C")
	seqB, _ := NewSeqType(b)
	setC, _ := NewSetType(c)
	substitution := NewTypeAliasSubstitution(map[string]Type{"A": seqB, "B": setC, "C": StrT1})
	closure, err := substitution.Closure()
	if err != nil {
		t.Fatal(err)
	}
	input, _ := NewTupleType(a, b, c)
	result, changed, err := closure.Apply(input)
	want := parsedType(t, "<<Seq(Set(Str)), Set(Str), Str>>")
	if err != nil || !changed || !EqualTypes(result, want) {
		t.Fatalf("closure Apply() = %v, %v, %v; want %v", result, changed, err, want)
	}
}

func TestTypeAliasSubstitutionRejectsCycle(t *testing.T) {
	a, _ := NewConstType("A")
	b, _ := NewConstType("B")
	_, err := NewTypeAliasSubstitution(map[string]Type{"A": b, "B": a}).Closure()
	if err == nil || !strings.Contains(err.Error(), "cyclic dependency") {
		t.Fatalf("cycle error = %v", err)
	}
}

func TestTypeAliasSubstitutionTraversesRowsAndVariants(t *testing.T) {
	a, _ := NewConstType("A")
	tail, _ := NewVarType(4)
	row, _ := NewRowType(map[string]Type{"value": a}, &tail)
	variant := NewVariantType(row)
	result, changed, err := NewTypeAliasSubstitution(map[string]Type{"A": BoolT1}).Apply(variant)
	if err != nil || !changed {
		t.Fatalf("Apply() = %v, %v, %v", result, changed, err)
	}
	wantRow, _ := NewRowType(map[string]Type{"value": BoolT1}, &tail)
	if !EqualTypes(result, NewVariantType(wantRow)) {
		t.Fatalf("got %v, want %v", result, NewVariantType(wantRow))
	}
}
