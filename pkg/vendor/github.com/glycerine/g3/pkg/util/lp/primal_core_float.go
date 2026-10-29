package lp

// Source: src/util/lp/lp_primal_core_solver.h:791-799,901-934;
// lp_primal_core_solver.hpp:630-664,974-981,1103-1226,1291-1304.

import "math"

// FloatPrimalCoreSolver is the retained stage-one double/double core. This
// layer supplies initialization, pricing, step selection and the floating Solve loop.
type FloatPrimalCoreSolver struct {
	*FloatCoreSolverBase
	Beta                                         *IndexedFloatVector
	BreakpointQueue                              *FloatPriorityQueue
	Breakpoints                                  []FloatBreakpoint
	EpsilonOfReducedCost, ConvertedHarrisEpsilon float64
	CostsBackup                                  []float64
	ColumnNormUpdateCounter, BlandModeThreshold  uint32
	SignOfEnteringDelta                          int32
	NonBasisList                                 []uint32
	lowBoundsDummy                               []float64
}

func NewFloatPrimalCoreSolver(in FloatCoreInputs) (*FloatPrimalCoreSolver, error) {
	b, err := NewFloatCoreSolverBase(in)
	if b == nil {
		return nil, err
	}
	m := &FloatPrimalCoreSolver{FloatCoreSolverBase: b}
	if err != nil {
		return m, err
	}
	m.Beta = NewIndexedFloatVector(in.A.RowCount())
	m.BreakpointQueue = NewFloatPriorityQueue(0)
	m.EpsilonOfReducedCost = 1.0 / 10000000
	m.BlandModeThreshold = 1000
	m.ConvertedHarrisEpsilon = in.Settings.HarrisFeasibilityTolerance
	m.Status = StatusUnknown
	return m, nil
}

func (m *FloatPrimalCoreSolver) InitInfSet() error {
	if err := m.InfSet.Clear(); err != nil {
		return err
	}
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(*m.Heading)) {
			return ErrBounds
		}
		if (*m.Heading)[j] < 0 {
			continue
		}
		ok, err := m.ColumnIsFeasible(j)
		if err != nil {
			return err
		}
		if !ok {
			if err := m.InfSet.Insert(j); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *FloatPrimalCoreSolver) columnBound(j uint32, upper bool) (float64, float64, error) {
	if uint64(j) >= uint64(len(*m.X)) {
		return 0, 0, ErrBounds
	}
	b := *m.LowBounds
	if upper {
		b = *m.UpperBounds
	}
	if uint64(j) >= uint64(len(b)) {
		return 0, 0, ErrBounds
	}
	return (*m.X)[j], b[j], nil
}
func (m *FloatPrimalCoreSolver) atColumnBound(j uint32, upper bool) (bool, error) {
	x, b, err := m.columnBound(j, upper)
	if err != nil {
		return false, err
	}
	return m.AtBound(x, b), nil
}

// basicInfeasibilityCost returns whether the source writes a cost. The native
// unknown-type branch of the initializer retains its previous cost, while the
// getter returns zero. Sharing only this calculation preserves both behaviors.
func (m *FloatPrimalCoreSolver) basicInfeasibilityCost(j uint32) (float64, bool, error) {
	if uint64(j) >= uint64(len(*m.ColumnTypes)) {
		return 0, false, ErrBounds
	}
	typ := (*m.ColumnTypes)[j]
	switch typ {
	case ColumnFixed, ColumnBoxed, ColumnUpperBound:
		x, b, err := m.columnBound(j, true)
		if err != nil {
			return 0, false, err
		}
		if m.AboveBound(x, b) {
			return 1, true, nil
		}
		if typ == ColumnUpperBound {
			return 0, true, nil
		}
	}
	switch typ {
	case ColumnFixed, ColumnBoxed, ColumnLowBound:
		x, b, err := m.columnBound(j, false)
		if err != nil {
			return 0, false, err
		}
		if m.BelowBound(x, b) {
			return -1, true, nil
		}
		return 0, true, nil
	case ColumnFree:
		return 0, true, nil
	default:
		return 0, false, sourceAssertionFailure()
	}
}

func (m *FloatPrimalCoreSolver) GetInfeasibilityCostForColumn(j uint32) (float64, error) {
	if uint64(j) >= uint64(len(*m.Heading)) {
		return 0, ErrBounds
	}
	if (*m.Heading)[j] < 0 {
		return 0, nil
	}
	v, _, err := m.basicInfeasibilityCost(j)
	if err != nil {
		return 0, err
	}
	if !m.Settings.UseBreakpointsInFeasibilitySearch {
		v = -v
	}
	return v, nil
}

func (m *FloatPrimalCoreSolver) InitInfeasibilityCostForColumn(j uint32) error {
	if uint64(j) >= uint64(len(*m.Heading)) {
		return ErrBounds
	}
	if (*m.Heading)[j] < 0 {
		if uint64(j) >= uint64(len(*m.Costs)) {
			return ErrBounds
		}
		(*m.Costs)[j] = 0
		m.InfSet.Erase(j)
		return nil
	}
	v, write, err := m.basicInfeasibilityCost(j)
	if err != nil {
		return err
	}
	if uint64(j) >= uint64(len(*m.Costs)) {
		return ErrBounds
	}
	if write {
		(*m.Costs)[j] = v
	}
	if (*m.Costs)[j] == 0 {
		m.InfSet.Erase(j)
	} else if err := m.InfSet.Insert(j); err != nil {
		return err
	}
	if !m.Settings.UseBreakpointsInFeasibilitySearch {
		(*m.Costs)[j] = -(*m.Costs)[j]
	}
	return nil
}
func (m *FloatPrimalCoreSolver) InitInfeasibilityCosts() error {
	n := m.A.ColumnCount()
	if sourceDebugEnabled && (uint64(len(*m.X)) < uint64(n) || uint64(len(*m.ColumnTypes)) < uint64(n)) {
		return ErrAssertion
	}
	for j := n; j > 0; {
		j--
		if err := m.InitInfeasibilityCostForColumn(j); err != nil {
			return err
		}
	}
	m.UsingInfeasCosts = true
	return nil
}
func (m *FloatPrimalCoreSolver) InitInfeasibilityCostsForChangedBasisOnly() error {
	for _, i := range m.Ed.Index {
		if uint64(i) >= uint64(len(*m.Basis)) {
			return ErrBounds
		}
		if err := m.InitInfeasibilityCostForColumn((*m.Basis)[i]); err != nil {
			return err
		}
	}
	m.UsingInfeasCosts = true
	return nil
}
func (m *FloatPrimalCoreSolver) InitInfeasibilityAfterUpdateXIfInf(leaving uint32) error {
	if !m.UsingInfeasCosts {
		return nil
	}
	if err := m.InitInfeasibilityCostsForChangedBasisOnly(); err != nil {
		return err
	}
	if uint64(leaving) >= uint64(len(*m.Costs)) {
		return ErrBounds
	}
	(*m.Costs)[leaving] = 0
	m.InfSet.Erase(leaving)
	return nil
}

func (m *FloatPrimalCoreSolver) BackupAndNormalizeCosts() error {
	if m.LookForFeasibleSolutionOnly {
		return nil
	}
	// Source std::max(abs(v),r) returns its first operand on an unordered
	// comparison. A NaN can therefore replace, then lose, the running maximum.
	max := 0.0
	for _, v := range *m.Costs {
		a := math.Abs(v)
		if !(a < max) {
			max = a
		}
	}
	if max < 1 {
		max = 1
	}
	if sourceDebugEnabled && len(m.CostsBackup) != 0 {
		return ErrAssertion
	}
	for j := range *m.Costs {
		(*m.Costs)[j] /= max
		m.CostsBackup = append(m.CostsBackup, (*m.Costs)[j])
	}
	return nil
}
func (m *FloatPrimalCoreSolver) InitReducedCosts() error {
	if sourceDebugEnabled && m.Settings.UseTableau() {
		return ErrAssertion
	}
	if m.CurrentXIsInfeasible() && !m.UsingInfeasCosts {
		if err := m.InitInfeasibilityCosts(); err != nil {
			return err
		}
	} else if m.CurrentXIsFeasible() && m.UsingInfeasCosts {
		if m.LookForFeasibleSolutionOnly {
			return nil
		}
		*m.Costs = append((*m.Costs)[:0], m.CostsBackup...)
		m.UsingInfeasCosts = false
	}
	return m.InitReducedCostsForOneIteration()
}
func (m *FloatPrimalCoreSolver) InitColumnNorms() error {
	for j := uint32(0); j < m.A.ColumnCount(); j++ {
		if uint64(j) >= uint64(len(m.ColumnNorms)) {
			return ErrBounds
		}
		// Preserve source unsigned size+1 then signed Linux int conversion.
		base := int32(uint32(len(m.A.Columns[j])) + 1)
		m.ColumnNorms[j] = float64(base) + float64(m.Settings.RandomNext()%10000)/100000
	}
	return nil
}
func (m *FloatPrimalCoreSolver) InitRun() error {
	m.BasisSortCounter = 0
	m.TotalIterations = 0
	m.IterationsNoCostGrowth = 0
	if err := m.InitInfSet(); err != nil {
		return err
	}
	if m.CurrentXIsFeasible() && m.LookForFeasibleSolutionOnly {
		return nil
	}
	m.UsingInfeasCosts = false
	if m.Settings.BackupCosts {
		if err := m.BackupAndNormalizeCosts(); err != nil {
			return err
		}
	}
	m.EpsilonOfReducedCost = 1.0 / 10000000
	m.BreakpointQueue.Resize(m.A.ColumnCount())
	if err := m.InitReducedCosts(); err != nil {
		return err
	}
	m.ColumnNormUpdateCounter = 0
	return m.InitColumnNorms()
}
