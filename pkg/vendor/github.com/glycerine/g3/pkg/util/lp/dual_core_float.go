package lp

// Source: src/util/lp/lp_dual_core_solver.h:32-105;
// lp_dual_core_solver.hpp:26-45,67-88. Retained floating dual construction.

type FloatDualCoreSolver struct {
	*FloatCoreSolverBase
	CanEnterBasis                         *[]bool
	AWave, Betas                          []float64
	HarrisTolerance                       float64
	R, P, Q                               int32
	SignOfAlphaR                          int
	Delta, ThetaD, ThetaP                 float64
	BreakpointSet, FlippedBoxed, TightSet map[uint32]bool
	EnteringBoundaryPosition              NonBasicColumnValuePosition
	ForbiddenRows                         map[uint32]bool
}

func NewFloatDualCoreSolver(in FloatCoreInputs, canEnter *[]bool) (*FloatDualCoreSolver, error) {
	if canEnter == nil {
		return nil, ErrBounds
	}
	b, err := NewFloatCoreSolverBase(in)
	if b == nil {
		return nil, err
	}
	m := &FloatDualCoreSolver{FloatCoreSolverBase: b, CanEnterBasis: canEnter}
	if err != nil {
		return m, err
	}
	m.AWave = make([]float64, in.A.RowCount())
	m.Betas = make([]float64, in.A.RowCount())
	m.HarrisTolerance = in.Settings.HarrisFeasibilityTolerance
	if err := m.SolveYB(m.Y); err != nil {
		return m, err
	}
	for i, j := range *m.Basis {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return m, ErrBounds
		}
		(*m.Heading)[j] = int32(i)
	}
	return m, m.FillNonBasisWithOnlyAbleToEnterColumns()
}
func (m *FloatDualCoreSolver) FillNonBasisWithOnlyAbleToEnterColumns() error {
	*m.NBasis = (*m.NBasis)[:0]
	for j := m.A.ColumnCount(); j > 0; {
		j--
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] >= 0 {
			continue
		}
		if uint64(j) >= uint64(len(*m.CanEnterBasis)) {
			return ErrBounds
		}
		if !(*m.CanEnterBasis)[j] {
			continue
		}
		*m.NBasis = append(*m.NBasis, j)
		(*m.Heading)[j] = -int32(len(*m.NBasis))
	}
	return nil
}
func (m *FloatDualCoreSolver) RestoreNonBasis() error {
	return m.FillNonBasisWithOnlyAbleToEnterColumns()
}
func (m *FloatDualCoreSolver) InitAWaveByZeros() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.AWave)) {
			return ErrBounds
		}
		m.AWave[i] = 0
	}
	return nil
}
func (m *FloatDualCoreSolver) InitBetas() error {
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(m.Betas)) {
			return ErrBounds
		}
		m.Betas[i] = 1
	}
	return nil
}
func (m *FloatDualCoreSolver) RecalculateD() error {
	if err := m.SolveYB(m.Y); err != nil {
		return err
	}
	return m.FillReducedCostsFromYByRows()
}
func (m *FloatDualCoreSolver) RecalculateXBAndD() error {
	if err := m.SolveAxEqualsB(); err != nil {
		return err
	}
	return m.RecalculateD()
}
func (m *FloatDualCoreSolver) AdjustXBForChangedXNAndInitBetas() error {
	if err := m.SolveAxEqualsB(); err != nil {
		return err
	}
	return m.InitBetas()
}
