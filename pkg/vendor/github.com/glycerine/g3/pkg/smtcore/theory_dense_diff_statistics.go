package smtcore

// Source: src/smt/theory_dense_diff_logic.h:32-42 and
// src/smt/theory_dense_diff_logic_def.h:263,565-576,705-709.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/statistics"
)

type smtDenseDiffStatistics struct {
	assertions, propagations uint32
}

// CollectStatistics retains theory_dense_diff_logic::collect_statistics.
func (t *SMTDenseDiffTheory) CollectStatistics(target *statistics.Statistics) error {
	if t == nil || target == nil {
		return ast.ErrASTHandle
	}
	if _, err := t.State(); err != nil {
		return err
	}
	target.UpdateUint("dd assertions", t.statistics.assertions)
	target.UpdateUint("dd propagations", t.statistics.propagations)
	target.UpdateUint64("arith eq adapter", t.adapter.NumAxioms())
	return nil
}
