package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp:67; src/util/trace.h:40-46.

import (
	"io"

	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// BindTrace borrows an isolated trace stream. Nil restores the process stream.
func (t *RewriterTactic) BindTrace(tags *diagnostic.TraceTags) error {
	if e := t.check(); e != nil {
		return e
	}
	t.trace = tags
	return nil
}

func (t *RewriterTactic) tracePublishedGoal(goal *tactic.Goal) error {
	tags := t.trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("ufbv-rewriter") {
		return nil
	}
	return tags.EmitConditional("ufbv-rewriter", "operator()", "src/tactic/ufbv/ufbv_rewriter_tactic.cpp", 67,
		func() bool { return true }, func(w io.Writer) error {
			return goal.Display(w)
		})
}
