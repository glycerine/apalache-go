package lp

// Source: src/util/lp/lp_core_solver_base.h:77-99,475-514;
// lp_core_solver_base.hpp:123-132,154-163,768-787,888-891,916-918.

func (m *FloatCoreSolverBase) StartTracingBasisChanges() { m.Trace = m.Trace[:0]; m.Tracing = true }
func (m *FloatCoreSolverBase) StopTracingBasisChanges()  { m.Tracing = false }
func (m *FloatCoreSolverBase) TraceBasisChange(entering, leaving uint32) {
	n := len(m.Trace)
	if n >= 2 && m.Trace[n-2] == leaving && m.Trace[n-1] == entering {
		m.Trace = m.Trace[:n-2]
	} else {
		m.Trace = append(m.Trace, entering, leaving)
	}
}

func (m *FloatCoreSolverBase) ChangeBasis(entering, leaving uint32) error {
	return m.changeBasis(entering, leaving, false)
}
func (m *FloatCoreSolverBase) ChangeBasisUnconditionally(entering, leaving uint32) error {
	return m.changeBasis(entering, leaving, true)
}
func (m *FloatCoreSolverBase) changeBasis(entering, leaving uint32, unconditional bool) error {
	h := *m.Heading
	if uint64(entering) >= uint64(len(h)) {
		return ErrBounds
	}
	if sourceDebugEnabled && h[entering] >= 0 {
		return ErrAssertion
	}
	var nonBasic int32
	if unconditional {
		nonBasic = -1 - h[entering]
		if uint64(uint32(nonBasic)) >= uint64(len(*m.NBasis)) {
			nonBasic = int32(len(*m.NBasis))
			h[entering] = nonBasic
			*m.NBasis = append(*m.NBasis, entering)
		}
	}
	if uint64(leaving) >= uint64(len(h)) {
		return ErrBounds
	}
	basic := h[leaving]
	if !unconditional {
		// Source evaluates unary signed minus before subtracting one. Its
		// INT_MIN overflow has an explicit Go domain guard.
		if h[entering] == -1<<31 {
			return ErrBounds
		}
		nonBasic = -h[entering] - 1
	}
	h[entering] = basic
	if basic < 0 || uint64(basic) >= uint64(len(*m.Basis)) {
		return ErrBounds
	}
	(*m.Basis)[basic] = entering
	h[leaving] = -nonBasic - 1
	if nonBasic < 0 || uint64(nonBasic) >= uint64(len(*m.NBasis)) {
		return ErrBounds
	}
	(*m.NBasis)[nonBasic] = leaving
	if m.Tracing {
		m.TraceBasisChange(entering, leaving)
	}
	return nil
}

func (m *FloatCoreSolverBase) RestoreBasisChange(entering, leaving uint32) error {
	if uint64(entering) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	if (*m.Heading)[entering] < 0 {
		return nil
	}
	return m.ChangeBasisUnconditionally(leaving, entering)
}

// FillCB publishes costs in increasing basis order, including a partial prefix
// if a later caller vector access is outside the supported Go domain.
func (m *FloatCoreSolverBase) FillCB(y []float64) error {
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.Costs)) || uint64(i) >= uint64(len(y)) {
			return ErrBounds
		}
		y[i] = (*m.Costs)[j]
	}
	return nil
}

func (m *FloatCoreSolverBase) SolveYB(y []float64) error {
	if err := m.FillCB(y); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	return m.Factorization.SolveYBWithErrorCheck(y, *m.Basis)
}

func (m *FloatCoreSolverBase) FillReducedCostsFromYByRows() error {
	for j := m.A.ColumnCount(); j > 0; {
		j--
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] < 0 {
			if uint64(j) >= uint64(len(*m.Costs)) || uint64(j) >= uint64(len(m.D)) {
				return ErrBounds
			}
			m.D[j] = (*m.Costs)[j]
		} else {
			if uint64(j) >= uint64(len(m.D)) {
				return ErrBounds
			}
			m.D[j] = 0
		}
	}
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.Y)) {
			return ErrBounds
		}
		y := m.Y[i]
		if y == 0 {
			continue
		}
		for _, c := range m.A.Rows[i] {
			j := c.Column
			if uint64(j) >= uint64(len(*m.Heading)) {
				return ErrBounds
			}
			if (*m.Heading)[j] < 0 {
				if uint64(j) >= uint64(len(m.D)) {
					return ErrBounds
				}
				m.D[j] -= y * c.Value
			}
		}
	}
	return nil
}

func (m *FloatCoreSolverBase) InitReducedCostsForOneIteration() error {
	if err := m.SolveYB(m.Y); err != nil {
		return err
	}
	return m.FillReducedCostsFromYByRows()
}

func (m *FloatCoreSolverBase) InitLU() error {
	return InitFloatFactorization(&m.Factorization, m.A, *m.Basis, m.Settings)
}

func (m *FloatCoreSolverBase) SolveBd(entering uint32) error {
	if sourceDebugEnabled && !m.Ed.IsOK() {
		return ErrAssertion
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	if err := m.Factorization.SolveBd(entering, m.Ed, m.W); err != nil {
		return err
	}
	// The precise-only column count update is erased for double/double.
	if sourceDebugEnabled && (!m.Ed.IsOK() || !m.W.IsOK()) {
		return ErrAssertion
	}
	return nil
}
