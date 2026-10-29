package interval

// Source: src/math/interval/interval_def.h:206-210,315-320;
// concrete precise config_mpq instantiation, src/util/trace.h.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (r RationalRoot) traceTags() *diagnostic.TraceTags {
	if r.TraceTags != nil {
		return r.TraceTags
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (r RationalRoot) traceNthRoot(a *numeral.Rational, n uint32, lo, hi *numeral.Rational) error {
	tags := r.traceTags()
	if !tags.ShouldTrace("nth_root_trace") {
		return nil
	}
	var computationErr error
	_ = tags.EmitShort("nth_root_trace", func(w io.Writer) error {
		s := &rootTraceOperandWriter{w: w}
		for _, bound := range [...]struct {
			value *numeral.Rational
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

func (r RationalRoot) traceNthRootIteration(a, x, xPrime, d *numeral.Rational) error {
	tags := r.traceTags()
	if !tags.ShouldTrace("nth_root") {
		return nil
	}
	var computationErr error
	_ = tags.EmitConditional("nth_root", "approx_nth_root", "src/math/interval/interval_def.h", 315,
		func() bool { return true }, func(w io.Writer) error {
			s := &rootTraceOperandWriter{w: w}
			for _, field := range [...]struct {
				label string
				value *numeral.Rational
			}{{"A:       ", a}, {"x:       ", x}, {"x_prime: ", xPrime}, {"d:       ", d}} {
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
