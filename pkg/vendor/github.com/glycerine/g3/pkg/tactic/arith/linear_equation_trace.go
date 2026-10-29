package arith

// Source: src/tactic/arith/linear_equation.cpp:71,76,78,99,121.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/numeral"
)

// BindTrace borrows the stream used by the source TRACE sites. Nil restores
// the process trace stream.
func (l *LinearEquationManager) BindTrace(tags *diagnostic.TraceTags) error {
	if err := l.open(); err != nil {
		return err
	}
	l.trace = tags
	return nil
}

func (l *LinearEquationManager) equationTraceTags() *diagnostic.TraceTags {
	if l.trace != nil {
		return l.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (l *LinearEquationManager) emitEquationTrace(tag, function string, line int, body string) {
	_ = l.equationTraceTags().EmitConditional(tag, function, "src/tactic/arith/linear_equation.cpp", line,
		func() bool { return true }, func(w io.Writer) error {
			_, err := io.WriteString(w, body)
			return err
		})
}

func (l *LinearEquationManager) traceEquationLCM(v *numeral.Int) error {
	if !l.equationTraceTags().ShouldTrace("linear_equation_mk") {
		return nil
	}
	s, err := l.manager.Integers().String(v)
	if err != nil {
		return err
	}
	l.emitEquationTrace("linear_equation_mk", "mk", 71, "lcm: "+s+"\n")
	return nil
}

func (l *LinearEquationManager) traceEquationRational(i int, v *numeral.Rational, after bool) error {
	if !l.equationTraceTags().ShouldTrace("linear_equation_mk") {
		return nil
	}
	s, err := l.manager.String(v)
	if err != nil {
		return err
	}
	label, line := "before", 76
	if after {
		label, line = "after", 78
	}
	l.emitEquationTrace("linear_equation_mk", "mk", line,
		label+" as["+strconv.Itoa(i)+"]: "+s+"\n")
	return nil
}

func (l *LinearEquationManager) traceEquationCore(as []numeral.Int, xs []uint32, gcd *numeral.Int) error {
	if !l.equationTraceTags().ShouldTrace("linear_equation_bug") {
		return nil
	}
	line, body := 99, ""
	if gcd != nil {
		s, err := l.manager.Integers().String(gcd)
		if err != nil {
			return err
		}
		line, body = 121, "g: "+s+"\n"
	}
	for i := range as {
		s, err := l.manager.Integers().String(&as[i])
		if err != nil {
			return err
		}
		body += s + "*x" + strconv.FormatUint(uint64(xs[i]), 10) + " "
	}
	l.emitEquationTrace("linear_equation_bug", "mk_core", line, body+"\n")
	return nil
}
