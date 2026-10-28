package apalache

import (
	"reflect"
	"testing"
)

func mustVar(t *testing.T, name string) VarType {
	t.Helper()
	value, err := ParseVarType(name)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustConst(t *testing.T, name string) ConstType {
	t.Helper()
	value, err := NewConstType(name)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func mustType[T Type](value T, err error) T {
	if err != nil {
		panic(err)
	}
	return value
}

func TestTypeStringsMatchScala(t *testing.T) {
	a := mustVar(t, "a")
	z := mustVar(t, "z")
	rec1 := NewRecordRowType(mustType(NewRowType(map[string]Type{"f": IntT1}, nil)))
	rec2 := NewRecordRowType(mustType(NewRowType(map[string]Type{"g": BoolT1}, nil)))

	tests := []struct {
		name string
		typ  Type
		want string
	}{
		{"int", IntT1, "Int"},
		{"real", RealT1, "Real"},
		{"bool", BoolT1, "Bool"},
		{"str", StrT1, "Str"},
		{"constant", mustConst(t, "PID"), "PID"},
		{"var a", a, "a"},
		{"var z", z, "z"},
		{"set", mustType(NewSetType(a)), "Set(a)"},
		{"sequence", mustType(NewSeqType(IntT1)), "Seq(Int)"},
		{"function", mustType(NewFunType(IntT1, BoolT1)), "(Int -> Bool)"},
		{"tuple", mustType(NewTupleType(IntT1, BoolT1, StrT1)), "<<Int, Bool, Str>>"},
		{"sparse tuple", mustType(NewSparseTupleType(map[int]Type{3: StrT1, 1: IntT1})), "<| 1: Int, 3: Str |>"},
		{"legacy record", mustType(NewRecordType(map[string]Type{"f3": StrT1, "f1": IntT1, "f2": BoolT1})), "[f1: Int, f2: Bool, f3: Str]"},
		{"operator", mustType(NewOperatorType([]Type{BoolT1, StrT1}, IntT1)), "((Bool, Str) => Int)"},
		{"empty row", mustType(NewRowType(nil, nil)), "(||)"},
		{"open row", mustType(NewRowType(map[string]Type{"f": IntT1}, &z)), "(| f: Int | z |)"},
		{"closed row", mustType(NewRowType(map[string]Type{"f": IntT1}, nil)), "(| f: Int |)"},
		{"sorted row", mustType(NewRowType(map[string]Type{"g": BoolT1, "f": IntT1}, &z)), "(| f: Int | g: Bool | z |)"},
		{"open record", NewRecordRowType(mustType(NewRowType(map[string]Type{"f": IntT1}, &z))), "{ f: Int, z }"},
		{"closed record", NewRecordRowType(mustType(NewRowType(map[string]Type{"g": a, "f": IntT1}, nil))), "{ f: Int, g: a }"},
		{"open variant", NewVariantType(mustType(NewRowType(map[string]Type{"tag1": rec1}, &z))), "tag1({ f: Int }) | z"},
		{"sorted variant", NewVariantType(mustType(NewRowType(map[string]Type{"tag2": rec2, "tag1": rec1}, &z))), "tag1({ f: Int }) | tag2({ g: Bool }) | z"},
		{"closed variant", NewVariantType(mustType(NewRowType(map[string]Type{"tag2": rec2, "tag1": rec1}, nil))), "tag1({ f: Int }) | tag2({ g: Bool })"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.typ.String(); got != test.want {
				t.Fatalf("String() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestVarTypeParsingMatchesScala(t *testing.T) {
	valid := map[string]int{"a": 0, "z": 25, "a26": 26, "a100": 100, "a01": 1}
	for text, want := range valid {
		got, err := ParseVarType(text)
		if err != nil || got.Number() != want {
			t.Errorf("ParseVarType(%q) = (%v, %v), want %d", text, got, err, want)
		}
		if !IsValidVarTypeName(text) {
			t.Errorf("IsValidVarTypeName(%q) = false", text)
		}
	}
	for _, text := range []string{"", "A", "aa", "b12", "a-1", "a2147483648"} {
		if _, err := ParseVarType(text); err == nil {
			t.Errorf("ParseVarType(%q) succeeded", text)
		}
		if IsValidVarTypeName(text) {
			t.Errorf("IsValidVarTypeName(%q) = true", text)
		}
	}
}

func TestConstTypeValidation(t *testing.T) {
	for _, name := range []string{"PID", "PROC_NAME", "_A1", "$aliasReference", "$"} {
		if _, err := NewConstType(name); err != nil {
			t.Errorf("NewConstType(%q): %v", name, err)
		}
	}
	for _, name := range []string{"", "Pid", "pid", "1PID"} {
		if _, err := NewConstType(name); err == nil {
			t.Errorf("NewConstType(%q) succeeded", name)
		}
	}
	if got, err := AliasName("$messages"); err != nil || got != "messages" {
		t.Fatalf("AliasName() = (%q, %v)", got, err)
	}
}

func TestUsedNamesAndMonotype(t *testing.T) {
	a := mustVar(t, "a")
	c := mustVar(t, "c")
	row := mustType(NewRowType(map[string]Type{
		"fixed": IntT1,
		"items": mustType(NewSetType(a)),
	}, &c))
	typ := NewVariantType(row)
	if got, want := typ.UsedNames(), []int{0, 2}; !reflect.DeepEqual(got, want) {
		t.Fatalf("UsedNames() = %v, want %v", got, want)
	}
	if typ.IsMono() {
		t.Fatal("IsMono() = true for polymorphic type")
	}
	mono := mustType(NewFunType(IntT1, mustType(NewSetType(mustConst(t, "PID")))))
	if !mono.IsMono() || len(mono.UsedNames()) != 0 {
		t.Fatalf("monotype = %v, names %v", mono.IsMono(), mono.UsedNames())
	}
}

func TestTypesAreStructurallyEqualAndDefensivelyCopied(t *testing.T) {
	fields := map[string]Type{"b": BoolT1, "a": IntT1}
	one := mustType(NewRecordType(fields))
	fields["a"] = StrT1
	two := mustType(NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1}))
	if !EqualTypes(one, two) {
		t.Fatalf("EqualTypes(%s, %s) = false", one, two)
	}
	copy := one.Fields()
	copy["a"] = StrT1
	if !EqualTypes(one, two) {
		t.Fatal("mutating Fields() result changed record")
	}
	three := mustType(NewRecordType(map[string]Type{"a": StrT1, "b": BoolT1}))
	if EqualTypes(one, three) {
		t.Fatalf("EqualTypes(%s, %s) = true", one, three)
	}
}

func TestTypeConstructorsRejectNil(t *testing.T) {
	if _, err := NewSetType(nil); err == nil {
		t.Fatal("NewSetType(nil) succeeded")
	}
	if _, err := NewFunType(IntT1, nil); err == nil {
		t.Fatal("NewFunType(_, nil) succeeded")
	}
	if _, err := NewRecordType(map[string]Type{"bad": nil}); err == nil {
		t.Fatal("NewRecordType() accepted nil field")
	}
}

func TestTypeFromTagAndSchemeMatchScala(t *testing.T) {
	if got, err := TypeFromTag(Typed(IntT1)); err != nil || !EqualTypes(got, IntT1) {
		t.Fatalf("TypeFromTag = %v, %v", got, err)
	}
	if _, err := TypeFromTag(Untyped()); err == nil {
		t.Fatal("TypeFromTag accepted Untyped")
	}
	a := mustVar(t, "a")
	c := mustVar(t, "c")
	principal := mustType(NewFunType(a, mustType(NewSetType(c))))
	quantified := []int{2, 0, 2}
	scheme, err := NewTypeScheme(principal, quantified...)
	if err != nil || !EqualTypes(scheme.Principal(), principal) || !reflect.DeepEqual(scheme.Quantified(), []int{0, 2}) {
		t.Fatalf("scheme = %v, %v, %v", scheme.Principal(), scheme.Quantified(), err)
	}
	quantified[0] = 99
	copy := scheme.Quantified()
	copy[0] = 99
	if !reflect.DeepEqual(scheme.Quantified(), []int{0, 2}) {
		t.Fatal("type scheme exposed quantified variables")
	}
	if _, err := NewTypeScheme(principal, -1); err == nil {
		t.Fatal("negative quantified variable was accepted")
	}
	if _, err := NewTypeScheme(nil); err == nil {
		t.Fatal("nil principal type was accepted")
	}
}
