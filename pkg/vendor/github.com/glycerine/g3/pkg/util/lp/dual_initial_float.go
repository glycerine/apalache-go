package lp

// Source: src/util/lp/lp_dual_core_solver.hpp:90-166.

func (m *FloatDualCoreSolver) StartWithInitialBasisAndMakeItDualFeasible() error {
	if err := m.SetNonBasicXToCorrectBounds(); err != nil {
		return err
	}
	return m.AdjustXBForChangedXNAndInitBetas()
}

func (m *FloatDualCoreSolver) Done() bool {
	if m.Status == StatusOptimal {
		return true
	}
	if m.TotalIterations > m.Settings.MaxTotalIterations {
		m.Status = StatusIterationsExhausted
		return true
	}
	return false
}

func (m *FloatDualCoreSolver) edgeSteepness(p uint32, bounds *[]float64) (float64, error) {
	if uint64(p) >= uint64(len(*m.Heading)) {
		return 0, ErrBounds
	}
	r := (*m.Heading)[p]
	if sourceDebugEnabled && (r < 0 || uint32(r) >= m.A.RowCount()) {
		return 0, ErrAssertion
	}
	if uint64(p) >= uint64(len(*m.X)) || uint64(p) >= uint64(len(*bounds)) {
		return 0, ErrBounds
	}
	del := (*m.X)[p] - (*bounds)[p]
	del *= del
	if r < 0 || uint64(r) >= uint64(len(m.Betas)) {
		return 0, ErrBounds
	}
	return del / m.Betas[r], nil
}
func (m *FloatDualCoreSolver) GetEdgeSteepnessForLowBound(p uint32) (float64, error) {
	return m.edgeSteepness(p, m.LowBounds)
}
func (m *FloatDualCoreSolver) GetEdgeSteepnessForUpperBound(p uint32) (float64, error) {
	return m.edgeSteepness(p, m.UpperBounds)
}

func (m *FloatDualCoreSolver) PricingForRow(i uint32) (float64, error) {
	if uint64(i) >= uint64(len(*m.Basis)) {
		return 0, ErrBounds
	}
	p := (*m.Basis)[i]
	if uint64(p) >= uint64(len(*m.ColumnTypes)) {
		return 0, ErrBounds
	}
	typ := (*m.ColumnTypes)[p]
	if typ == ColumnFree {
		if sourceDebugEnabled {
			if uint64(p) >= uint64(len(m.D)) {
				return 0, ErrBounds
			}
			if m.D[p] != 0 {
				return 0, ErrAssertion
			}
		}
		return 0, nil
	}
	if typ != ColumnLowBound && typ != ColumnUpperBound && typ != ColumnBoxed && typ != ColumnFixed {
		return 0, sourceAssertionFailure()
	}
	if uint64(p) >= uint64(len(*m.X)) {
		return 0, ErrBounds
	}
	if typ != ColumnUpperBound {
		if uint64(p) >= uint64(len(*m.LowBounds)) {
			return 0, ErrBounds
		}
		if m.BelowBound((*m.X)[p], (*m.LowBounds)[p]) {
			return m.GetEdgeSteepnessForLowBound(p)
		}
	}
	if typ != ColumnLowBound {
		if uint64(p) >= uint64(len(*m.UpperBounds)) {
			return 0, ErrBounds
		}
		if m.AboveBound((*m.X)[p], (*m.UpperBounds)[p]) {
			return m.GetEdgeSteepnessForUpperBound(p)
		}
	}
	return 0, nil
}
