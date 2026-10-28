package apalache

import (
	"math"
	"strings"
)

// RewriterConfig controls symbolic rewriter choices.
type RewriterConfig struct{ ShortCircuit bool }

// DefaultRewriterConfig corresponds to new RewriterConfig in Scala.
func DefaultRewriterConfig() RewriterConfig { return RewriterConfig{ShortCircuit: true} }

// NewRewriterConfig corresponds to RewriterConfig(options). In particular,
// an absent option disables short-circuiting, unlike the class-field default.
func NewRewriterConfig(options map[string]string) RewriterConfig {
	return RewriterConfig{ShortCircuit: strings.ToLower(options["rewriter.shortCircuit"]) == "true"}
}

// Recoverable is implemented by mutable components with restorable snapshots.
type Recoverable[Snapshot any] interface {
	Snapshot() Snapshot
	Recover(Snapshot)
}

// StackableContext is the common push/pop/dispose contract of SMT contexts
// and level-aware caches. Go returns an error in place of Scala assertions.
type StackableContext interface {
	Level() int
	Push()
	Pop(...int) error
	Dispose()
}

// DelayedConstraintGenerator separates arena/cache construction from SMT
// discharge. Callers decide when to encode the returned current-level
// constraints, matching Scala's addAllConstraints lifecycle without coupling
// caches to a solver instance.
type DelayedConstraintGenerator interface {
	Constraints() []Expr
}

// SolverContextMetrics records cumulative SMT construction counts.
type SolverContextMetrics struct {
	NCells          int64
	NConstants      int64
	NSMTExpressions int64
}

func EmptySolverContextMetrics() SolverContextMetrics { return SolverContextMetrics{} }

func (metrics SolverContextMetrics) Weight() int64 {
	expressions := metrics.NSMTExpressions
	if expressions < 0 {
		expressions = 0
	}
	return metrics.NCells + metrics.NConstants + int64(math.Sqrt(float64(expressions)))
}

func (metrics SolverContextMetrics) Delta(earlier SolverContextMetrics) SolverContextMetrics {
	return SolverContextMetrics{
		NCells:          metrics.NCells - earlier.NCells,
		NConstants:      metrics.NConstants - earlier.NConstants,
		NSMTExpressions: metrics.NSMTExpressions - earlier.NSMTExpressions,
	}
}

func (metrics SolverContextMetrics) Add(delta SolverContextMetrics) SolverContextMetrics {
	// Preserve Scala's observable nConsts + delta.nCells behavior.
	return SolverContextMetrics{
		NCells:          metrics.NCells + delta.NCells,
		NConstants:      metrics.NConstants + delta.NCells,
		NSMTExpressions: metrics.NSMTExpressions + delta.NSMTExpressions,
	}
}

func (metrics SolverContextMetrics) AddCells(number int64) SolverContextMetrics {
	metrics.NCells += number
	return metrics
}

func (metrics SolverContextMetrics) AddConstants(number int64) SolverContextMetrics {
	metrics.NConstants += number
	return metrics
}

func (metrics SolverContextMetrics) AddSMTExpressions(number int64) SolverContextMetrics {
	metrics.NSMTExpressions += number
	return metrics
}

// SymbolicStateRewriterListener observes completed expression rewrites.
type SymbolicStateRewriterListener interface {
	OnRewrite(translated Expr, metricsDelta SolverContextMetrics)
	Dispose()
}

// Rewriter is the solver-independent stratified rewriting boundary.
type Rewriter interface {
	Rewrite(expression Expr, startingScope RewriterScope) (RewriterScope, ArenaCell, error)
	Assert(expression Expr)
}
