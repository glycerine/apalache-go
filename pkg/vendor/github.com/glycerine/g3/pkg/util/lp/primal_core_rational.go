package lp

// Source: src/util/lp/lp_primal_core_solver.h:901-934,791-821.
// Concrete retained scalar mpq/mpq stage-one primal core.

import "github.com/glycerine/g3/pkg/util/numeral"

// RationalPrimalCoreSolver borrows scalar model storage and owns exact primal
// scratch. Construction, feasibility costs, initialization, entering pricing
// bound-step/breakpoint helpers and circular leaving selection are implemented.
// LU and tableau column/row iteration and precise Solve are implemented.
// Constructor errors return a partial owner to Close before borrowed inputs.
type RationalPrimalCoreSolver struct {
	*RationalCoreSolverBase
	Beta                                         *IndexedRationalVector
	BreakpointQueue                              *RationalPriorityQueue
	EpsilonOfReducedCost, ConvertedHarrisEpsilon numeral.Rational
	CostsBackup                                  *IndexedRationalVector
	ColumnNormUpdateCounter, BlandModeThreshold  uint32
	SignOfEnteringDelta                          int32
	NonBasisList                                 []uint32
	LeftBasisTableau                             IntSet
	BlandModeTableau                             bool
	LeftBasisRepeated, InfRowIndexForTableau     uint32
	LeavingCandidates                            []uint32
	calls                                        exactEtaCalls
	work                                         *IndexedRationalVector
	stepEpsilon, basisMultiplier                 *IndexedRationalVector
	leavingMultiplier, leavingRatio              *IndexedRationalVector
	advanceDelta                                 *IndexedRationalVector
	enteringStep                                 *IndexedRationalVector
	tableauDelta                                 *IndexedRationalVector
	// Automatic row-iteration values remain live through primal child calls.
	rowIterationBound                  *IndexedRationalVector
	rowIterationCoefficient            *IndexedRationalVector
	rowIterationTheta                  *IndexedRationalVector
	Breakpoints                        []*RationalBreakpoint
	breakpointWork                     *IndexedRationalVector
	breakpointCollect, breakpointSlope *IndexedRationalVector
	pendingBreakpoint                  *RationalBreakpoint
	stopped                            bool
}

func NewRationalPrimalCoreSolver(in RationalCoreInputs) (m *RationalPrimalCoreSolver, err error) {
	base, err := NewRationalCoreSolverBase(in)
	if base == nil {
		return nil, err
	}
	m = &RationalPrimalCoreSolver{RationalCoreSolverBase: base, EpsilonOfReducedCost: numeral.NewRational(), ConvertedHarrisEpsilon: numeral.NewRational()}
	defer func() {
		m.stopped = err != nil
		if err != nil {
			base.stopped = true
		}
	}()
	if err != nil {
		return m, err
	}
	m.BreakpointQueue = NewRationalPriorityQueue(base.manager, 0)
	m.Beta = NewIndexedRationalVector(base.manager, in.A.RowCount())
	before, err := m.calls.begin(base.manager)
	if err != nil {
		return m, err
	}
	defer func() { err = m.calls.after(base.manager, before, err) }()
	m.work = NewIndexedRationalVector(base.manager, 3)
	temporary, one, den := &m.work.Data[0], &m.work.Data[1], &m.work.Data[2]
	if err = base.manager.SetInt32(one, 1); err != nil {
		return m, err
	}
	if err = base.manager.SetInt32(den, 10000000); err != nil {
		return m, err
	}
	if err = etaRationalQuotient(base.manager, temporary, &m.EpsilonOfReducedCost, one, den); err != nil {
		return m, err
	}
	// The source finishes epsilon member construction before default backup and
	// converted-Harris members, then assigns threshold/Harris/status in the body.
	m.CostsBackup = NewIndexedRationalVector(base.manager, 0)
	m.BlandModeThreshold = 1000
	// Precise traits take the zero branch regardless of floating tolerance.
	m.Status = StatusUnknown
	if err = m.work.Close(); err != nil {
		return m, err
	}
	m.work = nil
	return m, nil
}
func (m *RationalPrimalCoreSolver) ready() error {
	if m.stopped || m.advanceDelta != nil || m.work != nil || m.leavingMultiplier != nil || m.leavingRatio != nil || m.stepEpsilon != nil || m.basisMultiplier != nil || m.breakpointCollect != nil || m.breakpointSlope != nil || m.breakpointWork != nil || m.pendingBreakpoint != nil || m.calls.pending() {
		return ErrExactVectorPending
	}
	return m.RationalCoreSolverBase.ready()
}
func (m *RationalPrimalCoreSolver) InitInfSet() (err error) {
	if err = m.ready(); err != nil {
		return err
	}
	defer func() { m.stopped = err != nil }()
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
			if err = m.InfSet.Insert(j); err != nil {
				return err
			}
		}
	}
	return nil
}
func (m *RationalPrimalCoreSolver) GetColumnOutOfBoundsDeltaSign(j uint32) (int32, error) {
	if err := m.ready(); err != nil {
		return 0, err
	}
	if uint64(j) >= uint64(len(*m.ColumnTypes)) || uint64(j) >= uint64(len(*m.X)) {
		return 0, ErrBounds
	}
	t := (*m.ColumnTypes)[j]
	switch t {
	case ColumnFixed, ColumnBoxed, ColumnLowBound:
		if uint64(j) >= uint64(len(*m.LowBounds)) {
			return 0, ErrBounds
		}
		below, err := m.BelowBound(&(*m.X)[j], &(*m.LowBounds)[j])
		if err != nil {
			return 0, err
		}
		if below {
			return -1, nil
		}
		if t == ColumnLowBound {
			return 0, nil
		}
		fallthrough
	case ColumnUpperBound:
		if uint64(j) >= uint64(len(*m.UpperBounds)) {
			return 0, ErrBounds
		}
		above, err := m.AboveBound(&(*m.X)[j], &(*m.UpperBounds)[j])
		if err != nil {
			return 0, err
		}
		if above {
			return 1, nil
		}
		return 0, nil
	case ColumnFree:
		return 0, nil
	default:
		return 0, sourceAssertionFailure()
	}
}
func (m *RationalPrimalCoreSolver) Close() error {
	if err := m.calls.cleanupRaw(m.manager); err != nil {
		return err
	}
	// A stopped base arithmetic call can still borrow primal-owned values.
	if err := m.RationalCoreSolverBase.Close(); err != nil {
		return err
	}
	// Queue arithmetic can still borrow the breakpoint priority expression.
	if m.BreakpointQueue != nil {
		if err := m.BreakpointQueue.Close(); err != nil {
			return err
		}
	}
	if err := m.closeRowIterationLocals(); err != nil {
		return err
	}
	if m.enteringStep != nil {
		if err := m.enteringStep.Close(); err != nil {
			return err
		}
		m.enteringStep = nil
	}
	if m.tableauDelta != nil {
		if err := m.tableauDelta.Close(); err != nil {
			return err
		}
		m.tableauDelta = nil
	}
	if m.advanceDelta != nil {
		if err := m.advanceDelta.Close(); err != nil {
			return err
		}
		m.advanceDelta = nil
	}
	if m.work != nil {
		if err := m.work.Close(); err != nil {
			return err
		}
		m.work = nil
	}
	if m.leavingMultiplier != nil {
		if err := m.leavingMultiplier.Close(); err != nil {
			return err
		}
		m.leavingMultiplier = nil
	}
	if m.leavingRatio != nil {
		if err := m.leavingRatio.Close(); err != nil {
			return err
		}
		m.leavingRatio = nil
	}
	if m.breakpointCollect != nil {
		if err := m.breakpointCollect.Close(); err != nil {
			return err
		}
		m.breakpointCollect = nil
	}
	if m.breakpointSlope != nil {
		if err := m.breakpointSlope.Close(); err != nil {
			return err
		}
		m.breakpointSlope = nil
	}
	if m.pendingBreakpoint != nil {
		if err := m.manager.Del(m.pendingBreakpoint.Delta); err != nil {
			return err
		}
		m.pendingBreakpoint = nil
	}
	if m.breakpointWork != nil {
		if err := m.breakpointWork.Close(); err != nil {
			return err
		}
		m.breakpointWork = nil
	}
	if m.stepEpsilon != nil {
		if err := m.stepEpsilon.Close(); err != nil {
			return err
		}
		m.stepEpsilon = nil
	}
	if m.basisMultiplier != nil {
		if err := m.basisMultiplier.Close(); err != nil {
			return err
		}
		m.basisMultiplier = nil
	}
	if err := m.manager.Del(&m.ConvertedHarrisEpsilon); err != nil {
		return err
	}
	if m.CostsBackup != nil {
		if err := m.CostsBackup.Close(); err != nil {
			return err
		}
	}
	if err := m.manager.Del(&m.EpsilonOfReducedCost); err != nil {
		return err
	}
	if m.Beta != nil {
		if err := m.Beta.Close(); err != nil {
			return err
		}
	}
	if err := m.destroyBreakpointRecords(); err != nil {
		return err
	}
	return m.calls.cleanupRoots(m.manager)
}
