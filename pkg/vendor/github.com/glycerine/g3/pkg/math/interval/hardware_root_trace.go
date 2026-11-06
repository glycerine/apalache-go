package interval

// Source: src/math/interval/interval_def.h:206-210,315-320,344-348;
// concrete config_hwf instantiation, src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (r HardwareRoot) traceTags() *diagnostic.TraceTags {
	if r.TraceTags != nil {
		return r.TraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (r HardwareRoot) traceNthRoot(a *numeral.HardwareFloat, n uint32, lo, hi *numeral.HardwareFloat) error {
	tags := r.traceTags()
	if !tags.ShouldTrace("nth_root_trace") {
		return nil
	}
	var formatErr error
	_ = tags.EmitShort("nth_root_trace", func(w io.Writer) error {
		write := func(s string) { _, _ = io.WriteString(w, s) }
		display := func(v *numeral.HardwareFloat) bool {
			value, e := r.Numbers.ToString(v)
			if e != nil {
				formatErr = e
				return false
			}
			write(value)
			return true
		}
		for _, item := range [...]struct {
			bound *numeral.HardwareFloat
			op    string
		}{{lo, " >= "}, {hi, " <= "}} {
			write("[nth-root] (")
			if !display(a) {
				return formatErr
			}
			write(")^(1/")
			write(strconv.FormatUint(uint64(n), 10))
			write(")")
			write(item.op)
			if !display(item.bound) {
				return formatErr
			}
			write("\n")
		}
		return nil
	})
	return formatErr
}

type hardwareRootTraceField struct {
	label string
	value *numeral.HardwareFloat
}

func (r HardwareRoot) traceRecord(tag, function string, line int, prefix string, fields ...hardwareRootTraceField) error {
	tags := r.traceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional(tag, function, "src/math/interval/interval_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, prefix)
			for _, field := range fields {
				_, _ = io.WriteString(w, field.label)
				value, e := r.Numbers.ToString(field.value)
				if e != nil {
					formatErr = e
					return e
				}
				_, _ = io.WriteString(w, value)
				_, _ = io.WriteString(w, "\n")
			}
			return nil
		})
	return formatErr
}

func (r HardwareRoot) traceNthRootIteration(a, x, xPrime, d *numeral.HardwareFloat) error {
	return r.traceRecord("nth_root", "approx_nth_root", 315, "",
		hardwareRootTraceField{"A:       ", a}, hardwareRootTraceField{"x:       ", x},
		hardwareRootTraceField{"x_prime: ", xPrime}, hardwareRootTraceField{"d:       ", d})
}

func (r HardwareRoot) traceNthRootBoundCheck(a, hi *numeral.HardwareFloat) error {
	return r.traceRecord("nth_root_bug", "nth_root_pos", 344, "Assuming upper\n",
		hardwareRootTraceField{"A: ", a}, hardwareRootTraceField{"hi: ", hi},
		hardwareRootTraceField{"lo: ", hi})
}
