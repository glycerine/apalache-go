package apalache

import "testing"

func TestCellTypeScalaSignaturesAndConversions(t *testing.T) {
	constant := mustNamedConst(t, "PID")
	closedRow := mustRowType(t, map[string]Type{"field": IntT1}, nil)
	tests := []struct {
		typ       Type
		signature string
	}{
		{constant, "UT_PID"},
		{StrT1, "UT_Str"},
		{BoolT1, "b"},
		{IntT1, "i"},
		{mustSetType(t, IntT1), "Si"},
		{mustSeqType(t, BoolT1), "Q_b"},
		{mustFunType(t, IntT1, StrT1), "fi_UT_Str"},
		{mustTupleType(t, IntT1, BoolT1, StrT1), "T_i_b_UT_Str"},
		{mustRecordType(t, map[string]Type{"x": IntT1}), "R"},
		{NewRecordRowType(closedRow), "U"},
		{NewVariantType(closedRow), "U"},
	}
	for _, test := range tests {
		cell, err := CellTypeFromType(test.typ)
		if err != nil || cell.Signature() != test.signature || !EqualTypes(cell.TLAType(), test.typ) {
			t.Errorf("CellTypeFromType(%v) = %v [%q], %v; want signature %q", test.typ, cell, cell.Signature(), err, test.signature)
		}
		fromTag, tagErr := CellTypeFromTag(Typed(test.typ))
		if tagErr != nil || !EqualCellTypes(fromTag, cell) {
			t.Errorf("CellTypeFromTag(%v) = %v, %v", test.typ, fromTag, tagErr)
		}
	}

	unknown := UnknownCellType{}
	if unknown.Signature() != "u" || unknown.String() != "Unknown" || unknown.TLAType().String() != "UNKNOWN" {
		t.Fatalf("unknown cell type = %q, %q, %v", unknown.Signature(), unknown.String(), unknown.TLAType())
	}
	if CompareCellTypes(BasicCellType{Type: BoolT1}, BasicCellType{Type: IntT1}) >= 0 || CompareCellTypes(unknown, unknown) != 0 {
		t.Fatal("cell type ordering does not use signatures")
	}
}

func TestCellTypeScalaRejectedTypeMatrix(t *testing.T) {
	variable := mustNumberedVar(t, 3)
	openRow := mustRowType(t, map[string]Type{"field": IntT1}, &variable)
	unsupported := []Type{
		RealT1,
		variable,
		mustSparseTupleType(t, map[int]Type{1: IntT1}),
		mustOperatorType(t, []Type{IntT1}, BoolT1),
		openRow,
		NewRecordRowType(openRow),
		NewVariantType(openRow),
	}
	for _, typ := range unsupported {
		if cell, err := CellTypeFromType(typ); err == nil {
			t.Errorf("CellTypeFromType(%v) = %v, want error", typ, cell)
		}
	}
	if _, err := CellTypeFromType(nil); err == nil {
		t.Fatal("nil type was accepted")
	}
	if _, err := CellTypeFromTag(Untyped()); err == nil {
		t.Fatal("untyped tag was accepted")
	}
}

func TestCellTypeScalaCollectionVariants(t *testing.T) {
	intCell := BasicCellType{Type: IntT1}
	infinite := InfiniteSetCellType{Element: intCell}
	if infinite.Signature() != "Zi" || infinite.String() != "InfSet[Int]" || !EqualTypes(infinite.TLAType(), mustSetType(t, IntT1)) {
		t.Fatalf("infinite set cell type = %v [%s] -> %v", infinite, infinite.Signature(), infinite.TLAType())
	}

	domain := BasicCellType{Type: mustSetType(t, IntT1)}
	power, err := NewPowerSetCellType(mustSetType(t, BoolT1))
	if err != nil || power.Signature() != "PSb" || power.String() != "PowSet[Set(Bool)]" || !EqualTypes(power.TLAType(), mustSetType(t, mustSetType(t, BoolT1))) {
		t.Fatalf("powerset cell type = %v [%s], %v", power, power.Signature(), err)
	}
	if _, err := NewPowerSetCellType(IntT1); err == nil {
		t.Fatal("powerset accepted a non-set domain")
	}

	functionSet, err := NewFiniteFunctionSetCellType(domain, power)
	if err != nil || functionSet.Signature() != "F%Si_%PSb" || functionSet.String() != "FinFunSet[Set(Int), PowSet[Set(Bool)]]" {
		t.Fatalf("function set = %v [%s], %v", functionSet, functionSet.Signature(), err)
	}
	if !EqualTypes(functionSet.ArgumentType(), IntT1) || !EqualTypes(functionSet.ResultType(), mustSetType(t, BoolT1)) {
		t.Fatalf("function-set endpoints = %v -> %v", functionSet.ArgumentType(), functionSet.ResultType())
	}
	want := mustSetType(t, mustFunType(t, IntT1, mustSetType(t, BoolT1)))
	if !EqualTypes(functionSet.TLAType(), want) {
		t.Fatalf("function-set TLA type = %v, want %v", functionSet.TLAType(), want)
	}

	if _, err := NewFiniteFunctionSetCellType(intCell, power); err == nil {
		t.Fatal("function set accepted a non-collection domain")
	}
	if _, err := NewFiniteFunctionSetCellType(domain, intCell); err == nil {
		t.Fatal("function set accepted a non-collection codomain")
	}
	nested, err := NewFiniteFunctionSetCellType(domain, functionSet)
	if err != nil || !EqualTypes(nested.ResultType(), mustFunType(t, IntT1, mustSetType(t, BoolT1))) {
		t.Fatalf("nested function-set codomain = %v, %v", nested, err)
	}
	withInfinite, err := NewFiniteFunctionSetCellType(domain, infinite)
	if err != nil || !EqualTypes(withInfinite.TLAType(), mustSetType(t, mustFunType(t, IntT1, IntT1))) {
		t.Fatalf("infinite-codomain function set = %v, %v", withInfinite, err)
	}
}

func TestEqualCellTypesEveryVariant(t *testing.T) {
	set := BasicCellType{Type: mustSetType(t, IntT1)}
	power, _ := NewPowerSetCellType(mustSetType(t, IntT1))
	functionSet, _ := NewFiniteFunctionSetCellType(set, power)
	types := []CellType{
		UnknownCellType{}, BasicCellType{Type: IntT1}, InfiniteSetCellType{Element: BasicCellType{Type: IntT1}},
		IntegerRangeCellType{}, power, functionSet,
	}
	for leftIndex, left := range types {
		for rightIndex, right := range types {
			if got, want := EqualCellTypes(left, right), leftIndex == rightIndex; got != want {
				t.Errorf("EqualCellTypes(%v, %v) = %v, want %v", left, right, got, want)
			}
		}
	}
}
