package apalache

import (
	"context"
	"testing"
)

func decoderForExpression(t *testing.T, expression Expr) (*SymbolicDecoder, RewriterScope, ArenaCell, func()) {
	t.Helper()
	rewriter := NewStratifiedRewriter()
	scope, cell, err := rewriter.Rewrite(expression, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver, err := NewG3SolverContext()
	if err != nil {
		t.Fatal(err)
	}
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		solver.Close()
		t.Fatal(err)
	}
	status, err := solver.Check(context.Background())
	if err != nil || status != SMTSatisfiable {
		solver.Close()
		t.Fatalf("decoder model = %v, %v", status, err)
	}
	decoder, err := NewSymbolicDecoder(encoder, rewriter)
	if err != nil {
		solver.Close()
		t.Fatal(err)
	}
	return decoder, scope, cell, func() { _ = solver.Close() }
}

func TestSymbolicDecoderScalarsSetsAndSequences(t *testing.T) {
	setType := mustSet(IntT1)
	sequenceType := newSeqTypeUnchecked(IntT1)
	tupleType := mustTuple(BoolT1, setType, sequenceType)
	expression := MustOperExpr(Op("TUPLE"), Typed(tupleType),
		NewValueExpr(BoolValue(true), Typed(BoolT1)),
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(2), intLiteral(1)),
		MustOperExpr(Op("TUPLE"), Typed(sequenceType), intLiteral(4), intLiteral(5)))
	decoder, scope, cell, closeSolver := decoderForExpression(t, expression)
	defer closeSolver()
	decoded, err := decoder.DecodeCell(scope.Arena, cell)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatTLAExpression(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if formatted != "<<TRUE, { 1, 2 }, <<4, 5>>>>" {
		t.Fatalf("decoded composite = %s", formatted)
	}
}

func TestSymbolicDecoderRecordsVariantsAndFunctions(t *testing.T) {
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	record := MustOperExpr(Op("RECORD"), Typed(recordType),
		NewValueExpr(StringValue("b"), Typed(StrT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(3))
	decoder, scope, cell, closeSolver := decoderForExpression(t, record)
	decoded, err := decoder.DecodeCell(scope.Arena, cell)
	closeSolver()
	if err != nil {
		t.Fatal(err)
	}
	formatted, _ := FormatTLAExpression(decoded)
	if formatted != "[a |-> 3, b |-> TRUE]" {
		t.Fatalf("decoded record = %s", formatted)
	}

	variantType := testVariantType(t)
	variant := testVariantExpression(variantType, "A", intLiteral(7))
	decoder, scope, cell, closeSolver = decoderForExpression(t, variant)
	decoded, err = decoder.DecodeCell(scope.Arena, cell)
	closeSolver()
	if err != nil {
		t.Fatal(err)
	}
	formatted, _ = FormatTLAExpression(decoded)
	if formatted != "Variants!Variant(\"A\", 7)" {
		t.Fatalf("decoded variant = %s", formatted)
	}

	setType := mustSet(IntT1)
	functionType := mustFun(IntT1, IntT1)
	function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
		MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("i", Typed(IntT1)), intLiteral(1)),
		NewNameExpr("i", Typed(IntT1)),
		MustOperExpr(Op("SET_ENUM"), Typed(setType), intLiteral(1), intLiteral(2)))
	decoder, scope, cell, closeSolver = decoderForExpression(t, function)
	decoded, err = decoder.DecodeCell(scope.Arena, cell)
	closeSolver()
	if err != nil {
		t.Fatal(err)
	}
	formatted, _ = FormatTLAExpression(decoded)
	if formatted != "SetAsFun({ <<1, 2>>, <<2, 3>> })" {
		t.Fatalf("decoded function = %s", formatted)
	}
}

func TestSymbolicDecoderBindingUsesLiteralCache(t *testing.T) {
	expression := NewValueExpr(StringValue("hello"), Typed(StrT1))
	decoder, scope, cell, closeSolver := decoderForExpression(t, expression)
	defer closeSolver()
	decoded, err := decoder.DecodeBinding(scope.Arena, BindingOf(BindingEntry{Name: "message", Cell: cell}))
	if err != nil {
		t.Fatal(err)
	}
	formatted, _ := FormatTLAExpression(decoded["message"])
	if formatted != "\"hello\"" {
		t.Fatalf("decoded literal binding = %s", formatted)
	}
}
