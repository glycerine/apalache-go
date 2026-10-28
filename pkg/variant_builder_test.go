package apalache

import "testing"

func requireVariantOperation(t *testing.T, result *OperExpr, operator, tag string, want Type, arguments ...Expr) {
	t.Helper()
	requireExprType(t, result, want)
	got := result.Arguments()
	if result.Operator().Name() != operator || len(got) != len(arguments)+1 {
		t.Fatalf("result = %s/%d, want %s/%d", result.Operator(), len(got), operator, len(arguments)+1)
	}
	tagExpr, ok := got[0].(*ValueExpr)
	if !ok || tagExpr.Value() != StringValue(tag) {
		t.Fatalf("tag argument = %v, want %q", got[0], tag)
	}
	for index := range arguments {
		if got[index+1] != arguments[index] {
			t.Fatalf("argument %d was not preserved", index)
		}
	}
}

func TestTypedVariantBuilderExactScalaCases(t *testing.T) {
	builder := NewTypedBuilder()
	for _, typ := range concreteActionBuilderTypes(t) {
		t.Run(typ.String(), func(t *testing.T) {
			otherType := differentConcreteType(typ)
			variantType := NewVariantType(mustRowType(t, map[string]Type{"Tag": typ, "Other": otherType}, nil))
			value := builder.Name("value", typ)
			variant, err := builder.Variant("Tag", value, variantType)
			if err != nil {
				t.Fatal(err)
			}
			requireVariantOperation(t, variant, "Variants!Variant", "Tag", variantType, value)
			if _, err = builder.Variant("Missing", value, variantType); err == nil {
				t.Fatal("Variant accepted a missing tag")
			}
			if _, err = builder.Variant("Tag", builder.Name("bad", otherType), variantType); err == nil {
				t.Fatal("Variant accepted an incompatible value")
			}

			variantExpr := builder.Name("variant", variantType)
			set := builder.Name("variants", mustSetType(t, variantType))
			filtered, err := builder.VariantFilter("Tag", set)
			if err != nil {
				t.Fatal(err)
			}
			requireVariantOperation(t, filtered, "Variants!VariantFilter", "Tag", mustSetType(t, typ), set)
			if _, err = builder.VariantFilter("Tag", builder.Name("bad", IntT1)); err == nil {
				t.Fatal("VariantFilter accepted a non-set")
			}
			if _, err = builder.VariantFilter("Tag", builder.Name("bad", mustSetType(t, IntT1))); err == nil {
				t.Fatal("VariantFilter accepted a set of non-variants")
			}
			if _, err = builder.VariantFilter("Missing", set); err == nil {
				t.Fatal("VariantFilter accepted a missing tag")
			}

			tagged, err := builder.VariantTag(variantExpr)
			if err != nil {
				t.Fatal(err)
			}
			requireActionOperation(t, tagged, "Variants!VariantTag", StrT1, variantExpr)
			if _, err = builder.VariantTag(builder.Name("bad", IntT1)); err == nil {
				t.Fatal("VariantTag accepted a non-variant")
			}

			unsafe, err := builder.VariantGetUnsafe("Tag", variantExpr)
			if err != nil {
				t.Fatal(err)
			}
			requireVariantOperation(t, unsafe, "Variants!VariantGetUnsafe", "Tag", typ, variantExpr)
			fallback := builder.Name("fallback", typ)
			withDefault, err := builder.VariantGetOrElse("Tag", variantExpr, fallback)
			if err != nil {
				t.Fatal(err)
			}
			requireVariantOperation(t, withDefault, "Variants!VariantGetOrElse", "Tag", typ, variantExpr, fallback)
			if _, err = builder.VariantGetUnsafe("Missing", variantExpr); err == nil {
				t.Fatal("VariantGetUnsafe accepted a missing tag")
			}
			if _, err = builder.VariantGetUnsafe("Tag", builder.Name("bad", IntT1)); err == nil {
				t.Fatal("VariantGetUnsafe accepted a non-variant")
			}
			if _, err = builder.VariantGetOrElse("Tag", variantExpr, builder.Name("bad", otherType)); err == nil {
				t.Fatal("VariantGetOrElse accepted an incompatible default")
			}
		})
	}
}

func TestVariantBuilderFlexibleConstruction(t *testing.T) {
	builder := NewTypedBuilder()
	actual, _ := NewRecordType(map[string]Type{"a": IntT1})
	expected, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	variantType := NewVariantType(mustRowType(t, map[string]Type{"Record": expected}, nil))
	if _, err := builder.Variant("Record", builder.Name("record", actual), variantType); err != nil {
		t.Fatalf("Variant rejected a flexibly compatible value: %v", err)
	}
}

func TestScopedVariantBuilderExactScalaCases(t *testing.T) {
	builder := NewScopedBuilder()
	variantType := NewVariantType(mustRowType(t, map[string]Type{"Number": IntT1, "Flag": BoolT1}, nil))
	constructed, err := builder.Variant("Number", builder.Int64(1), variantType).Build()
	if err != nil {
		t.Fatal(err)
	}
	requireExprType(t, constructed, variantType)
	variant := builder.Name("variant", variantType)
	set := builder.Name("variants", mustSetType(t, variantType))
	for _, test := range []struct {
		instruction ExprInstruction
		want        Type
	}{
		{builder.VariantFilter("Number", set), mustSetType(t, IntT1)},
		{builder.VariantTag(variant), StrT1},
		{builder.VariantGetUnsafe("Number", variant), IntT1},
		{builder.VariantGetOrElse("Number", variant, builder.Int64(0)), IntT1},
	} {
		result, buildErr := test.instruction.Build()
		if buildErr != nil {
			t.Fatal(buildErr)
		}
		requireExprType(t, result, test.want)
	}
	for _, test := range []struct {
		name        string
		instruction ExprInstruction
	}{
		{"construct missing", builder.Variant("Missing", builder.Int64(1), variantType)},
		{"filter missing", builder.VariantFilter("Missing", set)},
		{"tag nonvariant", builder.VariantTag(builder.Int64(1))},
		{"get missing", builder.VariantGetUnsafe("Missing", variant)},
		{"default mismatch", builder.VariantGetOrElse("Number", variant, builder.Bool(false))},
	} {
		if _, buildErr := test.instruction.Build(); buildErr == nil {
			t.Errorf("%s was accepted", test.name)
		}
	}
}
