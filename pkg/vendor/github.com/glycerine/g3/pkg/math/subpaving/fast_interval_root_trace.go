package subpaving

// Source: src/math/interval/interval_def.h:1427,1459,1470;
// concrete config_mpff instantiation, src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func fastIntervalTraceTags(roots interval.FastRoot) *diagnostic.TraceTags {
	if roots.TraceTags != nil {
		return roots.TraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func fastIntervalRootRecord(roots interval.FastRoot, tag, function string, line int, body func(*intervalRootTraceStream) error) error {
	tags := fastIntervalTraceTags(roots)
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var computationErr error
	_ = tags.EmitConditional(tag, function, "src/math/interval/interval_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			computationErr = body(&intervalRootTraceStream{w: w})
			return computationErr
		})
	return computationErr
}

func (v *FastInterval) traceFastNthRoot(src *FastInterval, roots interval.FastRoot) error {
	return fastIntervalRootRecord(roots, "interval_nth_root", "nth_root", 1427,
		func(s *intervalRootTraceStream) error {
			if err := s.operand(src.Display(s)); err != nil {
				return err
			}
			_, _ = io.WriteString(s, " --> ")
			if err := s.operand(v.Display(s)); err != nil {
				return err
			}
			_, _ = io.WriteString(s, "\n")
			return nil
		})
}

func (v *FastInterval) traceFastXnEqYBounds(src *FastInterval, lo, hi *numeral.FastFloat, roots interval.FastRoot) error {
	return fastIntervalRootRecord(roots, "interval_xn_eq_y", "xn_eq_y", 1459,
		func(s *intervalRootTraceStream) error {
			_, _ = io.WriteString(s, "x^n = ")
			if err := s.operand(src.Display(s)); err != nil {
				return err
			}
			_, _ = io.WriteString(s, "\nsqrt(y) in ")
			if err := s.operand(roots.Numbers.DisplayTo(s, lo)); err != nil {
				return err
			}
			_, _ = io.WriteString(s, " ")
			if err := s.operand(roots.Numbers.DisplayTo(s, hi)); err != nil {
				return err
			}
			_, _ = io.WriteString(s, "\n")
			return nil
		})
}

func (v *FastInterval) traceFastXnEqYResult(roots interval.FastRoot) error {
	return fastIntervalRootRecord(roots, "interval_xn_eq_y", "xn_eq_y", 1470,
		func(s *intervalRootTraceStream) error {
			_, _ = io.WriteString(s, "interval for x: ")
			if err := s.operand(v.Display(s)); err != nil {
				return err
			}
			_, _ = io.WriteString(s, "\n")
			return nil
		})
}
