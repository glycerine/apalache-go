package lp

// Source: src/util/lp/lp_dual_simplex.h:32-103;
// lp_dual_simplex.hpp:44-103,186-336. Retained mpq/mpq model preparation.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// RationalDualSimplex owns stage vectors and the core borrowing their headers.
// Source rational(double) boundaries remain checked assertions; native follows
// the existing exact-LP default-zero convention, not a floating conversion.
type RationalDualSimplex struct {
	*RationalLPSolver
	X, BCopy, LowBounds, UpperBounds            *IndexedRationalVector
	Basis, NBasis                               []uint32
	Heading                                     []int32
	CoreColumnTypes, LogicalColumnTypes         []ColumnType
	CanEnterBasis                               []bool
	CoreSolver                                  *RationalDualCoreSolver
	partialCore                                 *RationalDualCoreSolver
	previousCores                               []*RationalDualCoreSolver
	FirstStageIterations, SecondStageIterations uint32
	stageWork, columnWork                       *IndexedRationalVector
	outerCalls                                  exactEtaCalls
	outerStopped, outerClosed                   bool
}

func NewRationalDualSimplex(manager *numeral.RationalManager) (*RationalDualSimplex, error) {
	base, e := NewRationalLPSolver(manager)
	if e != nil {
		return nil, e
	}
	return &RationalDualSimplex{RationalLPSolver: base, X: NewIndexedRationalVector(manager, 0), BCopy: NewIndexedRationalVector(manager, 0), LowBounds: NewIndexedRationalVector(manager, 0), UpperBounds: NewIndexedRationalVector(manager, 0)}, nil
}
func (m *RationalDualSimplex) outerReady() error {
	if m.outerStopped || m.outerClosed || m.stageWork != nil || m.columnWork != nil || m.outerCalls.pending() {
		return ErrExactVectorPending
	}
	return m.RationalLPSolver.ready()
}
func (m *RationalDualSimplex) finishStageWork() error {
	if e := m.stageWork.Close(); e != nil {
		return e
	}
	m.stageWork = nil
	return nil
}
func (m *RationalDualSimplex) numberOfCoreStructurals() uint32 {
	return uint32(len(m.CoreColumnsToExternal))
}
func (m *RationalDualSimplex) setTypeForLogical(j uint32, typ ColumnType) error {
	i := j - m.numberOfCoreStructurals()
	if uint64(i) >= uint64(len(m.LogicalColumnTypes)) {
		return ErrBounds
	}
	m.LogicalColumnTypes[i] = typ
	return nil
}
func (m *RationalDualSimplex) SetTypeForLogical(j uint32, typ ColumnType) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	return m.setTypeForLogical(j, typ)
}
func (m *RationalDualSimplex) structuralInfo(j uint32, assertMaps bool) (*RationalColumnInfo, error) {
	external, ok := m.CoreColumnsToExternal[j]
	if !ok {
		if assertMaps && sourceDebugEnabled {
			return nil, ErrAssertion
		}
		m.CoreColumnsToExternal[j] = 0
	}
	ci, ok := m.Columns.Get2(external)
	if !ok {
		if assertMaps && sourceDebugEnabled {
			return nil, ErrAssertion
		}
		m.Columns.Set(external, nil)
	}
	if ci == nil {
		return nil, ErrBounds
	}
	return ci, nil
}
func (m *RationalDualSimplex) columnType(j uint32) (ColumnType, error) {
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
	c, e := m.structuralInfo(j, false)
	if e != nil {
		return ColumnFree, e
	}
	return c.ColumnType(), nil
}
func (m *RationalDualSimplex) GetColumnType(j uint32) (typ ColumnType, err error) {
	if err = m.outerReady(); err != nil {
		return ColumnFree, err
	}
	defer func() { m.outerStopped = err != nil }()
	return m.columnType(j)
}
func (m *RationalDualSimplex) setCanEnter(j uint32, v bool) error {
	if uint64(j) >= uint64(len(m.CanEnterBasis)) {
		return ErrBounds
	}
	m.CanEnterBasis[j] = v
	return nil
}
func (m *RationalDualSimplex) setCoreType(j uint32, v ColumnType) error {
	if uint64(j) >= uint64(len(m.CoreColumnTypes)) {
		return ErrBounds
	}
	m.CoreColumnTypes[j] = v
	return nil
}
func (m *RationalDualSimplex) setBasis(row, j uint32) error {
	if uint64(row) >= uint64(len(m.Basis)) {
		return ErrBounds
	}
	m.Basis[row] = j
	return nil
}
func (m *RationalDualSimplex) setConstant(v *IndexedRationalVector, j uint32, value int32) error {
	if v == nil || uint64(j) >= uint64(len(v.Data)) {
		return ErrBounds
	}
	x := numeral.NewRational()
	if e := m.manager.SetInt32(&x, value); e != nil {
		return e
	}
	return v.copySlot(&v.Data[j], &x)
}
func (m *RationalDualSimplex) FillFirstStageStructuralColumn(j uint32) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if sourceDebugEnabled {
		if _, ok := m.CoreColumnsToExternal[j]; !ok {
			return ErrAssertion
		}
	}
	m.stageWork = NewIndexedRationalVector(m.manager, 2)
	// Source T free_bound=T(1e4) is unconditional, before the column lookup.
	// rational(double) is UNREACHABLE; retain the existing exact-LP convention.
	if sourceDebugEnabled {
		return ErrAssertion
	}
	ci, e := m.structuralInfo(j, true)
	if e != nil {
		return e
	}
	switch ci.ColumnType() {
	case ColumnUpperBound:
		typ, e := m.columnType(j)
		if e != nil {
			return e
		}
		name, e := ColumnTypeString(typ)
		if e != nil {
			return e
		}
		return z3exception.NewDefaultException(fmt.Sprintf("unexpected bound type %d %s", j, name))
	case ColumnLowBound:
		if err = m.setCanEnter(j, true); err != nil {
			return err
		}
		if err = m.SetScaledCost(j); err != nil {
			return err
		}
		if err = m.setConstant(m.LowBounds, j, 0); err != nil {
			return err
		}
		if err = m.setConstant(m.UpperBounds, j, 1); err != nil {
			return err
		}
	case ColumnFree:
		if err = m.setCanEnter(j, true); err != nil {
			return err
		}
		if err = m.SetScaledCost(j); err != nil {
			return err
		}
		if uint64(j) >= uint64(len(m.UpperBounds.Data)) {
			return ErrBounds
		}
		if err = m.UpperBounds.copySlot(&m.UpperBounds.Data[j], &m.stageWork.Data[0]); err != nil {
			return err
		}
		if err = m.stageWork.copySlot(&m.stageWork.Data[1], &m.stageWork.Data[0]); err != nil {
			return err
		}
		before, e := m.outerCalls.begin(m.manager)
		if e != nil {
			return e
		}
		err = m.manager.Neg(&m.stageWork.Data[1])
		if err = m.outerCalls.after(m.manager, before, err); err != nil {
			return err
		}
		if uint64(j) >= uint64(len(m.LowBounds.Data)) {
			return ErrBounds
		}
		if err = m.LowBounds.copySlot(&m.LowBounds.Data[j], &m.stageWork.Data[1]); err != nil {
			return err
		}
	case ColumnBoxed:
		if err = m.setCanEnter(j, false); err != nil {
			return err
		}
		if err = m.setConstant(m.Costs, j, 0); err != nil {
			return err
		}
		// The source chained assignment stores low before upper.
		if err = m.setConstant(m.LowBounds, j, 0); err != nil {
			return err
		}
		if err = m.setConstant(m.UpperBounds, j, 0); err != nil {
			return err
		}
	default:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	if err = m.setCoreType(j, ColumnBoxed); err != nil {
		return err
	}
	return m.finishStageWork()
}
func (m *RationalDualSimplex) FillFirstStageLogicalColumn(j uint32) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if err = m.setConstant(m.Costs, j, 0); err != nil {
		return err
	}
	if sourceDebugEnabled {
		typ, e := m.columnType(j)
		if e != nil {
			return e
		}
		if typ == ColumnUpperBound {
			return ErrAssertion
		}
	}
	typ, e := m.columnType(j)
	if e != nil {
		return e
	}
	enter := typ == ColumnLowBound
	if err = m.setCanEnter(j, enter); err != nil {
		return err
	}
	if enter {
		err = m.setCoreType(j, ColumnBoxed)
	} else {
		err = m.setCoreType(j, ColumnFixed)
	}
	if err != nil {
		return err
	}
	if err = m.setConstant(m.LowBounds, j, 0); err != nil {
		return err
	}
	upper := int32(0)
	if enter {
		upper = 1
	}
	return m.setConstant(m.UpperBounds, j, upper)
}
func (m *RationalDualSimplex) FillFirstStageCostsBoundsAndTypes() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	for j := m.A.ColumnCount(); j > m.numberOfCoreStructurals(); {
		j--
		if err = m.FillFirstStageLogicalColumn(j); err != nil {
			return err
		}
	}
	for j := m.numberOfCoreStructurals(); j > 0; {
		j--
		if err = m.FillFirstStageStructuralColumn(j); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualSimplex) rowSign(positive bool) (yes bool, err error) {
	before, e := m.outerCalls.begin(m.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = m.outerCalls.after(m.manager, before, err) }()
	zero := numeral.NewRational()
	var cmp int
	if positive {
		cmp, err = m.manager.Compare(&zero, &m.stageWork.Data[0])
		return cmp < 0, err
	}
	cmp, err = m.manager.Compare(&m.stageWork.Data[0], &zero)
	return cmp < 0, err
}
func (m *RationalDualSimplex) setRowUnit(row, col uint32, value int32) (err error) {
	before, e := m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.SetInt32(&m.stageWork.Data[1], value)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	return m.A.Set(row, col, &m.stageWork.Data[1])
}
func (m *RationalDualSimplex) FillFirstStageRow(row uint32, slack, artificial *uint32) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
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
	c, ok := m.Constraints.Get2(external)
	if !ok {
		c = &RationalConstraint{RightSide: m.newValue()}
		m.Constraints.Set(external, c)
	}
	if c == nil || m.B == nil || uint64(row) >= uint64(len(m.B.Data)) {
		return ErrBounds
	}
	m.stageWork = NewIndexedRationalVector(m.manager, 2)
	if err = m.stageWork.copySlot(&m.stageWork.Data[0], &m.B.Data[row]); err != nil {
		return err
	}
	if !c.relationSet {
		return ErrBounds
	} // guard the source indeterminate relation after copying rs
	switch c.Relation {
	case RelationEqual:
		if err = m.setTypeForLogical(*artificial, ColumnFixed); err != nil {
			return err
		}
		if err = m.setBasis(row, *artificial); err != nil {
			return err
		}
		if err = m.setConstant(m.Costs, *artificial, 0); err != nil {
			return err
		}
		if err = m.setRowUnit(row, *artificial, 1); err != nil {
			return err
		}
		*artificial++
	case RelationGreaterOrEqual, RelationLessOrEqual:
		if err = m.setTypeForLogical(*slack, ColumnLowBound); err != nil {
			return err
		}
		coefficient := int32(1)
		positive := c.Relation == RelationGreaterOrEqual
		if positive {
			coefficient = -1
		}
		if err = m.setRowUnit(row, *slack, coefficient); err != nil {
			return err
		}
		needs, e := m.rowSign(positive)
		if e != nil {
			return e
		}
		if needs {
			if err = m.setTypeForLogical(*artificial, ColumnFixed); err != nil {
				return err
			}
			if err = m.setRowUnit(row, *artificial, -coefficient); err != nil {
				return err
			}
			if err = m.setBasis(row, *artificial); err != nil {
				return err
			}
			if err = m.setConstant(m.Costs, *artificial, 0); err != nil {
				return err
			}
			*artificial++
		} else {
			if err = m.setBasis(row, *slack); err != nil {
				return err
			}
			if err = m.setConstant(m.Costs, *slack, 0); err != nil {
				return err
			}
		}
		*slack++
	}
	return m.finishStageWork()
}
func (m *RationalDualSimplex) FillFirstStageSolverFields() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	slack := m.numberOfCoreStructurals()
	artificial := slack + m.Slacks
	for row := uint32(0); row < m.A.RowCount(); row++ {
		if err = m.FillFirstStageRow(row, &slack, &artificial); err != nil {
			return err
		}
	}
	return m.FillFirstStageCostsBoundsAndTypes()
}
func (m *RationalDualSimplex) AugmentMatrixAndAllocateFields() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	if err = m.CountSlacksAndArtificials(); err != nil {
		return err
	}
	logical := m.Slacks + m.Artificials
	if err = m.A.AddColumns(logical); err != nil {
		return err
	}
	n := m.A.ColumnCount()
	m.CoreColumnTypes = resizeDualColumnTypes(m.CoreColumnTypes, int(n))
	m.LogicalColumnTypes = resizeDualColumnTypes(m.LogicalColumnTypes, int(logical))
	for _, v := range []*IndexedRationalVector{m.Costs, m.UpperBounds, m.LowBounds} {
		if err = v.resizeData(n); err != nil {
			return err
		}
	}
	if len(m.CanEnterBasis) > int(n) {
		m.CanEnterBasis = m.CanEnterBasis[:n]
	} else {
		m.CanEnterBasis = append(m.CanEnterBasis, make([]bool, int(n)-len(m.CanEnterBasis))...)
	}
	rows := int(m.A.RowCount())
	if len(m.Basis) > rows {
		m.Basis = m.Basis[:rows]
	} else {
		m.Basis = append(m.Basis, make([]uint32, rows-len(m.Basis))...)
	}
	return nil // Source does not resize X despite its method name.
}
func (m *RationalDualSimplex) CopyBAsideAndSetToZeros() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.B == nil {
		return ErrBounds
	}
	for i := range m.B.Data {
		// push_back publishes size after successful element construction.
		m.stageWork = NewIndexedRationalVector(m.manager, 1)
		if err = m.stageWork.copySlot(&m.stageWork.Data[0], &m.B.Data[i]); err != nil {
			return err
		}
		m.BCopy.Data = append(m.BCopy.Data, m.stageWork.Data[0])
		m.stageWork.Data[0] = numeral.NewRational() // transfer the owning slot
		if err = m.finishStageWork(); err != nil {
			return err
		}
		if err = m.setConstant(m.B, uint32(i), 0); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualSimplex) RestoreRightSides() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.B.Data)) || uint64(i) >= uint64(len(m.BCopy.Data)) {
			return ErrBounds
		}
		if err = m.B.copySlot(&m.B.Data[i], &m.BCopy.Data[i]); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualSimplex) FixLogicalForStage2(j uint32) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if sourceDebugEnabled && j < m.numberOfCoreStructurals() {
		return ErrAssertion
	}
	i := j - m.numberOfCoreStructurals()
	if uint64(i) >= uint64(len(m.LogicalColumnTypes)) {
		return ErrBounds
	}
	switch m.LogicalColumnTypes[i] {
	case ColumnLowBound:
		if err = m.setConstant(m.LowBounds, j, 0); err != nil {
			return err
		}
		if err = m.setCoreType(j, ColumnLowBound); err != nil {
			return err
		}
		return m.setCanEnter(j, true)
	case ColumnFixed:
		if err = m.setConstant(m.LowBounds, j, 0); err != nil {
			return err
		}
		if err = m.setConstant(m.UpperBounds, j, 0); err != nil {
			return err
		}
		if err = m.setCoreType(j, ColumnFixed); err != nil {
			return err
		}
		return m.setCanEnter(j, false)
	default:
		return sourceAssertionFailure()
	}
}
func (m *RationalDualSimplex) FixStructuralForStage2(j uint32) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	ci, e := m.structuralInfo(j, false)
	if e != nil {
		return e
	}
	switch ci.ColumnType() {
	case ColumnLowBound:
		if err = m.setConstant(m.LowBounds, j, 0); err != nil {
			return err
		}
		if err = m.setCoreType(j, ColumnLowBound); err != nil {
			return err
		}
		if err = m.setCanEnter(j, true); err != nil {
			return err
		}
	case ColumnFixed, ColumnUpperBound:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		fallthrough
	case ColumnBoxed:
		m.stageWork = NewIndexedRationalVector(m.manager, 3)
		if err = ci.AdjustedUpperBound(&m.stageWork.Data[0]); err != nil {
			return err
		}
		if m.ColumnScale == nil || uint64(j) >= uint64(len(m.ColumnScale.Data)) {
			return ErrBounds
		}
		before, e := m.outerCalls.begin(m.manager)
		if e != nil {
			return e
		}
		err = etaRationalQuotient(m.manager, &m.stageWork.Data[1], &m.stageWork.Data[2], &m.stageWork.Data[0], &m.ColumnScale.Data[j])
		if err = m.outerCalls.after(m.manager, before, err); err != nil {
			return err
		}
		if uint64(j) >= uint64(len(m.UpperBounds.Data)) {
			return ErrBounds
		}
		if err = m.UpperBounds.copySlot(&m.UpperBounds.Data[j], &m.stageWork.Data[2]); err != nil {
			return err
		}
		if err = m.finishStageWork(); err != nil {
			return err
		}
		if err = m.setConstant(m.LowBounds, j, 0); err != nil {
			return err
		}
		if err = m.setCoreType(j, ColumnBoxed); err != nil {
			return err
		}
		if err = m.setCanEnter(j, true); err != nil {
			return err
		}
	case ColumnFree:
		if err = m.setCanEnter(j, true); err != nil {
			return err
		}
		if err = m.setCoreType(j, ColumnFree); err != nil {
			return err
		}
	default:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	return m.SetScaledCost(j)
}
func (m *RationalDualSimplex) UnmarkColumnsAndFixStructuralCosts() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	for j := m.A.ColumnCount(); j > m.numberOfCoreStructurals(); {
		j--
		if err = m.FixLogicalForStage2(j); err != nil {
			return err
		}
	}
	for j := m.numberOfCoreStructurals(); j > 0; {
		j--
		if err = m.FixStructuralForStage2(j); err != nil {
			return err
		}
	}
	return nil
}
