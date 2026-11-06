package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp:39,68.

import (
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func assertRewriterGoalWellSorted(goal *tactic.Goal, line int32) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	good, err := goal.IsWellSorted()
	if err != nil || good {
		return err
	}
	return diagnostic.DebugAssertionFailure(debug, "src/tactic/ufbv/ufbv_rewriter_tactic.cpp", line, "g->is_well_sorted()")
}
