package apalache

import (
	"fmt"
	"sort"
	"strconv"
)

// ChoiceOracle describes a finite symbolic choice. Model decoding is kept out
// of this solver-independent interface until g3 model evaluation is wired in.
type ChoiceOracle interface {
	Size() int
	Chosen(scope RewriterScope, index int) (Expr, error)
	Cases(scope RewriterScope, assertions []Expr, elseAssertions []Expr) (Expr, error)
}

// OracleFactory centralizes construction of the supported choice encodings.
// The default follows Scala and uses uninterpreted constants.
type OracleFactory struct{ literals *UninterpretedLiteralCache }

func NewOracleFactory(literals *UninterpretedLiteralCache) *OracleFactory {
	if literals == nil {
		literals = NewUninterpretedLiteralCache()
	}
	return &OracleFactory{literals: literals}
}

func (factory *OracleFactory) NewDefault(scope RewriterScope, nvalues int) (RewriterScope, ChoiceOracle, []Expr, error) {
	next, oracle, constraints, err := factory.NewUninterpreted(scope, nvalues)
	return next, oracle, constraints, err
}

func (factory *OracleFactory) NewUninterpreted(scope RewriterScope, nvalues int) (RewriterScope, *UninterpretedConstOracle, []Expr, error) {
	if factory == nil {
		return scope, nil, nil, fmt.Errorf("oracle factory cannot be nil")
	}
	return CreateUninterpretedConstOracle(factory.literals, scope, nvalues)
}

func (factory *OracleFactory) NewPropositional(scope RewriterScope, nvalues int) (RewriterScope, *PropositionalOracle, []Expr, error) {
	if factory == nil {
		return scope, nil, nil, fmt.Errorf("oracle factory cannot be nil")
	}
	return CreatePropositionalOracle(scope, nvalues)
}

func (factory *OracleFactory) NewInteger(scope RewriterScope, nvalues int) (RewriterScope, *IntOracle, []Expr, error) {
	if factory == nil {
		return scope, nil, nil, fmt.Errorf("oracle factory cannot be nil")
	}
	next, oracle, err := CreateIntOracle(scope, nvalues)
	return next, oracle, nil, err
}

func oracleCases(oracle ChoiceOracle, scope RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	if len(assertions) != oracle.Size() || (elseAssertions != nil && len(elseAssertions) != oracle.Size()) {
		return nil, fmt.Errorf("oracle assertion sequences must have length %d", oracle.Size())
	}
	switch oracle.Size() {
	case 0:
		return scope.Arena.TrueCell().NameExpr(), nil
	case 1:
		return assertions[0], nil
	}
	conjuncts := make([]Expr, oracle.Size())
	for index := range assertions {
		condition, err := oracle.Chosen(scope, index)
		if err != nil {
			return nil, err
		}
		otherwise := Expr(NewValueExpr(BoolValue(true), Typed(BoolT1)))
		if elseAssertions != nil {
			otherwise = elseAssertions[index]
		}
		conjuncts[index] = MustOperExpr(Op("IF_THEN_ELSE"), Typed(BoolT1), condition, assertions[index], otherwise)
	}
	return MustOperExpr(Op("AND"), Typed(BoolT1), conjuncts...), nil
}

// oraclePicksSetMembers ports OracleHelper.assertOraclePicksSetMembers. The
// final oracle case is the default and is enabled exactly when every candidate
// pointer is absent from the set.
func oraclePicksSetMembers(oracle ChoiceOracle, scope RewriterScope, pointers []ElementPointer) (Expr, error) {
	if oracle == nil || oracle.Size() != len(pointers)+1 {
		return nil, fmt.Errorf("set-member oracle requires %d cases", len(pointers)+1)
	}
	assertions := make([]Expr, oracle.Size())
	allAbsent := make([]Expr, len(pointers))
	for index, pointer := range pointers {
		assertions[index] = pointer.ToSMT()
		allAbsent[index] = MustOperExpr(Op("NOT"), Typed(BoolT1), pointer.ToSMT())
	}
	assertions[len(pointers)] = booleanAnd(allAbsent...)
	return oracle.Cases(scope, assertions, nil)
}

type IntOracle struct {
	Cell    ArenaCell
	NValues int
}

func NewIntOracle(cell ArenaCell, nvalues int) (*IntOracle, error) {
	if nvalues < 0 {
		return nil, fmt.Errorf("integer oracle must have a non-negative size")
	}
	if !EqualTypes(cell.Type.TLAType(), IntT1) {
		return nil, fmt.Errorf("integer oracle requires an integer cell")
	}
	return &IntOracle{Cell: cell, NValues: nvalues}, nil
}

func CreateIntOracle(scope RewriterScope, nvalues int) (RewriterScope, *IntOracle, error) {
	if nvalues < 0 {
		return scope, nil, fmt.Errorf("integer oracle must have a non-negative size")
	}
	newArena, cell := scope.Arena.AppendCellType(BasicCellType{Type: IntT1})
	oracle, err := NewIntOracle(cell, nvalues)
	scope.Arena = newArena
	return scope, oracle, err
}

func (oracle *IntOracle) Size() int { return oracle.NValues }
func (oracle *IntOracle) Chosen(_ RewriterScope, index int) (Expr, error) {
	return MustOperExpr(Op("EQ"), Typed(BoolT1), oracle.Cell.NameExpr(), NewValueExpr(Int64Value(int64(index)), Typed(IntT1))), nil
}
func (oracle *IntOracle) Cases(scope RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	return oracleCases(oracle, scope, assertions, elseAssertions)
}

func (oracle *IntOracle) Decode(encoder *G3GroundEncoder) (int, error) {
	value, err := encoder.EvaluateInt(oracle.Cell.NameExpr())
	if err != nil {
		return 0, err
	}
	if !value.IsInt64() {
		return 0, fmt.Errorf("integer oracle model value %s does not fit in int64", value)
	}
	decoded := value.Int64()
	if int64(int(decoded)) != decoded {
		return 0, fmt.Errorf("integer oracle model value %s does not fit in int", value)
	}
	return int(decoded), nil
}

type MockOracle struct{ Fixed int }

func NewMockOracle(fixed int) (*MockOracle, error) {
	if fixed < 0 {
		return nil, fmt.Errorf("mock oracle value must be non-negative")
	}
	return &MockOracle{Fixed: fixed}, nil
}

func (oracle *MockOracle) Size() int { return oracle.Fixed + 1 }
func (oracle *MockOracle) Chosen(_ RewriterScope, index int) (Expr, error) {
	return NewValueExpr(BoolValue(index == oracle.Fixed), Typed(BoolT1)), nil
}
func (oracle *MockOracle) Cases(_ RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	if len(assertions) != oracle.Size() || (elseAssertions != nil && len(elseAssertions) != oracle.Size()) {
		return nil, fmt.Errorf("oracle assertion sequences must have length %d", oracle.Size())
	}
	return assertions[oracle.Fixed], nil
}

func (oracle *MockOracle) Decode(*G3GroundEncoder) (int, error) { return oracle.Fixed, nil }

// PropositionalOracle represents a finite choice as Boolean bits, least
// significant bit first. Constraints returned by creation exclude unused bit
// patterns when the choice count is not a power of two.
type PropositionalOracle struct {
	Bits    []ArenaCell
	NValues int
}

func CreatePropositionalOracle(scope RewriterScope, nvalues int) (RewriterScope, *PropositionalOracle, []Expr, error) {
	if nvalues < 0 {
		return scope, nil, nil, fmt.Errorf("propositional oracle must have a non-negative size")
	}
	upperBound := 1
	bits := 0
	for upperBound < nvalues {
		upperBound *= 2
		bits++
	}
	oracle := &PropositionalOracle{Bits: make([]ArenaCell, bits), NValues: nvalues}
	for index := range oracle.Bits {
		scope.Arena, oracle.Bits[index] = scope.Arena.AppendCellType(BasicCellType{Type: BoolT1})
	}
	constraints := make([]Expr, 0, upperBound-nvalues)
	for value := nvalues; value < upperBound; value++ {
		chosen, err := oracle.Chosen(scope, value)
		if err != nil {
			return scope, nil, nil, err
		}
		constraints = append(constraints, MustOperExpr(Op("NOT"), Typed(BoolT1), chosen))
	}
	return scope, oracle, constraints, nil
}

func (oracle *PropositionalOracle) Size() int { return oracle.NValues }

func (oracle *PropositionalOracle) Chosen(scope RewriterScope, index int) (Expr, error) {
	if len(oracle.Bits) == 0 {
		return scope.Arena.TrueCell().NameExpr(), nil
	}
	literals := make([]Expr, len(oracle.Bits))
	for bit, cell := range oracle.Bits {
		literal := Expr(cell.NameExpr())
		if index&(1<<bit) == 0 {
			literal = MustOperExpr(Op("NOT"), Typed(BoolT1), literal)
		}
		literals[bit] = literal
	}
	if len(literals) == 1 {
		return literals[0], nil
	}
	return MustOperExpr(Op("AND"), Typed(BoolT1), literals...), nil
}

func (oracle *PropositionalOracle) Cases(scope RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	return oracleCases(oracle, scope, assertions, elseAssertions)
}

func (oracle *PropositionalOracle) Decode(encoder *G3GroundEncoder) (int, error) {
	value := 0
	for bit, cell := range oracle.Bits {
		enabled, err := encoder.EvaluateBool(cell.NameExpr())
		if err != nil {
			return 0, err
		}
		if enabled {
			value |= 1 << bit
		}
	}
	return value, nil
}

type SparseOracle struct {
	Oracle  ChoiceOracle
	Values  []int
	indices map[int]int
}

func NewSparseOracle(oracle ChoiceOracle, values []int) (*SparseOracle, error) {
	if oracle == nil {
		return nil, fmt.Errorf("sparse oracle requires a backend oracle")
	}
	values = append([]int(nil), values...)
	sort.Ints(values)
	values = compactInts(values)
	if oracle.Size() != len(values) {
		return nil, fmt.Errorf("sparse oracle backend has size %d, expected %d", oracle.Size(), len(values))
	}
	indices := make(map[int]int, len(values))
	for index, value := range values {
		indices[value] = index
	}
	return &SparseOracle{Oracle: oracle, Values: values, indices: indices}, nil
}

func compactInts(values []int) []int {
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

func (oracle *SparseOracle) Size() int { return len(oracle.Values) }
func (oracle *SparseOracle) Chosen(scope RewriterScope, index int) (Expr, error) {
	backendIndex, found := oracle.indices[index]
	if !found {
		return NewValueExpr(BoolValue(false), Typed(BoolT1)), nil
	}
	return oracle.Oracle.Chosen(scope, backendIndex)
}
func (oracle *SparseOracle) Cases(scope RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	return oracle.Oracle.Cases(scope, assertions, elseAssertions)
}

func (oracle *SparseOracle) Decode(encoder *G3GroundEncoder) (int, error) {
	decoder, ok := oracle.Oracle.(interface {
		Decode(*G3GroundEncoder) (int, error)
	})
	if !ok {
		return -1, fmt.Errorf("sparse oracle backend does not support model decoding")
	}
	index, err := decoder.Decode(encoder)
	if err != nil || index < 0 || index >= len(oracle.Values) {
		return -1, err
	}
	return oracle.Values[index], nil
}

type ZipOracle struct {
	Oracle ChoiceOracle
	Groups [][]int
}

func NewZipOracle(oracle ChoiceOracle, groups [][]int) (*ZipOracle, error) {
	if oracle == nil {
		return nil, fmt.Errorf("zip oracle requires a backend oracle")
	}
	copyGroups := make([][]int, len(groups))
	for index, group := range groups {
		copyGroups[index] = append([]int(nil), group...)
		if !sort.IntsAreSorted(copyGroups[index]) {
			return nil, fmt.Errorf("zip oracle group %d is not sorted", index)
		}
		for _, backendIndex := range group {
			if backendIndex < 0 || backendIndex >= oracle.Size() {
				return nil, fmt.Errorf("zip oracle backend index %d is outside 0..%d", backendIndex, oracle.Size()-1)
			}
		}
	}
	return &ZipOracle{Oracle: oracle, Groups: copyGroups}, nil
}

func (oracle *ZipOracle) Size() int { return len(oracle.Groups) }
func (oracle *ZipOracle) Chosen(scope RewriterScope, index int) (Expr, error) {
	if index < 0 || index >= len(oracle.Groups) {
		return NewValueExpr(BoolValue(false), Typed(BoolT1)), nil
	}
	disjuncts := make([]Expr, len(oracle.Groups[index]))
	for position, backendIndex := range oracle.Groups[index] {
		condition, err := oracle.Oracle.Chosen(scope, backendIndex)
		if err != nil {
			return nil, err
		}
		disjuncts[position] = condition
	}
	return MustOperExpr(Op("OR"), Typed(BoolT1), disjuncts...), nil
}
func (oracle *ZipOracle) Cases(scope RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	return oracleCases(oracle, scope, assertions, elseAssertions)
}

func (oracle *ZipOracle) Decode(encoder *G3GroundEncoder) (int, error) {
	decoder, ok := oracle.Oracle.(interface {
		Decode(*G3GroundEncoder) (int, error)
	})
	if !ok {
		return -1, fmt.Errorf("zip oracle backend does not support model decoding")
	}
	backendIndex, err := decoder.Decode(encoder)
	if err != nil {
		return -1, err
	}
	for index, group := range oracle.Groups {
		position := sort.SearchInts(group, backendIndex)
		if position < len(group) && group[position] == backendIndex {
			return index, nil
		}
	}
	return -1, nil
}

type UninterpretedConstOracle struct {
	Values []ArenaCell
	Cell   ArenaCell
}

func (oracle *UninterpretedConstOracle) Size() int { return len(oracle.Values) }
func (oracle *UninterpretedConstOracle) Chosen(_ RewriterScope, index int) (Expr, error) {
	if index < 0 || index >= len(oracle.Values) {
		return NewValueExpr(BoolValue(false), Typed(BoolT1)), nil
	}
	return MustOperExpr(Op("EQ"), Typed(BoolT1), oracle.Cell.NameExpr(), oracle.Values[index].NameExpr()), nil
}
func (oracle *UninterpretedConstOracle) Cases(scope RewriterScope, assertions, elseAssertions []Expr) (Expr, error) {
	return oracleCases(oracle, scope, assertions, elseAssertions)
}

func (oracle *UninterpretedConstOracle) Decode(encoder *G3GroundEncoder) (int, error) {
	for index, cell := range oracle.Values {
		equal := MustOperExpr(Op("EQ"), Typed(BoolT1), cell.NameExpr(), oracle.Cell.NameExpr())
		matches, err := encoder.EvaluateBool(equal)
		if err != nil {
			return -1, err
		}
		if matches {
			return index, nil
		}
	}
	return -1, nil
}

func CreateUninterpretedConstOracle(cache *UninterpretedLiteralCache, scope RewriterScope, nvalues int) (RewriterScope, *UninterpretedConstOracle, []Expr, error) {
	if cache == nil || nvalues < 0 {
		return scope, nil, nil, fmt.Errorf("uninterpreted oracle requires a cache and non-negative size")
	}
	typ, err := NewConstType("_ORA")
	if err != nil {
		return scope, nil, nil, err
	}
	values := make([]ArenaCell, nvalues)
	for index := 0; index < nvalues; index++ {
		scope.Arena, values[index], err = cache.GetOrCreate(scope.Arena, UninterpretedLiteral{Type: typ, Index: strconv.Itoa(index)})
		if err != nil {
			return scope, nil, nil, err
		}
	}
	newArena, cell := scope.Arena.AppendCellType(BasicCellType{Type: typ})
	scope.Arena = newArena
	oracle := &UninterpretedConstOracle{Values: values, Cell: cell}
	constraints := make([]Expr, 0, 1)
	if nvalues > 0 {
		disjuncts := make([]Expr, nvalues)
		for index := range values {
			disjuncts[index], _ = oracle.Chosen(scope, index)
		}
		constraints = append(constraints, MustOperExpr(Op("OR"), Typed(BoolT1), disjuncts...))
	}
	return scope, oracle, constraints, nil
}
