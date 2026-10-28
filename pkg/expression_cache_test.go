package apalache

import "testing"

func TestExpressionCacheGradeAndStackSemantics(t *testing.T) {
	store := NewExprGradeStore()
	cache := NewExpressionCache(store)
	constant := intLiteral(1)
	stateFree := NewNameExpr("x", Typed(IntT1))
	stateBound := NewNameExpr("i", Typed(IntT1))
	actionFree := MustOperExpr(Op("PRIME"), Typed(IntT1), stateFree)
	nonCacheable := MustOperExpr(Op("Apalache!Gen"), Typed(IntT1), intLiteral(1))
	variables := map[string]struct{}{"x": {}}
	for _, expression := range []Expr{constant, stateFree, stateBound, actionFree, nonCacheable} {
		LabelExpressionGrades(store, nil, variables, expression)
		cache.Put(expression, expression)
	}
	for _, expression := range []Expr{constant, stateFree, actionFree} {
		if _, found := cache.Get(expression); !found {
			t.Errorf("cacheable expression %v was not stored", expression)
		}
	}
	for _, expression := range []Expr{stateBound, nonCacheable} {
		if _, found := cache.Get(expression); found {
			t.Errorf("unsafe expression %v was cached", expression)
		}
	}
	unanalyzed := intLiteral(9)
	cache.Put(unanalyzed, unanalyzed)
	if _, found := cache.Get(unanalyzed); found {
		t.Fatal("unanalyzed expression was cached")
	}

	cache.Push()
	levelOne := intLiteral(2)
	cache.PutWithGrade(levelOne, ExpressionCacheValue{Expression: levelOne, Grade: ExprGradeConstant})
	cache.Push()
	levelTwo := intLiteral(3)
	cache.PutWithGrade(levelTwo, ExpressionCacheValue{Expression: levelTwo, Grade: ExprGradeConstant})
	if err := cache.Pop(); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get(levelTwo); found {
		t.Fatal("level-two entry survived pop")
	}
	if _, found := cache.Get(levelOne); !found {
		t.Fatal("level-one entry was removed too early")
	}
	if err := cache.Pop(); err != nil {
		t.Fatal(err)
	}
	if _, found := cache.Get(levelOne); found {
		t.Fatal("level-one entry survived pop")
	}
	if err := cache.Pop(); err == nil {
		t.Fatal("cache popped below level zero")
	}
}

func TestExpressionCacheStructuralSnapshotAndDisposal(t *testing.T) {
	cache := NewExpressionCache(NewExprGradeStore())
	original := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(IntT1)))
	result := intLiteral(1)
	cache.PutWithGrade(original, ExpressionCacheValue{Expression: result, Grade: ExprGradeConstant})
	structuralCopy := MustOperExpr(Op("SET_ENUM"), Typed(mustSet(mustSet(IntT1))))
	got, found := cache.Get(structuralCopy)
	if !found || !EqualExpr(got.Expression, result) {
		t.Fatal("structurally equal expression missed the cache")
	}
	if got.Expression.Tag().Equal(structuralCopy.Tag()) {
		t.Fatal("test did not exercise the caller's required type-tag check")
	}

	cache.PutWithGrade(NewNameExpr("x", Typed(IntT1)), ExpressionCacheValue{
		Expression: NewNameExpr("value", Typed(IntT1)), Grade: ExprGradeStateFree,
	})
	snapshot := cache.Snapshot()
	cache.DisposeActionLevel()
	if len(cache.Values()) != 1 || cache.Values()[0].Grade != ExprGradeConstant {
		t.Fatalf("action-level disposal retained %v", cache.Values())
	}
	cache.Dispose()
	cache.Recover(snapshot)
	if len(cache.Values()) != 2 {
		t.Fatalf("recovered values = %v", cache.Values())
	}
	other := NewExpressionCache(nil)
	other.Recover(snapshot)
	other.PutWithGrade(intLiteral(7), ExpressionCacheValue{Expression: intLiteral(7), Grade: ExprGradeConstant})
	if len(cache.Values()) != 2 || len(other.Values()) != 3 {
		t.Fatal("snapshot recovery aliased mutable cache state")
	}
}
