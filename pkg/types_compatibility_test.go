package apalache

import "testing"

func TestFlexibleTypeCompatibility(t *testing.T) {
	if !CompatibleTypes(IntT1, IntT1) || CompatibleTypes(IntT1, BoolT1) {
		t.Fatal("primitive compatibility is incorrect")
	}
	left, _ := NewRecordType(map[string]Type{"left": IntT1})
	right, _ := NewRecordType(map[string]Type{"right": BoolT1})
	want, _ := NewRecordType(map[string]Type{"left": IntT1, "right": BoolT1})
	got, ok, err := CommonSupertype(left, right)
	if err != nil || !ok || !EqualTypes(got, want) {
		t.Fatalf("record common supertype = %v, %v, %v; want %v", got, ok, err, want)
	}
	if got, ok, err := CommonSequenceSupertype([]Type{left, right, want}); err != nil || !ok || !EqualTypes(got, want) {
		t.Fatalf("sequence common supertype = %v, %v, %v", got, ok, err)
	}
	if _, ok, err := CommonSequenceSupertype(nil); err != nil || ok {
		t.Fatalf("empty sequence compatibility = %v, %v", ok, err)
	}
}
