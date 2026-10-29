package lp

// Source: src/util/lp/lp_dual_core_solver.h:32-105;
// lp_dual_core_solver.hpp:26-45,67-111. Concrete retained mpq/mpq core.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalDualCoreSolver borrows the model and eligibility flags. Numerical
// scratch belongs to this owner. Constructor errors return a partial owner.
type RationalDualCoreSolver struct {
	*RationalCoreSolverBase
	CanEnterBasis                                        *[]bool
	AWave, Betas                                         *IndexedRationalVector
	HarrisTolerance, Delta, ThetaD, ThetaP               numeral.Rational
	R, P, Q                                              int32
	SignOfAlphaR                                         int
	BreakpointSet, FlippedBoxed, TightSet, ForbiddenRows map[uint32]bool
	EnteringBoundaryPosition                             NonBasicColumnValuePosition
	calls                                                exactEtaCalls
	stopped                                              bool
	priceWork                                            *IndexedRationalVector
	// Pricing locals stay alive through calls to the child edge expression.
	pricingValue *IndexedRationalVector
	// Candidate-selection locals remain live through numerical child calls.
	harrisValues, tightValue, qValues, harrisDelta  *IndexedRationalVector
	flipLossValues, flipSpan, ratioLoss, ratioTrial *IndexedRationalVector
	signedSpan, processSpan                         *IndexedRationalVector
	betaVector                                      *IndexedRationalVector
	diagnosticValues                                *IndexedRationalVector
	loopValues                                      *IndexedRationalVector
}

func NewRationalDualCoreSolver(in RationalCoreInputs, canEnter *[]bool) (m *RationalDualCoreSolver, err error) {
	if canEnter == nil {
		return nil, ErrBounds
	}
	base, err := NewRationalCoreSolverBase(in)
	if base == nil {
		return nil, err
	}
	m = &RationalDualCoreSolver{RationalCoreSolverBase: base, CanEnterBasis: canEnter,
		HarrisTolerance: numeral.NewRational(), Delta: numeral.NewRational(), ThetaD: numeral.NewRational(), ThetaP: numeral.NewRational()}
	defer func() { m.stopped = err != nil }()
	if err != nil {
		return m, err
	}
	m.AWave = NewIndexedRationalVector(m.manager, in.A.RowCount())
	m.Betas = NewIndexedRationalVector(m.manager, in.A.RowCount())
	before, err := m.calls.begin(m.manager)
	if err != nil {
		return m, err
	}
	err = m.manager.SetInt32(&m.HarrisTolerance, 0)
	if err = m.calls.after(m.manager, before, err); err != nil {
		return m, err
	}
	if err = m.SolveYB(m.Y); err != nil {
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
func (m *RationalDualCoreSolver) ready() error {
	if m.stopped || m.priceWork != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return m.RationalCoreSolverBase.ready()
}
func (m *RationalDualCoreSolver) FillNonBasisWithOnlyAbleToEnterColumns() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
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

// RestoreNonBasis preserves excluded negative headings. The source uniqueness
// assertion is guaranteed by this descending, once-per-column traversal.
func (m *RationalDualCoreSolver) RestoreNonBasis() error {
	return m.FillNonBasisWithOnlyAbleToEnterColumns()
}
func (m *RationalDualCoreSolver) InitAWaveByZeros() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for i := m.A.RowCount(); i > 0; {
		i--
		if m.AWave == nil || uint64(i) >= uint64(len(m.AWave.Data)) {
			return ErrBounds
		}
		if err = m.manager.SetInt32(&m.AWave.Data[i], 0); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualCoreSolver) InitBetas() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	before, e := m.calls.begin(m.manager)
	if e != nil {
		return e
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for i := m.A.RowCount(); i > 0; {
		i--
		if m.Betas == nil || uint64(i) >= uint64(len(m.Betas.Data)) {
			return ErrBounds
		}
		if err = m.manager.SetInt32(&m.Betas.Data[i], 1); err != nil {
			return err
		}
	}
	return nil
}
func (m *RationalDualCoreSolver) RecalculateD() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.SolveYB(m.Y); err != nil {
		return err
	}
	return m.FillReducedCostsFromYByRows()
}
func (m *RationalDualCoreSolver) RecalculateXBAndD() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.SolveAxEqualsB(); err != nil {
		return err
	}
	return m.RecalculateD()
}
func (m *RationalDualCoreSolver) AdjustXBForChangedXNAndInitBetas() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.SolveAxEqualsB(); err != nil {
		return err
	}
	return m.InitBetas()
}
func (m *RationalDualCoreSolver) Done() (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
	if m.Status == StatusOptimal {
		return true, nil
	}
	if m.TotalIterations > m.Settings.MaxTotalIterations {
		m.Status = StatusIterationsExhausted
		return true, nil
	}
	return false, nil
}
func (m *RationalDualCoreSolver) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if err := m.RationalCoreSolverBase.Close(); err != nil {
		return err
	}
	for _, v := range []*IndexedRationalVector{m.AWave, m.Betas, m.priceWork, m.pricingValue, m.harrisValues, m.tightValue, m.qValues, m.harrisDelta, m.flipLossValues, m.flipSpan, m.ratioLoss, m.ratioTrial, m.signedSpan, m.processSpan, m.betaVector, m.diagnosticValues, m.loopValues} {
		if v != nil {
			if err := v.Close(); err != nil {
				return err
			}
		}
	}
	m.AWave, m.Betas = nil, nil
	m.priceWork, m.pricingValue = nil, nil
	m.harrisValues, m.tightValue, m.qValues, m.harrisDelta = nil, nil, nil, nil
	m.flipLossValues, m.flipSpan, m.ratioLoss, m.ratioTrial = nil, nil, nil, nil
	m.signedSpan, m.processSpan = nil, nil
	m.betaVector = nil
	m.diagnosticValues = nil
	m.loopValues = nil
	for _, v := range []*numeral.Rational{&m.HarrisTolerance, &m.Delta, &m.ThetaD, &m.ThetaP} {
		if err := m.manager.Del(v); err != nil {
			return err
		}
	}
	return m.calls.cleanupRoots(m.manager)
}

// StartWithInitialBasisAndMakeItDualFeasible follows source initialization;
// feasibility is not an additional precondition on the incoming X values.
func (m *RationalDualCoreSolver) StartWithInitialBasisAndMakeItDualFeasible() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err = m.SetNonBasicXToCorrectBounds(); err != nil {
		return err
	}
	return m.AdjustXBForChangedXNAndInitBetas()
}
