package lp

// Source: src/util/lp/lar_core_solver.h/.hpp, retained LAR constructor/state.

import "github.com/glycerine/g3/pkg/util/numeral"

// LARCoreSolver owns the real exact/floating model vectors and primal cores.
// Bounds composes exact bounds/types with their outer LAR witness/status state;
// LARSolver borrows that same owner, so there is one set of bounds and no copies.
// Settings, names and numeral manager are borrowed. Full LAR Solve remains open.
// Do not copy. A partial constructor result must be closed.
type LARCoreSolver struct {
	Bounds                                  *LARBounds
	RA                                      *RationalStaticMatrix
	RX, RightSides                          *IndexedPairVector
	Costs                                   *IndexedRationalVector
	RBasis, RNBasis                         []uint32
	RHeading                                []int32
	RPushedBasis, RColumnsNZ, RRowsNZ       StackedUintVector
	DA                                      *FloatStaticMatrix
	DX, DLow, DUpper, DRightSides, DCosts   []float64
	DBasis, DNBasis                         []uint32
	DHeading                                []int32
	DPushedBasis                            StackedUintVector
	RPrimal                                 *PairPrimalCoreSolver
	DPrimal                                 *FloatPrimalCoreSolver
	StackedSimplexStrategy                  StackedStrategy
	Infeasibility                           *SparseRationalVector
	InfeasibleSumSign                       int32
	evidenceNegated                         *IndexedRationalVector
	deltaWork, strictDelta, conversionDelta *IndexedRationalVector
	catchUpWork                             *IndexedRationalVector
	signatureDelta, signatureExpression     *IndexedPairVector
	signatureScalars                        *IndexedRationalVector
	calls                                   exactEtaCalls
	settings                                *Settings
	manager                                 *numeral.RationalManager
	stopped                                 bool
}

func NewLARCoreSolver(m *numeral.RationalManager, s *Settings, names ColumnNamer) (core *LARCoreSolver, err error) {
	if m == nil || s == nil {
		return nil, ErrBounds
	}
	core = &LARCoreSolver{Bounds: NewLARBounds(m), RA: NewRationalStaticMatrix(m, 0, 0), RX: NewIndexedPairVector(m, 0), RightSides: NewIndexedPairVector(m, 0), Costs: NewIndexedRationalVector(m, 0), DA: NewFloatStaticMatrix(0, 0), Infeasibility: NewSparseRationalVector(m), settings: s, manager: m}
	defer func() { core.stopped = err != nil }()
	core.RPrimal, err = NewPairPrimalCoreSolver(PairCoreInputs{A: core.RA, B: &core.RightSides.Data, X: &core.RX.Data, Costs: &core.Costs.Data, Basis: &core.RBasis, NBasis: &core.RNBasis, Heading: &core.RHeading, ColumnTypes: &core.Bounds.Types.values, LowBounds: &core.Bounds.Low.values, UpperBounds: &core.Bounds.Upper.values, Settings: s, Names: names})
	if err != nil {
		return core, err
	}
	core.DPrimal, err = NewFloatPrimalCoreSolver(FloatCoreInputs{A: core.DA, B: &core.DRightSides, X: &core.DX, Costs: &core.DCosts, Basis: &core.DBasis, NBasis: &core.DNBasis, Heading: &core.DHeading, ColumnTypes: &core.Bounds.Types.values, LowBounds: &core.DLow, UpperBounds: &core.DUpper, Settings: s, Names: names})
	return core, err
}
func (c *LARCoreSolver) NeedToPresolveWithDoubleSolver() bool {
	return c.settings.SimplexStrategy == StrategyLU
}
func (c *LARCoreSolver) IsTiny() bool  { return c.RA.RowCount() < 10 && c.RA.ColumnCount() < 20 }
func (c *LARCoreSolver) IsEmpty() bool { return c.RA.RowCount() == 0 && c.RA.ColumnCount() == 0 }
func (c *LARCoreSolver) Close() error {
	if err := c.calls.cleanupRaw(c.manager); err != nil {
		return err
	}
	for _, local := range []**IndexedRationalVector{&c.deltaWork, &c.strictDelta, &c.conversionDelta} {
		if *local != nil {
			if err := (*local).Close(); err != nil {
				return err
			}
			*local = nil
		}
	}
	// A stopped evidence append can still borrow its negated local or matrix.
	if err := c.Infeasibility.Close(); err != nil {
		return err
	}
	if c.evidenceNegated != nil {
		if err := c.evidenceNegated.Close(); err != nil {
			return err
		}
		c.evidenceNegated = nil
	}
	if c.DPrimal != nil {
		if err := c.DPrimal.Close(); err != nil {
			return err
		}
	}
	if c.RPrimal != nil {
		if err := c.RPrimal.Close(); err != nil {
			return err
		}
	}
	// A stopped LU replacement can still borrow the catch-up column.
	if c.catchUpWork != nil {
		if err := c.catchUpWork.CloseForward(); err != nil {
			return err
		}
		c.catchUpWork = nil
	}
	// A stopped primal validator may still borrow values reached while the
	// tableau signature's lexical delta and expressions were live.
	for _, local := range []**IndexedPairVector{&c.signatureExpression, &c.signatureDelta} {
		if *local != nil {
			if err := (*local).Close(); err != nil {
				return err
			}
			*local = nil
		}
	}
	if c.signatureScalars != nil {
		if err := c.signatureScalars.Close(); err != nil {
			return err
		}
		c.signatureScalars = nil
	}
	c.DA.Clear()
	if err := c.RA.Close(); err != nil {
		return err
	}
	if err := c.Bounds.Close(); err != nil {
		return err
	}
	if err := c.RX.Close(); err != nil {
		return err
	}
	if err := c.Costs.Close(); err != nil {
		return err
	}
	if err := c.RightSides.Close(); err != nil {
		return err
	}
	return c.calls.cleanupRoots(c.manager)
}
