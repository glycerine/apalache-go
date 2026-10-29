package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2899-2927.

import (
	"io"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// These public manager wrappers surround the imp operations. Internal core
// calls retain the imp dispatch and do not emit another public wrapper record.
func (m *AlgebraicManager) Power(a *Algebraic, k uint32, out *Algebraic) error {
	if e := m.ready(); e != nil {
		return e
	}
	c := m.AlgebraicRootCore
	if e := c.traceManagerPower(a, k, false); e != nil {
		return e
	}
	if e := c.Power(a, k, out); e != nil {
		return e
	}
	return c.traceManagerPower(out, k, true)
}
func (m *AlgebraicManager) Add(a, b, out *Algebraic) error { return m.binaryWithDetail(a, b, out, 0) }
func (m *AlgebraicManager) Sub(a, b, out *Algebraic) error { return m.binaryWithDetail(a, b, out, 1) }
func (m *AlgebraicManager) Mul(a, b, out *Algebraic) error { return m.binaryWithDetail(a, b, out, 2) }
func (m *AlgebraicManager) binaryWithDetail(a, b, out *Algebraic, kind uint8) error {
	if e := m.ready(); e != nil {
		return e
	}
	c := m.AlgebraicRootCore
	if e := c.traceManagerBinary(a, b, kind, false); e != nil {
		return e
	}
	var e error
	switch kind {
	case 0:
		e = c.Add(a, b, out)
	case 1:
		e = c.Sub(a, b, out)
	case 2:
		e = c.Mul(a, b, out)
	}
	if e != nil {
		return e
	}
	return c.traceManagerBinary(out, nil, kind, true)
}
func (m *AlgebraicManager) AddInteger(a *Algebraic, n *unumeral.Int, out *Algebraic) error {
	if e := m.ready(); e != nil {
		return e
	}
	return m.AlgebraicRootCore.addIntegerWithManager(a, n, out, m)
}

func (c *AlgebraicRootCore) traceManagerPower(a *Algebraic, k uint32, after bool) error {
	line := 2900
	if after {
		line = 2902
	}
	return c.emitRootTraceTag("anum_detail", "power", line, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		if after {
			_ = out.AppendCString("^ result: ")
		}
		l, e := c.locationCleanup(a)
		if e != nil {
			return e
		}
		if e = c.displayRootAtLocationWithReceipt(out, l, false, &c.rootTrace.root); e != nil {
			return e
		}
		if !after {
			_ = out.AppendCString("^")
			_ = out.AppendUint32(k)
		}
		return out.AppendCString("\n")
	})
}
func (c *AlgebraicRootCore) traceManagerBinary(a, b *Algebraic, kind uint8, after bool) error {
	lines := [3]int{2906, 2918, 2924}
	names := [3]string{"add", "sub", "mul"}
	operators := [3]string{"+", "-", "*"}
	line := lines[kind]
	if after {
		line += 2
	}
	return c.emitRootTraceTag("anum_detail", names[kind], line, func(w io.Writer) error {
		out := algebraicTranslationTraceStream{algebraicPredicateTraceStream{w}}
		if after {
			_ = out.AppendCString(operators[kind] + " result: ")
		}
		l, e := c.locationCleanup(a)
		if e != nil {
			return e
		}
		if e = c.displayRootAtLocationWithReceipt(out, l, false, &c.rootTrace.root); e != nil {
			return e
		}
		if !after {
			_ = out.AppendCString(" " + operators[kind] + " ")
			l, e = c.locationCleanup(b)
			if e != nil {
				return e
			}
			if e = c.displayRootAtLocationWithReceipt(out, l, false, &c.rootTrace.root); e != nil {
				return e
			}
		}
		return out.AppendCString("\n")
	})
}
