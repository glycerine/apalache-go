package numeral

// Source: src/util/mpzzp.h:55,175,177,179.

import (
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (m *IntegerModularManager) BindTrace(tags *diagnostic.TraceTags) error {
	if e := m.check(); e != nil {
		return e
	}
	m.trace = tags
	return nil
}

func (m *IntegerModularManager) modularTraceTags() *diagnostic.TraceTags {
	if m.trace != nil {
		return m.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

// std::ofstream keeps its failbit after a refused insertion, but later
// to_string operands in a C++ << expression are still evaluated.
type modularTraceBody struct {
	writer io.Writer
	failed error
}

func (b *modularTraceBody) part(s string) {
	if b.failed != nil {
		return
	}
	n, err := io.WriteString(b.writer, s)
	if err != nil {
		b.failed = err
	} else if n != len(s) {
		b.failed = io.ErrShortWrite
	}
}

func (m *IntegerModularManager) traceSetupP() error {
	var numberErr error
	_ = m.modularTraceTags().EmitConditional("mpzzp", "setup_p", "src/util/mpzzp.h", 55,
		func() bool { return true }, func(w io.Writer) error {
			b := modularTraceBody{writer: w}
			b.part("lower: ")
			lower, e := m.manager.String(&m.Lower)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(lower)
			b.part(", upper: ")
			upper, e := m.manager.String(&m.Upper)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(upper)
			b.part("\n")
			return b.failed
		})
	return numberErr
}

func (m *IntegerModularManager) traceInvStart(a *Int) error {
	var numberErr error
	_ = m.modularTraceTags().EmitConditional("mpzp_inv_bug", "inv", "src/util/mpzzp.h", 175,
		func() bool { return true }, func(w io.Writer) error {
			b := modularTraceBody{writer: w}
			b.part("a: ")
			value, e := m.manager.String(a)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(value)
			b.part(", p: ")
			prime, e := m.manager.String(&m.P)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(prime)
			b.part("\n")
			return b.failed
		})
	return numberErr
}

func (m *IntegerModularManager) traceInvNormalized(a *Int) error {
	var numberErr error
	_ = m.modularTraceTags().EmitConditional("mpzp_inv_bug", "inv", "src/util/mpzzp.h", 177,
		func() bool { return true }, func(w io.Writer) error {
			b := modularTraceBody{writer: w}
			b.part("after normalization a: ")
			value, e := m.manager.String(a)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(value)
			b.part("\n")
			return b.failed
		})
	return numberErr
}

func (m *IntegerModularManager) traceInvGCD() error {
	var numberErr error
	_ = m.modularTraceTags().EmitConditional("mpzp_inv_bug", "inv", "src/util/mpzzp.h", 179,
		func() bool { return true }, func(w io.Writer) error {
			b := modularTraceBody{writer: w}
			b.part("tmp1: ")
			first, e := m.manager.String(&m.InvTmp1)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(first)
			b.part("\ntmp2: ")
			second, e := m.manager.String(&m.InvTmp2)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(second)
			b.part("\ntmp3: ")
			third, e := m.manager.String(&m.InvTmp3)
			if e != nil {
				numberErr = e
				return e
			}
			b.part(third)
			b.part("\n")
			return b.failed
		})
	return numberErr
}
