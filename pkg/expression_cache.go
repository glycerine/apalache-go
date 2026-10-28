package apalache

import (
	"fmt"
	"sync"
)

type ExpressionCacheValue struct {
	Expression Expr
	Grade      ExprGrade
}

type expressionCacheEntry struct {
	key   Expr
	value ExpressionCacheValue
	level int
}

type ExpressionCacheSnapshot struct {
	entries []expressionCacheEntry
}

// ExpressionCache ports Scala's grade-aware ExprCache. Keys deliberately use
// structural expression equality; callers must still compare result type tags
// before reusing a cached expression.
type ExpressionCache struct {
	mu      sync.RWMutex
	store   *ExprGradeStore
	level   int
	entries []expressionCacheEntry
}

func NewExpressionCache(store *ExprGradeStore) *ExpressionCache {
	return &ExpressionCache{store: store}
}

func expressionCacheable(grade ExprGrade) bool {
	return grade == ExprGradeConstant || grade == ExprGradeStateFree || grade == ExprGradeActionFree
}

// Put obtains the key grade from the analysis store. An unanalyzed or unsafe
// expression is intentionally ignored.
func (cache *ExpressionCache) Put(key, value Expr) {
	if key == nil || value == nil || cache.store == nil {
		return
	}
	grade, found := cache.store.Get(key.ID())
	if !found {
		return
	}
	cache.PutWithGrade(key, ExpressionCacheValue{Expression: value, Grade: grade})
}

func (cache *ExpressionCache) PutWithGrade(key Expr, value ExpressionCacheValue) {
	if key == nil || value.Expression == nil || !expressionCacheable(value.Grade) {
		return
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	for index := range cache.entries {
		if EqualExpr(cache.entries[index].key, key) {
			cache.entries[index] = expressionCacheEntry{key: key, value: value, level: cache.level}
			return
		}
	}
	cache.entries = append(cache.entries, expressionCacheEntry{key: key, value: value, level: cache.level})
}

func (cache *ExpressionCache) Get(key Expr) (ExpressionCacheValue, bool) {
	if key == nil {
		return ExpressionCacheValue{}, false
	}
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	for _, entry := range cache.entries {
		if EqualExpr(entry.key, key) {
			return entry.value, true
		}
	}
	return ExpressionCacheValue{}, false
}

func (cache *ExpressionCache) Values() []ExpressionCacheValue {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	values := make([]ExpressionCacheValue, len(cache.entries))
	for index, entry := range cache.entries {
		values[index] = entry.value
	}
	return values
}

func (cache *ExpressionCache) Level() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.level
}

func (cache *ExpressionCache) Push() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.level++
}

func (cache *ExpressionCache) Pop(count ...int) error {
	n := 1
	if len(count) > 1 {
		return fmt.Errorf("expression cache pop accepts at most one count")
	}
	if len(count) == 1 {
		n = count[0]
	}
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if n < 0 || n > cache.level {
		return fmt.Errorf("cannot pop %d levels from expression cache level %d", n, cache.level)
	}
	cache.level -= n
	kept := cache.entries[:0]
	for _, entry := range cache.entries {
		if entry.level <= cache.level {
			kept = append(kept, entry)
		}
	}
	cache.entries = kept
	return nil
}

func (cache *ExpressionCache) Snapshot() ExpressionCacheSnapshot {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	entries := make([]expressionCacheEntry, len(cache.entries))
	for index, entry := range cache.entries {
		entries[index] = expressionCacheEntry{key: entry.key, value: entry.value, level: 0}
	}
	return ExpressionCacheSnapshot{entries: entries}
}

func (cache *ExpressionCache) Recover(snapshot ExpressionCacheSnapshot) {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries = append([]expressionCacheEntry(nil), snapshot.entries...)
}

func (cache *ExpressionCache) DisposeActionLevel() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	kept := cache.entries[:0]
	for _, entry := range cache.entries {
		if entry.value.Grade == ExprGradeConstant {
			kept = append(kept, entry)
		}
	}
	cache.entries = kept
}

func (cache *ExpressionCache) Dispose() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries = nil
	cache.level = 0
}
