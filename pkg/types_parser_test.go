package apalache

import "testing"

func TestTypeParserScalaExamples(t *testing.T) {
	tests := []struct {
		text string
		want Type
	}{
		{"Int", IntT1},
		{"Real", RealT1},
		{"Bool", BoolT1},
		{"Str", StrT1},
		{"a", mustNumberedVar(t, 0)},
		{"PID", mustNamedConst(t, "PID")},
		{"Set(Int)", mustSetType(t, IntT1)},
		{"Seq(Int)", mustSeqType(t, IntT1)},
		{"<<Bool>>", mustTupleType(t, BoolT1)},
		{"<<Bool, Int>>", mustTupleType(t, BoolT1, IntT1)},
		{"(Bool, Int)", mustTupleType(t, BoolT1, IntT1)},
		{"Set(Int) -> Bool", mustFunType(t, mustSetType(t, IntT1), BoolT1)},
		{"(Bool, Int) -> Str", mustFunType(t, mustTupleType(t, BoolT1, IntT1), StrT1)},
		{"Set(Int) -> Bool -> Str", mustFunType(t, mustSetType(t, IntT1), mustFunType(t, BoolT1, StrT1))},
		{"(Set(Int) -> Bool) -> Str", mustFunType(t, mustFunType(t, mustSetType(t, IntT1), BoolT1), StrT1)},
		{"(Set(Int), Bool) => Str", mustOperatorType(t, []Type{mustSetType(t, IntT1), BoolT1}, StrT1)},
		{"(Set(Int) => Bool) => Str", mustOperatorType(t, []Type{mustOperatorType(t, []Type{mustSetType(t, IntT1)}, BoolT1)}, StrT1)},
		{"() => Int", mustOperatorType(t, nil, IntT1)},
		{"() => (Bool, Int)", mustOperatorType(t, nil, mustTupleType(t, BoolT1, IntT1))},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			got, err := ParseType(test.text)
			if err != nil || !EqualTypes(got, test.want) {
				t.Fatalf("ParseType() = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func mustSeqType(t *testing.T, element Type) SeqType {
	t.Helper()
	typ, err := NewSeqType(element)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func mustFunType(t *testing.T, argument, result Type) FunType {
	t.Helper()
	typ, err := NewFunType(argument, result)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func mustOperatorType(t *testing.T, arguments []Type, result Type) OperatorType {
	t.Helper()
	typ, err := NewOperatorType(arguments, result)
	if err != nil {
		t.Fatal(err)
	}
	return typ
}

func TestTypeParserRecordsRowsAndVariants(t *testing.T) {
	a := mustNumberedVar(t, 0)
	b := mustNumberedVar(t, 1)
	c := mustNumberedVar(t, 2)
	x := mustNumberedVar(t, 23)
	legacy, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	sparse, _ := NewSparseTupleType(map[int]Type{3: BoolT1, 5: IntT1})
	row := mustRowType(t, map[string]Type{"f": IntT1, "g": BoolT1}, &x)
	recordRow := NewRecordRowType(mustRowType(t, map[string]Type{"f": IntT1, "g": a}, &x))
	nestedRecord := NewRecordRowType(mustRowType(t, map[string]Type{"g": BoolT1}, &c))
	variant := NewVariantType(mustRowType(t, map[string]Type{"Tag1": IntT1, "Tag2": nestedRecord}, nil))
	variantTail := NewVariantType(mustRowType(t, map[string]Type{"Tag": a}, &b))

	tests := []struct {
		text string
		want Type
	}{
		{"<| 3: Bool, 5: Int |>", sparse},
		{"[a: Int, b: Bool]", legacy},
		{"(||)", mustRowType(t, nil, nil)},
		{"(| f: Int | g: Bool | x |)", row},
		{"{ f: Int, g: a, x }", recordRow},
		{"Tag1(Int) | Tag2({ g: Bool, c })", variant},
		{"Tag(a) | b", variantTail},
		{"Variant()", NewVariantType(mustRowType(t, nil, nil))},
		{"Variant(a)", NewVariantType(mustRowType(t, nil, &a))},
		{"(| v: Tag(Int) | next: Bool |)", mustRowType(t, map[string]Type{"v": NewVariantType(mustRowType(t, map[string]Type{"Tag": IntT1}, nil)), "next": BoolT1}, nil)},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			got, err := ParseType(test.text)
			if err != nil || !EqualTypes(got, test.want) {
				t.Fatalf("ParseType() = %v, %v; want %v", got, err, test.want)
			}
		})
	}
}

func TestTypeParserAliasesCommentsAndFailures(t *testing.T) {
	name, typ, err := ParseTypeAlias("newSyntaxInCamelCase = Set($refToAnotherAlias)")
	aliasRef := mustNamedConst(t, "$refToAnotherAlias")
	want := mustSetType(t, aliasRef)
	if err != nil || name != "$newSyntaxInCamelCase" || !EqualTypes(typ, want) {
		t.Fatalf("ParseTypeAlias() = %q, %v, %v", name, typ, err)
	}
	name, typ, err = ParseTypeAlias("ALIAS1 = [a: Int, b: Bool]")
	wantRecord, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	if err != nil || name != "ALIAS1" || !EqualTypes(typ, wantRecord) {
		t.Fatalf("legacy alias = %q, %v, %v", name, typ, err)
	}

	commented := "Set([\n// tag explanation\ntag: Str,\n// value explanation\nvalue: Int\n])"
	got, err := ParseType(commented)
	commentRecord, _ := NewRecordType(map[string]Type{"tag": StrT1, "value": IntT1})
	commentWant := mustSetType(t, commentRecord)
	if err != nil || !EqualTypes(got, commentWant) {
		t.Fatalf("commented type = %v, %v", got, err)
	}

	for _, invalid := range []string{
		"", "non-sense", "vc", "<<>>", "{ f: Int, f: Bool }",
		"Tag(Int) | Tag(Bool)", "Variant(Int)", "$Uppercase", "Set()",
	} {
		t.Run("reject_"+invalid, func(t *testing.T) {
			if got, err := ParseType(invalid); err == nil {
				t.Fatalf("ParseType(%q) = %v, want error", invalid, got)
			}
		})
	}
}

func TestTypeParserRoundTripsCanonicalTypes(t *testing.T) {
	a := mustNumberedVar(t, 0)
	b := mustNumberedVar(t, 1)
	row := mustRowType(t, map[string]Type{"left": IntT1, "right": mustSetType(t, a)}, &b)
	types := []Type{
		IntT1,
		mustNamedConst(t, "MODEL"),
		a,
		mustSetType(t, mustSeqType(t, BoolT1)),
		mustFunType(t, mustTupleType(t, IntT1, StrT1), BoolT1),
		mustOperatorType(t, []Type{a, mustSetType(t, a)}, BoolT1),
		row,
		NewRecordRowType(row),
		NewVariantType(row),
	}
	for _, typ := range types {
		text := typ.String()
		parsed, err := ParseType(text)
		if err != nil || !EqualTypes(parsed, typ) {
			t.Errorf("round trip %T %q = %v, %v", typ, text, parsed, err)
		}
	}
}
