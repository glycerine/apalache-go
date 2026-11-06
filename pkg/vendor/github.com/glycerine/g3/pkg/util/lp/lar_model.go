package lp

// Source: src/util/lp/lar_solver.h and init_lar_solver.h: construction and
// concrete variable registration. Whole-model scopes are in lar_scopes.go;
// full LAR optimization remains open.

import (
	"github.com/glycerine/g3/pkg/util/numeral"
	"strconv"
)

// LARSolver owns its model and cores in a borrowed numeral manager. Bounds is
// a borrowed alias of Core.Bounds, never a second bound owner. The cores borrow
// this solver's settings and column namer. Close all propagators/analyzers and
// other borrowers before this owner. Do not copy, including after construction.
type LARSolver struct {
	Settings                                           *Settings
	Core                                               *LARCoreSolver
	Bounds                                             *LARBounds
	SimplexStrategy                                    StackedStrategy
	ExternalToColumns                                  map[uint32]uint32
	ColumnsToExternal                                  []uint32
	RowsWithChangedBounds, BasicColumnsWithChangedCost IntSet
	Terms, OriginalTerms                               []*LARTerm
	TermCount, ConstraintCount                         StackedUint32
	ColumnBuffer                                       *IndexedRationalVector
	Constraints                                        []LARStoredConstraint
	pendingConstraint                                  LARStoredConstraint
	pendingTerm                                        *LARTerm
	pendingCleanupDelta                                *RationalPair
	pendingIterator                                    *LARTermIterator
	pendingRowIterator                                 RationalLinearIterator
	pendingRowAnalyzer                                 *LARRowBoundAnalyzer
	work                                               *IndexedPairVector
	rationalWork                                       *IndexedRationalVector
	modelPairs                                         *IndexedPairVector
	modelSingles                                       *IndexedRationalVector
	validationCoefficients                             *SparseRationalVector
	validationModel                                    *LARRationalModel
	explanationCoefficients                            *LARRationalModel
	validationExplanation                              *SparseRationalVector
	certificateResult                                  *numeral.Rational
	randomUpdater                                      *RandomUpdater
	calls                                              exactEtaCalls
	manager                                            *numeral.RationalManager
	stopped                                            bool
}

func NewLARSolver(m *numeral.RationalManager) (solver *LARSolver, err error) {
	if m == nil {
		return nil, ErrBounds
	}
	solver = &LARSolver{manager: m, ExternalToColumns: make(map[uint32]uint32), ColumnBuffer: NewIndexedRationalVector(m, 0)}
	solver.Settings, err = NewSettings()
	if err != nil {
		solver.stopped = true
		return solver, err
	}
	solver.Core, err = NewLARCoreSolver(m, solver.Settings, solver)
	if solver.Core != nil {
		solver.Bounds = solver.Core.Bounds
	}
	solver.stopped = err != nil
	return solver, err
}
func (s *LARSolver) ready() error {
	if s.stopped || s.Core == nil || s.Core.stopped || s.pendingCleanupDelta != nil || s.pendingConstraint != nil || s.pendingTerm != nil || s.pendingIterator != nil || s.pendingRowIterator != nil || s.pendingRowAnalyzer != nil || s.work != nil || s.rationalWork != nil || s.modelPairs != nil || s.modelSingles != nil || s.validationCoefficients != nil || s.explanationCoefficients != nil || s.certificateResult != nil || s.randomUpdater != nil || s.calls.pending() {
		return ErrExactVectorPending
	}
	return nil
}
func (s *LARSolver) StrategyIsUndecided() bool {
	return s.Settings.SimplexStrategy == StrategyUndecided
}
func (s *LARSolver) UseLU() bool { return s.Settings.SimplexStrategy == StrategyLU }
func (s *LARSolver) ColumnName(j uint32) string {
	if j >= LARTermStartIndex {
		return "_t" + strconv.FormatUint(uint64(j), 10)
	}
	if uint64(j) >= uint64(len(s.ColumnsToExternal)) {
		return "_s" + strconv.FormatUint(uint64(j), 10)
	}
	return "v" + strconv.FormatUint(uint64(s.ColumnsToExternal[j]), 10)
}
func (s *LARSolver) IsTerm(j uint32) bool {
	return j >= LARTermStartIndex && uint64(j-LARTermStartIndex) < uint64(len(s.Terms))
}
func (s *LARSolver) VarIsRegistered(j uint32) bool {
	if j >= LARTermStartIndex {
		return uint64(j-LARTermStartIndex) < uint64(len(s.Terms))
	}
	return j < s.Core.RA.ColumnCount()
}
func (s *LARSolver) SetPropagateBoundsOnPivotedRowsMode(v bool) {
	if v {
		s.Core.RPrimal.PivotedRows = &s.RowsWithChangedBounds
	} else {
		s.Core.RPrimal.PivotedRows = nil
	}
}
func (s *LARSolver) SizesAreCorrect() (bool, error) {
	if sourceDebugEnabled {
		c := s.Core
		n := c.RA.ColumnCount()
		if !s.StrategyIsUndecided() && c.NeedToPresolveWithDoubleSolver() && n != c.DA.ColumnCount() {
			return false, ErrAssertion
		}
		if n != uint32(len(*c.RPrimal.ColumnTypes)) || n != uint32(len(*c.RPrimal.Costs)) || n != uint32(len(*c.RPrimal.X)) {
			return false, ErrAssertion
		}
	}
	return true, nil
}
func (s *LARSolver) registerNewExternalIndex(ext uint32) error {
	if sourceDebugEnabled {
		if _, present := s.ExternalToColumns[ext]; present {
			return ErrAssertion
		}
	}
	j := uint32(len(s.ExternalToColumns))
	s.ExternalToColumns[ext] = j // Source publishes map before directory assertion.
	if sourceDebugEnabled && uint32(len(s.ColumnsToExternal)) != j {
		return ErrAssertion
	}
	s.ColumnsToExternal = append(s.ColumnsToExternal, ext)
	return nil
}
func (s *LARSolver) addRationalCoreFields(basic bool) error {
	c := s.Core
	j := c.RA.ColumnCount()
	if err := c.RA.AddColumn(); err != nil {
		return err
	}
	if sourceDebugEnabled && c.RX.DataSize() != j {
		return ErrAssertion
	}
	if err := c.RX.resizeData(j + 1); err != nil {
		return err
	}
	if err := s.Bounds.Low.IncreaseSizeByOne(); err != nil {
		return err
	}
	if err := s.Bounds.Upper.IncreaseSizeByOne(); err != nil {
		return err
	}
	if err := c.RPrimal.InfSet.IncreaseSizeByOne(); err != nil {
		return err
	}
	if err := c.Costs.resizeData(j + 1); err != nil {
		return err
	}
	if err := c.RPrimal.D.resizeData(j + 1); err != nil {
		return err
	}
	if sourceDebugEnabled && uint32(len(c.RHeading)) != j {
		return ErrAssertion
	}
	if basic {
		if err := c.RA.AddRow(); err != nil {
			return err
		}
		c.RHeading = append(c.RHeading, int32(len(c.RBasis)))
		c.RBasis = append(c.RBasis, j)
		if s.Settings.BoundPropagation {
			if err := s.RowsWithChangedBounds.Insert(c.RA.RowCount() - 1); err != nil {
				return err
			}
		}
	} else {
		c.RHeading = append(c.RHeading, -int32(len(c.RNBasis))-1)
		c.RNBasis = append(c.RNBasis, j)
	}
	return nil
}
func (s *LARSolver) addDoubleCoreFields(basic bool) error {
	c := s.Core
	j := c.DA.ColumnCount()
	c.DA.AddColumn()
	if sourceDebugEnabled && uint32(len(c.DX)) != j {
		return ErrAssertion
	}
	// These source vectors grow by one here; each new scalar is value-initialized.
	c.DX = resizeFloatSolverValues(c.DX, int(j)+1)
	c.DLow = resizeFloatSolverValues(c.DLow, int(j)+1)
	c.DUpper = resizeFloatSolverValues(c.DUpper, int(j)+1)
	if sourceDebugEnabled && uint32(len(c.DHeading)) != j {
		return ErrAssertion
	}
	if basic {
		c.DA.AddRow()
		c.DHeading = append(c.DHeading, int32(len(c.DBasis)))
		c.DBasis = append(c.DBasis, j)
	} else {
		c.DHeading = append(c.DHeading, -int32(len(c.DNBasis))-1)
		c.DNBasis = append(c.DNBasis, j)
	}
	return nil
}
func (s *LARSolver) AddVar(ext uint32) (j uint32, err error) {
	if err = s.ready(); err != nil {
		return 0, err
	}
	defer func() {
		if err != nil {
			s.stopped = true
		}
	}()
	if ext >= LARTermStartIndex {
		if sourceDebugEnabled {
			return 0, ErrAssertion
		}
		return 0, ErrBounds
	} // Source throws integer zero after the debug assertion.
	if j, ok := s.ExternalToColumns[ext]; ok {
		return j, nil
	}
	if sourceDebugEnabled && s.Bounds.Witnesses.Size() != s.Core.RA.ColumnCount() {
		return 0, ErrAssertion
	}
	j = s.Core.RA.ColumnCount()
	s.Bounds.Witnesses.Append(NewLARBoundWitnesses(InvalidLARIndex))
	if err = s.registerNewExternalIndex(ext); err != nil {
		return j, err
	}
	s.Bounds.Types.Append(ColumnFree)
	if err = s.Bounds.ChangedColumns.IncreaseSizeByOne(); err != nil {
		return j, err
	}
	if err = s.addRationalCoreFields(false); err != nil {
		return j, err
	}
	if s.UseLU() {
		if err = s.addDoubleCoreFields(false); err != nil {
			return j, err
		}
	}
	_, err = s.SizesAreCorrect()
	return j, err
}
func (s *LARSolver) NewBoundPropagator() *LARBoundPropagator {
	return NewLARBoundPropagator(s.Bounds, &s.ColumnsToExternal, &s.Terms)
}
func (s *LARSolver) Close() error {
	if err := s.calls.cleanupRaw(s.manager); err != nil {
		return err
	}
	if s.randomUpdater != nil {
		if err := s.randomUpdater.Close(); err != nil {
			return err
		}
		s.randomUpdater = nil
	}
	if err := s.closeRowAnalysis(); err != nil {
		return err
	}
	if s.validationExplanation != nil {
		if err := s.validationExplanation.Close(); err != nil {
			return err
		}
		s.validationExplanation = nil
	}

	if s.validationCoefficients != nil {
		if err := s.validationCoefficients.Close(); err != nil {
			return err
		}
		s.validationCoefficients = nil
	}
	if s.explanationCoefficients != nil {
		if err := s.explanationCoefficients.Close(); err != nil {
			return err
		}
		s.explanationCoefficients = nil
	}
	if s.validationModel != nil {
		if err := s.validationModel.Close(); err != nil {
			return err
		}
		s.validationModel = nil
	}
	if s.pendingConstraint != nil {
		if err := s.pendingConstraint.Close(); err != nil {
			return err
		}
		s.pendingConstraint = nil
	}
	for len(s.Constraints) > 0 {
		if s.Constraints[0] != nil {
			if err := s.Constraints[0].Close(); err != nil {
				return err
			}
		}
		s.Constraints = s.Constraints[1:]
	}
	if s.pendingIterator != nil {
		if err := s.pendingIterator.Close(); err != nil {
			return err
		}
		s.pendingIterator = nil
	}
	if s.pendingTerm != nil {
		if err := s.pendingTerm.Close(); err != nil {
			return err
		}
		s.pendingTerm = nil
	}
	for len(s.Terms) > 0 {
		if s.Terms[0] != nil {
			if err := s.Terms[0].Close(); err != nil {
				return err
			}
		}
		s.Terms = s.Terms[1:]
	}
	for len(s.OriginalTerms) > 0 {
		if s.OriginalTerms[0] != nil {
			if err := s.OriginalTerms[0].Close(); err != nil {
				return err
			}
		}
		s.OriginalTerms = s.OriginalTerms[1:]
	}
	if s.Core != nil {
		if err := s.Core.Close(); err != nil {
			return err
		}
	}
	if s.pendingCleanupDelta != nil {
		if err := s.pendingCleanupDelta.Del(s.manager); err != nil {
			return err
		}
		s.pendingCleanupDelta = nil
	}
	if s.work != nil {
		if err := s.work.Close(); err != nil {
			return err
		}
		s.work = nil
	}
	if err := s.closeModelSets(); err != nil {
		return err
	}
	if s.rationalWork != nil {
		if err := s.rationalWork.Close(); err != nil {
			return err
		}
		s.rationalWork = nil
	}
	if err := s.ColumnBuffer.CloseForward(); err != nil {
		return err
	}
	if s.certificateResult != nil {
		if err := s.manager.Del(s.certificateResult); err != nil {
			return err
		}
		s.certificateResult = nil
	}
	return s.calls.cleanupRoots(s.manager)
}
