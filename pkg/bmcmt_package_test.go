package apalache

import (
	"errors"
	"reflect"
	"testing"
)

func TestCrossMatchesScalaLeftMajorCartesianProduct(t *testing.T) {
	got := Cross([]int{1, 2}, []string{"a", "b"})
	want := []Pair[int, string]{{1, "a"}, {1, "b"}, {2, "a"}, {2, "b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Cross = %#v, want %#v", got, want)
	}
	if got := Cross([]int{}, []bool{true}); len(got) != 0 {
		t.Fatalf("empty Cross = %#v", got)
	}
}

func TestCellTheoryMatchesScalaPackageContract(t *testing.T) {
	theory := CellTheory{}
	cell := NewNameExpr("$C$12", Untyped())
	notCell := NewNameExpr("x", Untyped())
	if theory.String() != "Cell" || !theory.HasConst("$C$") || !theory.HasConst("$C$12") || theory.HasConst("C$12") {
		t.Fatal("cell theory naming convention mismatch")
	}
	if !TheoryHasNameExpr(theory, cell) || TheoryHasNameExpr(theory, notCell) ||
		TheoryHasNameExpr(theory, NewValueExpr(Int64Value(1), Typed(IntT1))) {
		t.Fatal("cell theory NameExpr recognition mismatch")
	}
	if got, err := TheoryNameExprToString(theory, cell); err != nil || got != "$C$12" {
		t.Fatalf("cell name = %q, %v", got, err)
	}
	_, err := TheoryNameExprToString(theory, notCell)
	var checker *CheckerException
	if !errors.As(err, &checker) || checker.CauseExpr() != notCell {
		t.Fatalf("invalid cell error = %#v", err)
	}
}
