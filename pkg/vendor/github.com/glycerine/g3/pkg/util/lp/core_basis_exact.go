package lp

// Source: src/util/lp/lp_core_solver_base.h:77-99,475-514,665-668;
// src/util/lp/lp_core_solver_base.hpp:937-940; retained mpq/impq core.

func (m *PairCoreSolverBase) StartTracingBasisChanges() { m.Trace = m.Trace[:0]; m.Tracing = true }
func (m *PairCoreSolverBase) StopTracingBasisChanges()  { m.Tracing = false }
func (m *PairCoreSolverBase) TraceBasisChange(entering, leaving uint32) {
	n := len(m.Trace)
	if n >= 2 && m.Trace[n-2] == leaving && m.Trace[n-1] == entering {
		m.Trace = m.Trace[:n-2]
	} else {
		m.Trace = append(m.Trace, entering, leaving)
	}
}

func (m *PairCoreSolverBase) ChangeBasis(entering, leaving uint32) error {
	return m.changeBasis(entering, leaving, false)
}
func (m *PairCoreSolverBase) ChangeBasisUnconditionally(entering, leaving uint32) error {
	return m.changeBasis(entering, leaving, true)
}
func (m *PairCoreSolverBase) changeBasis(entering, leaving uint32, unconditional bool) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
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

// TransposeBasis swaps row positions without changing column values or traces.
func (m *PairCoreSolverBase) TransposeBasis(i, j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	b := *m.Basis
	if uint64(i) >= uint64(len(b)) || uint64(j) >= uint64(len(b)) {
		return ErrBounds
	}
	b[i], b[j] = b[j], b[i]
	h := *m.Heading
	if uint64(b[i]) >= uint64(len(h)) || uint64(b[j]) >= uint64(len(h)) {
		return ErrBounds
	}
	h[b[i]], h[b[j]] = h[b[j]], h[b[i]]
	return nil
}

// TransposeRowsTableau publishes the basis swap before copying matrix rows.
// The matrix retains its own stopped copy and owns its numerical cleanup.
func (m *PairCoreSolverBase) TransposeRowsTableau(i, j uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.TransposeBasis(i, j); err != nil {
		return err
	}
	return m.A.TransposeRows(i, j)
}
