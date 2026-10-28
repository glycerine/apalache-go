package apalache

import "fmt"

// LazyEquality generates structural equality constraints on demand and caches
// the resulting cell equality. It is solver-independent: generated ground
// constraints are delivered through assert and can later be encoded by g3.
type LazyEquality struct {
	cache     *EqualityCache
	assert    func(Expr)
	stringTag func(string) (ArenaCell, bool)
	computing map[equalityPair]bool
}

type ArenaCellPair struct {
	Left  ArenaCell
	Right ArenaCell
}

const unsafeEqualityMessage = "VORSICHT! SMT equality should be used only after calling cacheEqualities, unless you know what you are doing."

func NewLazyEquality(assert func(Expr), stringTags ...func(string) (ArenaCell, bool)) *LazyEquality {
	if assert == nil {
		assert = func(Expr) {}
	}
	result := &LazyEquality{
		cache: NewEqualityCache(), assert: assert, computing: make(map[equalityPair]bool),
	}
	if len(stringTags) > 0 {
		result.stringTag = stringTags[0]
	}
	return result
}

func (equality *LazyEquality) Cache() *EqualityCache { return equality.cache }

func (equality *LazyEquality) Level() int { return equality.cache.Level() }
func (equality *LazyEquality) Push()      { equality.cache.Push() }
func (equality *LazyEquality) Pop(count ...int) error {
	return equality.cache.Pop(count...)
}
func (equality *LazyEquality) Snapshot() EqualityCacheSnapshot { return equality.cache.Snapshot() }
func (equality *LazyEquality) Recover(snapshot EqualityCacheSnapshot) {
	equality.cache.Recover(snapshot)
	clear(equality.computing)
}
func (equality *LazyEquality) Dispose() {
	equality.cache.Dispose()
	clear(equality.computing)
}

// SafeEqual returns a cached equality and rejects pairs whose defining
// constraints have not been generated. CachedEqual is the expression-level
// counterpart; Go's native IR does not need Scala's BuilderT distinction.
func (equality *LazyEquality) SafeEqual(left, right ArenaCell) (Expr, error) {
	return equality.CachedEqual(left, right)
}

func (equality *LazyEquality) CachedEqual(left, right ArenaCell) (Expr, error) {
	if left.Equal(right) {
		return boolExpr(true), nil
	}
	entry, found := equality.cache.Get(left, right)
	if !found {
		return nil, fmt.Errorf("%s", unsafeEqualityMessage)
	}
	return equality.cache.ToExpr(left, right, entry)
}

func (equality *LazyEquality) CacheAsSMTEqualityByMagic(left, right ArenaCell) error {
	return equality.cache.Put(left, right, EqualityDirect{})
}

func (equality *LazyEquality) CacheEqualities(arena *PureArena, pairs []ArenaCellPair) error {
	for _, pair := range pairs {
		if _, err := equality.Equal(arena, pair.Left, pair.Right); err != nil {
			return err
		}
	}
	return nil
}

func (equality *LazyEquality) CacheOneEquality(arena *PureArena, left, right ArenaCell) error {
	_, err := equality.Equal(arena, left, right)
	return err
}

// CountConstantEqualities applies the caller's solver-backed constant test to
// every non-static cache entry and returns (constant, non-static).
func (equality *LazyEquality) CountConstantEqualities(
	isConstant func(Expr) (bool, error),
) (int, int, error) {
	if isConstant == nil {
		return 0, 0, fmt.Errorf("constant-equality predicate is required")
	}
	constant, nonStatic := 0, 0
	for _, value := range equality.cache.entries {
		var expression Expr
		switch entry := value.entry.(type) {
		case EqualityFalse, EqualityTrue:
			continue
		case EqualityDirect:
			expression = directEquality(value.left, value.right)
		case EqualityExpression:
			expression = entry.Expression
		default:
			return 0, 0, fmt.Errorf("unknown equality cache entry %T", value.entry)
		}
		nonStatic++
		fixed, err := isConstant(expression)
		if err != nil {
			return 0, 0, err
		}
		if fixed {
			constant++
		}
	}
	return constant, nonStatic, nil
}

// Equal returns a ground Boolean expression for equality between two cells.
// Structural constraints are emitted only on the first comparison of a pair.
func (equality *LazyEquality) Equal(arena *PureArena, left, right ArenaCell) (Expr, error) {
	if arena == nil {
		return nil, fmt.Errorf("lazy equality requires an arena")
	}
	if left.Equal(right) {
		return boolExpr(true), nil
	}
	if entry, found := equality.cache.Get(left, right); found {
		return equality.cache.ToExpr(left, right, entry)
	}
	if !EqualCellTypes(left.Type, right.Type) {
		return nil, fmt.Errorf("cannot compare cells of types %s and %s", left.Type, right.Type)
	}

	pair := canonicalEqualityPair(left, right)
	if equality.computing[pair] {
		return nil, fmt.Errorf("cyclic structural equality between %s and %s", left, right)
	}
	_, unknown := left.Type.(UnknownCellType)
	if unknown || isDirectSMTCellType(left.Type) {
		if err := equality.cache.Put(left, right, EqualityDirect{}); err != nil {
			return nil, err
		}
		return directEquality(left, right), nil
	}
	equality.computing[pair] = true
	var condition Expr
	var err error
	switch {
	case isFiniteSetCellType(left.Type):
		condition, err = equality.setEqualityCondition(arena, left, right)
	case isTupleCellType(left.Type):
		condition, err = equality.tupleEqualityCondition(arena, left, right)
	case isRecordCellType(left.Type):
		condition, err = equality.recordEqualityCondition(arena, left, right)
	case isSequenceCellType(left.Type):
		condition, err = equality.sequenceEqualityCondition(arena, left, right)
	case isFunctionCellType(left.Type):
		condition, err = equality.functionEqualityCondition(arena, left, right)
	case isVariantCellType(left.Type):
		condition, err = equality.variantEqualityCondition(arena, left, right)
	case isFiniteFunctionSetCellType(left.Type):
		condition, err = equality.functionSetEqualityCondition(arena, left, right)
	default:
		err = fmt.Errorf("structural equality is not yet implemented for %s", left.Type)
	}
	delete(equality.computing, pair)
	if err != nil {
		return nil, err
	}
	rawEquality := directEquality(left, right)
	equality.assert(MustOperExpr(Op("EQUIV"), Typed(BoolT1), rawEquality, condition))
	if err := equality.cache.Put(left, right, EqualityDirect{}); err != nil {
		return nil, err
	}
	return rawEquality, nil
}

func isFiniteFunctionSetCellType(cellType CellType) bool {
	_, ok := cellType.(FiniteFunctionSetCellType)
	return ok
}

func (equality *LazyEquality) functionSetEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	leftDomain, err := arena.Domain(left)
	if err != nil {
		return nil, err
	}
	rightDomain, err := arena.Domain(right)
	if err != nil {
		return nil, err
	}
	leftCodomain, err := arena.Codomain(left)
	if err != nil {
		return nil, err
	}
	rightCodomain, err := arena.Codomain(right)
	if err != nil {
		return nil, err
	}
	domainsEqual, err := equality.Equal(arena, leftDomain, rightDomain)
	if err != nil {
		return nil, err
	}
	codomainsEqual, err := equality.Equal(arena, leftCodomain, rightCodomain)
	if err != nil {
		return nil, err
	}
	return booleanAnd(domainsEqual, codomainsEqual), nil
}

func (equality *LazyEquality) SubsetEqual(arena *PureArena, left, right ArenaCell) (Expr, error) {
	if arena == nil {
		return nil, fmt.Errorf("lazy equality requires an arena")
	}
	if !isFiniteSetCellType(left.Type) || !isFiniteSetCellType(right.Type) {
		return nil, fmt.Errorf("subset equality requires finite sets, found %s and %s", left.Type, right.Type)
	}
	return equality.subsetCondition(arena, left, right)
}

func (equality *LazyEquality) variantEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	options, ok := variantTypeOptions(left.Type)
	if !ok {
		return nil, fmt.Errorf("variant equality requires a closed variant type, found %s", left.Type)
	}
	leftTag, err := variantTag(arena, left)
	if err != nil {
		return nil, err
	}
	rightTag, err := variantTag(arena, right)
	if err != nil {
		return nil, err
	}
	tagsEqual, err := equality.Equal(arena, leftTag, rightTag)
	if err != nil {
		return nil, err
	}
	conditions := []Expr{tagsEqual}
	for _, name := range sortedStringKeys(options) {
		leftValue, fieldErr := variantField(arena, left, name)
		if fieldErr != nil {
			return nil, fieldErr
		}
		rightValue, fieldErr := variantField(arena, right, name)
		if fieldErr != nil {
			return nil, fieldErr
		}
		valuesEqual, equalErr := equality.Equal(arena, leftValue, rightValue)
		if equalErr != nil {
			return nil, equalErr
		}
		if equality.stringTag == nil {
			return nil, fmt.Errorf("variant equality has no string-tag resolver")
		}
		optionTag, found := equality.stringTag(name)
		if !found {
			return nil, fmt.Errorf("variant option tag %q has not been materialized", name)
		}
		active, activeErr := equality.Equal(arena, optionTag, leftTag)
		if activeErr != nil {
			return nil, activeErr
		}
		conditions = append(conditions, booleanOr(
			MustOperExpr(Op("NOT"), Typed(BoolT1), active), valuesEqual))
	}
	return booleanAnd(conditions...), nil
}

func (equality *LazyEquality) functionEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	leftRelation, err := arena.Codomain(left)
	if err != nil {
		return nil, err
	}
	rightRelation, err := arena.Codomain(right)
	if err != nil {
		return nil, err
	}
	return equality.Equal(arena, leftRelation, rightRelation)
}

func (equality *LazyEquality) sequenceEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	leftLength, _, leftElements, err := unpackSequence(arena, left)
	if err != nil {
		return nil, err
	}
	rightLength, _, rightElements, err := unpackSequence(arena, right)
	if err != nil {
		return nil, err
	}
	lengthsEqual, err := equality.Equal(arena, leftLength, rightLength)
	if err != nil {
		return nil, err
	}
	shared := len(leftElements)
	if len(rightElements) < shared {
		shared = len(rightElements)
	}
	conditions := make([]Expr, 1, shared+1)
	conditions[0] = lengthsEqual
	for index := 0; index < shared; index++ {
		elementsEqual, equalityErr := equality.Equal(arena, leftElements[index].Element(), rightElements[index].Element())
		if equalityErr != nil {
			return nil, equalityErr
		}
		indexValue := NewValueExpr(Int64Value(int64(index+1)), Typed(IntT1))
		conditions = append(conditions, booleanOr(
			MustOperExpr(Op("LT"), Typed(BoolT1), leftLength.NameExpr(), indexValue),
			MustOperExpr(Op("LT"), Typed(BoolT1), rightLength.NameExpr(), indexValue),
			elementsEqual,
		))
	}
	return booleanAnd(conditions...), nil
}

func isRecordCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	switch basic.Type.(type) {
	case RecordType, RecordRowType:
		return true
	default:
		return false
	}
}

func recordTypeFields(cellType CellType) (map[string]Type, bool) {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return nil, false
	}
	switch typed := basic.Type.(type) {
	case RecordType:
		return typed.Fields(), true
	case RecordRowType:
		if _, open := typed.Row().Other(); open {
			return nil, false
		}
		return typed.Row().Fields(), true
	default:
		return nil, false
	}
}

func (equality *LazyEquality) recordEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	fields, ok := recordTypeFields(left.Type)
	if !ok {
		return nil, fmt.Errorf("record equality requires a closed record type, found %s", left.Type)
	}
	leftValues := arena.GetHas(left)
	rightValues := arena.GetHas(right)
	if len(leftValues) != len(fields) || len(rightValues) != len(fields) {
		return nil, fmt.Errorf("record cells %s and %s do not contain all %d fields", left, right, len(fields))
	}
	conditions := make([]Expr, len(leftValues))
	for index := range leftValues {
		condition, err := equality.Equal(arena, leftValues[index].Element(), rightValues[index].Element())
		if err != nil {
			return nil, fmt.Errorf("compare record field %d of %s and %s: %w", index+1, left, right, err)
		}
		conditions[index] = condition
	}
	if _, legacy := left.Type.TLAType().(RecordType); legacy {
		leftDomain, err := arena.Domain(left)
		if err != nil {
			return nil, err
		}
		rightDomain, err := arena.Domain(right)
		if err != nil {
			return nil, err
		}
		domainsEqual, err := equality.Equal(arena, leftDomain, rightDomain)
		if err != nil {
			return nil, fmt.Errorf("compare record domains: %w", err)
		}
		conditions = append([]Expr{domainsEqual}, conditions...)
	}
	return booleanAnd(conditions...), nil
}

func isTupleCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	_, ok = basic.Type.(TupleType)
	return ok
}

func (equality *LazyEquality) tupleEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	leftElements := arena.GetHas(left)
	rightElements := arena.GetHas(right)
	if len(leftElements) != len(rightElements) {
		return nil, fmt.Errorf("tuple cells %s and %s have different arities %d and %d", left, right, len(leftElements), len(rightElements))
	}
	conditions := make([]Expr, len(leftElements))
	for index := range leftElements {
		condition, err := equality.Equal(arena, leftElements[index].Element(), rightElements[index].Element())
		if err != nil {
			return nil, fmt.Errorf("compare tuple field %d of %s and %s: %w", index+1, left, right, err)
		}
		conditions[index] = condition
	}
	return booleanAnd(conditions...), nil
}

func isFiniteSetCellType(cellType CellType) bool {
	basic, ok := cellType.(BasicCellType)
	if !ok {
		return false
	}
	_, ok = basic.Type.(SetType)
	return ok
}

func (equality *LazyEquality) setEqualityCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	leftToRight, err := equality.subsetCondition(arena, left, right)
	if err != nil {
		return nil, err
	}
	rightToLeft, err := equality.subsetCondition(arena, right, left)
	if err != nil {
		return nil, err
	}
	return booleanAnd(leftToRight, rightToLeft), nil
}

func (equality *LazyEquality) subsetCondition(arena *PureArena, left, right ArenaCell) (Expr, error) {
	leftElements := arena.GetHas(left)
	rightElements := arena.GetHas(right)
	conditions := make([]Expr, len(leftElements))
	for leftIndex, leftPointer := range leftElements {
		matches := make([]Expr, len(rightElements))
		for rightIndex, rightPointer := range rightElements {
			equalElements, err := equality.Equal(arena, leftPointer.Element(), rightPointer.Element())
			if err != nil {
				return nil, fmt.Errorf("compare elements of %s and %s: %w", left, right, err)
			}
			matches[rightIndex] = booleanAnd(rightPointer.ToSMT(), equalElements)
		}
		conditions[leftIndex] = booleanOr(
			MustOperExpr(Op("NOT"), Typed(BoolT1), leftPointer.ToSMT()),
			booleanOr(matches...),
		)
	}
	return booleanAnd(conditions...), nil
}

func directEquality(left, right ArenaCell) Expr {
	return MustOperExpr(Op("EQ"), Typed(BoolT1), left.NameExpr(), right.NameExpr())
}

func boolExpr(value bool) Expr {
	return NewValueExpr(BoolValue(value), Typed(BoolT1))
}

func booleanAnd(arguments ...Expr) Expr {
	switch len(arguments) {
	case 0:
		return boolExpr(true)
	case 1:
		return arguments[0]
	default:
		return MustOperExpr(Op("AND"), Typed(BoolT1), arguments...)
	}
}

func booleanOr(arguments ...Expr) Expr {
	switch len(arguments) {
	case 0:
		return boolExpr(false)
	case 1:
		return arguments[0]
	default:
		return MustOperExpr(Op("OR"), Typed(BoolT1), arguments...)
	}
}
