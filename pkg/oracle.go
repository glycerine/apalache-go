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
