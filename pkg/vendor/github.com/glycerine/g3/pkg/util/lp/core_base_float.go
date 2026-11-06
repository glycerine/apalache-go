package lp

// Source: src/util/lp/lp_core_solver_base.h/.hpp: floating constructor,
// basis headings and bound feasibility; numeric_pair.h:316-326.

// ColumnNamer is the retained source column_namer interface.
type ColumnNamer interface{ ColumnName(uint32) string }

// FloatCoreInputs borrows the actual solver vectors, including their slice
// headers: resizing a caller vector remains visible to the core and vice versa.
type FloatCoreInputs struct {
	A                      *FloatStaticMatrix
	B, X, Costs            *[]float64
	Basis, NBasis          *[]uint32
	Heading                *[]int32
	Settings               *Settings
	Names                  ColumnNamer
	ColumnTypes            *[]ColumnType
	LowBounds, UpperBounds *[]float64
}

// FloatCoreSolverBase owns scratch and its LU factorization, while borrowing
// FloatCoreInputs. Construction initializes bookkeeping, not a feasible solution.
type FloatCoreSolverBase struct {
	FloatCoreInputs
	Status                                  Status
	TotalIterations, IterationsNoCostGrowth uint32
	InfSet                                  *IntSet
	UsingInfeasCosts                        bool
	PivotRowB1, PivotRow, W, Ed             *IndexedFloatVector
	Y, D, ColumnNorms, CopyXB, SteepestEdge []float64
	Factorization                           *FloatLU
	BasisSortCounter                        uint32
	Trace                                   []uint32
	Tracing                                 bool
	PivotedRows                             *IntSet
	LookForFeasibleSolutionOnly             bool
	ColumnsNZ, RowsNZ                       []uint32
}

func NewFloatCoreSolverBase(in FloatCoreInputs) (*FloatCoreSolverBase, error) {
	// Null source references have no C++ semantics; use explicit Go guards.
	if in.A == nil || in.B == nil || in.X == nil || in.Costs == nil || in.Basis == nil || in.NBasis == nil || in.Heading == nil || in.Settings == nil || in.ColumnTypes == nil || in.LowBounds == nil || in.UpperBounds == nil {
		return nil, ErrBounds
	}
	n, r := in.A.ColumnCount(), in.A.RowCount()
	set, err := NewIntSet(n)
	if err != nil {
		return nil, err
	}
	m := &FloatCoreSolverBase{FloatCoreInputs: in, Status: StatusFeasible, InfSet: set,
		PivotRowB1: NewIndexedFloatVector(r), PivotRow: NewIndexedFloatVector(n),
		W: NewIndexedFloatVector(r), Ed: NewIndexedFloatVector(r),
		Y: make([]float64, r), D: make([]float64, n), ColumnNorms: make([]float64, n),
		CopyXB: make([]float64, r), SteepestEdge: make([]float64, n)}
	if sourceDebugEnabled {
		ok, err := m.BoundsForBoxedAreSetCorrectly()
		if err != nil {
			return m, err
		}
		if !ok {
			return m, ErrAssertion
		}
	}
	if err := m.Init(); err != nil {
		return m, err
	}
	// The source constructor repeats heading initialization after init/LU.
	return m, m.InitBasisHeadingAndNonBasicColumns()
}

func (m *FloatCoreSolverBase) Close() error {
	if m == nil || m.Factorization == nil {
		return nil
	}
	// Floating factorization has native Go storage and an infallible Close.
	m.Factorization.Close()
	m.Factorization = nil
	return nil
}

func (m *FloatCoreSolverBase) Init() error {
	if err := m.AllocateBasisHeading(); err != nil {
		return err
	}
	if m.Settings.UseLU() {
		return InitFloatFactorization(&m.Factorization, m.A, *m.Basis, m.Settings)
	}
	return nil
}

func (m *FloatCoreSolverBase) AllocateBasisHeading() error {
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

func (m *FloatCoreSolverBase) InitBasisHeadingAndNonBasicColumns() error {
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

func (m *FloatCoreSolverBase) BasisHeadingIsCorrect() (bool, error) {
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
	// Only debug evaluates the source inverse-basic assertion traversal.
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

func (m *FloatCoreSolverBase) BoundsForBoxedAreSetCorrectly() (bool, error) {
	for j, typ := range *m.ColumnTypes {
		if typ != ColumnBoxed {
			continue
		}
		if j >= len(*m.LowBounds) || j >= len(*m.UpperBounds) {
			return false, ErrBounds
		}
		if (*m.LowBounds)[j] > (*m.UpperBounds)[j] {
			return false, nil
		}
	}
	return true, nil
}

func (m *FloatCoreSolverBase) BelowBound(x, bound float64) bool {
	eps := m.Settings.PrimalFeasibilityTolerance
	if bound == 0 {
		return x < -eps
	}
	relative := eps
	if bound > 0 {
		relative = -eps
	}
	return x < bound*(1+relative)-eps
}
func (m *FloatCoreSolverBase) AboveBound(x, bound float64) bool {
	eps := m.Settings.PrimalFeasibilityTolerance
	if bound == 0 {
		return x > eps
	}
	relative := -eps
	if bound > 0 {
		relative = eps
	}
	return x > bound*(1+relative)+eps
}
func (m *FloatCoreSolverBase) AtBound(x, bound float64) bool {
	return !m.BelowBound(x, bound) && !m.AboveBound(x, bound)
}

func (m *FloatCoreSolverBase) ColumnIsFeasible(j uint32) (bool, error) {
	if uint64(j) >= uint64(len(*m.X)) || uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return false, ErrBounds
	}
	x, typ := (*m.X)[j], (*m.ColumnTypes)[j]
	switch typ {
	case ColumnFixed, ColumnBoxed:
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return false, ErrBounds
		}
		if m.AboveBound(x, (*m.UpperBounds)[j]) {
			return false, nil
		}
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		return !m.BelowBound(x, (*m.LowBounds)[j]), nil
	case ColumnLowBound:
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return false, ErrBounds
		}
		return !m.BelowBound(x, (*m.LowBounds)[j]), nil
	case ColumnUpperBound:
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return false, ErrBounds
		}
		return !m.AboveBound(x, (*m.UpperBounds)[j]), nil
	case ColumnFree:
		return true, nil
	default:
		return false, sourceAssertionFailure()
	}
}

func (m *FloatCoreSolverBase) CalcCurrentXIsFeasible() (bool, error) {
	for i := m.A.RowCount(); i > 0; {
		i--
		if uint64(i) >= uint64(len(*m.Basis)) {
			return false, ErrBounds
		}
		ok, err := m.ColumnIsFeasible((*m.Basis)[i])
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}
func (m *FloatCoreSolverBase) CalcCurrentXIsFeasibleIncludeNonBasis() (bool, error) {
	for j := m.A.ColumnCount(); j > 0; {
		j--
		ok, err := m.ColumnIsFeasible(j)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}
func (m *FloatCoreSolverBase) CurrentXIsFeasible() bool   { return m.InfSet.Empty() }
func (m *FloatCoreSolverBase) CurrentXIsInfeasible() bool { return !m.InfSet.Empty() }
func (m *FloatCoreSolverBase) UpdateColumnInInfSet(j uint32) error {
	ok, err := m.ColumnIsFeasible(j)
	if err != nil {
		return err
	}
	if ok {
		m.InfSet.Erase(j)
		return nil
	}
	return m.InfSet.Insert(j)
}
