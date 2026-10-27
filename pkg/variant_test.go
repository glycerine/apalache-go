package apalache

import (
	"context"
	"testing"
)

func testVariantType(t *testing.T) VariantType {
	t.Helper()
	row, err := NewRowType(map[string]Type{"A": IntT1, "B": BoolT1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return NewVariantType(row)
}

func testVariantExpression(typ VariantType, tag string, value Expr) Expr {
	return MustOperExpr(Op("Variants!Variant"), Typed(typ),
		NewValueExpr(StringValue(tag), Typed(StrT1)), value)
}

func checkRewrittenTruth(t *testing.T, rewriter *StratifiedRewriter, scope RewriterScope, cell ArenaCell, want bool) {
	t.Helper()
	solver := newTestG3Solver(t)
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	var claim Expr = cell.NameExpr()
	if !want {
		claim = MustOperExpr(Op("NOT"), Typed(BoolT1), claim)
	}
	if err := encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), claim)); err != nil {
		t.Fatal(err)
	}
	if status, err := solver.Check(context.Background()); err != nil || status != SMTUnsatisfiable {
		t.Fatalf("rewritten truth %v = %v, %v", want, status, err)
	}
}

func TestStratifiedVariantConstructionAndAccess(t *testing.T) {
	variantType := testVariantType(t)
	variant := testVariantExpression(variantType, "A", NewValueExpr(Int64Value(7), Typed(IntT1)))
	rewriter := NewStratifiedRewriter()
	scope, variantCell, err := rewriter.Rewrite(variant, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	fields := scope.Arena.GetHas(variantCell)
	if len(fields) != 3 {
		t.Fatalf("variant fields = %#v", fields)
	}
	// Sorted Scala record layout is A, B, __tag for these field names.
	value, found := rewriter.IntegerCache().FindValue(fields[0].Element())
	if !found || value.Int64() != 7 || !fields[1].Element().Equal(scope.Arena.FalseCell()) {
		t.Fatalf("variant payload fields = %#v, integer %v", fields, value)
	}
	tagLiteral, found := rewriter.LiteralCache().FindLiteral(fields[2].Element())
	if !found || tagLiteral.Index != "A" {
		t.Fatalf("variant tag = %#v, found %v", tagLiteral, found)
	}

	unsafe := MustOperExpr(Op("Variants!VariantGetUnsafe"), Typed(IntT1),
		NewValueExpr(StringValue("A"), Typed(StrT1)), variant)
	_, unsafeCell, err := rewriter.Rewrite(unsafe, scope)
	if err != nil {
		t.Fatal(err)
	}
	unsafeValue, found := rewriter.IntegerCache().FindValue(unsafeCell)
	if !found || unsafeValue.Int64() != 7 {
		t.Fatalf("unsafe variant value = %v, found %v", unsafeValue, found)
	}

	tag := MustOperExpr(Op("Variants!VariantTag"), Typed(StrT1), variant)
	tagEquals := MustOperExpr(Op("EQ"), Typed(BoolT1), tag, NewValueExpr(StringValue("A"), Typed(StrT1)))
	tagRewriter := NewStratifiedRewriter()
	tagScope, tagResult, err := tagRewriter.Rewrite(tagEquals, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, tagRewriter, tagScope, tagResult, true)
}

func TestStratifiedVariantEqualityAndGetOrElseThroughG3(t *testing.T) {
	variantType := testVariantType(t)
	variantA := func(value int64) Expr {
		return testVariantExpression(variantType, "A", NewValueExpr(Int64Value(value), Typed(IntT1)))
	}
	variantB := testVariantExpression(variantType, "B", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	tests := []struct {
		name       string
		expression Expr
		want       bool
	}{
		{"same active value", MustOperExpr(Op("EQ"), Typed(BoolT1), variantA(1), variantA(1)), true},
		{"different active value", MustOperExpr(Op("EQ"), Typed(BoolT1), variantA(1), variantA(2)), false},
		{"different tag", MustOperExpr(Op("EQ"), Typed(BoolT1), variantA(1), variantB), false},
		{"matching get or else", MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("Variants!VariantGetOrElse"), Typed(IntT1),
				NewValueExpr(StringValue("A"), Typed(StrT1)), variantA(3), NewValueExpr(Int64Value(9), Typed(IntT1))),
			NewValueExpr(Int64Value(3), Typed(IntT1))), true},
		{"fallback get or else", MustOperExpr(Op("EQ"), Typed(BoolT1),
			MustOperExpr(Op("Variants!VariantGetOrElse"), Typed(IntT1),
				NewValueExpr(StringValue("A"), Typed(StrT1)), variantB, NewValueExpr(Int64Value(9), Typed(IntT1))),
			NewValueExpr(Int64Value(9), Typed(IntT1))), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rewriter := NewStratifiedRewriter()
			scope, result, err := rewriter.Rewrite(test.expression, InitialRewriterScope())
			if err != nil {
				t.Fatal(err)
			}
			checkRewrittenTruth(t, rewriter, scope, result, test.want)
		})
	}
}

func TestStratifiedVariantFilterAndDefaultThroughG3(t *testing.T) {
	variantType := testVariantType(t)
	setType := mustSet(variantType)
	variantA := testVariantExpression(variantType, "A", NewValueExpr(Int64Value(4), Typed(IntT1)))
	variantB := testVariantExpression(variantType, "B", NewValueExpr(BoolValue(true), Typed(BoolT1)))
	variants := MustOperExpr(Op("SET_ENUM"), Typed(setType), variantA, variantB)
	filtered := MustOperExpr(Op("Variants!VariantFilter"), Typed(mustSet(IntT1)),
		NewValueExpr(StringValue("A"), Typed(StrT1)), variants)
	expected := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), NewValueExpr(Int64Value(4), Typed(IntT1)))
	equal := MustOperExpr(Op("EQ"), Typed(BoolT1), filtered, expected)
	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	checkRewrittenTruth(t, rewriter, scope, result, true)

	defaultRewriter := NewStratifiedRewriter()
	defaultScope, defaultVariant, err := defaultRewriter.DefaultValueCache().GetOrCreate(InitialRewriterScope(), variantType)
	if err != nil {
		t.Fatal(err)
	}
	tag, err := variantTag(defaultScope.Arena, defaultVariant)
	if err != nil {
		t.Fatal(err)
	}
	literal, found := defaultRewriter.LiteralCache().FindLiteral(tag)
	if !found || literal.Index != "A" {
		t.Fatalf("default variant tag = %#v, found %v", literal, found)
	}
}

func TestCherryPickerPreservesVariantCorrelationThroughG3(t *testing.T) {
	variantType := testVariantType(t)
	rewriter := NewStratifiedRewriter()
	scope, first, err := rewriter.Rewrite(testVariantExpression(variantType, "A",
		NewValueExpr(Int64Value(1), Typed(IntT1))), InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope, second, err := rewriter.Rewrite(testVariantExpression(variantType, "B",
		NewValueExpr(BoolValue(true), Typed(BoolT1))), scope)
	if err != nil {
		t.Fatal(err)
	}
	oracle, _ := NewMockOracle(1)
	scope, picked, err := NewCherryPicker(rewriter).Pick(scope, oracle, []ArenaCell{first, second})
	if err != nil {
		t.Fatal(err)
	}
	equal, err := rewriter.equality.Equal(scope.Arena, picked, second)
	if err != nil {
		t.Fatal(err)
	}
	scope, result := rewriter.bindBoolean(scope, equal)
	checkRewrittenTruth(t, rewriter, scope, result, true)
}
