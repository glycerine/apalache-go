package lp

// Source: src/util/lp/lp_dual_simplex.hpp:377-384; lp_dual_simplex.h:34-38,99-103; lp_solver.hpp:43-48.

import "github.com/glycerine/g3/pkg/util/numeral"

// columnCost owns the getter and product-expression locals. The returned
// product belongs to the caller and survives only through its compound sum.
func (m *RationalDualSimplex) columnCost(j uint32, c *RationalColumnInfo, out *numeral.Rational) (err error) {
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
func (m *RationalDualSimplex) GetCurrentCost(out *numeral.Rational) (err error) {
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
	// Source return -ret copies the accumulator before unary negation.
	before, e := m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.Copy(product, sum)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	before, e = m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.Neg(product)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	before, e = m.outerCalls.begin(m.manager)
	if e != nil {
		return e
	}
	err = m.manager.Copy(out, product)
	if err = m.outerCalls.after(m.manager, before, err); err != nil {
		return err
	}
	return m.finishStageWork()
}
func (m *RationalDualSimplex) Close() error {
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
	for _, v := range []*IndexedRationalVector{m.X, m.BCopy, m.LowBounds, m.UpperBounds, m.columnWork, m.stageWork} {
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
