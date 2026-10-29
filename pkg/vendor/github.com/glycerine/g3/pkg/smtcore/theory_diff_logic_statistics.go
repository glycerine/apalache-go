package smtcore

// Source: src/smt/theory_diff_logic_def.h:312-319 and
// src/smt/diff_logic.h:123-144.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

func collectSMTDiffStatistics(target *statistics.Statistics, counters smtDiffStatistics, adapter *smtArithmeticEqualityAdapter, propagationCost, relaxations uint32) error {
	if target == nil || adapter == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint("dl conflicts", counters.conflicts)
	target.UpdateUint("dl asserts", counters.assertions)
	target.UpdateUint("core->dl eqs", counters.coreEqualities)
	target.UpdateUint("core->dl diseqs", counters.coreDisequalities)
	target.UpdateUint64("arith eq adapter", adapter.NumAxioms())
	target.UpdateUint("dl prop steps", propagationCost)
	// The retained difference owners never invoke the source implied-bound
	// discovery entry point. Its three counters therefore remain zero.
	target.UpdateUint("dl bound relax", relaxations)
	return nil
}

// CollectStatistics retains theory_diff_logic::collect_statistics for the
// concrete integer graph.
func (t *SMTIntegerDiffTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || t.graph == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	return collectSMTDiffStatistics(target, t.statistics, t.equalityAdapter, t.graph.propagationCost, t.graph.numRelax)
}

// CollectStatistics retains theory_diff_logic::collect_statistics for the
// concrete real/mixed graph.
func (t *SMTRealDiffTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || t.realGraph == nil || t.SMTIntegerDiffTheory == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	return collectSMTDiffStatistics(target, t.realStatistics, t.equalityAdapter, t.realGraph.propagationCost, 0)
}
