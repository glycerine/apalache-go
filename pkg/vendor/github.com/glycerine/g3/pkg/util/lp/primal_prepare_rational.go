package lp

// Source: src/util/lp/lp_primal_simplex.h:32-88;
// lp_primal_simplex.hpp:63-68,146-165,189-250. Retained mpq/mpq instance.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

// RationalPrimalSimplex owns stage-one vectors and the core that borrows them.
// Close the solver before its numeral manager; do not copy an owning solver.
type RationalPrimalSimplex struct {
	*RationalLPSolver
	X, LowBounds, UpperBounds *IndexedRationalVector
	Basis, NBasis             []uint32
	Heading                   []int32
	ColumnTypes               []ColumnType
	CoreSolver                *RationalPrimalCoreSolver
	partialCore               *RationalPrimalCoreSolver
	previousCores             []*RationalPrimalCoreSolver
	stageWork, columnWork     *IndexedRationalVector
	outerCalls                exactEtaCalls
	outerStopped, outerClosed bool
}

func NewRationalPrimalSimplex(manager *numeral.RationalManager) (*RationalPrimalSimplex, error) {
	base, e := NewRationalLPSolver(manager)
	if e != nil {
		return nil, e
	}
	return &RationalPrimalSimplex{RationalLPSolver: base, X: NewIndexedRationalVector(manager, 0), LowBounds: NewIndexedRationalVector(manager, 0), UpperBounds: NewIndexedRationalVector(manager, 0)}, nil
}
func (m *RationalPrimalSimplex) outerReady() error {
	if m.outerStopped || m.outerClosed || m.stageWork != nil || m.columnWork != nil || m.outerCalls.pending() {
		return ErrExactVectorPending
	}
	return m.RationalLPSolver.ready()
}
func (m *RationalPrimalSimplex) finishStageWork() error {
	if e := m.stageWork.Close(); e != nil {
		return e
	}
	m.stageWork = nil
	return nil
}
func (m *RationalPrimalSimplex) FillAcceptableValuesForX() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	zero := numeral.NewRational()
	for j := range m.CoreColumnsToExternal {
		if uint64(j) >= uint64(len(m.X.Data)) {
			return ErrBounds
		}
		if err = m.X.copySlot(&m.X.Data[j], &zero); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalPrimalSimplex) SetCoreSolverBounds() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	total := m.A.ColumnCount() + m.Slacks + m.Artificials
	for uint64(len(m.ColumnTypes)) > uint64(total) {
		m.ColumnTypes = m.ColumnTypes[:len(m.ColumnTypes)-1]
	}
	for uint64(len(m.ColumnTypes)) < uint64(total) {
		m.ColumnTypes = append(m.ColumnTypes, ColumnFree)
	}
	if err = m.UpperBounds.resizeData(total); err != nil {
		return err
	}
	zero := numeral.NewRational()
	for _, c := range m.Columns.All() {
		if c == nil {
			return ErrBounds
		}
		j := c.Index
		if !IsValidColumnIndex(j) {
			continue
		}
		if uint64(j) >= uint64(len(m.ColumnTypes)) {
			return ErrBounds
		}
		// Source keeps upper-only types after flipping the model coefficient.
		m.ColumnTypes[j] = c.ColumnType()
		switch m.ColumnTypes[j] {
		case ColumnFixed:
			if err = m.UpperBounds.copySlot(&m.UpperBounds.Data[j], &zero); err != nil {
				return err
			}
		case ColumnBoxed:
			m.stageWork = NewIndexedRationalVector(m.manager, 3)
			if err = c.AdjustedUpperBound(&m.stageWork.Data[0]); err != nil {
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
			if err = m.UpperBounds.copySlot(&m.UpperBounds.Data[j], &m.stageWork.Data[2]); err != nil {
				return err
			}
			if err = m.finishStageWork(); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *RationalPrimalSimplex) FillAXAndBasisForStageOneTotalInfForRow(row uint32) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	if err = lpSolverDomain(row < m.A.RowCount()); err != nil {
		return err
	}
	ext, exists := m.CoreRowsToExternal[row]
	if err = lpSolverDomain(exists); err != nil {
		return err
	}
	c, exists := m.Constraints.Get2(ext)
	if err = lpSolverDomain(exists); err != nil {
		return err
	}
	if c == nil {
		return ErrBounds
	}
	j := m.A.ColumnCount()
	if err = m.A.AddColumn(); err != nil {
		return err
	}
	if uint64(row) >= uint64(len(m.Basis)) {
		return ErrBounds
	}
	m.Basis[row] = j
	switch c.Relation {
	case RelationEqual, RelationGreaterOrEqual, RelationLessOrEqual:
	default:
		return sourceAssertionFailure()
	}
	if m.B == nil || uint64(row) >= uint64(len(m.B.Data)) || uint64(j) >= uint64(len(m.X.Data)) {
		return ErrBounds
	}
	m.stageWork = NewIndexedRationalVector(m.manager, 2)
	v := &m.B.Data[row]
	if c.Relation == RelationGreaterOrEqual {
		if err = m.stageWork.copySlot(&m.stageWork.Data[0], v); err != nil {
			return err
		}
		before, e := m.outerCalls.begin(m.manager)
		if e != nil {
			return e
		}
		err = m.manager.Neg(&m.stageWork.Data[0])
		if err = m.outerCalls.after(m.manager, before, err); err != nil {
			return err
		}
		v = &m.stageWork.Data[0]
	}
	if err = m.X.copySlot(&m.X.Data[j], v); err != nil {
		return err
	}
	sign := int32(1)
	if c.Relation == RelationGreaterOrEqual {
		sign = -1
	}
	before, e := m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.SetInt32(&m.stageWork.Data[1], sign)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	if err = m.A.Set(row, j, &m.stageWork.Data[1]); err != nil {
		return err
	}
	if uint64(j) >= uint64(len(m.ColumnTypes)) {
		return ErrBounds
	}
	if c.Relation == RelationEqual {
		m.ColumnTypes[j] = ColumnFixed
	} else {
		m.ColumnTypes[j] = ColumnLowBound
	}
	zero := numeral.NewRational()
	if c.Relation != RelationGreaterOrEqual {
		if uint64(j) >= uint64(len(m.LowBounds.Data)) {
			return ErrBounds
		}
		if err = m.LowBounds.copySlot(&m.LowBounds.Data[j], &zero); err != nil {
			return err
		}
	}
	if uint64(j) >= uint64(len(m.UpperBounds.Data)) {
		return ErrBounds
	}
	if err = m.UpperBounds.copySlot(&m.UpperBounds.Data[j], &zero); err != nil {
		return err
	}
	return m.finishStageWork()
}
func (m *RationalPrimalSimplex) FillAXAndBasisForStageOneTotalInf() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	for row := uint32(0); row < m.A.RowCount(); row++ {
		if err = m.FillAXAndBasisForStageOneTotalInfForRow(row); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalPrimalSimplex) SetScaledCosts() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	for j := uint32(len(m.CoreColumnsToExternal)); j > 0; {
		j--
		if err = m.SetScaledCost(j); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalPrimalSimplex) PrepareStageOneTotalInf() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.A == nil {
		return ErrBounds
	}
	total := m.A.ColumnCount() + m.A.RowCount()
	if int32(total) < 0 {
		return ErrBounds
	}
	if total == 0 {
		m.Status = StatusOptimal
		return nil
	}
	if err = m.LowBounds.resizeData(0); err != nil {
		return err
	}
	if err = m.LowBounds.resizeData(total); err != nil {
		return err
	}
	if err = m.X.resizeData(total); err != nil {
		return err
	}
	rows := int(m.A.RowCount())
	if len(m.Basis) > rows {
		m.Basis = m.Basis[:rows]
	}
	for len(m.Basis) < rows {
		m.Basis = append(m.Basis, 0)
	}
	if err = m.Costs.resizeData(0); err != nil {
		return err
	}
	if err = m.Costs.resizeData(total); err != nil {
		return err
	}
	if err = m.FillAXAndBasisForStageOneTotalInf(); err != nil {
		return err
	}
	if m.Settings == nil {
		return ErrBounds
	}
	if m.Settings.MessageOut != nil {
		if err = writeDriverLine(m.Settings.MessageOut, fmt.Sprintf("extended A[%d,%d]", m.A.RowCount(), m.A.ColumnCount())); err != nil {
			return err
		}
	}
	return m.SetScaledCosts()
}
