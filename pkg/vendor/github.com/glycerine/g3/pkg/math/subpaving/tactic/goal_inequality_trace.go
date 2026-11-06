package tactic

// Source: src/math/subpaving/tactic/subpaving_tactic.cpp:158.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func inequalityTraceTags(tags *diagnostic.TraceTags) *diagnostic.TraceTags {
	if tags != nil {
		return tags
	}
	return diagnostic.BorrowProcessTraceTags()
}

// The source emits this record after the sign-dependent direction change and
// before context mk_ineq. Stream failures stay in the borrowed trace stream.
func emitGoalInequalityTrace(tags *diagnostic.TraceTags, qm *numeral.RationalManager, x uint32, k *numeral.Rational, lower, open bool) error {
	tags = inequalityTraceTags(tags)
	if !tags.ShouldTrace("subpaving_tactic") {
		return nil
	}
	var computationErr error
	_ = tags.EmitConditional("subpaving_tactic", "mk_ineq", "src/math/subpaving/tactic/subpaving_tactic.cpp", 158,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, strconv.FormatUint(uint64(x), 10))
			_, _ = io.WriteString(w, " ")
			var shown string
			shown, computationErr = qm.String(k)
			if computationErr != nil {
				return computationErr
			}
			_, _ = io.WriteString(w, shown)
			if lower {
				_, _ = io.WriteString(w, " 1")
			} else {
				_, _ = io.WriteString(w, " 0")
			}
			if open {
				_, _ = io.WriteString(w, " 1\n")
			} else {
				_, _ = io.WriteString(w, " 0\n")
			}
			return nil
		})
	return computationErr
}

// BindTrace borrows an explicit tag stream for this concrete converter.
// Nil restores the process-global source stream.
func (c *HardwareExprConverter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.check(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}
func (c *FastExprConverter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.check(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}
func (c *ArbitraryExprConverter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.check(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}
func (c *RationalExprConverter) BindTrace(tags *diagnostic.TraceTags) error {
	if err := c.check(); err != nil {
		return err
	}
	c.trace = tags
	return nil
}
