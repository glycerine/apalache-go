package lp

// Source: src/util/lp/lp_core_solver_base.h/.hpp, retained scalar mpq/mpq core.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalCoreInputs borrows the actual scalar mpq/mpq matrix and vector
// headers. Their numerical owners must outlive this core and its LU. No pair
// or floating arithmetic participates in this concrete instantiation.
type RationalCoreInputs struct {
	A                            *RationalStaticMatrix
	B, X, LowBounds, UpperBounds *[]numeral.Rational
	Costs                        *[]numeral.Rational
	Basis, NBasis                *[]uint32
	Heading                      *[]int32
	ColumnTypes                  *[]ColumnType
	Settings                     *Settings
	Names                        ColumnNamer
}

// RationalCoreSolverBase owns rational scratch and RationalLU. Constructor
// errors return a partial owner to Close before releasing borrowed inputs.
// This increment implements construction, headings, bound checks and LU
// entry points; primal/dual optimization is still supplied by later layers.
type RationalCoreSolverBase struct {
	RationalCoreInputs
	Status                                             Status
	TotalIterations, IterationsNoCostGrowth            uint32
	InfSet                                             *IntSet
	UsingInfeasCosts                                   bool
	PivotRowB1, PivotRow, W, Ed                        *IndexedRationalVector
	Y, D, ColumnNorms, SteepestEdge                    *IndexedRationalVector
	CopyXB                                             *IndexedRationalVector
	Factorization                                      *RationalLU
	BasisSortCounter                                   uint32
	Trace                                              []uint32
	Tracing                                            bool
	PivotedRows                                        *IntSet
	LookForFeasibleSolutionOnly                        bool
	ColumnsNZ, RowsNZ                                  []uint32
	manager                                            *numeral.RationalManager
	valueWork, reconstructionWork, pivotWork, costWork *IndexedRationalVector
	residualWork                                       *IndexedRationalVector
	calls                                              exactEtaCalls
	stopped, closed                                    bool
	// Reporting locals remain live through calls into this owner.
	statisticsCost, statisticsArgument *IndexedRationalVector
}

func NewRationalCoreSolverBase(in RationalCoreInputs) (m *RationalCoreSolverBase, err error) {
	if in.A == nil || in.B == nil || in.X == nil || in.Costs == nil || in.Basis == nil || in.NBasis == nil || in.Heading == nil || in.ColumnTypes == nil || in.LowBounds == nil || in.UpperBounds == nil || in.Settings == nil {
		return nil, ErrBounds
	}
	if in.A.manager == nil {
		return nil, ErrExactVectorOwner
	}
	n, r := in.A.ColumnCount(), in.A.RowCount()
	set, err := NewIntSet(n)
	if err != nil {
		return nil, err
	}
	z := in.A.manager
	m = &RationalCoreSolverBase{RationalCoreInputs: in, Status: StatusFeasible, InfSet: set, manager: z,
		PivotRowB1: NewIndexedRationalVector(z, r), PivotRow: NewIndexedRationalVector(z, n),
		Y: NewIndexedRationalVector(z, r), W: NewIndexedRationalVector(z, r), D: NewIndexedRationalVector(z, n), Ed: NewIndexedRationalVector(z, r),
		ColumnNorms: NewIndexedRationalVector(z, n), CopyXB: NewIndexedRationalVector(z, r), SteepestEdge: NewIndexedRationalVector(z, n)}
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
func (m *RationalCoreSolverBase) ready() error {
	if m.closed || m.stopped || m.valueWork != nil || m.reconstructionWork != nil || m.pivotWork != nil || m.costWork != nil || m.residualWork != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (m *RationalCoreSolverBase) Init() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err := m.AllocateBasisHeading(); err != nil {
		return err
	}
	if m.Settings == nil {
		return ErrBounds
	}
	if m.Settings.UseLU() {
		return InitRationalFactorization(&m.Factorization, m.A, *m.Basis, m.Settings)
	}
	return nil
}
func (m *RationalCoreSolverBase) AllocateBasisHeading() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
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
func (m *RationalCoreSolverBase) InitBasisHeadingAndNonBasicColumns() error {
	if err := m.ready(); err != nil {
		return err
	}
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
func (m *RationalCoreSolverBase) BasisHeadingIsCorrect() (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
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
func (m *RationalCoreSolverBase) compare(x, bound *numeral.Rational) (c int, err error) {
	if err = m.ready(); err != nil {
		return
	}
	before, err := m.calls.begin(m.manager)
	if err != nil {
		return 0, err
	}
	defer func() { err = m.calls.after(m.manager, before, err); m.stopped = err != nil }()
	return m.manager.Compare(x, bound)
}
func (m *RationalCoreSolverBase) BelowBound(x, bound *numeral.Rational) (bool, error) {
	c, e := m.compare(x, bound)
	return c < 0, e
}
func (m *RationalCoreSolverBase) AboveBound(x, bound *numeral.Rational) (bool, error) {
	c, e := m.compare(x, bound)
	return c > 0, e
}
func (m *RationalCoreSolverBase) AtBound(x, bound *numeral.Rational) (bool, error) {
	below, err := m.BelowBound(x, bound)
	if err != nil || below {
		return false, err
	}
	above, err := m.AboveBound(x, bound)
	return !above, err
}
func (m *RationalCoreSolverBase) BoundsForBoxedAreSetCorrectly() (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
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
func (m *RationalCoreSolverBase) ColumnIsFeasible(j uint32) (bool, error) {
	if err := m.ready(); err != nil {
		return false, err
	}
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
func (m *RationalCoreSolverBase) FillCB(out *IndexedRationalVector) (err error) {
	if out == nil {
		return ErrBounds
	}
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
	defer func() { err = m.calls.after(m.manager, before, err); m.stopped = err != nil }()
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
func (m *RationalCoreSolverBase) SolveYB(out *IndexedRationalVector) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if err := m.FillCB(out); err != nil {
		return err
	}
	if m.Factorization == nil {
		return ErrBounds
	}
	// The LU owns this call's stopped arithmetic; out retains its own values.
	return m.Factorization.SolveYBWithErrorCheck(out.Data, *m.Basis)
}
func (m *RationalCoreSolverBase) SolveBd(entering uint32) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
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
func (m *RationalCoreSolverBase) SolveBdIndexed(entering uint32, out *IndexedRationalVector) (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if out.manager != m.manager {
		return ErrExactVectorOwner
	}
	if m.Settings == nil {
		return ErrBounds
	}
	if sourceDebugEnabled && m.Settings.UseTableau() {
		return ErrAssertion
	}
	if m.Factorization == nil {
		if err := InitRationalFactorization(&m.Factorization, m.A, *m.Basis, m.Settings); err != nil {
			return err
		}
	}
	return m.Factorization.SolveBdFaster(entering, out)
}

// Close does not release borrowed matrix, input vectors, or output vectors.
// Stopped LU calls close before the scratch and external numerals they borrow.
func (m *RationalCoreSolverBase) Close() error {
	if m.closed {
		return nil
	}
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	if m.Factorization != nil {
		if err := m.Factorization.Close(); err != nil {
			return err
		}
		m.Factorization = nil
	}
	for _, v := range []*IndexedRationalVector{m.SteepestEdge, m.CopyXB, m.ColumnNorms, m.Ed, m.D, m.W, m.Y, m.PivotRow, m.PivotRowB1} {
		if err := v.Close(); err != nil {
			return err
		}
	}
	for _, v := range []*IndexedRationalVector{m.valueWork, m.pivotWork, m.costWork, m.reconstructionWork, m.residualWork, m.statisticsArgument, m.statisticsCost} {
		if v != nil {
			if err := v.Close(); err != nil {
				return err
			}
		}
	}
	m.valueWork, m.pivotWork, m.costWork, m.reconstructionWork = nil, nil, nil, nil
	m.residualWork = nil
	m.statisticsArgument, m.statisticsCost = nil, nil
	if err := m.calls.cleanupRoots(m.manager); err != nil {
		return err
	}
	m.closed = true
	return nil
}
func (m *RationalCoreSolverBase) CurrentXIsFeasible() bool   { return m.InfSet.Size() == 0 }
func (m *RationalCoreSolverBase) CurrentXIsInfeasible() bool { return m.InfSet.Size() != 0 }
