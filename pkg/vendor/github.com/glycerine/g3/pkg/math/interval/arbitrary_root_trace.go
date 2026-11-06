package interval

// Source: src/math/interval/interval_def.h:208-210,315-320,344-348; src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (r ArbitraryRoot) traceNthRoot(a *numeral.ArbitraryFloat, n uint32, lo, hi *numeral.ArbitraryFloat) error {
	tags := r.TraceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace("nth_root_trace") {
		return nil
	}
	var formatErr error
	_ = tags.EmitShort("nth_root_trace", func(w io.Writer) error {
		write := func(s string) { _, _ = io.WriteString(w, s) }
		display := func(v *numeral.ArbitraryFloat) bool {
			text, e := r.Numbers.ToString(v)
			if e != nil {
				formatErr = e
				return false
			}
			write(text)
			return true
		}
		for _, item := range [...]struct {
			bound *numeral.ArbitraryFloat
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

type rootTraceField struct {
	label string
	value *numeral.ArbitraryFloat
}

func (r ArbitraryRoot) traceRootRecord(tag, function string, line int, prefix string, fields ...rootTraceField) error {
	tags := r.TraceTags
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var formatErr error
	_ = tags.EmitConditional(tag, function, "src/math/interval/interval_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			_, _ = io.WriteString(w, prefix)
			for _, field := range fields {
				_, _ = io.WriteString(w, field.label)
				text, e := r.Numbers.ToString(field.value)
				if e != nil {
					formatErr = e
					return e
				}
				_, _ = io.WriteString(w, text)
				_, _ = io.WriteString(w, "\n")
			}
			return nil
		})
	return formatErr
}

func (r ArbitraryRoot) traceNthRootIteration(a, x, xPrime, d *numeral.ArbitraryFloat) error {
	return r.traceRootRecord("nth_root", "approx_nth_root", 315, "",
		rootTraceField{"A:       ", a}, rootTraceField{"x:       ", x},
		rootTraceField{"x_prime: ", xPrime}, rootTraceField{"d:       ", d})
}

func (r ArbitraryRoot) traceNthRootBoundCheck(a, hi *numeral.ArbitraryFloat) error {
	return r.traceRootRecord("nth_root_bug", "nth_root_pos", 344, "Assuming upper\n",
		rootTraceField{"A: ", a}, rootTraceField{"hi: ", hi},
		rootTraceField{"lo: ", hi})
}
