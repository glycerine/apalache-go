package apalache

import "testing"

func equalityCacheCells() (ArenaCell, ArenaCell, ArenaCell) {
	arena, cells := EmptyPureArena().AppendCellTypes(
		BasicCellType{Type: IntT1},
		BasicCellType{Type: IntT1},
		BasicCellType{Type: BoolT1},
	)
	_ = arena
	return cells[0], cells[1], cells[2]
}

func TestEqualityCacheCanonicalizesPairsAndTranslatesEntries(t *testing.T) {
	cache := NewEqualityCache()
	left, right, predicate := equalityCacheCells()
	if err := cache.Put(right, left, EqualityDirect{}); err != nil {
		t.Fatal(err)
	}
	entry, found := cache.Get(left, right)
	if !found {
		t.Fatal("symmetric equality lookup missed the cached pair")
	}
	direct, err := cache.ToExpr(left, right, entry)
	if err != nil || direct.(*OperExpr).Operator().Name() != "EQ" {
		t.Fatalf("direct cache expression = %#v, %v", direct, err)
	}

	cases := []struct {
		entry EqualityCacheEntry
		want  Expr
	}{
		{EqualityFalse{}, NewValueExpr(BoolValue(false), Typed(BoolT1))},
		{EqualityTrue{}, NewValueExpr(BoolValue(true), Typed(BoolT1))},
		{EqualityExpression{Expression: predicate.NameExpr()}, predicate.NameExpr()},
	}
	for _, test := range cases {
		actual, err := cache.ToExpr(left, right, test.entry)
		if err != nil || !EqualExpr(actual, test.want) {
			t.Fatalf("cache expression for %T = %#v, %v", test.entry, actual, err)
		}
	}
	if err := cache.Put(left, right, EqualityExpression{}); err == nil {
		t.Fatal("nil equality expression was accepted")
	}
}

func TestEqualityCacheRollsBackNestedLevels(t *testing.T) {
	cache := NewEqualityCache()
	first, second, third := equalityCacheCells()
	if err := cache.Put(first, second, EqualityTrue{}); err != nil {
		t.Fatal(err)
	}
	cache.Push()
	if err := cache.Put(first, third, EqualityFalse{}); err != nil {
		t.Fatal(err)
	}
	cache.Push()
	if err := cache.Put(second, third, EqualityDirect{}); err != nil {
		t.Fatal(err)
	}
	if err := cache.Pop(2); err != nil {
		t.Fatal(err)
	}
	if cache.Level() != 0 {
		t.Fatalf("cache level = %d", cache.Level())
	}
	if _, found := cache.Get(first, second); !found {
		t.Fatal("base equality was removed")
	}
	if _, found := cache.Get(first, third); found {
		t.Fatal("level-one equality survived rollback")
	}
	if _, found := cache.Get(second, third); found {
		t.Fatal("level-two equality survived rollback")
	}
	if err := cache.Pop(); err == nil {
		t.Fatal("cache popped below level zero")
	}
}

func TestEqualityCacheSnapshotsAreSquashedAndIndependent(t *testing.T) {
	cache := NewEqualityCache()
	first, second, third := equalityCacheCells()
	cache.Push()
	if err := cache.Put(first, second, EqualityDirect{}); err != nil {
		t.Fatal(err)
	}
	snapshot := cache.Snapshot()

	cache.Dispose()
	if cache.Level() != 0 {
		t.Fatal("disposed cache retained its level")
	}
	cache.Recover(snapshot)
	if cache.Level() != 0 {
		t.Fatal("recovered snapshot retained the source stack level")
	}
	if _, found := cache.Get(second, first); !found {
		t.Fatal("recovered snapshot lost an entry")
	}
	cache.Push()
	if err := cache.Put(first, third, EqualityTrue{}); err != nil {
		t.Fatal(err)
	}

	other := NewEqualityCache()
	other.Recover(snapshot)
	if _, found := other.Get(first, third); found {
		t.Fatal("mutating a recovered cache changed its snapshot")
	}
	if err := cache.Pop(); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get(first, second); !found {
		t.Fatal("squashed snapshot entry was removed at level zero")
	}
}

func TestEqualityCacheRecoveryRetainsReceiverLevel(t *testing.T) {
	source := NewEqualityCache()
	first, second, _ := equalityCacheCells()
	if err := source.Put(first, second, EqualityTrue{}); err != nil {
		t.Fatal(err)
	}
	snapshot := source.Snapshot()
	target := NewEqualityCache()
	target.Push()
	target.Push()
	target.Recover(snapshot)
	if target.Level() != 2 {
		t.Fatalf("recovery reset receiver level to %d", target.Level())
	}
	if err := target.Pop(2); err != nil {
		t.Fatal(err)
	}
	if _, found := target.Get(first, second); !found {
		t.Fatal("squashed equality entry was removed")
	}
}

func TestEqualityCacheAcceptsCellsWithNonComparableTypes(t *testing.T) {
	record, err := NewRecordType(map[string]Type{"x": IntT1})
	if err != nil {
		t.Fatal(err)
	}
	left := NewArenaCell(7, BasicCellType{Type: record})
	right := NewArenaCell(8, BasicCellType{Type: record})
	cache := NewEqualityCache()
	if err := cache.Put(left, right, EqualityDirect{}); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get(right, left); !found {
		t.Fatal("cache missed cells whose types contain maps")
	}
}
