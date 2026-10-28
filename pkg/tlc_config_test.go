package apalache

import (
	"errors"
	"math/big"
	"reflect"
	"testing"
)

func TestParseTLCConfigAllSections(t *testing.T) {
	input := `
CONSTANTS
  N = 10
  K = -20
  Flag = TRUE
  Text = "Chain-A"
  Model = M
  Values = {"foo", {1, Moo}, {}}
  A <- B
\* a line comment
CONSTRAINT Cons1 Cons2
ACTION_CONSTRAINT ActCons
(* a multiline
   comment *)
INIT Init
NEXT Next
INVARIANTS Inv1 Inv2
PROPERTIES Live Safe
SYMMETRY SymmetrySet
VIEW ViewDef
ALIAS AliasDef
POSTCONDITION Post
CHECK_DEADLOCK FALSE
`
	config, err := ParseTLCConfig(input)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := config.Behavior.(InitNextSpec); !ok || got != (InitNextSpec{Init: "Init", Next: "Next"}) {
		t.Fatalf("behavior = %#v", config.Behavior)
	}
	if config.CheckDeadlock == nil || *config.CheckDeadlock {
		t.Fatalf("deadlock = %v", config.CheckDeadlock)
	}
	if !reflect.DeepEqual(config.StateConstraints, []string{"Cons1", "Cons2"}) ||
		!reflect.DeepEqual(config.ActionConstraints, []string{"ActCons"}) ||
		!reflect.DeepEqual(config.Invariants, []string{"Inv1", "Inv2"}) ||
		!reflect.DeepEqual(config.TemporalProperties, []string{"Live", "Safe"}) {
		t.Fatalf("name lists = %#v", config)
	}
	if config.ConstReplacements["A"] != "B" || len(config.ConstAssignments) != 6 {
		t.Fatalf("constant bindings = %#v %#v", config.ConstAssignments, config.ConstReplacements)
	}
	if config.ConstAssignments["N"].(ConfigIntValue).Value.String() != "10" || config.ConstAssignments["K"].(ConfigIntValue).Value.String() != "-20" {
		t.Fatal("integer assignments changed")
	}
	if !config.ConstAssignments["Flag"].(ConfigBoolValue).Value || config.ConstAssignments["Text"].(ConfigStringValue).Value != "Chain-A" || config.ConstAssignments["Model"].(ConfigModelValue).Name != "M" {
		t.Fatal("scalar assignments changed")
	}
	set := config.ConstAssignments["Values"].(ConfigSetValue)
	if len(set.Elements) != 3 || len(set.Elements[1].(ConfigSetValue).Elements) != 2 || len(set.Elements[2].(ConfigSetValue).Elements) != 0 {
		t.Fatalf("nested set = %#v", set)
	}
}

func TestParseTLCConfigNextBeforeInitAndSpecification(t *testing.T) {
	config, err := ParseTLCConfig("NEXT Step INIT Start")
	if err != nil {
		t.Fatal(err)
	}
	if config.Behavior != (InitNextSpec{Init: "Start", Next: "Step"}) {
		t.Fatalf("behavior = %#v", config.Behavior)
	}
	if config.CheckDeadlock != nil {
		t.Fatalf("missing CHECK_DEADLOCK was recorded as %v", *config.CheckDeadlock)
	}
	config, err = ParseTLCConfig("SPECIFICATION Spec")
	if err != nil {
		t.Fatal(err)
	}
	if config.Behavior != (TemporalSpec{Name: "Spec"}) {
		t.Fatalf("behavior = %#v", config.Behavior)
	}
}

func TestParseTLCConfigAccumulatesRepeatedListsAndBindings(t *testing.T) {
	config, err := ParseTLCConfig(`
CONSTANT N = Old
CONSTANT N = New K <- Def
INVARIANT Inv1
INVARIANTS Inv2
INIT Init NEXT Next`)
	if err != nil {
		t.Fatal(err)
	}
	if config.ConstAssignments["N"].(ConfigModelValue).Name != "New" || config.ConstReplacements["K"] != "Def" {
		t.Fatalf("bindings = %#v %#v", config.ConstAssignments, config.ConstReplacements)
	}
	if !reflect.DeepEqual(config.Invariants, []string{"Inv1", "Inv2"}) {
		t.Fatalf("invariants = %v", config.Invariants)
	}
}

func TestParseTLCConfigRejectsMalformedAndConflictingInput(t *testing.T) {
	cases := []string{
		"", "CONSTANT", "CONSTANT FOO", "INIT NEXT Next", "INIT Init INVARIANT Inv NEXT Next",
		"INIT Init NEXT Next SPECIFICATION Spec", "CHECK_DEADLOCK TRUE CHECK_DEADLOCK FALSE",
		"CONSTANT N = {1,}", "CONSTANT N = \"unterminated", "(* unterminated",
	}
	for _, input := range cases {
		_, err := ParseTLCConfig(input)
		var parseError *TLCConfigParseError
		if !errors.As(err, &parseError) {
			t.Errorf("input %q: got %T %v", input, err, err)
		}
		if parseError != nil && (parseError.Line < 1 || parseError.Column < 1) {
			t.Errorf("input %q: invalid position %+v", input, parseError)
		}
	}
}

func TestParseTLCConfigDuplicateEqualDeadlockIsAllowed(t *testing.T) {
	config, err := ParseTLCConfig("CHECK_DEADLOCK TRUE CHECK_DEADLOCK TRUE")
	if err != nil {
		t.Fatal(err)
	}
	if config.CheckDeadlock == nil || !*config.CheckDeadlock {
		t.Fatalf("deadlock = %v", config.CheckDeadlock)
	}
}

func TestConfigConstToExpr(t *testing.T) {
	cases := []struct {
		value    ConfigConstExpr
		typeText string
	}{
		{ConfigModelValue{Name: "M"}, "Str"},
		{ConfigIntValue{Value: bigIntForTest(t, "12345678901234567890")}, "Int"},
		{ConfigBoolValue{Value: true}, "Bool"},
		{ConfigStringValue{Value: "plain"}, "Str"},
		{ConfigStringValue{Value: "node1_OF_NODE"}, "NODE"},
		{ConfigSetValue{}, "Set(a)"},
		{ConfigSetValue{Elements: []ConfigConstExpr{ConfigIntValue{Value: bigIntForTest(t, "1")}}}, "Set(Int)"},
	}
	for _, test := range cases {
		expression, err := ConfigConstToExpr(test.value)
		if err != nil {
			t.Fatalf("%#v: %v", test.value, err)
		}
		typ, ok := expression.Tag().Type()
		if !ok || typ.String() != test.typeText {
			t.Errorf("%#v: type = %v, want %s", test.value, typ, test.typeText)
		}
	}
	model, err := ConfigConstToExpr(ConfigModelValue{Name: "M"})
	if err != nil {
		t.Fatal(err)
	}
	if model.(*ValueExpr).Value().(StringValue).Value() != "ModelValue_M" {
		t.Fatalf("model value = %#v", model)
	}
	if _, err := ConfigConstToExpr(ConfigSetValue{Elements: []ConfigConstExpr{
		ConfigIntValue{Value: bigIntForTest(t, "1")}, ConfigBoolValue{Value: true},
	}}); err == nil {
		t.Fatal("accepted heterogeneous set")
	}
}

func bigIntForTest(t *testing.T, text string) *big.Int {
	t.Helper()
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		t.Fatalf("bad integer %q", text)
	}
	return value
}
