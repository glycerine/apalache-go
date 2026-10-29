package lp

// Source: src/util/lp/lp_primal_simplex.hpp:169-187,227-271,360-367;
// lp_primal_simplex.h:86-88; lp_solver.hpp:43-48. Retained mpq/mpq instance.

import "github.com/glycerine/g3/pkg/util/numeral"

func (m *RationalPrimalSimplex) FindMaximalSolution() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if m.ProblemIsEmpty() {
		m.Status = StatusEmpty
		return nil
	}
	if err = m.Cleanup(); err != nil {
		return err
	}
	if err = m.FillMatrixAAndInitRightSide(); err != nil {
		return err
	}
	if m.Status == StatusInfeasible {
		return nil
	}
	if err = m.X.resizeData(m.A.ColumnCount()); err != nil {
		return err
	}
	if err = m.FillB(); err != nil {
		return err
	}
	if err = m.Scale(); err != nil {
		return err
	}
	if err = m.FillAcceptableValuesForX(); err != nil {
		return err
	}
	if err = m.CountSlacksAndArtificials(); err != nil {
		return err
	}
	if err = m.SetCoreSolverBounds(); err != nil {
		return err
	}
	return m.SolveWithTotalInf()
}
func (m *RationalPrimalSimplex) SolveWithTotalInf() (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if err = m.PrepareStageOneTotalInf(); err != nil {
		return err
	}
	if m.A.ColumnCount()+m.A.RowCount() == 0 {
		return nil
	}
	core, e := NewRationalPrimalCoreSolver(RationalCoreInputs{A: m.A, B: &m.B.Data, X: &m.X.Data, Costs: &m.Costs.Data, Basis: &m.Basis, NBasis: &m.NBasis, Heading: &m.Heading, Settings: m.Settings, Names: m, ColumnTypes: &m.ColumnTypes, LowBounds: &m.LowBounds.Data, UpperBounds: &m.UpperBounds.Data})
	if e != nil {
		m.partialCore = core
		return e
	}
	if m.CoreSolver != nil {
		m.previousCores = append(m.previousCores, m.CoreSolver)
	}
	m.CoreSolver = core
	if _, err = core.Solve(); err != nil {
		return err
	}
	m.Status = core.Status
	m.TotalIterations = core.TotalIterations
	return nil
}
func (m *RationalPrimalSimplex) GetColumnValue(column uint32, out *numeral.Rational) (err error) {
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

// columnCost owns the getter and product-expression locals. The returned
// product belongs to the caller and survives only through its compound sum.
func (m *RationalPrimalSimplex) columnCost(j uint32, c *RationalColumnInfo, out *numeral.Rational) (err error) {
	if c == nil {
		return ErrBounds
	}
	m.columnWork = NewIndexedRationalVector(m.manager, 3)
	cost, value, tmp := &m.columnWork.Data[0], &m.columnWork.Data[1], &m.columnWork.Data[2]
	if err = c.GetCost(cost); err != nil {
		return err
	}
	if c.Fixed {
		err = c.GetFixedValue(value)
	} else {
		var x []numeral.Rational
		if m.CoreSolver != nil {
			x = *m.CoreSolver.X
		}
		err = m.ColumnValueFromCore(j, x, value)
	}
	if err != nil {
		return err
	}
	before, e := m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = etaRationalProduct(m.manager, tmp, out, cost, value)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	if err = m.columnWork.Close(); err != nil {
		return err
	}
	m.columnWork = nil
	return nil
}
func (m *RationalPrimalSimplex) GetCurrentCost(out *numeral.Rational) (err error) {
	if err = m.outerReady(); err != nil {
		return err
	}
	defer func() { m.outerStopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	// Accumulator is lexical: a caller output can alias a model cost or X slot.
	m.stageWork = NewIndexedRationalVector(m.manager, 2)
	sum, product := &m.stageWork.Data[0], &m.stageWork.Data[1]
	for j, c := range m.Columns.All() {
		if err = m.columnCost(j, c, product); err != nil {
			return err
		}
		before, e := m.outerCalls.begin(m.manager)
		if e != nil {
			return e
		}
		err = m.manager.Add(sum, product, sum)
		if err = m.outerCalls.after(m.manager, before, err); err != nil {
			return err
		}
		if err = m.manager.Del(product); err != nil {
			return err
		}
		*product = numeral.NewRational()
	}
	before, e := m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.Copy(out, sum)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	return m.finishStageWork()
}
func (m *RationalPrimalSimplex) Close() error {
	if m.outerClosed {
		return nil
	}
	if e := m.outerCalls.cleanupRaw(m.manager); e != nil {
		return e
	}
	if m.partialCore != nil {
		if e := m.partialCore.Close(); e != nil {
			return e
		}
		m.partialCore = nil
	}
	if m.CoreSolver != nil {
		if e := m.CoreSolver.Close(); e != nil {
			return e
		}
		m.CoreSolver = nil
	}
	for len(m.previousCores) > 0 {
		last := len(m.previousCores) - 1
		if e := m.previousCores[last].Close(); e != nil {
			return e
		}
		m.previousCores = m.previousCores[:last]
	}
	if e := m.RationalLPSolver.Close(); e != nil {
		return e
	}
	for _, v := range []*IndexedRationalVector{m.X, m.LowBounds, m.UpperBounds, m.columnWork, m.stageWork} {
		if v != nil {
			if e := v.Close(); e != nil {
				return e
			}
		}
	}
	m.stageWork, m.columnWork = nil, nil
	if e := m.outerCalls.cleanupRoots(m.manager); e != nil {
		return e
	}
	m.outerClosed = true
	return nil
}
