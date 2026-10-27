package apalache

import (
	"math/big"
	"strings"
	"testing"
)

func TestImportTLCConfigAssignmentsReplacementsAndPredicates(t *testing.T) {
	intOperator := mustOperator(nil, IntT1)
	module := NewModule("M", []Decl{
		NewOperDecl("B", nil, intLiteral(1), Typed(intOperator)),
		NewConstDecl("D", Typed(StrT1)),
	})
	module.AddProperty(PropertyTypeChecked)
	config := TLCConfig{
		ConstAssignments: map[string]ConfigConstExpr{
			"N": ConfigIntValue{Value: big.NewInt(3)},
			"K": ConfigModelValue{Name: "M"},
		},
		ConstReplacements:  map[string]string{"A": "B", "C": "D"},
		StateConstraints:   []string{"StateOK"},
		ActionConstraints:  []string{"ActionOK"},
		Invariants:         []string{"Inv", "Inv2"},
		TemporalProperties: []string{"Live"},
		Behavior:           InitNextSpec{Init: "MyInit", Next: "MyNext"},
	}
	got, err := ImportTLCConfig(module, config)
	if err != nil {
		t.Fatal(err)
	}
	if !got.HasProperty(PropertyTypeChecked) {
		t.Fatal("module properties were not preserved")
	}
	for _, name := range []string{
		"OVERRIDE_A", "OVERRIDE_C", "OVERRIDE_K", "OVERRIDE_N",
		"CONSTRAINT_$0", "ACTION_CONSTRAINT_$0", "INVARIANT_$0", "INVARIANT_$1", "PROPERTY_$0", "INIT", "NEXT",
	} {
		if _, found := OperatorBody(got, name); !found {
			t.Errorf("synthetic declaration %s is missing", name)
		}
	}
	replacement, _ := OperatorBody(got, "OVERRIDE_A")
	if replacement.(*OperExpr).Operator().Name() != "OPER_APP" || replacement.(*OperExpr).Arguments()[0].(*NameExpr).Name() != "B" {
		t.Fatalf("operator replacement = %v", replacement)
	}
	constantReplacement, _ := OperatorBody(got, "OVERRIDE_C")
	if constantReplacement.(*NameExpr).Name() != "D" || !constantReplacement.Tag().Equal(Typed(StrT1)) {
		t.Fatalf("constant replacement = %v", constantReplacement)
	}
	modelValue, _ := OperatorBody(got, "OVERRIDE_K")
	if modelValue.(*ValueExpr).Value().(StringValue) != StringValue("ModelValue_M") {
		t.Fatalf("model value assignment = %v", modelValue)
	}
}

func TestImportTLCConfigSpecification(t *testing.T) {
	got, err := ImportTLCConfig(NewModule("M", nil), TLCConfig{Behavior: TemporalSpec{Name: "Spec"}})
	if err != nil {
		t.Fatal(err)
	}
	body, found := OperatorBody(got, TLCSpecificationDecl)
	if !found || body.(*NameExpr).Name() != "Spec" {
		t.Fatalf("SPECIFICATION declaration = %v, %v", body, found)
	}
}

func TestImportTLCConfigRejectsBadReplacementAndBehavior(t *testing.T) {
	parameterized := NewOperDecl("F", []OperatorParam{ValueParam("x")}, NewNameExpr("x", Typed(IntT1)), Typed(mustOperator([]Type{IntT1}, IntT1)))
	module := NewModule("M", []Decl{parameterized})
	for _, test := range []TLCConfig{
		{ConstReplacements: map[string]string{"A": "Missing"}, Behavior: InitNextSpec{"Init", "Next"}},
		{ConstReplacements: map[string]string{"A": "F"}, Behavior: InitNextSpec{"Init", "Next"}},
		{},
	} {
		_, err := ImportTLCConfig(module, test)
		if err == nil {
			t.Fatal("invalid TLC configuration was accepted")
		}
		if _, typed := err.(*TLCConfigurationError); !typed {
			t.Fatalf("configuration error = %T %v", err, err)
		}
	}
	_, err := ImportTLCConfig(module, TLCConfig{ConstReplacements: map[string]string{"A": "F"}, Behavior: InitNextSpec{"Init", "Next"}})
	if err == nil || !strings.Contains(err.Error(), "1 parameters") {
		t.Fatalf("parameterized replacement error = %v", err)
	}
}
