package lp

// Source: src/util/lp/lp_core_solver_base.hpp:768-788,888-891.

// FillReducedCostsFromYByRows initializes nonbasic costs, then subtracts each
// stored row contribution. Y and Costs remain live references throughout.
func (m *PairCoreSolverBase) FillReducedCostsFromYByRows() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	return m.fillReducedCostsFromYByRowsValue()
}
func (m *PairCoreSolverBase) fillReducedCostsFromYByRowsValue() (err error) {
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for j := m.A.ColumnCount(); j > 0; {
		j--
		if uint64(j) >= uint64(len(*m.Heading)) || uint64(j) >= uint64(len(m.D.Data)) {
			return ErrBounds
		}
		if (*m.Heading)[j] < 0 {
			if uint64(j) >= uint64(len(*m.Costs)) {
				return ErrBounds
			}
			if err = m.manager.Copy(&m.D.Data[j], &(*m.Costs)[j]); err != nil {
				return err
			}
		} else if err = m.manager.SetInt32(&m.D.Data[j], 0); err != nil {
			return err
		}
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.Y.Data)) {
			return ErrBounds
		}
		y := &m.Y.Data[i]
		zero, e := m.manager.IsZero(y)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		for k := range m.A.Rows[i] {
			c := &m.A.Rows[i][k]
			j := c.Column
			if uint64(j) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[j] >= 0 {
				continue
			}
			if uint64(j) >= uint64(len(m.D.Data)) {
				return ErrBounds
			}
			m.pivotWork = NewIndexedRationalVector(m.manager, 2)
			tmp, product := &m.pivotWork.Data[0], &m.pivotWork.Data[1]
			if err = etaRationalProduct(m.manager, tmp, product, y, &c.Value); err != nil {
				return err
			}
			if err = m.manager.Sub(&m.D.Data[j], product, &m.D.Data[j]); err != nil {
				return err
			}
			if err = m.pivotWork.Close(); err != nil {
				return err
			}
			m.pivotWork = nil
		}
	}
	return nil
}

// InitReducedCostsForOneIteration keeps LU failures with the factorization;
// row arithmetic starts only after the precise transpose solve succeeds.
func (m *PairCoreSolverBase) InitReducedCostsForOneIteration() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.SolveYB(m.Y); err != nil {
		return err
	}
	return m.fillReducedCostsFromYByRowsValue()
}
