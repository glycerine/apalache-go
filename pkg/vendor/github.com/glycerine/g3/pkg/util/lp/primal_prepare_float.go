package lp

// Source: src/util/lp/lp_primal_simplex.hpp:51-56,63-68,146-165,189-250.

// FloatPrimalSimplex owns the floating core and borrows its prepared vectors
// into that core for optimization and external solution reconstruction.
type FloatPrimalSimplex struct {
	*FloatLPSolver
	LowBounds  []float64
	CoreSolver *FloatPrimalCoreSolver
}

func NewFloatPrimalSimplex() (*FloatPrimalSimplex, error) {
	m, err := NewFloatLPSolver()
	if err != nil {
		return nil, err
	}
	return &FloatPrimalSimplex{FloatLPSolver: m}, nil
}

func (m *FloatPrimalSimplex) FillAcceptableValuesForX() error {
	for j := range m.CoreColumnsToExternal {
		if uint64(j) >= uint64(len(m.X)) {
			return ErrBounds
		}
		m.X[j] = 0
	}
	return nil
}

func (m *FloatPrimalSimplex) SetCoreSolverBounds() error {
	// The source unsigned extent is assigned to an unsigned total_vars.
	total := m.A.ColumnCount() + m.Slacks + m.Artificials
	n := int(total)
	if len(m.ColumnTypes) > n {
		m.ColumnTypes = m.ColumnTypes[:n]
	}
	for len(m.ColumnTypes) < n {
		m.ColumnTypes = append(m.ColumnTypes, ColumnFree)
	}
	m.UpperBounds = resizeFloatSolverValues(m.UpperBounds, n)
	for _, c := range m.Columns {
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
		m.ColumnTypes[j] = c.ColumnType()
		switch m.ColumnTypes[j] {
		case ColumnFixed:
			m.UpperBounds[j] = 0
		case ColumnBoxed:
			u, err := c.AdjustedUpperBound()
			if err != nil {
				return err
			}
			if uint64(j) >= uint64(len(m.ColumnScale)) {
				return ErrBounds
			}
			m.UpperBounds[j] = u / m.ColumnScale[j]
		}
	}
	return nil
}

func resizeFloatSolverValues(v []float64, n int) []float64 {
	if len(v) > n {
		return v[:n]
	}
	return append(v, make([]float64, n-len(v))...)
}

func (m *FloatPrimalSimplex) FillAXAndBasisForStageOneTotalInfForRow(row uint32) error {
	if err := lpSolverDomain(row < m.A.RowCount()); err != nil {
		return err
	}
	ext, exists := m.CoreRowsToExternal[row]
	if err := lpSolverDomain(exists); err != nil {
		return err
	}
	c, exists := m.Constraints[ext]
	if err := lpSolverDomain(exists); err != nil {
		return err
	}
	j := m.A.ColumnCount()
	m.A.AddColumn()
	if uint64(row) >= uint64(len(m.Basis)) {
		return ErrBounds
	}
	m.Basis[row] = j
	switch c.Relation {
	case RelationEqual, RelationGreaterOrEqual, RelationLessOrEqual:
	default:
		return sourceAssertionFailure()
	}
	if uint64(row) >= uint64(len(m.B)) || uint64(j) >= uint64(len(m.X)) {
		return ErrBounds
	}
	v := m.B[row]
	sign := 1.
	if c.Relation == RelationGreaterOrEqual {
		v = -v
		sign = -1
	}
	m.X[j] = v
	if err := m.A.Set(row, j, sign); err != nil {
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
	// The chained source assignment stores the low bound before the upper.
	if c.Relation != RelationGreaterOrEqual {
		if uint64(j) >= uint64(len(m.LowBounds)) {
			return ErrBounds
		}
		m.LowBounds[j] = 0
	}
	if uint64(j) >= uint64(len(m.UpperBounds)) {
		return ErrBounds
	}
	m.UpperBounds[j] = 0
	return nil
}

func (m *FloatPrimalSimplex) FillAXAndBasisForStageOneTotalInf() error {
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if err := m.FillAXAndBasisForStageOneTotalInfForRow(i); err != nil {
			return err
		}
	}
	return nil
}

func (m *FloatPrimalSimplex) SetScaledCosts() error {
	for j := uint32(len(m.CoreColumnsToExternal)); j > 0; {
		j--
		if err := m.SetScaledCost(j); err != nil {
			return err
		}
	}
	return nil
}

// PrepareStageOneTotalInf executes solve_with_total_inf's preparation through
// set_scaled_costs. It constructs no core solver and performs no optimization.
func (m *FloatPrimalSimplex) PrepareStageOneTotalInf() error {
	total := m.A.ColumnCount() + m.A.RowCount()
	// Preserve the unsigned sum, then its signed Linux int conversion. A
	// negative resize extent gets the explicit Go domain guard.
	if int32(total) < 0 {
		return ErrBounds
	}
	if total == 0 {
		m.Status = StatusOptimal
		return nil
	}
	n := int(total)
	m.LowBounds = make([]float64, n)
	m.X = resizeFloatSolverValues(m.X, n)
	rows := int(m.A.RowCount())
	if len(m.Basis) > rows {
		m.Basis = m.Basis[:rows]
	}
	for len(m.Basis) < rows {
		m.Basis = append(m.Basis, 0)
	}
	m.Costs = make([]float64, n)
	if err := m.FillAXAndBasisForStageOneTotalInf(); err != nil {
		return err
	}
	if m.Settings.MessageOut != nil {
		if err := m.PrintStatisticsOnA(m.Settings.MessageOut); err != nil {
			return err
		}
	}
	return m.SetScaledCosts()
}
