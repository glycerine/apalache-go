package apalache

import (
	"reflect"
	"testing"
)

func TestRewriterConfigScalaConstructionDistinction(t *testing.T) {
	if !DefaultRewriterConfig().ShortCircuit {
		t.Fatal("class default disabled short-circuiting")
	}
	cases := []struct {
		options map[string]string
		want    bool
	}{
		{nil, false},
		{map[string]string{}, false},
		{map[string]string{"rewriter.shortCircuit": "true"}, true},
		{map[string]string{"rewriter.shortCircuit": "TRUE"}, true},
		{map[string]string{"rewriter.shortCircuit": " true "}, false},
		{map[string]string{"rewriter.shortCircuit": "yes"}, false},
	}
	for _, test := range cases {
		if got := NewRewriterConfig(test.options).ShortCircuit; got != test.want {
			t.Errorf("options %v: got %v, want %v", test.options, got, test.want)
		}
	}
}

func TestSolverContextMetricsExactScalaAlgebra(t *testing.T) {
	if EmptySolverContextMetrics() != (SolverContextMetrics{}) {
		t.Fatal("empty metrics are nonzero")
	}
	metrics := SolverContextMetrics{NCells: 3, NConstants: 5, NSMTExpressions: 15}
	if metrics.Weight() != 11 {
		t.Fatalf("weight = %d", metrics.Weight())
	}
	if got := (SolverContextMetrics{NCells: 3, NConstants: 5, NSMTExpressions: -100}).Weight(); got != 8 {
		t.Fatalf("negative-expression weight = %d", got)
	}
	earlier := SolverContextMetrics{NCells: 1, NConstants: 2, NSMTExpressions: 4}
	if got, want := metrics.Delta(earlier), (SolverContextMetrics{NCells: 2, NConstants: 3, NSMTExpressions: 11}); got != want {
		t.Fatalf("delta = %+v, want %+v", got, want)
	}
	delta := SolverContextMetrics{NCells: 7, NConstants: 13, NSMTExpressions: 17}
	// Scala currently adds delta.nCells to nConsts; this is deliberately
	// oracle-tested rather than silently corrected in the port.
	if got, want := metrics.Add(delta), (SolverContextMetrics{NCells: 10, NConstants: 12, NSMTExpressions: 32}); got != want {
		t.Fatalf("add = %+v, want Scala %+v", got, want)
	}
	if got := metrics.AddCells(2); got != (SolverContextMetrics{5, 5, 15}) {
		t.Fatalf("add cells = %+v", got)
	}
	if got := metrics.AddConstants(2); got != (SolverContextMetrics{3, 7, 15}) {
		t.Fatalf("add constants = %+v", got)
	}
	if got := metrics.AddSMTExpressions(2); got != (SolverContextMetrics{3, 5, 17}) {
		t.Fatalf("add expressions = %+v", got)
	}
}

type recordingRewriterListener struct {
	expressions []Expr
	metrics     []SolverContextMetrics
	disposed    bool
}

func (listener *recordingRewriterListener) OnRewrite(expression Expr, metrics SolverContextMetrics) {
	listener.expressions = append(listener.expressions, expression)
	listener.metrics = append(listener.metrics, metrics)
}

func (listener *recordingRewriterListener) Dispose() { listener.disposed = true }

func TestSymbolicStateRewriterListenerPayloadsAndDispose(t *testing.T) {
	var listener SymbolicStateRewriterListener = &recordingRewriterListener{}
	expression := NewNameExpr("x", Untyped())
	delta := SolverContextMetrics{1, 2, 3}
	listener.OnRewrite(expression, delta)
	listener.Dispose()
	recording := listener.(*recordingRewriterListener)
	if !reflect.DeepEqual(recording.expressions, []Expr{expression}) ||
		!reflect.DeepEqual(recording.metrics, []SolverContextMetrics{delta}) || !recording.disposed {
		t.Fatalf("listener = %+v", recording)
	}
}

func TestExistingCachesImplementRecoverable(t *testing.T) {
	var _ Recoverable[ExpressionCacheSnapshot] = (*ExpressionCache)(nil)
	var _ Recoverable[EqualityCacheSnapshot] = (*EqualityCache)(nil)
}

func TestStratifiedRewriterImplementsRewriterContract(t *testing.T) {
	var rewriter Rewriter = NewStratifiedRewriter()
	constraint := NewValueExpr(BoolValue(true), Typed(BoolT1))
	rewriter.Assert(constraint)
	concrete := rewriter.(*StratifiedRewriter)
	if got := concrete.Constraints(); len(got) != 1 || got[0] != constraint {
		t.Fatalf("asserted constraints = %+v", got)
	}
	scope, cell, err := rewriter.Rewrite(NewValueExpr(Int64Value(1), Typed(IntT1)), InitialRewriterScope())
	if err != nil || scope.Arena == nil || cell.ID < 0 {
		t.Fatalf("rewrite = scope %+v, cell %v, error %v", scope, cell, err)
	}
}
