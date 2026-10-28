package apalache

import "testing"

func TestDefaultValueCacheBuildsAndReusesStructuralValues(t *testing.T) {
	rewriter := NewStratifiedRewriter()
	cache := rewriter.DefaultValueCache()
	scope := InitialRewriterScope()
	tuple, _ := NewTupleType(IntT1, BoolT1)
	sequence, _ := NewSeqType(IntT1)
	function, _ := NewFunType(IntT1, BoolT1)
	record, _ := NewRecordType(map[string]Type{"items": mustSet(IntT1), "pair": tuple})
	types := []Type{IntT1, BoolT1, StrT1, mustSet(IntT1), tuple, sequence, record, function}
	for _, typ := range types {
		var cell ArenaCell
		var err error
		scope, cell, err = cache.GetOrCreate(scope, typ)
		if err != nil || !EqualTypes(cell.Type.TLAType(), typ) {
			t.Fatalf("default %s = %v, %v", typ, cell, err)
		}
		sameScope, same, err := cache.GetOrCreate(scope, typ)
		if err != nil || sameScope.Arena != scope.Arena || !same.Equal(cell) {
			t.Fatalf("reused default %s = %v, %v", typ, same, err)
		}
	}
	sequenceCell, _ := cache.Get(sequence)
	length, _, elements, err := unpackSequence(scope.Arena, sequenceCell)
	if err != nil || len(elements) != 0 {
		t.Fatalf("default sequence = length %v, elements %#v, %v", length, elements, err)
	}
	lengthValue, found := rewriter.IntegerCache().FindValue(length)
	if !found || lengthValue.Sign() != 0 {
		t.Fatalf("default sequence length = %v, %v", lengthValue, found)
	}
	functionCell, _ := cache.Get(function)
	domain, err := scope.Arena.Domain(functionCell)
	if err != nil || len(scope.Arena.GetHas(domain)) != 0 {
		t.Fatalf("default function domain = %v, %v", domain, err)
	}
	relation, err := scope.Arena.Codomain(functionCell)
	if err != nil || len(scope.Arena.GetHas(relation)) != 0 {
		t.Fatalf("default function relation = %v, %v", relation, err)
	}
}

func TestDefaultValueCacheSupportsUninterpretedConstants(t *testing.T) {
	constant, err := NewConstType("PERSON")
	if err != nil {
		t.Fatal(err)
	}
	rewriter := NewStratifiedRewriter()
	scope, cell, err := rewriter.DefaultValueCache().GetOrCreate(InitialRewriterScope(), constant)
	if err != nil || !EqualTypes(cell.Type.TLAType(), constant) {
		t.Fatalf("constant default = %v, %v", cell, err)
	}
	literal, found := rewriter.LiteralCache().FindLiteral(cell)
	if !found || !EqualTypes(literal.Type, constant) || literal.Index != ConstructModelValue("PERSON", "None") {
		t.Fatalf("constant default literal = %#v, %v", literal, found)
	}
	if scope.Arena.CellCount() != InitialPureArena().CellCount()+1 {
		t.Fatalf("constant default arena count = %d", scope.Arena.CellCount())
	}
}

func TestDefaultValueCacheBuildsCanonicalRecordValues(t *testing.T) {
	recordType, err := NewRecordType(map[string]Type{"a": IntT1, "b": BoolT1})
	if err != nil {
		t.Fatal(err)
	}
	rewriter := NewStratifiedRewriter()
	scope, record, err := rewriter.DefaultValueCache().GetOrCreate(InitialRewriterScope(), recordType)
	if err != nil {
		t.Fatal(err)
	}
	fields := scope.Arena.GetHas(record)
	if len(fields) != 2 {
		t.Fatalf("default record fields = %#v", fields)
	}
	integer, found := rewriter.IntegerCache().FindValue(fields[0].Element())
	if !found || integer.Sign() != 0 {
		t.Fatalf("default integer field = %v, found %v", integer, found)
	}
	if !fields[1].Element().Equal(scope.Arena.FalseCell()) {
		t.Fatalf("default Boolean field = %s", fields[1].Element())
	}
}
