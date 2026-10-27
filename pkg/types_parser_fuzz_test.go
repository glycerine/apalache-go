package apalache

import "testing"

func FuzzTypeParserRoundTrip(f *testing.F) {
	for _, seed := range []string{
		"Int",
		"Set(Seq(Bool))",
		"(Set(Int), Bool) => Str",
		"{ left: Int, right: Set(a), b }",
		"Tag(Int) | Other({ value: Str })",
		"<| 1: Int, 3: Bool |>",
		"not a type",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// Keep generated recursive syntax within a useful frontend-sized bound.
		if len(input) > 1024 {
			t.Skip()
		}
		typ, err := ParseType(input)
		if err != nil {
			return
		}
		canonical := typ.String()
		roundTrip, err := ParseType(canonical)
		if err != nil {
			t.Fatalf("canonical type %q from %q does not parse: %v", canonical, input, err)
		}
		if !EqualTypes(roundTrip, typ) {
			t.Fatalf("type round trip from %q changed %s to %s", input, typ, roundTrip)
		}
	})
}
