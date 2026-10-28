package apalache

import (
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
)

type arenaCacheEntry[S, T any] struct {
	source S
	target T
	level  int
}

// ArenaCacheSnapshot is an immutable, level-squashed cache image. Its maps
// remain private so recovery cannot observe caller mutation.
type ArenaCacheSnapshot[S, T any] struct {
	entries map[string]arenaCacheEntry[S, T]
	reverse map[string]arenaCacheEntry[S, T]
}

// arenaCache implements the stack discipline shared by the symbolic caches.
// Keys are explicit strings because several Scala cache keys (BigInt, Type,
// sorted sets) are not comparable Go values.
type arenaCache[S, T any] struct {
	level     int
	entries   map[string]arenaCacheEntry[S, T]
	reverse   map[string]arenaCacheEntry[S, T]
	sourceKey func(S) string
	targetKey func(T) string
	create    func(*PureArena, S) (*PureArena, T, error)
}

func newArenaCache[S, T any](sourceKey func(S) string, targetKey func(T) string, create func(*PureArena, S) (*PureArena, T, error)) *arenaCache[S, T] {
	return &arenaCache[S, T]{
		entries: make(map[string]arenaCacheEntry[S, T]), reverse: make(map[string]arenaCacheEntry[S, T]),
		sourceKey: sourceKey, targetKey: targetKey, create: create,
	}
}

func (cache *arenaCache[S, T]) getOrCreate(arena *PureArena, source S) (*PureArena, T, error) {
	key := cache.sourceKey(source)
	if entry, found := cache.entries[key]; found {
		return arena, entry.target, nil
	}
	newArena, target, err := cache.create(arena, source)
	if err != nil {
		var zero T
		return nil, zero, err
	}
	entry := arenaCacheEntry[S, T]{source: source, target: target, level: cache.level}
	cache.entries[key] = entry
	cache.reverse[cache.targetKey(target)] = entry
	return newArena, target, nil
}

func (cache *arenaCache[S, T]) get(source S) (T, bool) {
	entry, found := cache.entries[cache.sourceKey(source)]
	return entry.target, found
}

func (cache *arenaCache[S, T]) findSource(target T) (S, bool) {
	entry, found := cache.reverse[cache.targetKey(target)]
	return entry.source, found
}

func (cache *arenaCache[S, T]) sortedEntries(currentLevelOnly bool) []arenaCacheEntry[S, T] {
	keys := make([]string, 0, len(cache.entries))
	for key, entry := range cache.entries {
		if !currentLevelOnly || entry.level == cache.level {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	entries := make([]arenaCacheEntry[S, T], len(keys))
	for index, key := range keys {
		entries[index] = cache.entries[key]
	}
	return entries
}

func (cache *arenaCache[S, T]) push() { cache.level++ }

func (cache *arenaCache[S, T]) pop(count int) error {
	if count < 0 || count > cache.level {
		return fmt.Errorf("cannot pop %d levels from a cache at level %d", count, cache.level)
	}
	cache.level -= count
	for key, entry := range cache.entries {
		if entry.level > cache.level {
			delete(cache.entries, key)
			delete(cache.reverse, cache.targetKey(entry.target))
		}
	}
	return nil
}

func (cache *arenaCache[S, T]) dispose() {
	cache.entries = make(map[string]arenaCacheEntry[S, T])
	cache.reverse = make(map[string]arenaCacheEntry[S, T])
	cache.level = 0
}

func (cache *arenaCache[S, T]) snapshot() ArenaCacheSnapshot[S, T] {
	entries := make(map[string]arenaCacheEntry[S, T], len(cache.entries))
	for key, entry := range cache.entries {
		entry.level = 0
		entries[key] = entry
	}
	reverse := make(map[string]arenaCacheEntry[S, T], len(cache.reverse))
	for key, entry := range cache.reverse {
		entry.level = 0
		reverse[key] = entry
	}
	return ArenaCacheSnapshot[S, T]{entries: entries, reverse: reverse}
}

func (cache *arenaCache[S, T]) recover(snapshot ArenaCacheSnapshot[S, T]) {
	cache.entries = make(map[string]arenaCacheEntry[S, T], len(snapshot.entries))
	for key, entry := range snapshot.entries {
		cache.entries[key] = entry
	}
	cache.reverse = make(map[string]arenaCacheEntry[S, T], len(snapshot.reverse))
	for key, entry := range snapshot.reverse {
		cache.reverse[key] = entry
	}
	// Scala recovery replaces both maps but deliberately retains the receiver's
	// current stack level.
}

func cellTargetKey(cell ArenaCell) string { return strconv.Itoa(cell.ID) }

// IntValueCache maps arbitrary-precision integer literals to arena cells.
type IntValueCache struct {
	cache *arenaCache[*big.Int, ArenaCell]
}

type IntValueCacheSnapshot = ArenaCacheSnapshot[*big.Int, ArenaCell]

func NewIntValueCache() *IntValueCache {
	cache := &IntValueCache{}
	cache.cache = newArenaCache(
		func(value *big.Int) string { return value.String() },
		cellTargetKey,
		func(arena *PureArena, _ *big.Int) (*PureArena, ArenaCell, error) {
			newArena, cell := arena.AppendCellType(BasicCellType{Type: IntT1})
			return newArena, cell, nil
		},
	)
	return cache
}

func (cache *IntValueCache) GetOrCreate(arena *PureArena, value *big.Int) (*PureArena, ArenaCell, error) {
	if value == nil {
		return nil, ArenaCell{}, fmt.Errorf("integer cache key cannot be nil")
	}
	return cache.cache.getOrCreate(arena, new(big.Int).Set(value))
}

func (cache *IntValueCache) Get(value *big.Int) (ArenaCell, bool) {
	if value == nil {
		return ArenaCell{}, false
	}
	return cache.cache.get(value)
}

func (cache *IntValueCache) FindValue(cell ArenaCell) (*big.Int, bool) {
	value, found := cache.cache.findSource(cell)
	if !found {
		return nil, false
	}
	return new(big.Int).Set(value), true
}

func (cache *IntValueCache) Level() int { return cache.cache.level }
func (cache *IntValueCache) Push()      { cache.cache.push() }
func (cache *IntValueCache) Pop(count ...int) error {
	n := 1
	if len(count) != 0 {
		n = count[0]
	}
	return cache.cache.pop(n)
}
func (cache *IntValueCache) Dispose()                               { cache.cache.dispose() }
func (cache *IntValueCache) Snapshot() IntValueCacheSnapshot        { return cache.cache.snapshot() }
func (cache *IntValueCache) Recover(snapshot IntValueCacheSnapshot) { cache.cache.recover(snapshot) }

// Constraints returns equalities for entries introduced at the current stack
// level, matching Cache.addAllConstraints in the Scala implementation.
func (cache *IntValueCache) Constraints() []Expr {
	entries := cache.cache.sortedEntries(true)
	result := make([]Expr, len(entries))
	for index, entry := range entries {
		result[index] = MustOperExpr(Op("EQ"), Typed(BoolT1), entry.target.NameExpr(), NewValueExpr(NewIntValue(entry.source), Typed(IntT1)))
	}
	return result
}

// UninterpretedLiteral identifies a string or model value by both type and
// index. Equal indices of different uninterpreted types remain distinct keys.
type UninterpretedLiteral struct {
	Type  Type
	Index string
}

type UninterpretedLiteralCache struct {
	cache *arenaCache[UninterpretedLiteral, ArenaCell]
}

type UninterpretedLiteralCacheSnapshot = ArenaCacheSnapshot[UninterpretedLiteral, ArenaCell]

func NewUninterpretedLiteralCache() *UninterpretedLiteralCache {
	cache := &UninterpretedLiteralCache{}
	cache.cache = newArenaCache(
		func(literal UninterpretedLiteral) string { return literal.Type.String() + "\x00" + literal.Index },
		cellTargetKey,
		func(arena *PureArena, literal UninterpretedLiteral) (*PureArena, ArenaCell, error) {
			if err := validateUninterpretedLiteral(literal); err != nil {
				return nil, ArenaCell{}, err
			}
			cellType, err := CellTypeFromType(literal.Type)
			if err != nil {
				return nil, ArenaCell{}, err
			}
			newArena, cell := arena.AppendCellType(cellType)
			return newArena, cell, nil
		},
	)
	return cache
}

func validateUninterpretedLiteral(literal UninterpretedLiteral) error {
	if literal.Type == nil {
		return fmt.Errorf("uninterpreted literal type cannot be nil")
	}
	if EqualTypes(literal.Type, StrT1) {
		return nil
	}
	if _, ok := literal.Type.(ConstType); !ok {
		return fmt.Errorf("uninterpreted literal type must be Str or a constant type, found %s", literal.Type)
	}
	return nil
}

func (cache *UninterpretedLiteralCache) GetOrCreate(arena *PureArena, literal UninterpretedLiteral) (*PureArena, ArenaCell, error) {
	if err := validateUninterpretedLiteral(literal); err != nil {
		return nil, ArenaCell{}, err
	}
	return cache.cache.getOrCreate(arena, literal)
}

func (cache *UninterpretedLiteralCache) Get(literal UninterpretedLiteral) (ArenaCell, bool) {
	if validateUninterpretedLiteral(literal) != nil {
		return ArenaCell{}, false
	}
	return cache.cache.get(literal)
}

func (cache *UninterpretedLiteralCache) FindLiteral(cell ArenaCell) (UninterpretedLiteral, bool) {
	return cache.cache.findSource(cell)
}

func (cache *UninterpretedLiteralCache) Values() []ArenaCell {
	entries := cache.cache.sortedEntries(false)
	values := make([]ArenaCell, len(entries))
	for index, entry := range entries {
		values[index] = entry.target
	}
	return values
}

func (cache *UninterpretedLiteralCache) Level() int { return cache.cache.level }
func (cache *UninterpretedLiteralCache) Push()      { cache.cache.push() }
func (cache *UninterpretedLiteralCache) Pop(count ...int) error {
	n := 1
	if len(count) != 0 {
		n = count[0]
	}
	return cache.cache.pop(n)
}
func (cache *UninterpretedLiteralCache) Dispose() { cache.cache.dispose() }
func (cache *UninterpretedLiteralCache) Snapshot() UninterpretedLiteralCacheSnapshot {
	return cache.cache.snapshot()
}
func (cache *UninterpretedLiteralCache) Recover(snapshot UninterpretedLiteralCacheSnapshot) {
	cache.cache.recover(snapshot)
}

// ConstraintFor returns the distinctness constraint generated when one
// literal is discharged individually.
func (cache *UninterpretedLiteralCache) ConstraintFor(literal UninterpretedLiteral, cell ArenaCell) (Expr, error) {
	if err := validateUninterpretedLiteral(literal); err != nil {
		return nil, err
	}
	args := []Expr{cell.NameExpr()}
	for _, entry := range cache.cache.sortedEntries(false) {
		if entry.target.Equal(cell) || !EqualTypes(entry.source.Type, literal.Type) {
			continue
		}
		args = append(args, entry.target.NameExpr())
	}
	return MustOperExpr(Op("Apalache!Distinct"), Typed(BoolT1), args...), nil
}

// Constraints emits one distinctness assertion per uninterpreted type. Like
// Scala's optimized override it considers all live levels, not only the top.
func (cache *UninterpretedLiteralCache) Constraints() []Expr {
	groups := make(map[string][]ArenaCell)
	for _, entry := range cache.cache.sortedEntries(false) {
		key := entry.source.Type.String()
		groups[key] = append(groups[key], entry.target)
	}
	types := make([]string, 0, len(groups))
	for typ := range groups {
		types = append(types, typ)
	}
	sort.Strings(types)
	result := make([]Expr, len(types))
	for index, typ := range types {
		cells := groups[typ]
		sort.Slice(cells, func(left, right int) bool { return cells[left].ID > cells[right].ID })
		args := make([]Expr, len(cells))
		for cellIndex, cell := range cells {
			args[cellIndex] = cell.NameExpr()
		}
		result[index] = MustOperExpr(Op("Apalache!Distinct"), Typed(BoolT1), args...)
	}
	return result
}

// ArenaSetValue is the cached representation of a finite set and its ordered
// element cells.
type ArenaSetValue struct {
	Set      ArenaCell
	Elements []ArenaCell
}

func cloneArenaSetValue(value ArenaSetValue) ArenaSetValue {
	value.Elements = append([]ArenaCell(nil), value.Elements...)
	return value
}

type IntRangeCache struct {
	cache     *arenaCache[[2]int, ArenaSetValue]
	intValues *IntValueCache
}

type IntRangeCacheSnapshot = ArenaCacheSnapshot[[2]int, ArenaSetValue]

func NewIntRangeCache(intValues *IntValueCache) *IntRangeCache {
	if intValues == nil {
		panic("integer range cache requires an integer value cache")
	}
	result := &IntRangeCache{intValues: intValues}
	result.cache = newArenaCache(
		func(bounds [2]int) string { return fmt.Sprintf("%d:%d", bounds[0], bounds[1]) },
		func(value ArenaSetValue) string { return cellTargetKey(value.Set) },
		func(arena *PureArena, bounds [2]int) (*PureArena, ArenaSetValue, error) {
			from, to := bounds[0], bounds[1]
			if from > to {
				from, to = 1, 0
			}
			current := arena
			elements := make([]ArenaCell, 0)
			for value := from; value <= to; value++ {
				var cell ArenaCell
				var err error
				current, cell, err = intValues.GetOrCreate(current, big.NewInt(int64(value)))
				if err != nil {
					return nil, ArenaSetValue{}, err
				}
				elements = append(elements, cell)
				if value == int(^uint(0)>>1) {
					break
				}
			}
			current, set := current.AppendCellType(BasicCellType{Type: mustSet(IntT1)})
			pointers := make([]ElementPointer, len(elements))
			for index, element := range elements {
				pointers[index] = FixedElementPointer{Cell: element}
			}
			return current.AppendHas(set, pointers...), ArenaSetValue{Set: set, Elements: elements}, nil
		},
	)
	return result
}

func (cache *IntRangeCache) GetOrCreate(arena *PureArena, from, to int) (*PureArena, ArenaSetValue, error) {
	newArena, value, err := cache.cache.getOrCreate(arena, [2]int{from, to})
	return newArena, cloneArenaSetValue(value), err
}

func (cache *IntRangeCache) Get(from, to int) (ArenaSetValue, bool) {
	value, found := cache.cache.get([2]int{from, to})
	return cloneArenaSetValue(value), found
}

func (cache *IntRangeCache) Constraints() []Expr {
	return membershipConstraints(cache.cache.sortedEntries(true))
}
func (cache *IntRangeCache) Level() int { return cache.cache.level }
func (cache *IntRangeCache) Push()      { cache.cache.push() }
func (cache *IntRangeCache) Pop(count ...int) error {
	n := 1
	if len(count) != 0 {
		n = count[0]
	}
	return cache.cache.pop(n)
}
func (cache *IntRangeCache) Dispose()                               { cache.cache.dispose() }
func (cache *IntRangeCache) Snapshot() IntRangeCacheSnapshot        { return cache.cache.snapshot() }
func (cache *IntRangeCache) Recover(snapshot IntRangeCacheSnapshot) { cache.cache.recover(snapshot) }

type RecordDomainCache struct {
	cache   *arenaCache[[]string, ArenaSetValue]
	strings *UninterpretedLiteralCache
}

type RecordDomainCacheSnapshot = ArenaCacheSnapshot[[]string, ArenaSetValue]

func NewRecordDomainCache(stringsCache *UninterpretedLiteralCache) *RecordDomainCache {
	if stringsCache == nil {
		panic("record domain cache requires an uninterpreted literal cache")
	}
	result := &RecordDomainCache{strings: stringsCache}
	result.cache = newArenaCache(
		func(keys []string) string { return strings.Join(keys, "\x00") },
		func(value ArenaSetValue) string { return cellTargetKey(value.Set) },
		func(arena *PureArena, keys []string) (*PureArena, ArenaSetValue, error) {
			current := arena
			elements := make([]ArenaCell, len(keys))
			for index, key := range keys {
				var err error
				current, elements[index], err = stringsCache.GetOrCreate(current, UninterpretedLiteral{Type: StrT1, Index: key})
				if err != nil {
					return nil, ArenaSetValue{}, err
				}
			}
			current, set := current.AppendCellType(BasicCellType{Type: mustSet(StrT1)})
			pointers := make([]ElementPointer, len(elements))
			for index, element := range elements {
				pointers[index] = FixedElementPointer{Cell: element}
			}
			return current.AppendHas(set, pointers...), ArenaSetValue{Set: set, Elements: elements}, nil
		},
	)
	return result
}

func normalizeRecordKeys(keys []string) []string {
	result := append([]string(nil), keys...)
	sort.Strings(result)
	result = compactStrings(result)
	return result
}

func compactStrings(values []string) []string {
	if len(values) < 2 {
		return values
	}
	write := 1
	for _, value := range values[1:] {
		if value != values[write-1] {
			values[write] = value
			write++
		}
	}
	return values[:write]
}

func (cache *RecordDomainCache) GetOrCreate(arena *PureArena, keys []string) (*PureArena, ArenaSetValue, error) {
	newArena, value, err := cache.cache.getOrCreate(arena, normalizeRecordKeys(keys))
	return newArena, cloneArenaSetValue(value), err
}

func (cache *RecordDomainCache) Get(keys []string) (ArenaSetValue, bool) {
	value, found := cache.cache.get(normalizeRecordKeys(keys))
	return cloneArenaSetValue(value), found
}

func (cache *RecordDomainCache) Constraints() []Expr {
	return membershipConstraints(cache.cache.sortedEntries(true))
}
func (cache *RecordDomainCache) Level() int { return cache.cache.level }
func (cache *RecordDomainCache) Push()      { cache.cache.push() }
func (cache *RecordDomainCache) Pop(count ...int) error {
	n := 1
	if len(count) != 0 {
		n = count[0]
	}
	return cache.cache.pop(n)
}
func (cache *RecordDomainCache) Dispose()                            { cache.cache.dispose() }
func (cache *RecordDomainCache) Snapshot() RecordDomainCacheSnapshot { return cache.cache.snapshot() }
func (cache *RecordDomainCache) Recover(snapshot RecordDomainCacheSnapshot) {
	cache.cache.recover(snapshot)
}

func membershipConstraints[S any](entries []arenaCacheEntry[S, ArenaSetValue]) []Expr {
	result := make([]Expr, 0)
	for _, entry := range entries {
		for _, element := range entry.target.Elements {
			result = append(result, MustOperExpr(Op("SET_IN"), Typed(BoolT1), element.NameExpr(), entry.target.Set.NameExpr()))
		}
	}
	return result
}
