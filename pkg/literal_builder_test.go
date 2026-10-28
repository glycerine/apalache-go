package apalache

import (
	"math/big"
	"testing"
)

func requireLiteral(t *testing.T, expression Expr, value Value, typ Type) {
	t.Helper()
	literal, ok := expression.(*ValueExpr)
	if !ok || !EqualValues(literal.Value(), value) {
		t.Fatalf("literal = %T %v, want %T %v", expression, expression, value, value)
	}
	requireExprType(t, literal, typ)
}

func TestTypedLiteralAndNameBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	integers := []*big.Int{
		big.NewInt(-5), big.NewInt(0), big.NewInt(5),
		new(big.Int).Lsh(big.NewInt(1), 200),
	}
	for _, integer := range integers {
		requireLiteral(t, builder.Int(integer), NewIntValue(integer), IntT1)
	}
	for _, value := range []bool{false, true} {
		requireLiteral(t, builder.Bool(value), BoolValue(value), BoolT1)
	}
	for _, value := range []string{"a", "hello", "with spaces", "_OF_"} {
		literal, err := builder.Str(value)
		if err != nil {
			t.Fatalf("Str(%q): %v", value, err)
		}
		requireLiteral(t, literal, StringValue(value), StrT1)
	}
	if _, err := builder.Str("1_OF_X"); err == nil {
		t.Fatal("Str accepted an uninterpreted literal")
	}

	for _, typeName := range []string{"A", "PERSON", "PERSON_ID"} {
		typ := mustNamedConst(t, typeName)
		for _, root := range []string{"1", "alice", "one_two"} {
			literal, err := builder.Const(root, typ)
			if err != nil {
				t.Fatalf("Const(%q, %s): %v", root, typ, err)
			}
			requireLiteral(t, literal, StringValue(ConstructModelValue(typeName, root)), typ)
			parsed, parseErr := builder.ConstParsed(ConstructModelValue(typeName, root))
			if parseErr != nil {
				t.Fatalf("ConstParsed(%q): %v", ConstructModelValue(typeName, root), parseErr)
			}
			requireLiteral(t, parsed, StringValue(ConstructModelValue(typeName, root)), typ)
		}
	}
	for _, ambiguous := range []string{"1_OF_A", "1_OF_B"} {
		if _, err := builder.Const(ambiguous, mustNamedConst(t, "A")); err == nil {
			t.Errorf("Const accepted ambiguous root %q", ambiguous)
		}
	}
	if _, err := builder.ConstParsed("x"); err == nil {
		t.Fatal("ConstParsed accepted an ordinary string")
	}

	sets := []struct {
		expression Expr
		value      PredefinedSet
		typ        Type
	}{
		{builder.BooleanSet(), BoolSet, mustSetType(t, BoolT1)},
		{builder.StringSet(), StringSet, mustSetType(t, StrT1)},
		{builder.IntSet(), IntSet, mustSetType(t, IntT1)},
		{builder.NatSet(), NatSet, mustSetType(t, IntT1)},
	}
	for _, set := range sets {
		requireLiteral(t, set.expression, set.value, set.typ)
	}

	for _, typ := range concreteActionBuilderTypes(t) {
		name := builder.Name("name", typ)
		if name.Name() != "name" {
			t.Fatalf("name = %q", name.Name())
		}
		requireExprType(t, name, typ)
	}
}

func TestScopedLiteralAndNameBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	huge := new(big.Int).Neg(new(big.Int).Lsh(big.NewInt(1), 300))
	integer, err := builder.Int(huge).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireLiteral(t, integer, NewIntValue(huge), IntT1)

	stringLiteral, err := builder.Str("value").Build()
	if err != nil {
		t.Fatal(err)
	}
	requireLiteral(t, stringLiteral, StringValue("value"), StrT1)
	if _, err = builder.Str("1_OF_X").Build(); err == nil {
		t.Fatal("Str accepted an uninterpreted literal")
	}

	typ := mustNamedConst(t, "PERSON")
	model, err := builder.Const("alice", typ).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireLiteral(t, model, StringValue("alice_OF_PERSON"), typ)
	parsed, err := builder.ConstParsed("alice_OF_PERSON").Build()
	if err != nil {
		t.Fatal(err)
	}
	requireLiteral(t, parsed, StringValue("alice_OF_PERSON"), typ)
	if _, err = builder.Const("alice_OF_PERSON", typ).Build(); err == nil {
		t.Fatal("Const accepted an ambiguous root")
	}
	if _, err = builder.ConstParsed("alice").Build(); err == nil {
		t.Fatal("ConstParsed accepted an ordinary string")
	}

	sets := []struct {
		instruction ExprInstruction
		value       PredefinedSet
		typ         Type
	}{
		{builder.BooleanSet(), BoolSet, mustSetType(t, BoolT1)},
		{builder.StringSet(), StringSet, mustSetType(t, StrT1)},
		{builder.IntSet(), IntSet, mustSetType(t, IntT1)},
		{builder.NatSet(), NatSet, mustSetType(t, IntT1)},
	}
	for _, set := range sets {
		expression, buildErr := set.instruction.Build()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		requireLiteral(t, expression, set.value, set.typ)
	}

	for _, nameType := range concreteActionBuilderTypes(t) {
		name, buildErr := builder.Name("name", nameType).Build()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		requireExprType(t, name, nameType)
	}
	if _, err = builder.Tuple(builder.Name("x", IntT1), builder.Name("x", BoolT1)).Build(); err == nil {
		t.Fatal("scope accepted inconsistent types for the same name")
	}
	inferred, err := builder.Tuple(builder.Name("x", IntT1), builder.NameWithInferredType("x")).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, inferred, mustTupleType(t, IntT1, IntT1))
}
