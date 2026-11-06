package subpaving

// Source: src/math/interval/interval_def.h:1427,1459,1470;
// concrete config_hwf instantiation, src/util/trace.h.

import (
	"io"

	"github.com/glycerine/g3/pkg/math/interval"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func hardwareIntervalRootTraceTags(roots interval.HardwareRoot) *diagnostic.TraceTags {
	if roots.TraceTags != nil {
		return roots.TraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (v *HardwareInterval) traceNthRoot(src *HardwareInterval, roots interval.HardwareRoot) {
	tags := hardwareIntervalRootTraceTags(roots)
	if !tags.ShouldTrace("interval_nth_root") {
		return
	}
	_ = tags.EmitConditional("interval_nth_root", "nth_root", "src/math/interval/interval_def.h", 1427,
		func() bool { return true }, func(w io.Writer) error {
			_ = src.Display(w)
			_, _ = io.WriteString(w, " --> ")
			_ = v.Display(w)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}

func (v *HardwareInterval) traceXnEqYBounds(src *HardwareInterval, lo, hi *numeral.HardwareFloat, roots interval.HardwareRoot) {
	tags := hardwareIntervalRootTraceTags(roots)
	if !tags.ShouldTrace("interval_xn_eq_y") {
		return
	}
	_ = tags.EmitConditional("interval_xn_eq_y", "xn_eq_y", "src/math/interval/interval_def.h", 1459,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "x^n = ")
			_ = src.Display(w)
			_, _ = io.WriteString(w, "\nsqrt(y) in ")
			_ = roots.Numbers.DisplayTo(w, lo)
			_, _ = io.WriteString(w, " ")
			_ = roots.Numbers.DisplayTo(w, hi)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}

func (v *HardwareInterval) traceXnEqYResult(roots interval.HardwareRoot) {
	tags := hardwareIntervalRootTraceTags(roots)
	if !tags.ShouldTrace("interval_xn_eq_y") {
		return
	}
	_ = tags.EmitConditional("interval_xn_eq_y", "xn_eq_y", "src/math/interval/interval_def.h", 1470,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, "interval for x: ")
			_ = v.Display(w)
			_, _ = io.WriteString(w, "\n")
			return nil
		})
}
