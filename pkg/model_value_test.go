package apalache

import "testing"

func TestModelValueHandlerMatchesScalaRegexExactly(t *testing.T) {
	tests := []struct {
		text, typeName, name string
		model                bool
	}{
		{"1_OF_A", "A", "1", true},
		{"one_two_OF_PERSON_ID", "PERSON_ID", "one_two", true},
		{"_OF__", "_", "", false},
		{"__OF__", "_", "_", true},
		{"foo_OF_BAR_OF_TYPE_2", "TYPE_2", "foo_OF_BAR", true},
		{"0_1_OF_A0_B2", "A0_B2", "0_1", true},
		{"ordinary", "", "", false},
		{"_OF_A", "", "", false},
		{"name_OF_", "", "", false},
		{"name_OF_2TYPE", "", "", false},
		{"name_OF_lower", "", "", false},
		{"name-of-TYPE", "", "", false},
		{" name_OF_TYPE", "", "", false},
		{"name_OF_TYPE ", "", "", false},
		{"naïve_OF_TYPE", "", "", false},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			typ, name, ok := ParseModelValue(test.text)
			if ok != test.model || IsModelValue(test.text) != test.model {
				t.Fatalf("model classification = %v / %v", ok, IsModelValue(test.text))
			}
			if !test.model {
				if name != "" || !EqualTypes(ModelValueOrStringType(test.text), StrT1) {
					t.Fatalf("ordinary string = %v, %q, %v", typ, name, ModelValueOrStringType(test.text))
				}
				return
			}
			if typ.Name() != test.typeName || name != test.name || !EqualTypes(ModelValueOrStringType(test.text), typ) {
				t.Fatalf("parsed = %v, %q", typ, name)
			}
			if reconstructed := ConstructModelValue(typ.Name(), name); reconstructed != test.text {
				t.Fatalf("reconstructed = %q", reconstructed)
			}
		})
	}
}

func TestConstructModelValuePreservesComponentsWithoutValidation(t *testing.T) {
	if got := ConstructModelValue("lower-case", "name with spaces"); got != "name with spaces_OF_lower-case" {
		t.Fatalf("constructed = %q", got)
	}
	if IsModelValue(ConstructModelValue("lower-case", "name with spaces")) {
		t.Fatal("construction unexpectedly validates its components")
	}
}
