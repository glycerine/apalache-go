package lp

// Source: src/util/lp/lp_dual_simplex.hpp:23-43,105-184,342-375.
// Concrete retained mpq/mpq two-stage outer solver.

import (
	"fmt"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func (m *RationalDualSimplex) coreCostIsZero() (small bool, err error) {
	if m.CoreSolver == nil {
		return false, ErrBounds
	}
	m.stageWork = NewIndexedRationalVector(m.manager, 1)
	if err = m.CoreSolver.GetCost(&m.stageWork.Data[0]); err != nil {
		return false, err
	}
	small, err = m.manager.IsZero(&m.stageWork.Data[0])
	if err != nil {
		return false, err
	}
	err = m.finishStageWork()
	return small, err
}
func (m *RationalDualSimplex) DecideOnStatusAfterStage1() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.CoreSolver == nil {
		return ErrBounds
	}
	switch m.CoreSolver.Status {
	case StatusOptimal:
		small, e := m.coreCostIsZero()
		if e != nil {
			return e
		}
		if small {
			m.Status = StatusFeasible
		} else {
			m.Status = StatusUnbounded
		}
	case StatusDualUnbounded:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
		fallthrough
	case StatusIterationsExhausted:
		m.Status = StatusIterationsExhausted
	case StatusTimeExhausted:
		m.Status = StatusTimeExhausted
	case StatusFloatingPointError:
		m.Status = StatusFloatingPointError
	default:
		return sourceAssertionFailure()
	}
	return nil
}
func (m *RationalDualSimplex) Stage1() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if sourceDebugEnabled && m.CoreSolver != nil {
		return ErrAssertion
	}
	if m.A == nil || m.Settings == nil {
		return ErrBounds
	}
	if err = m.X.resizeData(m.A.ColumnCount()); err != nil {
		return err
	}
	if m.Settings.MessageOut != nil {
		if err = writeDriverLine(m.Settings.MessageOut, fmt.Sprintf("extended A[%d,%d]", m.A.RowCount(), m.A.ColumnCount())); err != nil {
			return err
		}
	}
	core, e := NewRationalDualCoreSolver(RationalCoreInputs{A: m.A, B: &m.B.Data, X: &m.X.Data, Costs: &m.Costs.Data, Basis: &m.Basis, NBasis: &m.NBasis, Heading: &m.Heading, Settings: m.Settings, Names: m, ColumnTypes: &m.CoreColumnTypes, LowBounds: &m.LowBounds.Data, UpperBounds: &m.UpperBounds.Data}, &m.CanEnterBasis)
	if e != nil {
		m.partialCore = core
		return e
	}
	if m.CoreSolver != nil {
		m.previousCores = append(m.previousCores, m.CoreSolver)
	}
	m.CoreSolver = core
	if err = core.FillReducedCostsFromYByRows(); err != nil {
		return err
	}
	if err = core.StartWithInitialBasisAndMakeItDualFeasible(); err != nil {
		return err
	}
	small, e := m.coreCostIsZero()
	if e != nil {
		return e
	}
	if small {
		core.Status = StatusOptimal
		core.TotalIterations = 0
	} else {
		if _, err = core.Solve(); err != nil {
			return err
		}
	}
	if err = m.DecideOnStatusAfterStage1(); err != nil {
		return err
	}
	m.FirstStageIterations = core.TotalIterations
	return nil
}
func (m *RationalDualSimplex) SolveForStage2() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	core := m.CoreSolver
	if core == nil {
		return ErrBounds
	}
	if err = core.RestoreNonBasis(); err != nil {
		return err
	}
	if err = core.SolveYB(core.Y); err != nil {
		return err
	}
	if err = core.FillReducedCostsFromYByRows(); err != nil {
		return err
	}
	if err = core.StartWithInitialBasisAndMakeItDualFeasible(); err != nil {
		return err
	}
	core.Status = StatusFeasible
	if _, err = core.Solve(); err != nil {
		return err
	}
	switch core.Status {
	case StatusOptimal:
		m.Status = StatusOptimal
	case StatusDualUnbounded:
		m.Status = StatusInfeasible
	case StatusTimeExhausted:
		m.Status = StatusTimeExhausted
	case StatusFloatingPointError:
		m.Status = StatusFloatingPointError
	default:
		if err = sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.SecondStageIterations = core.TotalIterations
	m.TotalIterations = m.FirstStageIterations + m.SecondStageIterations
	return nil
}
func (m *RationalDualSimplex) Stage2() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if err = m.UnmarkColumnsAndFixStructuralCosts(); err != nil {
		return err
	}
	if err = m.RestoreRightSides(); err != nil {
		return err
	}
	return m.SolveForStage2()
}
func (m *RationalDualSimplex) FindMaximalSolution() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.ProblemIsEmpty() {
		m.Status = StatusEmpty
		return nil
	}
	if err = m.FlipCosts(); err != nil {
		return err
	}
	if err = m.Cleanup(); err != nil {
		return err
	}
	if m.Status == StatusInfeasible {
		return nil
	}
	if err = m.FillMatrixAAndInitRightSide(); err != nil {
		return err
	}
	if err = m.FillB(); err != nil {
		return err
	}
	if err = m.Scale(); err != nil {
		return err
	}
	if err = m.AugmentMatrixAndAllocateFields(); err != nil {
		return err
	}
	if err = m.FillFirstStageSolverFields(); err != nil {
		return err
	}
	if err = m.CopyBAsideAndSetToZeros(); err != nil {
		return err
	}
	if err = m.Stage1(); err != nil {
		return err
	}
	if m.Status == StatusFeasible {
		return m.Stage2()
	}
	return nil
}
func (m *RationalDualSimplex) GetColumnValue(column uint32, out *numeral.Rational) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	var x []numeral.Rational
	if m.CoreSolver != nil {
		x = *m.CoreSolver.X
	}
	return m.ColumnValueFromCore(column, x, out)
}
