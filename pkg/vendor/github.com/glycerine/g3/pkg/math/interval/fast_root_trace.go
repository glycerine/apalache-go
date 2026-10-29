package interval

// Source: src/math/interval/interval_def.h:206-210,315-320,344-348;
// concrete config_mpff instantiation, src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (r FastRoot) traceTags() *diagnostic.TraceTags {
	if r.TraceTags != nil {
		return r.TraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (r FastRoot) traceNthRoot(a *numeral.FastFloat, n uint32, lo, hi *numeral.FastFloat) error {
	tags := r.traceTags()
	if !tags.ShouldTrace("nth_root_trace") {
		return nil
	}
	var computationErr error
	_ = tags.EmitShort("nth_root_trace", func(w io.Writer) error {
		s := &rootTraceOperandWriter{w: w}
		for _, bound := range [...]struct {
			value *numeral.FastFloat
			op    string
		}{{lo, " >= "}, {hi, " <= "}} {
			_, _ = io.WriteString(s, "[nth-root] (")
			if computationErr = s.operand(r.Numbers.DisplayTo(s, a)); computationErr != nil {
				return computationErr
			}
			_, _ = io.WriteString(s, ")^(1/")
			_, _ = io.WriteString(s, strconv.FormatUint(uint64(n), 10))
			_, _ = io.WriteString(s, ")"+bound.op)
			if computationErr = s.operand(r.Numbers.DisplayTo(s, bound.value)); computationErr != nil {
				return computationErr
			}
			_, _ = io.WriteString(s, "\n")
		}
		return nil
	})
	return computationErr
}

type fastRootTraceField struct {
	label string
	value *numeral.FastFloat
}

func (r FastRoot) traceRecord(tag, function string, line int, prefix string, fields ...fastRootTraceField) error {
	tags := r.traceTags()
	if !tags.ShouldTrace(tag) {
		return nil
	}
	var computationErr error
	_ = tags.EmitConditional(tag, function, "src/math/interval/interval_def.h", line,
		func() bool { return true }, func(w io.Writer) error {
			s := &rootTraceOperandWriter{w: w}
			_, _ = io.WriteString(s, prefix)
			for _, field := range fields {
				_, _ = io.WriteString(s, field.label)
				if computationErr = s.operand(r.Numbers.DisplayTo(s, field.value)); computationErr != nil {
					return computationErr
				}
				_, _ = io.WriteString(s, "\n")
			}
			return nil
		})
	return computationErr
}

func (r FastRoot) traceNthRootIteration(a, x, xPrime, d *numeral.FastFloat) error {
	return r.traceRecord("nth_root", "approx_nth_root", 315, "",
		fastRootTraceField{"A:       ", a}, fastRootTraceField{"x:       ", x},
		fastRootTraceField{"x_prime: ", xPrime}, fastRootTraceField{"d:       ", d})
}

func (r FastRoot) traceNthRootBoundCheck(a, hi *numeral.FastFloat) error {
	return r.traceRecord("nth_root_bug", "nth_root_pos", 344, "Assuming upper\n",
		fastRootTraceField{"A: ", a}, fastRootTraceField{"hi: ", hi},
		fastRootTraceField{"lo: ", hi})
}
