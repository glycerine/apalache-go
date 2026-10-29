package lp

// Source: src/util/lp/lp_dual_simplex.hpp:23-43,105-135,145-184,342-384;
// lp_dual_simplex.h:40-44,99-103; lp_solver.h:253-255.

import (
	"fmt"
	"io"
	"sort"
)

func (m *FloatLPSolver) PrintStatisticsOnA(out io.Writer) error {
	if m.A == nil || out == nil {
		return ErrBounds
	}
	return writeDriverLine(out, fmt.Sprintf("extended A[%d,%d]", m.A.RowCount(), m.A.ColumnCount()))
}
func (m *FloatDualSimplex) DecideOnStatusAfterStage1() error {
	if m.CoreSolver == nil {
		return ErrBounds
	}
	switch m.CoreSolver.Status {
	case StatusOptimal:
		cost, err := m.CoreSolver.GetCost()
		if err != nil {
			return err
		}
		if m.Settings.FloatArtificialSmall(cost) {
			m.Status = StatusFeasible
		} else {
			m.Status = StatusUnbounded
		}
	case StatusDualUnbounded:
		if err := sourceAssertionFailure(); err != nil {
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
func (m *FloatDualSimplex) Stage1() error {
	if sourceDebugEnabled && m.CoreSolver != nil {
		return ErrAssertion
	}
	if m.A == nil {
		return ErrBounds
	}
	m.X = resizeFloatSolverValues(m.X, int(m.A.ColumnCount()))
	if m.Settings.MessageOut != nil {
		if err := m.PrintStatisticsOnA(m.Settings.MessageOut); err != nil {
			return err
		}
	}
	core, err := NewFloatDualCoreSolver(FloatCoreInputs{
		A: m.A, B: &m.B, X: &m.X, Costs: &m.Costs, Basis: &m.Basis, NBasis: &m.NBasis, Heading: &m.Heading,
		Settings: m.Settings, Names: m, ColumnTypes: &m.CoreColumnTypes, LowBounds: &m.LowBounds, UpperBounds: &m.UpperBounds,
	}, &m.CanEnterBasis)
	if err != nil {
		if core != nil {
			core.Close()
		}
		return err
	}
	m.CoreSolver = core
	if err = core.FillReducedCostsFromYByRows(); err != nil {
		return err
	}
	if err = core.StartWithInitialBasisAndMakeItDualFeasible(); err != nil {
		return err
	}
	cost, err := core.GetCost()
	if err != nil {
		return err
	}
	if m.Settings.FloatArtificialSmall(cost) {
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
func (m *FloatDualSimplex) SolveForStage2() error {
	core := m.CoreSolver
	if core == nil {
		return ErrBounds
	}
	if err := core.RestoreNonBasis(); err != nil {
		return err
	}
	if err := core.SolveYB(core.Y); err != nil {
		return err
	}
	if err := core.FillReducedCostsFromYByRows(); err != nil {
		return err
	}
	if err := core.StartWithInitialBasisAndMakeItDualFeasible(); err != nil {
		return err
	}
	core.Status = StatusFeasible
	if _, err := core.Solve(); err != nil {
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
		if err := sourceAssertionFailure(); err != nil {
			return err
		}
	}
	m.SecondStageIterations = core.TotalIterations
	m.TotalIterations = m.FirstStageIterations + m.SecondStageIterations
	return nil
}
func (m *FloatDualSimplex) Stage2() error {
	if err := m.UnmarkColumnsAndFixStructuralCosts(); err != nil {
		return err
	}
	if err := m.RestoreRightSides(); err != nil {
		return err
	}
	return m.SolveForStage2()
}
func (m *FloatDualSimplex) FindMaximalSolution() error {
	if m.ProblemIsEmpty() {
		m.Status = StatusEmpty
		return nil
	}
	if err := m.FlipCosts(); err != nil {
		return err
	}
	if err := m.Cleanup(); err != nil {
		return err
	}
	if m.Status == StatusInfeasible {
		return nil
	}
	if err := m.FillMatrixAAndInitRightSide(); err != nil {
		return err
	}
	if err := m.FillB(); err != nil {
		return err
	}
	if err := m.Scale(); err != nil {
		return err
	}
	if err := m.AugmentMatrixAndAllocateFields(); err != nil {
		return err
	}
	if err := m.FillFirstStageSolverFields(); err != nil {
		return err
	}
	m.CopyBAsideAndSetToZeros()
	if err := m.Stage1(); err != nil {
		return err
	}
	if m.Status == StatusFeasible {
		return m.Stage2()
	}
	return nil
}
func (m *FloatDualSimplex) GetColumnValue(column uint32) (float64, error) {
	var x []float64
	if m.CoreSolver != nil {
		x = *m.CoreSolver.X
	}
	return m.ColumnValueFromCore(column, x)
}
func (m *FloatDualSimplex) GetCurrentCost() (float64, error) {
	keys := make([]uint32, 0, len(m.Columns))
	for j := range m.Columns {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	ret := 0.0
	for _, j := range keys {
		ci := m.Columns[j]
		if ci == nil {
			return -ret, ErrBounds
		}
		value, err := m.GetColumnValue(j)
		if err != nil {
			return -ret, err
		}
		ret += ci.Cost * value
	}
	return -ret, nil
}
func (m *FloatDualSimplex) Close() error {
	if m.CoreSolver == nil {
		return nil
	}
	err := m.CoreSolver.Close()
	if err == nil {
		m.CoreSolver = nil
	}
	return err
}
