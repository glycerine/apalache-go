package lp

// Source: src/util/lp/lp_primal_simplex.hpp:169-187,227-271,360-367;
// lp_primal_simplex.h:86-88; lp_solver.hpp:43-48.

import "sort"

func (m *FloatPrimalSimplex) FindMaximalSolution() error {
	if m.ProblemIsEmpty() {
		m.Status = StatusEmpty
		return nil
	}
	if err := m.Cleanup(); err != nil {
		return err
	}
	if err := m.FillMatrixAAndInitRightSide(); err != nil {
		return err
	}
	if m.Status == StatusInfeasible {
		return nil
	}
	m.X = resizeFloatSolverValues(m.X, int(m.A.ColumnCount()))
	if err := m.FillB(); err != nil {
		return err
	}
	if err := m.Scale(); err != nil {
		return err
	}
	if err := m.FillAcceptableValuesForX(); err != nil {
		return err
	}
	if err := m.CountSlacksAndArtificials(); err != nil {
		return err
	}
	if err := m.SetCoreSolverBounds(); err != nil {
		return err
	}
	return m.SolveWithTotalInf()
}

func (m *FloatPrimalSimplex) SolveWithTotalInf() error {
	if m.A == nil {
		return ErrBounds
	}
	if err := m.PrepareStageOneTotalInf(); err != nil {
		return err
	}
	if m.A.ColumnCount()+m.A.RowCount() == 0 {
		return nil
	}
	core, err := NewFloatPrimalCoreSolver(FloatCoreInputs{
		A: m.A, B: &m.B, X: &m.X, Costs: &m.Costs, Basis: &m.Basis, NBasis: &m.NBasis, Heading: &m.Heading,
		Settings: m.Settings, Names: m, ColumnTypes: &m.ColumnTypes, LowBounds: &m.LowBounds, UpperBounds: &m.UpperBounds,
	})
	// Publication occurs after construction, as for the source new-expression.
	// A partial Go constructor owner is returned for explicit cleanup instead
	// of publishing it as the source successfully constructed core.
	if err != nil {
		if core != nil {
			core.Close()
		}
		return err
	}
	m.CoreSolver = core
	if _, err := core.Solve(); err != nil {
		return err
	}
	m.Status = core.Status
	m.TotalIterations = core.TotalIterations
	return nil
}

func (m *FloatPrimalSimplex) GetColumnValue(column uint32) (float64, error) {
	var x []float64
	if m.CoreSolver != nil {
		x = *m.CoreSolver.X
	}
	return m.ColumnValueFromCore(column, x)
}

// GetCurrentCost reconstructs the original objective, including shifts,
// upper-bound flips, scaling and fixed columns. Native external indices choose
// a deterministic order for the source unordered-map sum.
func (m *FloatPrimalSimplex) GetCurrentCost() (float64, error) {
	keys := make([]uint32, 0, len(m.Columns))
	for j := range m.Columns {
		keys = append(keys, j)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	cost := 0.0
	for _, j := range keys {
		c := m.Columns[j]
		if c == nil {
			return cost, ErrBounds
		}
		value, err := m.GetColumnValue(j)
		if err != nil {
			return cost, err
		}
		cost += c.Cost * value
	}
	return cost, nil
}

func (m *FloatPrimalSimplex) Close() error {
	if m.CoreSolver == nil {
		return nil
	}
	err := m.CoreSolver.Close()
	if err == nil {
		m.CoreSolver = nil
	}
	return err
}
