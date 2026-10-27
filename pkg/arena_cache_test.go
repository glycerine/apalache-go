package apalache

import (
	"math/big"
	"testing"
)

func TestIntValueCacheStoresValuesAndConstraints(t *testing.T) {
	cache := NewIntValueCache()
	arena := EmptyPureArena()
	value := new(big.Int).Exp(big.NewInt(2), big.NewInt(80), nil)
	firstArena, first, err := cache.GetOrCreate(arena, value)
	if err != nil || firstArena == arena || firstArena.CellCount() != 1 {
		t.Fatalf("first integer cache insertion = %v, %v", first, err)
	}
	secondArena, second, err := cache.GetOrCreate(firstArena, new(big.Int).Set(value))
	if err != nil || secondArena != firstArena || !second.Equal(first) {
		t.Fatalf("second integer cache lookup = %v, %v", second, err)
	}
	value.SetInt64(0)
	stored, found := cache.FindValue(first)
	if !found || stored.String() != "1208925819614629174706176" {
		t.Fatalf("cached integer was aliased: %v, %v", stored, found)
	}
	constraints := cache.Constraints()
	if len(constraints) != 1 || constraints[0].(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("integer constraints = %#v", constraints)
	}
}

func TestArenaCacheStackDiscipline(t *testing.T) {
	cache := NewIntValueCache()
	arena := EmptyPureArena()
	baseArena, base, _ := cache.GetOrCreate(arena, big.NewInt(1))
	cache.Push()
	topArena, top, _ := cache.GetOrCreate(baseArena, big.NewInt(2))
	if cache.Level() != 1 || len(cache.Constraints()) != 1 {
		t.Fatalf("top-level cache state = level %d, constraints %d", cache.Level(), len(cache.Constraints()))
	}
	if err := cache.Pop(); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get(big.NewInt(2)); found {
		t.Fatal("popped cache entry survived")
	}
	if foundBase, found := cache.Get(big.NewInt(1)); !found || !foundBase.Equal(base) {
		t.Fatal("base cache entry was removed")
	}
	if recreatedArena, recreated, err := cache.GetOrCreate(topArena, big.NewInt(2)); err != nil || recreatedArena.CellCount() != 3 || recreated.Equal(top) {
		t.Fatalf("recreated entry = arena %d, cell %v, error %v", recreatedArena.CellCount(), recreated, err)
	}
	if err := cache.Pop(1); err == nil {
		t.Fatal("cache popped below level zero")
	}
	cache.Dispose()
	if cache.Level() != 0 {
		t.Fatal("disposed cache retained its level")
	}
}

func TestUninterpretedLiteralCacheTypesAndDistinctness(t *testing.T) {
	cache := NewUninterpretedLiteralCache()
	typeA, _ := NewConstType("A")
	typeB, _ := NewConstType("B")
	arena := EmptyPureArena()
	var a1, a2, b1 ArenaCell
	var err error
	arena, a1, err = cache.GetOrCreate(arena, UninterpretedLiteral{Type: typeA, Index: "1"})
	if err != nil {
		t.Fatal(err)
	}
	arena, a2, err = cache.GetOrCreate(arena, UninterpretedLiteral{Type: typeA, Index: "2"})
	if err != nil {
		t.Fatal(err)
	}
	arena, b1, err = cache.GetOrCreate(arena, UninterpretedLiteral{Type: typeB, Index: "1"})
	if err != nil {
		t.Fatal(err)
	}
	if a1.Equal(a2) || a1.Equal(b1) || len(cache.Values()) != 3 {
		t.Fatalf("literal cache cells = %v, %v, %v", a1, a2, b1)
	}
	constraints := cache.Constraints()
	if len(constraints) != 2 {
		t.Fatalf("literal constraints = %#v", constraints)
	}
	for _, constraint := range constraints {
		if constraint.(*OperExpr).Operator().Name() != "Apalache!Distinct" {
			t.Fatalf("literal constraint = %#v", constraint)
		}
	}
	individual, err := cache.ConstraintFor(UninterpretedLiteral{Type: typeA, Index: "2"}, a2)
	if err != nil || len(individual.(*OperExpr).Arguments()) != 2 {
		t.Fatalf("individual literal constraint = %#v, %v", individual, err)
	}
	if _, _, err := cache.GetOrCreate(arena, UninterpretedLiteral{Type: IntT1, Index: "1"}); err == nil {
		t.Fatal("integer type was accepted as an uninterpreted literal type")
	}
}

func TestIntRangeCacheBuildsAndReusesArenaSets(t *testing.T) {
	ints := NewIntValueCache()
	cache := NewIntRangeCache(ints)
	arena, value, err := cache.GetOrCreate(EmptyPureArena(), -1, 1)
	if err != nil || len(value.Elements) != 3 || arena.CellCount() != 4 {
		t.Fatalf("range cache value = %#v, count %d, error %v", value, arena.CellCount(), err)
	}
	for index, expected := range []int64{-1, 0, 1} {
		actual, found := ints.FindValue(value.Elements[index])
		if !found || actual.Int64() != expected {
			t.Fatalf("range element %d = %v, %v", index, actual, found)
		}
	}
	sameArena, same, err := cache.GetOrCreate(arena, -1, 1)
	if err != nil || sameArena != arena || !same.Set.Equal(value.Set) {
		t.Fatalf("cached range lookup = %#v, %v", same, err)
	}
	if len(cache.Constraints()) != 3 || len(ints.Constraints()) != 3 {
		t.Fatalf("range/int constraints = %d/%d", len(cache.Constraints()), len(ints.Constraints()))
	}
	emptyArena, empty, err := cache.GetOrCreate(arena, 10, 2)
	if err != nil || len(empty.Elements) != 0 || emptyArena.CellCount() != arena.CellCount()+1 {
		t.Fatalf("empty range = %#v, %v", empty, err)
	}
}

func TestRecordDomainCacheSortsDeduplicatesAndSharesStrings(t *testing.T) {
	stringsCache := NewUninterpretedLiteralCache()
	cache := NewRecordDomainCache(stringsCache)
	arena, first, err := cache.GetOrCreate(EmptyPureArena(), []string{"b", "a", "b"})
	if err != nil || len(first.Elements) != 2 || arena.CellCount() != 3 {
		t.Fatalf("record domain = %#v, count %d, error %v", first, arena.CellCount(), err)
	}
	for index, expected := range []string{"a", "b"} {
		literal, found := stringsCache.FindLiteral(first.Elements[index])
		if !found || literal.Index != expected || !EqualTypes(literal.Type, StrT1) {
			t.Fatalf("domain key %d = %#v, %v", index, literal, found)
		}
	}
	sameArena, same, err := cache.GetOrCreate(arena, []string{"a", "b"})
	if err != nil || sameArena != arena || !same.Set.Equal(first.Set) {
		t.Fatalf("normalized domain lookup = %#v, %v", same, err)
	}
	secondArena, second, err := cache.GetOrCreate(arena, []string{"a", "c"})
	if err != nil || secondArena.CellCount() != 5 || len(stringsCache.Values()) != 3 || len(second.Elements) != 2 {
		t.Fatalf("second domain = %#v, count %d, strings %d, error %v", second, secondArena.CellCount(), len(stringsCache.Values()), err)
	}
	if len(cache.Constraints()) != 4 {
		t.Fatalf("record-domain constraints = %d", len(cache.Constraints()))
	}
	if len(stringsCache.Constraints()) != 1 {
		t.Fatalf("string-cache constraints = %d", len(stringsCache.Constraints()))
	}
}
