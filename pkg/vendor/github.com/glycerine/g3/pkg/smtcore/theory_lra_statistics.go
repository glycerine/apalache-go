package smtcore

// Source: src/smt/theory_lra.cpp lra_lp::stats, make_feasible and
// collect_statistics. All source counters have unsigned (32-bit) width.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtLRANumericStatistics struct {
	rows, makeFeasible, maxColumns, maxRows uint32
	iterations, factorizations, pivots      uint32
}

type smtLRAStatistics struct {
	conflicts, disequalities, boundsPropagations uint32
}

func (*SMTLRATheory) Name() string { return "lra" }

func (t *SMTLRATheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || t.closed || target == nil {
		return ast.ErrASTHandle
	}
	target.UpdateUint("arith eq adapter", uint32(t.equalityAdapter.NumAxioms()))
	var n smtLRANumericStatistics
	if t.numeric != nil {
		n = t.numeric.statistics
		target.UpdateUint("arith-lower", uint32(t.numeric.assertLower))
		target.UpdateUint("arith-upper", uint32(t.numeric.assertUpper))
	}
	target.UpdateUint("arith-rows", n.rows)
	target.UpdateUint("arith-propagations", t.statistics.boundsPropagations)
	// Native make_feasible samples these values; a later optimization call
	// must not silently change the previously collected frontend statistics.
	target.UpdateUint("arith-iterations", n.iterations)
	target.UpdateUint("arith-factorizations", n.factorizations)
	target.UpdateUint("arith-pivots", n.pivots)
	// m_num_iterations_with_no_progress has no retained writer and stays zero.
	target.UpdateUint("arith-fixed-eqs", uint32(t.fixedEqualities))
	target.UpdateUint("arith-conflicts", t.statistics.conflicts)
	target.UpdateUint("arith-bound-propagations-lp", uint32(t.lpPropagations))
	target.UpdateUint("arith-bound-propagations-cheap", uint32(t.basicPropagations))
	target.UpdateUint("arith-diseq", t.statistics.disequalities)
	target.UpdateUint("arith-make-feasible", n.makeFeasible)
	target.UpdateUint("arith-max-columns", n.maxColumns)
	target.UpdateUint("arith-max-rows", n.maxRows)
	return nil
}
