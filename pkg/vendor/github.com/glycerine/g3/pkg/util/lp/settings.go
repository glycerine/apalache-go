package lp

// Source: src/util/lp/lp_settings.h, concrete settings/state and float predicates.

import (
	"io"
	"math"
	"os"

	z3rand "github.com/glycerine/g3/pkg/util/random"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// ResourceLimit is the actual borrowed cancellation service. The error adapts
// failed host clock reads; source timer semantics and strict > are retained.
type ResourceLimit interface{ CancelFlag() (bool, error) }
type Stats struct {
	TotalIterations, IterationsWithoutCostGrowth, Factorizations, ImpliedBounds, NeedToSolveInf uint32
}

func (s *Stats) Reset() { *s = Stats{} }

// Settings owns the source default resource timer, random generator and stats.
// Public numerical fields remain mutable. Construct with NewSettings for the
// source defaults and running process-time watch; do not shallow-copy afterward.
type Settings struct {
	watch                                                                                    stopwatch.Stopwatch
	limit                                                                                    ResourceLimit
	random                                                                                   z3rand.Generator
	DebugOut, MessageOut                                                                     io.Writer
	Stats                                                                                    Stats
	RepsInScaler                                                                             uint32
	PivotEpsilon, PositivePriceEpsilon, EnteringDiagEpsilon                                  float64
	PartialPivotingConstant                                                                  int32
	DepthOfRookSearch                                                                        uint32
	UsingPartialPivoting                                                                     bool
	RefactorTolerance, PivotTolerance, ZeroTolerance, DropTolerance                          float64
	ToleranceForArtificials, CanBeTakenToBasisTolerance                                      float64
	PercentOfEnteringToCheck                                                                 uint32
	UseScaling                                                                               bool
	ScalingMaximum, ScalingMinimum, HarrisFeasibilityTolerance, IgnoreEpsilonOfHarris        float64
	MaxIterationsWithoutImprovements, MaxTotalIterations                                     uint32
	TimeLimit                                                                                float64
	DualFeasibilityTolerance, PrimalFeasibilityTolerance, RelativePrimalFeasibilityTolerance float64
	BoundPropagation, PresolveWithDoubleSolverForLAR                                         bool
	SimplexStrategy                                                                          SimplexStrategy
	ReportFrequency                                                                          int32
	PrintStatistics                                                                          bool
	ColumnNormsUpdateFrequency                                                               uint32
	ScaleWithRatio                                                                           bool
	DensityThreshold                                                                         float64
	UseBreakpointsInFeasibilitySearch                                                        bool
	MaxRowLengthForBoundPropagation                                                          uint32
	BackupCosts                                                                              bool
	ColumnThresholdForLUInLAR                                                                uint32
}

func NewSettings() (*Settings, error) {
	s := &Settings{}
	// Source constructs/starts the default resource timer before the remaining
	// settings members. Preserve that start boundary during initialization.
	if err := s.watch.Start(); err != nil {
		return s, err
	}
	*s = Settings{watch: s.watch,
		DebugOut: os.Stdout, MessageOut: os.Stdout,
		RepsInScaler: 20, PivotEpsilon: 1e-8, PositivePriceEpsilon: 1e-7, EnteringDiagEpsilon: 1e-8,
		PartialPivotingConstant: 10, DepthOfRookSearch: 4, UsingPartialPivoting: true,
		RefactorTolerance: 1e-4, PivotTolerance: 1e-6, ZeroTolerance: 1e-12, DropTolerance: 1e-14,
		ToleranceForArtificials: 1e-4, CanBeTakenToBasisTolerance: 1e-5,
		PercentOfEnteringToCheck: 5, UseScaling: true, ScalingMaximum: 1, ScalingMinimum: 0.5,
		HarrisFeasibilityTolerance: 1e-7, IgnoreEpsilonOfHarris: 10e-5,
		MaxIterationsWithoutImprovements: 2000000, MaxTotalIterations: 20000000, TimeLimit: math.MaxFloat64,
		DualFeasibilityTolerance: 1e-7, PrimalFeasibilityTolerance: 1e-7, RelativePrimalFeasibilityTolerance: 1e-9,
		BoundPropagation: true, PresolveWithDoubleSolverForLAR: true, SimplexStrategy: StrategyTableauRows,
		ReportFrequency: 1000, PrintStatistics: false, ColumnNormsUpdateFrequency: 12000,
		ScaleWithRatio: true, DensityThreshold: 0.7, UseBreakpointsInFeasibilitySearch: false,
		MaxRowLengthForBoundPropagation: 300, BackupCosts: true, ColumnThresholdForLUInLAR: 4000,
	}
	return s, nil
}
func (s *Settings) SetResourceLimit(limit ResourceLimit) { s.limit = limit }
func (s *Settings) CancelFlag() (bool, error) {
	if s.limit != nil {
		return s.limit.CancelFlag()
	}
	elapsed, err := s.watch.CurrentSeconds()
	return elapsed > s.TimeLimit, err
}
func (s *Settings) RandomNext() uint32     { return s.random.Next() }
func (s *Settings) RandomSeed(seed uint32) { s.random.SetSeed(seed) }
func (s *Settings) UseLU() bool            { return s.SimplexStrategy == StrategyLU }
func (s *Settings) UseTableau() bool {
	return s.SimplexStrategy == StrategyTableauRows || s.SimplexStrategy == StrategyTableauCosts
}
func (s *Settings) UseTableauRows() bool { return s.SimplexStrategy == StrategyTableauRows }

// FloatEpsilonSmall is the literal convert_struct<double,double> predicate.
// Both bounds are strict, so NaN, zero/negative eps and boundary values survive.
func FloatEpsilonSmall(value, eps float64) bool { return value < eps && value > -eps }
func (s *Settings) FloatDualFeasibilitySmall(value float64) bool {
	return FloatEpsilonSmall(value, s.DualFeasibilityTolerance)
}
func (s *Settings) FloatPrimalFeasibilitySmall(value float64) bool {
	return FloatEpsilonSmall(value, s.PrimalFeasibilityTolerance)
}
func (s *Settings) FloatCanBeTakenToBasisSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.CanBeTakenToBasisTolerance)
}
func (s *Settings) FloatDropSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.DropTolerance)
}
func (s *Settings) FloatZeroSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.ZeroTolerance)
}
func (s *Settings) FloatRefactorSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.RefactorTolerance)
}
func (s *Settings) FloatPivotSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.PivotTolerance)
}
func (s *Settings) FloatHarrisSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.HarrisFeasibilityTolerance)
}
func (s *Settings) FloatIgnoreHarrisSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.IgnoreEpsilonOfHarris)
}
func (s *Settings) FloatArtificialSmall(value float64) bool {
	return FloatEpsilonSmall(value, s.ToleranceForArtificials)
}
