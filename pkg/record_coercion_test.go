package apalache

import (
	"context"
	"testing"
)

func TestStratifiedSparseLegacyRecordChoicePreservesDomains(t *testing.T) {
	recordType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	recordTag := Typed(recordType)
	first := MustOperExpr(Op("RECORD"), recordTag,
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
	second := MustOperExpr(Op("RECORD"), recordTag,
		NewValueExpr(StringValue("b"), Typed(StrT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	setType := mustSet(recordType)
	set := MustOperExpr(Op("SET_ENUM"), Typed(setType), first, second)
	choice := MustOperExpr(Op("Apalache!Guess"), recordTag, set)

	rewriter := NewStratifiedRewriter()
	scope, chosen, err := rewriter.Rewrite(choice, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	if len(scope.Arena.GetHas(chosen)) != 2 || !scope.Arena.HasDomain(chosen) {
		t.Fatalf("chosen sparse record lacks total fields or domain: %s", chosen)
	}
	// The Guess membership constraint must imply equality with one of the two
	// sparse records, including equality of their dynamic domains.
	sets := scope.Arena.FindCellsByType(BasicCellType{Type: setType})
	if len(sets) != 1 {
		t.Fatalf("found %d candidate record sets", len(sets))
	}
	records := scope.Arena.GetHas(sets[0])
	if len(records) != 2 {
		t.Fatalf("record set has %d elements", len(records))
	}
	left, err := rewriter.equality.Equal(scope.Arena, chosen, records[0].Element())
	if err != nil {
		t.Fatal(err)
	}
	right, err := rewriter.equality.Equal(scope.Arena, chosen, records[1].Element())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	defer solver.Close()
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), booleanOr(left, right))); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("sparse record choice escaped candidates: %v, %v", status, checkErr)
	}
}

func TestStratifiedWidensFiniteSetsOfLegacyRecords(t *testing.T) {
	narrowType, _ := NewRecordType(map[string]Type{"b": BoolT1})
	wideType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	narrow := MustOperExpr(Op("RECORD"), Typed(narrowType),
		NewValueExpr(StringValue("b"), Typed(StrT1)), NewValueExpr(BoolValue(true), Typed(BoolT1)))
	narrowSet := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(narrowType)), narrow)

	rewriter := NewStratifiedRewriter()
	scope, setCell, err := rewriter.Rewrite(narrowSet, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope, widened, err := rewriter.coerceCell(scope, setCell, mustSet(wideType))
	if err != nil {
		t.Fatal(err)
	}
	pointers := scope.Arena.GetHas(widened)
	if len(pointers) != 1 || !EqualTypes(pointers[0].Element().Type.TLAType(), wideType) ||
		!scope.Arena.HasDomain(pointers[0].Element()) {
		t.Fatalf("widened record set = %s, %#v", widened, pointers)
	}
}

func TestStratifiedCoercesClosedRowRecordToLegacyRecord(t *testing.T) {
	row, err := NewRowType(map[string]Type{"cnt": IntT1}, nil)
	if err != nil {
		t.Fatal(err)
	}
	rowType := NewRecordRowType(row)
	legacyType, _ := NewRecordType(map[string]Type{"cnt": IntT1})
	record := MustOperExpr(Op("RECORD"), Typed(rowType),
		NewValueExpr(StringValue("cnt"), Typed(StrT1)), intLiteral(2))

	rewriter := NewStratifiedRewriter()
	scope, rowRecord, err := rewriter.Rewrite(record, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope, coerced, err := rewriter.coerceCell(scope, rowRecord, legacyType)
	if err != nil {
		t.Fatal(err)
	}
	if !EqualTypes(coerced.Type.TLAType(), legacyType) || len(scope.Arena.GetHas(coerced)) != 1 || !scope.Arena.HasDomain(coerced) {
		t.Fatalf("coerced row record = %s, %#v", coerced, scope.Arena.GetHas(coerced))
	}
}

func TestStratifiedEqualityWidensSparseLegacyRecords(t *testing.T) {
	narrowType, _ := NewRecordType(map[string]Type{"a": IntT1})
	wideType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	narrow := MustOperExpr(Op("RECORD"), Typed(narrowType),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
	wideSparse := MustOperExpr(Op("RECORD"), Typed(wideType),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
	equal := MustOperExpr(Op("EQ"), Typed(BoolT1), narrow, wideSparse)

	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(equal, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	solver := newTestG3Solver(t)
	defer solver.Close()
	encoder, err := EncodeRewriterScope(solver, scope, rewriter.Constraints())
	if err != nil {
		t.Fatal(err)
	}
	if err = encoder.Assert(MustOperExpr(Op("NOT"), Typed(BoolT1), result.NameExpr())); err != nil {
		t.Fatal(err)
	}
	if status, checkErr := solver.Check(context.Background()); checkErr != nil || status != SMTUnsatisfiable {
		t.Fatalf("equal sparse records with different static shapes were distinguished: %v, %v", status, checkErr)
	}
}

func TestStratifiedWidensNestedLegacyRecordFields(t *testing.T) {
	narrowType, _ := NewRecordType(map[string]Type{"a": IntT1})
	wideType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	outerType, _ := NewRecordType(map[string]Type{"nested": wideType})
	narrow := MustOperExpr(Op("RECORD"), Typed(narrowType),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
	outer := MustOperExpr(Op("RECORD"), Typed(outerType),
		NewValueExpr(StringValue("nested"), Typed(StrT1)), narrow)

	rewriter := NewStratifiedRewriter()
	scope, result, err := rewriter.Rewrite(outer, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	nested := scope.Arena.GetHas(result)[0].Element()
	if !EqualTypes(nested.Type.TLAType(), wideType) || !scope.Arena.HasDomain(nested) {
		t.Fatalf("nested sparse record was not widened: %s", nested)
	}
}

func TestStratifiedWidensMappedAndFunctionResults(t *testing.T) {
	narrowType, _ := NewRecordType(map[string]Type{"a": IntT1})
	wideType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	narrow := MustOperExpr(Op("RECORD"), Typed(narrowType),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
	domain := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)), intLiteral(1))
	binder := NewNameExpr("x", Typed(IntT1))

	t.Run("set map", func(t *testing.T) {
		mapped := MustOperExpr(Op("SET_MAP"), Typed(mustSet(wideType)), narrow, binder, domain)
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(mapped, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		element := scope.Arena.GetHas(result)[0].Element()
		if !EqualTypes(element.Type.TLAType(), wideType) || !scope.Arena.HasDomain(element) {
			t.Fatalf("mapped sparse record was not widened: %s", element)
		}
	})

	t.Run("function constructor", func(t *testing.T) {
		functionType, _ := NewFunType(IntT1, wideType)
		function := MustOperExpr(Op("FUN_CTOR"), Typed(functionType), narrow, binder, domain)
		rewriter := NewStratifiedRewriter()
		scope, result, err := rewriter.Rewrite(function, InitialRewriterScope())
		if err != nil {
			t.Fatal(err)
		}
		relation, err := scope.Arena.Codomain(result)
		if err != nil {
			t.Fatal(err)
		}
		pair := scope.Arena.GetHas(relation)[0].Element()
		mapped := scope.Arena.GetHas(pair)[1].Element()
		if !EqualTypes(mapped.Type.TLAType(), wideType) || !scope.Arena.HasDomain(mapped) {
			t.Fatalf("function sparse result was not widened: %s", mapped)
		}
	})
}

func TestStratifiedWidensTupleElements(t *testing.T) {
	narrowType, _ := NewRecordType(map[string]Type{"a": IntT1})
	wideType, _ := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	narrowTuple, _ := NewTupleType(narrowType)
	wideTuple, _ := NewTupleType(wideType)
	narrow := MustOperExpr(Op("RECORD"), Typed(narrowType),
		NewValueExpr(StringValue("a"), Typed(StrT1)), intLiteral(1))
	tuple := MustOperExpr(Op("TUPLE"), Typed(narrowTuple), narrow)

	rewriter := NewStratifiedRewriter()
	scope, tupleCell, err := rewriter.Rewrite(tuple, InitialRewriterScope())
	if err != nil {
		t.Fatal(err)
	}
	scope, widened, err := rewriter.coerceCell(scope, tupleCell, wideTuple)
	if err != nil {
		t.Fatal(err)
	}
	element := scope.Arena.GetHas(widened)[0].Element()
	if !EqualTypes(element.Type.TLAType(), wideType) || !scope.Arena.HasDomain(element) {
		t.Fatalf("tuple sparse record was not widened: %s", element)
	}
}
