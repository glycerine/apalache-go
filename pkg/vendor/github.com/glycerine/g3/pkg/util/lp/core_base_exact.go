package lp

// Source: src/util/lp/lp_core_solver_base.h/.hpp, retained mpq/impq core.

import "github.com/glycerine/g3/pkg/util/numeral"

// PairCoreInputs borrows the actual source vectors, including slice headers.
// Their owning storage and the matrix's numeral manager must outlive the core.
// In the retained LAR model the bound/type headers belong to scoped vectors.
type PairCoreInputs struct {
	A                            *RationalStaticMatrix
	B, X, LowBounds, UpperBounds *[]RationalPair
	Costs                        *[]numeral.Rational
	Basis, NBasis                *[]uint32
	Heading                      *[]int32
	ColumnTypes                  *[]ColumnType
	Settings                     *Settings
	Names                        ColumnNamer
}

// PairCoreSolverBase owns exact scratch and its concrete PairLU. Inputs are
// borrowed. This layer supplies construction, basis headings, precise bound
// checks and LU entry points. PairPrimalCoreSolver supplies precise Solve;
// exact outer preparation and the remaining solver variants remain open.
// Constructor errors return a partial owner to Close. Do not copy this owner.
type PairCoreSolverBase struct {
	PairCoreInputs
	Status                                  Status
	TotalIterations, IterationsNoCostGrowth uint32
	InfSet                                  *IntSet
	UsingInfeasCosts                        bool
	PivotRowB1, PivotRow, W, Ed             *IndexedRationalVector
	Y, D, ColumnNorms, SteepestEdge         *IndexedRationalVector
	CopyXB                                  *IndexedPairVector
	Factorization                           *PairLU
	BasisSortCounter                        uint32
	Trace                                   []uint32
	Tracing                                 bool
	PivotedRows                             *IntSet
	LookForFeasibleSolutionOnly             bool
	ColumnsNZ, RowsNZ                       []uint32
	manager                                 *numeral.RationalManager
	// Source statistics locals survive calls into this owner. They do not
	// block child ready checks; an interrupted operation stops the owner.
	statisticsCost, statisticsArgument *IndexedPairVector
	calls                              exactEtaCalls
	pivotWork                          *IndexedRationalVector
	feasibilityWork                    *IndexedPairVector
	valueWork                          *IndexedPairVector
	valueScalars                       *IndexedRationalVector
	residualWork                       *IndexedPairVector
	residualScalars                    *IndexedRationalVector
	reconstructionWork                 *IndexedPairVector
	stopped                            bool
}

func NewPairCoreSolverBase(in PairCoreInputs) (m *PairCoreSolverBase, err error) {
	if in.A == nil || in.B == nil || in.X == nil || in.Costs == nil || in.Basis == nil || in.NBasis == nil || in.Heading == nil || in.ColumnTypes == nil || in.LowBounds == nil || in.UpperBounds == nil || in.Settings == nil {
		return nil, ErrBounds
	}
	n, r := in.A.ColumnCount(), in.A.RowCount()
	set, err := NewIntSet(n)
	if err != nil {
		return nil, err
	}
	z := in.A.manager
	m = &PairCoreSolverBase{PairCoreInputs: in, Status: StatusFeasible, InfSet: set, manager: z,
		PivotRowB1: NewIndexedRationalVector(z, r), PivotRow: NewIndexedRationalVector(z, n),
		Y: NewIndexedRationalVector(z, r), W: NewIndexedRationalVector(z, r), D: NewIndexedRationalVector(z, n), Ed: NewIndexedRationalVector(z, r),
		ColumnNorms: NewIndexedRationalVector(z, n), CopyXB: NewIndexedPairVector(z, r), SteepestEdge: NewIndexedRationalVector(z, n)}
	defer func() { m.stopped = err != nil }()
	if sourceDebugEnabled {
		var ok bool
		ok, err = m.BoundsForBoxedAreSetCorrectly()
		if err != nil {
			return m, err
		}
		if !ok {
			return m, ErrAssertion
		}
	}
	if err = m.Init(); err != nil {
		return m, err
	}
	// The source repeats heading initialization after init/factorization.
	return m, m.InitBasisHeadingAndNonBasicColumns()
}
func (m *PairCoreSolverBase) ready() error {
	if m.stopped || m.pivotWork != nil || m.feasibilityWork != nil || m.valueWork != nil || m.valueScalars != nil || m.residualWork != nil || m.residualScalars != nil || m.reconstructionWork != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (m *PairCoreSolverBase) Init() error {
	if err := m.ready(); err != nil {
		return err
	}
	if err := m.AllocateBasisHeading(); err != nil {
		return err
	}
	if m.Settings.UseLU() {
		return InitPairFactorization(&m.Factorization, m.A, *m.Basis, m.Settings)
	}
	return nil
}
func (m *PairCoreSolverBase) AllocateBasisHeading() error {
	if err := m.InitBasisHeadingAndNonBasicColumns(); err != nil {
		return err
	}
	if sourceDebugEnabled {
		ok, err := m.BasisHeadingIsCorrect()
		if err != nil {
			return err
		}
		if !ok {
			return ErrAssertion
		}
	}
	return nil
}
func (m *PairCoreSolverBase) InitBasisHeadingAndNonBasicColumns() error {
	n := int(m.A.ColumnCount())
	h := (*m.Heading)[:0]
	if cap(h) >= n {
		h = h[:n]
	} else {
		h = make([]int32, n)
	}
	for j := range h {
		h[j] = -1
	}
	*m.Heading = h
	for i, j := range *m.Basis {
		if uint64(j) >= uint64(len(h)) {
			return ErrBounds
		}
		h[j] = int32(i)
	}
	*m.NBasis = (*m.NBasis)[:0]
	for j := len(h); j > 0; {
		j--
		if h[j] < 0 {
			*m.NBasis = append(*m.NBasis, uint32(j))
			h[j] = -int32(len(*m.NBasis))
		}
	}
	return nil
}
func (m *PairCoreSolverBase) BasisHeadingIsCorrect() (bool, error) {
	b, nb, h := *m.Basis, *m.NBasis, *m.Heading
	n, r := m.A.ColumnCount(), m.A.RowCount()
	if sourceDebugEnabled && (uint32(len(h)) != n || uint32(len(b)) != r || uint32(len(nb)) > n-r) {
		return false, ErrAssertion
	}
	seen := make(map[uint32]bool)
	for i := uint32(0); i < r; i++ {
		if uint64(i) >= uint64(len(b)) {
			return false, ErrBounds
		}
		seen[b[i]] = true
	}
	if uint32(len(seen)) != r {
		return false, nil
	}
	clear(seen)
	for _, j := range nb {
		seen[j] = true
	}
	if len(seen) != len(nb) {
		return false, nil
	}
	for i := uint32(0); i < r; i++ {
		if uint64(b[i]) >= uint64(len(h)) {
			return false, ErrBounds
		}
		if h[b[i]] != int32(i) {
			return false, nil
		}
	}
	for i, j := range nb {
		if uint64(j) >= uint64(len(h)) {
			return false, ErrBounds
		}
		if h[j] != -int32(i)-1 {
			return false, nil
		}
	}
	if sourceDebugEnabled {
		for j := uint32(0); j < n; j++ {
			if uint64(j) >= uint64(len(h)) {
				return false, ErrBounds
			}
			if h[j] >= 0 && (uint32(h[j]) >= r || b[h[j]] != j) {
				return false, ErrAssertion
			}
		}
	}
	return true, nil
}
func (m *PairCoreSolverBase) compare(x, bound *RationalPair) (c int, err error) {
	if err = m.ready(); err != nil {
		return
	}
	before, err := m.calls.begin(m.manager)
	if err != nil {
		return 0, err
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	return x.Compare(m.manager, bound)
}
func (m *PairCoreSolverBase) BelowBound(x, bound *RationalPair) (bool, error) {
	c, e := m.compare(x, bound)
	return c < 0, e
}
func (m *PairCoreSolverBase) AboveBound(x, bound *RationalPair) (bool, error) {
	c, e := m.compare(x, bound)
	return c > 0, e
}
func (m *PairCoreSolverBase) AtBound(x, bound *RationalPair) (bool, error) {
	below, err := m.BelowBound(x, bound)
	if err != nil || below {
		return false, err
	}
	above, err := m.AboveBound(x, bound)
	return !above, err
}
func (m *PairCoreSolverBase) BoundsForBoxedAreSetCorrectly() (bool, error) {
	for j, t := range *m.ColumnTypes {
		if t != ColumnBoxed {
			continue
		}
		if j >= len(*m.LowBounds) || j >= len(*m.UpperBounds) {
			return false, ErrBounds
		}
		above, err := m.AboveBound(&(*m.LowBounds)[j], &(*m.UpperBounds)[j])
		if err != nil || above {
			return false, err
		}
	}
	return true, nil
}
func (m *PairCoreSolverBase) ColumnIsFeasible(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	x := &(*m.X)[j]
	switch (*m.ColumnTypes)[j] {
	case ColumnFixed, ColumnBoxed, ColumnUpperBound:
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return false, ErrBounds
		}
		above, err := m.AboveBound(x, &(*m.UpperBounds)[j])
		if err != nil || above {
			return false, err
		}
		if (*m.ColumnTypes)[j] == ColumnUpperBound {
			return true, nil
		}
		fallthrough
	case ColumnLowBound:
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		below, err := m.BelowBound(x, &(*m.LowBounds)[j])
		return !below, err
	case ColumnFree:
		return true, nil
	default:
		return false, sourceAssertionFailure()
	}
}
func (m *PairCoreSolverBase) FillCB(out *IndexedRationalVector) (err error) {
	if out.calls.pending() {
		return ErrExactVectorPending
	}
	if out.manager != m.manager {
		return ErrExactVectorOwner
	}
	if err = m.ready(); err != nil {
		return err
	}
	before, err := m.calls.begin(m.manager)
	if err != nil {
		return err
	}
	defer func() { err = m.calls.after(m.manager, before, err) }()
	for i := uint32(0); i < m.A.RowCount(); i++ {
		if uint64(i) >= uint64(len(*m.Basis)) || uint64(i) >= uint64(len(out.Data)) {
			return ErrBounds
		}
		j := (*m.Basis)[i]
		if uint64(j) >= uint64(len(*m.Costs)) {
			return ErrBounds
		}
		if err = m.manager.Copy(&out.Data[i], &(*m.Costs)[j]); err != nil {
			return err
		}
	}
	return nil
}
func (m *PairCoreSolverBase) SolveYB(out *IndexedRationalVector) error {
	if err := m.FillCB(out); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	// The LU owns this call's stopped arithmetic; out retains its own values.
	return m.Factorization.SolveYBWithErrorCheck(out.Data, *m.Basis)
}
func (m *PairCoreSolverBase) SolveBd(entering uint32) error {
	if err := m.ready(); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	if err := m.Factorization.SolveBd(entering, m.Ed, m.W); err != nil {
		return err
	}
	if uint64(entering) >= uint64(len(m.ColumnsNZ)) {
		return ErrBounds
	}
	m.ColumnsNZ[entering] = m.Ed.Size()
	return nil
}
func (m *PairCoreSolverBase) SolveBdIndexed(entering uint32, out *IndexedRationalVector) error {
	if err := m.ready(); err != nil {
		return err
	}
	if out.manager != m.manager {
		return ErrExactVectorOwner
	}
	if sourceDebugEnabled && m.Settings.UseTableau() {
		return ErrAssertion
	}
	if m.Factorization == nil {
		if err := InitPairFactorization(&m.Factorization, m.A, *m.Basis, m.Settings); err != nil {
			return err
		}
	}
	return m.Factorization.SolveBdFaster(entering, out)
}
func (m *PairCoreSolverBase) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.Factorization != nil {
		if err := m.Factorization.Close(); err != nil {
			return err
		}
		m.Factorization = nil
	}
	if err := m.SteepestEdge.CloseForward(); err != nil {
		return err
	}
	if err := m.CopyXB.CloseForward(); err != nil {
		return err
	}
	for _, v := range []*IndexedRationalVector{m.ColumnNorms, m.Ed, m.D, m.W, m.Y, m.PivotRow, m.PivotRowB1} {
		if err := v.CloseForward(); err != nil {
			return err
		}
	}
	if m.feasibilityWork != nil {
		if err := m.feasibilityWork.Close(); err != nil {
			return err
		}
		m.feasibilityWork = nil
	}
	if m.valueWork != nil {
		if err := m.valueWork.Close(); err != nil {
			return err
		}
		m.valueWork = nil
	}
	if m.valueScalars != nil {
		if err := m.valueScalars.Close(); err != nil {
			return err
		}
		m.valueScalars = nil
	}
	if m.residualWork != nil {
		if err := m.residualWork.Close(); err != nil {
			return err
		}
		m.residualWork = nil
	}
	if m.residualScalars != nil {
		if err := m.residualScalars.Close(); err != nil {
			return err
		}
		m.residualScalars = nil
	}
	if m.reconstructionWork != nil {
		if err := m.reconstructionWork.Close(); err != nil {
			return err
		}
		m.reconstructionWork = nil
	}
	// A failed indexed add may still borrow its product input during cleanup.
	// The same owner retains tableau reduced-cost expression temporaries.
	if m.pivotWork != nil {
		if err := m.pivotWork.Close(); err != nil {
			return err
		}
		m.pivotWork = nil
	}
	if m.statisticsArgument != nil {
		if err := m.statisticsArgument.Close(); err != nil {
			return err
		}
		m.statisticsArgument = nil
	}
	if m.statisticsCost != nil {
		if err := m.statisticsCost.Close(); err != nil {
			return err
		}
		m.statisticsCost = nil
	}
	return m.calls.cleanupRoots(m.manager)
}

func (m *PairCoreSolverBase) CurrentXIsFeasible() bool   { return m.InfSet.Size() == 0 }
func (m *PairCoreSolverBase) CurrentXIsInfeasible() bool { return m.InfSet.Size() != 0 }
