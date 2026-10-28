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

func TestIntValueCacheScalaOracleCases(t *testing.T) {
	cache := NewIntValueCache()
	arena := EmptyPureArena()
	if _, found := cache.Get(big.NewInt(42)); found {
		t.Fatal("uncached integer 42 was found")
	}
	firstArena, fortyTwo, err := cache.GetOrCreate(arena, big.NewInt(42))
	if err != nil || firstArena == arena {
		t.Fatalf("first insertion = arena %p, cell %v, error %v", firstArena, fortyTwo, err)
	}
	secondArena, same, err := cache.GetOrCreate(firstArena, big.NewInt(42))
	if err != nil || secondArena != firstArena || !same.Equal(fortyTwo) {
		t.Fatalf("repeated insertion = arena %p, cell %v, error %v", secondArena, same, err)
	}
	finalArena, zero, err := cache.GetOrCreate(secondArena, big.NewInt(0))
	if err != nil || finalArena == secondArena {
		t.Fatalf("second value insertion = arena %p, cell %v, error %v", finalArena, zero, err)
	}
	constraints := cache.Constraints()
	if len(constraints) != 2 {
		t.Fatalf("integer constraints = %v", constraints)
	}
	for _, test := range []struct {
		cell ArenaCell
		want int64
	}{{fortyTwo, 42}, {zero, 0}} {
		found := false
		for _, constraint := range constraints {
			arguments := constraint.(*OperExpr).Arguments()
			if arguments[0].(*NameExpr).Name() == test.cell.NameExpr().Name() &&
				arguments[1].(*ValueExpr).Value().(IntValue).BigInt().Int64() == test.want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing equality for %v = %d", test.cell, test.want)
		}
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

func TestArenaCacheSnapshotSquashesEntriesAndRetainsReceiverLevel(t *testing.T) {
	source := NewIntValueCache()
	arena := EmptyPureArena()
	source.Push()
	arena, cell, err := source.GetOrCreate(arena, big.NewInt(9))
	if err != nil {
		t.Fatal(err)
	}
	snapshot := source.Snapshot()
	source.Dispose()

	target := NewIntValueCache()
	target.Push()
	target.Push()
	target.Recover(snapshot)
	if target.Level() != 2 {
		t.Fatalf("recovery reset receiver level to %d", target.Level())
	}
	if got, found := target.Get(big.NewInt(9)); !found || !got.Equal(cell) {
		t.Fatal("recovered snapshot lost its forward mapping")
	}
	if got, found := target.FindValue(cell); !found || got.Int64() != 9 {
		t.Fatal("recovered snapshot lost its reverse mapping")
	}
	if err := target.Pop(2); err != nil {
		t.Fatal(err)
	}
	if _, found := target.Get(big.NewInt(9)); !found {
		t.Fatal("squashed level-zero entry was removed")
	}

	// Recovery copies both maps; later cache mutation cannot mutate the shot.
	_, _, err = target.GetOrCreate(arena, big.NewInt(10))
	if err != nil {
		t.Fatal(err)
	}
	other := NewIntValueCache()
	other.Recover(snapshot)
	if _, found := other.Get(big.NewInt(10)); found {
		t.Fatal("cache mutation changed its snapshot")
	}
}

func TestArenaCachesImplementStackableAndRecoverableContracts(t *testing.T) {
	var _ StackableContext = (*IntValueCache)(nil)
	var _ StackableContext = (*UninterpretedLiteralCache)(nil)
	var _ StackableContext = (*IntRangeCache)(nil)
	var _ StackableContext = (*RecordDomainCache)(nil)
	var _ Recoverable[IntValueCacheSnapshot] = (*IntValueCache)(nil)
	var _ Recoverable[UninterpretedLiteralCacheSnapshot] = (*UninterpretedLiteralCache)(nil)
	var _ Recoverable[IntRangeCacheSnapshot] = (*IntRangeCache)(nil)
	var _ Recoverable[RecordDomainCacheSnapshot] = (*RecordDomainCache)(nil)
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

func TestUninterpretedLiteralCacheScalaOracleCases(t *testing.T) {
	cache := NewUninterpretedLiteralCache()
	plain := UninterpretedLiteral{Type: StrT1, Index: "idx"}
	base := EmptyPureArena()
	if _, found := cache.Get(plain); found {
		t.Fatal("uncached string literal was found")
	}
	arena, plainCell, err := cache.GetOrCreate(base, plain)
	if err != nil || arena == base {
		t.Fatalf("first literal insertion = arena %p, cell %v, error %v", arena, plainCell, err)
	}
	sameArena, sameCell, err := cache.GetOrCreate(arena, plain)
	if err != nil || sameArena != arena || !sameCell.Equal(plainCell) {
		t.Fatalf("literal reuse = arena %p, cell %v, error %v", sameArena, sameCell, err)
	}
	typeA := mustNamedConst(t, "A")
	typeB := mustNamedConst(t, "B")
	arena, aCell, _ := cache.GetOrCreate(arena, UninterpretedLiteral{Type: typeA, Index: "idx"})
	_, bCell, _ := cache.GetOrCreate(arena, UninterpretedLiteral{Type: typeB, Index: "idx"})
	if plainCell.Equal(aCell) || aCell.Equal(bCell) {
		t.Fatal("same literal index was shared across distinct types")
	}

	distinctCache := NewUninterpretedLiteralCache()
	distinctArena := base
	cells := make([]ArenaCell, 3)
	for index, value := range []string{"1", "2", "3"} {
		distinctArena, cells[index], err = distinctCache.GetOrCreate(distinctArena,
			UninterpretedLiteral{Type: typeA, Index: value})
		if err != nil {
			t.Fatal(err)
		}
		constraint, err := distinctCache.ConstraintFor(UninterpretedLiteral{Type: typeA, Index: value}, cells[index])
		if err != nil || len(constraint.(*OperExpr).Arguments()) != index+1 {
			t.Fatalf("incremental distinctness %d = %v, %v", index, constraint, err)
		}
	}
	constraints := distinctCache.Constraints()
	if len(constraints) != 1 {
		t.Fatalf("global distinctness constraints = %v", constraints)
	}
	arguments := constraints[0].(*OperExpr).Arguments()
	if len(arguments) != 3 || arguments[0].(*NameExpr).Name() != cells[2].NameExpr().Name() ||
		arguments[1].(*NameExpr).Name() != cells[1].NameExpr().Name() ||
		arguments[2].(*NameExpr).Name() != cells[0].NameExpr().Name() {
		t.Fatalf("global distinctness arguments = %v", arguments)
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

func TestIntRangeCacheScalaOracleCases(t *testing.T) {
	integers := NewIntValueCache()
	cache := NewIntRangeCache(integers)
	base := EmptyPureArena()
	if _, found := cache.Get(1, 5); found {
		t.Fatal("uncached range 1..5 was found")
	}
	arena, oneToFive, err := cache.GetOrCreate(base, 1, 5)
	if err != nil || len(oneToFive.Elements) != 5 || arena == base {
		t.Fatalf("range 1..5 = %#v, arena %p, error %v", oneToFive, arena, err)
	}
	sameArena, same, err := cache.GetOrCreate(arena, 1, 5)
	if err != nil || sameArena != arena || !same.Set.Equal(oneToFive.Set) {
		t.Fatalf("reused range = %#v, arena %p, error %v", same, sameArena, err)
	}

	emptyArena, empty, err := cache.GetOrCreate(base, 5, 1)
	if err != nil || len(empty.Elements) != 0 {
		t.Fatalf("empty 5..1 = %#v, %v", empty, err)
	}
	otherEmptyArena, otherEmpty, err := cache.GetOrCreate(base, 1203134, -221)
	if err != nil || len(otherEmpty.Elements) != 0 || !otherEmpty.Set.Equal(empty.Set) ||
		otherEmptyArena.CellCount() != emptyArena.CellCount() {
		t.Fatalf("second empty range = %#v, count %d, error %v", otherEmpty, otherEmptyArena.CellCount(), err)
	}

	constraintInts := NewIntValueCache()
	constraintCache := NewIntRangeCache(constraintInts)
	a1, r1, _ := constraintCache.GetOrCreate(base, 0, 10)
	constraintCache.GetOrCreate(base, 0, 10)
	a2, r2, _ := constraintCache.GetOrCreate(a1, 3, 7)
	constraintCache.GetOrCreate(a1, 3, 7)
	_, r3, _ := constraintCache.GetOrCreate(a2, 100, 0)
	constraints := constraintCache.Constraints()
	if len(constraints) != len(r1.Elements)+len(r2.Elements)+len(r3.Elements) || len(constraints) != 16 {
		t.Fatalf("range constraints = %d, want 16", len(constraints))
	}
	if len(constraintInts.Constraints()) != 11 {
		t.Fatalf("dependent integer constraints = %d, want 11 when discharged separately", len(constraintInts.Constraints()))
	}
	for _, constraint := range constraints {
		if constraint.(*OperExpr).Operator().Name() != "SET_IN" {
			t.Fatalf("range constraint = %v", constraint)
		}
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

func TestRecordDomainCacheScalaOracleCases(t *testing.T) {
	stringsCache := NewUninterpretedLiteralCache()
	cache := NewRecordDomainCache(stringsCache)
	base := EmptyPureArena()
	if _, found := cache.Get([]string{"a", "b", "c"}); found {
		t.Fatal("uncached record domain was found")
	}
	arena, abc, err := cache.GetOrCreate(base, []string{"a", "b", "c"})
	if err != nil || arena == base || len(abc.Elements) != 3 {
		t.Fatalf("record domain insertion = %#v, arena %p, error %v", abc, arena, err)
	}
	sameArena, same, err := cache.GetOrCreate(arena, []string{"c", "a", "b"})
	if err != nil || sameArena != arena || !same.Set.Equal(abc.Set) {
		t.Fatalf("record domain reuse = %#v, arena %p, error %v", same, sameArena, err)
	}

	constraintStrings := NewUninterpretedLiteralCache()
	constraintCache := NewRecordDomainCache(constraintStrings)
	a1, empty, _ := constraintCache.GetOrCreate(base, nil)
	constraintCache.GetOrCreate(base, nil)
	a2, ab, _ := constraintCache.GetOrCreate(a1, []string{"a", "b"})
	constraintCache.GetOrCreate(a1, []string{"a", "b"})
	_, ac, _ := constraintCache.GetOrCreate(a2, []string{"a", "c"})
	constraintCache.GetOrCreate(a2, []string{"a", "c"})
	if len(empty.Elements) != 0 || len(constraintStrings.Values()) != 3 {
		t.Fatalf("domain elements/strings = %d/%d", len(empty.Elements), len(constraintStrings.Values()))
	}
	constraints := constraintCache.Constraints()
	if len(constraints) != len(empty.Elements)+len(ab.Elements)+len(ac.Elements) || len(constraints) != 4 {
		t.Fatalf("record-domain constraints = %d, want 4", len(constraints))
	}
	for _, constraint := range constraints {
		if constraint.(*OperExpr).Operator().Name() != "SET_IN" {
			t.Fatalf("record-domain constraint = %v", constraint)
		}
	}
}
