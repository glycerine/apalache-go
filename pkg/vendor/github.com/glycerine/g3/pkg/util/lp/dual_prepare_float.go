package lp

// Source: src/util/lp/lp_dual_simplex.h:32-75;
// lp_dual_simplex.hpp:44-103,136-142,186-336. Floating dual model preparation.

import (
	"fmt"
	"sort"

	"github.com/glycerine/g3/pkg/util/z3exception"
)

// FlipCosts is the common lp_solver.hpp:97-102 operation used by dual setup.
// Numeric indices choose deterministic order for the source unordered map.
func (m *FloatLPSolver) FlipCosts() error {
	keys := make([]uint32, 0, len(m.Columns))
	for j := range m.Columns {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	for _, j := range keys {
		c := m.Columns[j]
		if c == nil {
			return ErrBounds
		}
		c.Cost = -c.Cost
	}
	return nil
}

// FloatDualSimplex owns the retained floating two-stage dual model and core.
type FloatDualSimplex struct {
	*FloatLPSolver
	BCopy, LowBounds                            []float64
	CoreColumnTypes, LogicalColumnTypes         []ColumnType
	CanEnterBasis                               []bool
	CoreSolver                                  *FloatDualCoreSolver
	FirstStageIterations, SecondStageIterations uint32
}

func NewFloatDualSimplex() (*FloatDualSimplex, error) {
	b, err := NewFloatLPSolver()
	if err != nil {
		return nil, err
	}
	return &FloatDualSimplex{FloatLPSolver: b}, nil
}
func (m *FloatDualSimplex) numberOfCoreStructurals() uint32 {
	return uint32(len(m.CoreColumnsToExternal))
}
func (m *FloatDualSimplex) SetTypeForLogical(j uint32, typ ColumnType) error {
	i := j - m.numberOfCoreStructurals()
	if uint64(i) >= uint64(len(m.LogicalColumnTypes)) {
		return ErrBounds
	}
	m.LogicalColumnTypes[i] = typ
	return nil
}
func (m *FloatDualSimplex) structuralInfo(j uint32, assertMaps bool) (*FloatColumnInfo, error) {
	external, ok := m.CoreColumnsToExternal[j]
	if !ok {
		if assertMaps && sourceDebugEnabled {
			return nil, ErrAssertion
		}
		m.CoreColumnsToExternal[j] = 0
	}
	ci, ok := m.Columns[external]
	if !ok {
		if assertMaps && sourceDebugEnabled {
			return nil, ErrAssertion
		}
		m.Columns[external] = nil
	}
	if ci == nil {
		return nil, ErrBounds
	}
	return ci, nil
}
func (m *FloatDualSimplex) GetColumnType(j uint32) (ColumnType, error) {
	if m.A == nil {
		return ColumnFree, ErrBounds
	}
	if sourceDebugEnabled && j >= m.A.ColumnCount() {
		return ColumnFree, ErrAssertion
	}
	if j >= m.numberOfCoreStructurals() {
		i := j - m.numberOfCoreStructurals()
		if uint64(i) >= uint64(len(m.LogicalColumnTypes)) {
			return ColumnFree, ErrBounds
		}
		return m.LogicalColumnTypes[i], nil
	}
	ci, err := m.structuralInfo(j, false)
	if err != nil {
		return ColumnFree, err
	}
	return ci.ColumnType(), nil
}
func (m *FloatDualSimplex) setCanEnter(j uint32, v bool) error {
	if uint64(j) >= uint64(len(m.CanEnterBasis)) {
		return ErrBounds
	}
	m.CanEnterBasis[j] = v
	return nil
}
func (m *FloatDualSimplex) setCoreType(j uint32, v ColumnType) error {
	if uint64(j) >= uint64(len(m.CoreColumnTypes)) {
		return ErrBounds
	}
	m.CoreColumnTypes[j] = v
	return nil
}
func (m *FloatDualSimplex) setLow(j uint32, v float64) error {
	if uint64(j) >= uint64(len(m.LowBounds)) {
		return ErrBounds
	}
	m.LowBounds[j] = v
	return nil
}
func (m *FloatDualSimplex) setUpper(j uint32, v float64) error {
	if uint64(j) >= uint64(len(m.UpperBounds)) {
		return ErrBounds
	}
	m.UpperBounds[j] = v
	return nil
}
func (m *FloatDualSimplex) setCost(j uint32, v float64) error {
	if uint64(j) >= uint64(len(m.Costs)) {
		return ErrBounds
	}
	m.Costs[j] = v
	return nil
}
func (m *FloatDualSimplex) FillFirstStageStructuralColumn(j uint32) error {
	ci, err := m.structuralInfo(j, true)
	if err != nil {
		return err
	}
	switch ci.ColumnType() {
	case ColumnUpperBound:
		typ, err := m.GetColumnType(j)
		if err != nil {
			return err
		}
		name, err := ColumnTypeString(typ)
		if err != nil {
			return err
		}
		return z3exception.NewDefaultException(fmt.Sprintf("unexpected bound type %d %s", j, name))
	case ColumnLowBound:
		if err := m.setCanEnter(j, true); err != nil {
			return err
		}
		if err := m.SetScaledCost(j); err != nil {
			return err
		}
		if err := m.setLow(j, 0); err != nil {
			return err
		}
		if err := m.setUpper(j, 1); err != nil {
			return err
		}
	case ColumnFree:
		if err := m.setCanEnter(j, true); err != nil {
			return err
		}
		if err := m.SetScaledCost(j); err != nil {
			return err
		}
		if err := m.setUpper(j, 1e4); err != nil {
			return err
		}
		if err := m.setLow(j, -1e4); err != nil {
			return err
		}
	case ColumnBoxed:
		if err := m.setCanEnter(j, false); err != nil {
			return err
		}
		if err := m.setCost(j, 0); err != nil {
			return err
		}
		if err := m.setLow(j, 0); err != nil {
			return err
		}
		if err := m.setUpper(j, 0); err != nil {
			return err
		}
	default:
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return m.setCoreType(j, ColumnBoxed)
}
func (m *FloatDualSimplex) FillFirstStageLogicalColumn(j uint32) error {
	if err := m.setCost(j, 0); err != nil {
		return err
	}
	typ, err := m.GetColumnType(j)
	if err != nil {
		return err
	}
	if sourceDebugEnabled && typ == ColumnUpperBound {
		return ErrAssertion
	}
	enter := typ == ColumnLowBound
	if err := m.setCanEnter(j, enter); err != nil {
		return err
	}
	if enter {
		if err := m.setCoreType(j, ColumnBoxed); err != nil {
			return err
		}
	} else {
		if err := m.setCoreType(j, ColumnFixed); err != nil {
			return err
		}
	}
	if err := m.setLow(j, 0); err != nil {
		return err
	}
	upper := 0.0
	if enter {
		upper = 1
	}
	return m.setUpper(j, upper)
}
func (m *FloatDualSimplex) FillFirstStageCostsBoundsAndTypes() error {
	if m.A == nil {
		return ErrBounds
	}
	for j := m.A.ColumnCount(); j > m.numberOfCoreStructurals(); {
		j--
		if err := m.FillFirstStageLogicalColumn(j); err != nil {
			return err
		}
	}
	for j := m.numberOfCoreStructurals(); j > 0; {
		j--
		if err := m.FillFirstStageStructuralColumn(j); err != nil {
			return err
		}
	}
	return nil
}
func (m *FloatDualSimplex) setBasis(row, j uint32) error {
	if uint64(row) >= uint64(len(m.Basis)) {
		return ErrBounds
	}
	m.Basis[row] = j
	return nil
}
func (m *FloatDualSimplex) FillFirstStageRow(row uint32, slack, artificial *uint32) error {
	if m.A == nil || slack == nil || artificial == nil {
		return ErrBounds
	}
	if sourceDebugEnabled && row >= m.A.RowCount() {
		return ErrAssertion
	}
	external, ok := m.CoreRowsToExternal[row]
	if !ok {
		m.CoreRowsToExternal[row] = 0
	}
	c, ok := m.Constraints[external]
	if !ok {
		m.Constraints[external] = FloatConstraint{}
		return ErrBounds
	}
	if uint64(row) >= uint64(len(m.B)) {
		return ErrBounds
	}
	rs := m.B[row]
	switch c.Relation {
	case RelationEqual:
		if err := m.SetTypeForLogical(*artificial, ColumnFixed); err != nil {
			return err
		}
		if err := m.setBasis(row, *artificial); err != nil {
			return err
		}
		if err := m.setCost(*artificial, 0); err != nil {
			return err
		}
		if err := m.A.Set(row, *artificial, 1); err != nil {
			return err
		}
		*artificial++
	case RelationGreaterOrEqual, RelationLessOrEqual:
		if err := m.SetTypeForLogical(*slack, ColumnLowBound); err != nil {
			return err
		}
		coefficient := 1.0
		needsArtificial := rs < 0
		if c.Relation == RelationGreaterOrEqual {
			coefficient = -1
			needsArtificial = rs > 0
		}
		if err := m.A.Set(row, *slack, coefficient); err != nil {
			return err
		}
		if needsArtificial {
			if err := m.SetTypeForLogical(*artificial, ColumnFixed); err != nil {
				return err
			}
			if err := m.A.Set(row, *artificial, -coefficient); err != nil {
				return err
			}
			if err := m.setBasis(row, *artificial); err != nil {
				return err
			}
			if err := m.setCost(*artificial, 0); err != nil {
				return err
			}
			*artificial++
		} else {
			if err := m.setBasis(row, *slack); err != nil {
				return err
			}
			if err := m.setCost(*slack, 0); err != nil {
				return err
			}
		}
		*slack++
	}
	return nil
}
func (m *FloatDualSimplex) FillFirstStageSolverFields() error {
	if m.A == nil {
		return ErrBounds
	}
	slack := m.numberOfCoreStructurals()
	artificial := slack + m.Slacks
	for row := uint32(0); row < m.A.RowCount(); row++ {
		if err := m.FillFirstStageRow(row, &slack, &artificial); err != nil {
			return err
		}
	}
	return m.FillFirstStageCostsBoundsAndTypes()
}
func resizeDualColumnTypes(v []ColumnType, n int) []ColumnType {
	if len(v) > n {
		return v[:n]
	}
	return append(v, make([]ColumnType, n-len(v))...)
}
func (m *FloatDualSimplex) AugmentMatrixAndAllocateFields() error {
	if m.A == nil {
		return ErrBounds
	}
	if err := m.CountSlacksAndArtificials(); err != nil {
		return err
	}
	logical := m.Slacks + m.Artificials
	m.A.AddColumns(logical)
	n := int(m.A.ColumnCount())
	m.CoreColumnTypes = resizeDualColumnTypes(m.CoreColumnTypes, n)
	m.LogicalColumnTypes = resizeDualColumnTypes(m.LogicalColumnTypes, int(logical))
	m.Costs = resizeFloatSolverValues(m.Costs, n)
	m.UpperBounds = resizeFloatSolverValues(m.UpperBounds, n)
	m.LowBounds = resizeFloatSolverValues(m.LowBounds, n)
	if len(m.CanEnterBasis) > n {
		m.CanEnterBasis = m.CanEnterBasis[:n]
	} else {
		m.CanEnterBasis = append(m.CanEnterBasis, make([]bool, n-len(m.CanEnterBasis))...)
	}
	rows := int(m.A.RowCount())
	if len(m.Basis) > rows {
		m.Basis = m.Basis[:rows]
	} else {
		m.Basis = append(m.Basis, make([]uint32, rows-len(m.Basis))...)
	}
	return nil // Source does not resize or write X here despite its method name.
}
func (m *FloatDualSimplex) CopyBAsideAndSetToZeros() {
	for i, v := range m.B {
		m.BCopy = append(m.BCopy, v)
		m.B[i] = 0
	}
}
func (m *FloatDualSimplex) RestoreRightSides() error {
	if m.A == nil {
		return ErrBounds
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.B)) || uint64(i) >= uint64(len(m.BCopy)) {
			return ErrBounds
		}
		m.B[i] = m.BCopy[i]
	}
	return nil
}
func (m *FloatDualSimplex) FixLogicalForStage2(j uint32) error {
	if sourceDebugEnabled && j < m.numberOfCoreStructurals() {
		return ErrAssertion
	}
	i := j - m.numberOfCoreStructurals()
	if uint64(i) >= uint64(len(m.LogicalColumnTypes)) {
		return ErrBounds
	}
	switch m.LogicalColumnTypes[i] {
	case ColumnLowBound:
		if err := m.setLow(j, 0); err != nil {
			return err
		}
		if err := m.setCoreType(j, ColumnLowBound); err != nil {
			return err
		}
		return m.setCanEnter(j, true)
	case ColumnFixed:
		if err := m.setLow(j, 0); err != nil {
			return err
		}
		if err := m.setUpper(j, 0); err != nil {
			return err
		}
		if err := m.setCoreType(j, ColumnFixed); err != nil {
			return err
		}
		return m.setCanEnter(j, false)
	default:
		return sourceAssertionFailure()
	}
}
func (m *FloatDualSimplex) FixStructuralForStage2(j uint32) error {
	ci, err := m.structuralInfo(j, false)
	if err != nil {
		return err
	}
	switch ci.ColumnType() {
	case ColumnLowBound:
		if err := m.setLow(j, 0); err != nil {
			return err
		}
		if err := m.setCoreType(j, ColumnLowBound); err != nil {
			return err
		}
		if err := m.setCanEnter(j, true); err != nil {
			return err
		}
	case ColumnFixed, ColumnUpperBound:
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
		fallthrough
	case ColumnBoxed:
		upper, err := ci.AdjustedUpperBound()
		if err != nil {
			return err
		}
		if uint64(j) >= uint64(len(m.ColumnScale)) {
			return ErrBounds
		}
		if err := m.setUpper(j, upper/m.ColumnScale[j]); err != nil {
			return err
		}
		if err := m.setLow(j, 0); err != nil {
			return err
		}
		if err := m.setCoreType(j, ColumnBoxed); err != nil {
			return err
		}
		if err := m.setCanEnter(j, true); err != nil {
			return err
		}
	case ColumnFree:
		if err := m.setCanEnter(j, true); err != nil {
			return err
		}
		if err := m.setCoreType(j, ColumnFree); err != nil {
			return err
		}
	default:
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return m.SetScaledCost(j)
}
func (m *FloatDualSimplex) UnmarkColumnsAndFixStructuralCosts() error {
	if m.A == nil {
		return ErrBounds
	}
	for j := m.A.ColumnCount(); j > m.numberOfCoreStructurals(); {
		j--
		if err := m.FixLogicalForStage2(j); err != nil {
			return err
		}
	}
	for j := m.numberOfCoreStructurals(); j > 0; {
		j--
		if err := m.FixStructuralForStage2(j); err != nil {
			return err
		}
	}
	return nil
}
