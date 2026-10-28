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

func TestSymbolicDecoderClosedRowRecord(t *testing.T) {
	row, err := NewRowType(map[string]Type{"a": IntT1, "b": BoolT1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	recordType := NewRecordRowType(row)
	record := MustOperExpr(Op("RECORD"), Typed(recordType),
		NewValueExpr(StringValue("b"), Typed(StrT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(3))
	decoder, scope, cell, closeSolver := decoderForExpression(t, record)
	defer closeSolver()
	decoded, err := decoder.DecodeCell(scope.Arena, cell)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatTLAExpression(decoded)
	if err != nil {
		t.Fatal(err)
	}
	if formatted != "[a |-> 3, b |-> TRUE]" {
		t.Fatalf("decoded row record = %s", formatted)
	}
}

func TestSymbolicDecoderOmitsAbsentLegacyRecordFields(t *testing.T) {
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	record := MustOperExpr(Op("RECORD"), Typed(recordType),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(3))
	decoder, scope, cell, closeSolver := decoderForExpression(t, record)
	defer closeSolver()
	decoded, err := decoder.DecodeCell(scope.Arena, cell)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatTLAExpression(decoded)
	if err != nil || formatted != "[a |-> 3]" {
		t.Fatalf("decoded sparse record = %q, %v", formatted, err)
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

func TestSymbolicDecoderScalaCollectionOracleCases(t *testing.T) {
	intSetType := mustSet(IntT1)
	functionType := mustFun(IntT1, IntT1)
	pairType := mustTuple(IntT1, IntT1)
	tests := []struct {
		name       string
		expression Expr
		want       string
	}{
		{"Int set", NewValueExpr(IntSet, Typed(intSetType)), "Int"},
		{"Nat set", NewValueExpr(NatSet, Typed(intSetType)), "Nat"},
		{"deduplicated sorted set", MustOperExpr(Op("SET_ENUM"), Typed(intSetType),
			intLiteral(2), intLiteral(1), intLiteral(2)), "{ 1, 2 }"},
		{"function set", MustOperExpr(Op("FUN_SET"), Typed(mustSet(functionType)),
			MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intLiteral(1), intLiteral(2)),
			MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intLiteral(3), intLiteral(4))),
			"[{ 1, 2 } -> { 3, 4 }]"},
		{"powerset", MustOperExpr(Op("SET_POWERSET"), Typed(mustSet(intSetType)),
			MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intLiteral(1), intLiteral(2))),
			"SUBSET { 1, 2 }"},
		{"empty function", MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
			MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("i", Typed(IntT1)), intLiteral(1)),
			NewNameExpr("i", Typed(IntT1)), MustOperExpr(Op("SET_ENUM"), Typed(intSetType))),
			"SetAsFun({})"},
		{"duplicate function indices", MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
			MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("i", Typed(IntT1)), intLiteral(1)),
			NewNameExpr("i", Typed(IntT1)), MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intLiteral(1), intLiteral(1))),
			"SetAsFun({<<1, 2>>})"},
		{"statically empty function relation type", MustOperExpr(Op("Apalache!SetAsFun"), Typed(functionType),
			MustOperExpr(Op("SET_ENUM"), Typed(mustSet(pairType)))), "SetAsFun({})"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			decoder, scope, cell, closeSolver := decoderForExpression(t, test.expression)
			defer closeSolver()
			decoded, err := decoder.DecodeCell(scope.Arena, cell)
			if err != nil {
				t.Fatal(err)
			}
			formatted, err := FormatTLAExpression(decoded)
			if err != nil || formatted != test.want {
				t.Fatalf("decoded = %q, %v; want %q", formatted, err, test.want)
			}
		})
	}
}

func TestSymbolicDecoderSubsequenceOracle(t *testing.T) {
	sequenceType := newSeqTypeUnchecked(IntT1)
	sequence := MustOperExpr(Op("TUPLE"), Typed(sequenceType),
		intLiteral(1), intLiteral(2), intLiteral(3), intLiteral(4))
	expression := MustOperExpr(Op("Sequences!SubSeq"), Typed(sequenceType), sequence, intLiteral(2), intLiteral(3))
	decoder, scope, cell, closeSolver := decoderForExpression(t, expression)
	defer closeSolver()
	decoded, err := decoder.DecodeCell(scope.Arena, cell)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatTLAExpression(decoded)
	if err != nil || formatted != "<<2, 3>>" {
		t.Fatalf("decoded subsequence = %q, %v", formatted, err)
	}
}

func TestSymbolicDecoderDynamicallyEmptyFunctionOracle(t *testing.T) {
	intSetType := mustSet(IntT1)
	functionType := mustFun(IntT1, IntT1)
	binder := NewNameExpr("t", Typed(IntT1))
	domain := MustOperExpr(Op("SET_FILTER"), Typed(intSetType), binder,
		MustOperExpr(Op("SET_ENUM"), Typed(intSetType), intLiteral(1)), boolExpr(false))
	expression := MustOperExpr(Op("FUN_CTOR"), Typed(functionType),
		MustOperExpr(Op("PLUS"), Typed(IntT1), NewNameExpr("x", Typed(IntT1)), intLiteral(1)),
		NewNameExpr("x", Typed(IntT1)), domain)
	decoder, scope, cell, closeSolver := decoderForExpression(t, expression)
	defer closeSolver()
	decoded, err := decoder.DecodeCell(scope.Arena, cell)
	if err != nil {
		t.Fatal(err)
	}
	formatted, err := FormatTLAExpression(decoded)
	if err != nil || formatted != "SetAsFun({})" {
		t.Fatalf("decoded dynamically empty function = %q, %v", formatted, err)
	}
}

func TestReverseMapVariableMatchesScalaRecursiveContract(t *testing.T) {
	cell := NewArenaCell(17, BasicCellType{Type: IntT1})
	other := NewArenaCell(18, BasicCellType{Type: IntT1})
	expression := MustOperExpr(Op("PLUS"), Typed(IntT1), cell.NameExpr(),
		MustOperExpr(Op("PLUS"), Typed(IntT1), other.NameExpr(), cell.NameExpr()))
	mapped := ReverseMapVariable(expression, "x", cell)
	formatted, err := FormatTLAExpression(mapped)
	if err != nil || formatted != "x + (_si_C_si_18 + x)" {
		t.Fatalf("reverse-mapped expression = %q, %v", formatted, err)
	}
}
