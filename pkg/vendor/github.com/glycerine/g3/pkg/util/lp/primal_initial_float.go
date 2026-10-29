package lp

// Source: src/util/lp/lp_primal_core_solver.h:937-999.

import "fmt"

// NewFloatPrimalCoreSolverWithZeroLowBounds ports the constructor without a
// lower-bound argument. LowBounds in in is ignored; the core borrows its own
// initially empty lower-bound header, resized only after the debug X check.
// Pricing requires InitRun: this source constructor leaves pricing thresholds
// indeterminate. The default floating/tableau source path still lacks an LU.
func NewFloatPrimalCoreSolverWithZeroLowBounds(in FloatCoreInputs) (*FloatPrimalCoreSolver, error) {
	m := &FloatPrimalCoreSolver{}
	in.LowBounds = &m.lowBoundsDummy
	b, err := NewFloatCoreSolverBase(in)
	if b == nil {
		return nil, err
	}
	m.FloatCoreSolverBase = b
	if err != nil {
		return m, err
	}
	m.Beta = NewIndexedFloatVector(in.A.RowCount())
	m.BreakpointQueue = NewFloatPriorityQueue(0)
	m.ConvertedHarrisEpsilon = in.Settings.HarrisFeasibilityTolerance
	if sourceDebugEnabled {
		ok, err := m.InitialXIsCorrect()
		if err != nil {
			return m, err
		}
		if !ok {
			return m, ErrAssertion
		}
	}
	m.lowBoundsDummy = make([]float64, in.A.ColumnCount())
	// m_enter_price_eps has no retained source read; no unused field is copied.
	return m, nil
}

// InitialXIsCorrect checks zero lower bounds with exact comparisons. Basic
// values need only satisfy bounds; nonbasic low/boxed values must be at an
// endpoint. Free and upper-only nonbasic values have no endpoint requirement.
func (m *FloatPrimalCoreSolver) InitialXIsCorrect() (bool, error) {
	basis := make(map[uint32]bool)
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return false, ErrBounds
		}
		basis[(*m.Basis)[i]] = true
	}
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(*m.ColumnTypes)) {
			return false, ErrBounds
		}
		typ := (*m.ColumnTypes)[j]
		if typ != ColumnFree && typ != ColumnUpperBound {
			if uint64(j) >= uint64(len(*m.X)) {
				return false, ErrBounds
			}
			if (*m.X)[j] < 0 {
				if out := m.Settings.DebugOut; out != nil {
					if _, err := fmt.Fprintf(out, "low bound for variable %d does not hold: this->m_x[%d] = %s is negative \n", j, j, floatLPNumberText((*m.X)[j])); err != nil {
						return false, err
					}
				}
				return false, nil
			}
		}
		if typ != ColumnFree && typ != ColumnLowBound {
			if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.UpperBounds)) {
				return false, ErrBounds
			}
			if (*m.X)[j] > (*m.UpperBounds)[j] {
				if out := m.Settings.DebugOut; out != nil {
					if _, err := fmt.Fprintf(out, "upper bound for %d does not hold: %s>%s\n", j, floatLPNumberText((*m.UpperBounds)[j]), floatLPNumberText((*m.X)[j])); err != nil {
						return false, err
					}
				}
				return false, nil
			}
		}
		if basis[j] {
			continue
		}
		if typ == ColumnLowBound && (*m.X)[j] != 0 {
			if out := m.Settings.DebugOut; out != nil {
				if _, err := fmt.Fprintf(out, "only low bound is set for %d but low bound value 0 is not equal to %s\n", j, floatLPNumberText((*m.X)[j])); err != nil {
					return false, err
				}
			}
			return false, nil
		}
		if typ == ColumnBoxed && (*m.UpperBounds)[j] != (*m.X)[j] && (*m.X)[j] != 0 {
			return false, nil
		}
	}
	return true, nil
}
