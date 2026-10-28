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
		"CONSTRAINT__si_0", "ACTION_CONSTRAINT__si_0", "INVARIANT__si_0", "INVARIANT__si_1", "PROPERTY__si_0", "INIT", "NEXT",
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

func TestParseAndImportTLCConfigScalaAllFeaturesCase(t *testing.T) {
	intOperator := mustOperator(nil, IntT1)
	boolOperator := mustOperator(nil, BoolT1)
	module := NewModule("test", []Decl{
		NewOperDecl("M", nil, intLiteral(1), Typed(intOperator)),
		NewOperDecl("B", nil, NewValueExpr(StringValue("foo"), Typed(StrT1)), Typed(mustOperator(nil, StrT1))),
		NewOperDecl("Init", nil, boolExpr(true), Typed(boolOperator)),
		NewOperDecl("Next", nil, boolExpr(true), Typed(boolOperator)),
		NewOperDecl("Prop", nil, boolExpr(true), Typed(boolOperator)),
	})
	config, err := ParseTLCConfig(`
CONSTANTS
N = M
A <- B
SYMMETRY ignored
CONSTRAINT Cons1
ACTION_CONSTRAINT Cons2
INIT Init
NEXT Next
INVARIANT Inv
PROPERTY Prop`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ImportTLCConfig(module, config)
	if err != nil {
		t.Fatal(err)
	}
	wantNames := []string{
		"M", "B", "Init", "Next", "Prop",
		"OVERRIDE_N", "OVERRIDE_A", "CONSTRAINT__si_0", "ACTION_CONSTRAINT__si_0",
		"INVARIANT__si_0", "PROPERTY__si_0", "INIT", "NEXT",
	}
	declarations := got.Declarations()
	if len(declarations) != len(wantNames) {
		t.Fatalf("imported declarations = %d, want %d", len(declarations), len(wantNames))
	}
	for index, want := range wantNames {
		if declarations[index].Name() != want {
			t.Fatalf("imported declaration %d = %q, want %q", index, declarations[index].Name(), want)
		}
	}
	modelValue, _ := OperatorBody(got, "OVERRIDE_N")
	if modelValue.(*ValueExpr).Value().(StringValue) != "ModelValue_M" {
		t.Fatalf("model value override = %v", modelValue)
	}
	replacement, _ := OperatorBody(got, "OVERRIDE_A")
	if replacement.(*OperExpr).Arguments()[0].(*NameExpr).Name() != "B" {
		t.Fatalf("replacement override = %v", replacement)
	}
}

func TestParseAndImportTLCConfigPreservesScalaBindingGroupsAndOrder(t *testing.T) {
	module := NewModule("test", []Decl{
		NewOperDecl("B", nil, intLiteral(1), Typed(mustOperator(nil, IntT1))),
		NewOperDecl("D", nil, intLiteral(3), Typed(mustOperator(nil, IntT1))),
	})
	config, err := ParseTLCConfig(`
CONSTANTS
N = M
A <- B
K = L
C <- D
INIT Init
NEXT Next`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := ImportTLCConfig(module, config)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"B", "D", "OVERRIDE_N", "OVERRIDE_K", "OVERRIDE_A", "OVERRIDE_C", "INIT", "NEXT"}
	for index, declaration := range got.Declarations() {
		if declaration.Name() != want[index] {
			t.Fatalf("declaration %d = %q, want %q", index, declaration.Name(), want[index])
		}
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
