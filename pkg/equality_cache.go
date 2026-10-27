package apalache

import "fmt"

// EqualityCacheEntry describes how equality between a pair of arena cells is
// represented. The entry variants mirror EqCache.CacheEntry in the Scala
// implementation.
type EqualityCacheEntry interface {
	equalityCacheEntry()
}

type EqualityFalse struct{}
type EqualityTrue struct{}
type EqualityDirect struct{}
type EqualityExpression struct{ Expression Expr }

func (EqualityFalse) equalityCacheEntry()      {}
func (EqualityTrue) equalityCacheEntry()       {}
func (EqualityDirect) equalityCacheEntry()     {}
func (EqualityExpression) equalityCacheEntry() {}

type equalityPair struct {
	left  ArenaCell
	right ArenaCell
}

type equalityCacheValue struct {
	entry EqualityCacheEntry
	level int
}

// EqualityCacheSnapshot is an immutable, level-squashed copy of an equality
// cache. Recovering it starts a new cache stack at level zero.
type EqualityCacheSnapshot struct {
	entries map[equalityPair]equalityCacheValue
}

// EqualityCache remembers equalities whose defining constraints have already
// been generated. Pairs are canonicalized, so lookup is symmetric.
type EqualityCache struct {
	level   int
	entries map[equalityPair]equalityCacheValue
}

func NewEqualityCache() *EqualityCache {
	return &EqualityCache{entries: make(map[equalityPair]equalityCacheValue)}
}

func canonicalEqualityPair(left, right ArenaCell) equalityPair {
	if left.ID <= right.ID {
		return equalityPair{left: left, right: right}
	}
	return equalityPair{left: right, right: left}
}

func (cache *EqualityCache) Put(left, right ArenaCell, entry EqualityCacheEntry) error {
	if entry == nil {
		return fmt.Errorf("equality cache entry cannot be nil")
	}
	if expression, ok := entry.(EqualityExpression); ok && expression.Expression == nil {
		return fmt.Errorf("cached equality expression cannot be nil")
	}
	cache.entries[canonicalEqualityPair(left, right)] = equalityCacheValue{entry: entry, level: cache.level}
	return nil
}

func (cache *EqualityCache) Get(left, right ArenaCell) (EqualityCacheEntry, bool) {
	value, found := cache.entries[canonicalEqualityPair(left, right)]
	return value.entry, found
}

// ToExpr translates a cache entry into the Boolean expression it denotes.
func (cache *EqualityCache) ToExpr(left, right ArenaCell, entry EqualityCacheEntry) (Expr, error) {
	switch typed := entry.(type) {
	case EqualityFalse:
		return NewValueExpr(BoolValue(false), Typed(BoolT1)), nil
	case EqualityTrue:
		return NewValueExpr(BoolValue(true), Typed(BoolT1)), nil
	case EqualityDirect:
		return MustOperExpr(Op("EQ"), Typed(BoolT1), left.NameExpr(), right.NameExpr()), nil
	case EqualityExpression:
		if typed.Expression == nil {
			return nil, fmt.Errorf("cached equality expression cannot be nil")
		}
		return typed.Expression, nil
	default:
		return nil, fmt.Errorf("unknown equality cache entry %T", entry)
	}
}

func (cache *EqualityCache) Level() int { return cache.level }

func (cache *EqualityCache) Push() { cache.level++ }

func (cache *EqualityCache) Pop(count ...int) error {
	n := 1
	if len(count) > 1 {
		return fmt.Errorf("equality cache pop accepts at most one count")
	}
	if len(count) == 1 {
		n = count[0]
	}
	if n < 0 || n > cache.level {
		return fmt.Errorf("cannot pop %d levels from an equality cache at level %d", n, cache.level)
	}
	cache.level -= n
	for pair, value := range cache.entries {
		if value.level > cache.level {
			delete(cache.entries, pair)
		}
	}
	return nil
}

func (cache *EqualityCache) Snapshot() EqualityCacheSnapshot {
	entries := make(map[equalityPair]equalityCacheValue, len(cache.entries))
	for pair, value := range cache.entries {
		entries[pair] = equalityCacheValue{entry: value.entry, level: 0}
	}
	return EqualityCacheSnapshot{entries: entries}
}

func (cache *EqualityCache) Recover(snapshot EqualityCacheSnapshot) {
	cache.entries = make(map[equalityPair]equalityCacheValue, len(snapshot.entries))
	for pair, value := range snapshot.entries {
		cache.entries[pair] = equalityCacheValue{entry: value.entry, level: 0}
	}
	cache.level = 0
}

func (cache *EqualityCache) Dispose() {
	cache.entries = make(map[equalityPair]equalityCacheValue)
	cache.level = 0
}
