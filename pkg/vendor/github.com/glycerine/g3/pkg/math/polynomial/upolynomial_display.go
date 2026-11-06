package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1108-1225,3120-3124;
// src/util/mpz.cpp:1385-1410.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Output is a call-local borrowed stream. It never stores an assembled decimal
// string; BIG insertion uses the existing exact lexical numeric stream engine.
func (m *UPolynomialManager) DisplayInto(p UPolynomialView, out unumeral.RationalSourceStream, variable string, star bool) error {
	return m.displayIntoWithReceipt(p, out, variable, star, nil)
}
func (m *UPolynomialManager) displayIntoWithReceipt(p UPolynomialView, out unumeral.RationalSourceStream, variable string, star bool, receipt *upolyDisplayInvocationReceipt) (cause error) {
	q, e := m.beginDisplay(false)
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = upolyDisplayInvocationReceipt{m, q, q.invocation}
	}
	defer func() { q.executing = false; cause = q.finish(cause); q.endCall() }()
	if out == nil {
		return arena.ErrConfig
	}
	out = upolyDisplayStream{scope: q, out: out}
	q.constructed = true // scoped_numeral a is declared even for sz==0.
	displayed := false
	for i := p.count; i > 0; {
		i--
		a, e := m.read(p, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if e = numericIntegerModularManager_copyWithDisplayContext(m.modular, &q.numeral, a, q.context()); e != nil {
			return e
		}
		if displayed {
			if e = m.config.Integer.WithAllocationContext((q.context()).utility(m.config.Integer)).Abs(&q.numeral); e != nil {
				return e
			}
			// The sign is read at its actual later source site, not from the Copy.
			a, e = m.read(p, i)
			if e != nil {
				return e
			}
			positive, e := m.modular.IsPos(a)
			if e != nil {
				return e
			}
			sign := " - "
			if positive {
				sign = " + "
			}
			if e = out.AppendCString(sign); e != nil {
				return e
			}
		}
		displayed = true
		if i == 0 {
			if e = q.insert(out, &q.numeral); e != nil {
				return e
			}
			continue
		}
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && i == 0 {
			if e := m.scalarDivisionAssertionFailure(1129, "i > 0"); e != nil {
				return e
			}
		}
		one, e := m.modular.IsOne(&q.numeral)
		if e != nil {
			return e
		}
		if !one {
			if e = q.insert(out, &q.numeral); e != nil {
				return e
			}
			separator := " "
			if star {
				separator = "*"
			}
			if e = out.AppendCString(separator); e != nil {
				return e
			}
		}
		if e = out.AppendCString(variable); e != nil {
			return e
		}
		if i > 1 {
			if e = out.AppendCString("^"); e != nil {
				return e
			}
			if e = displayUnsigned(out, i); e != nil {
				return e
			}
		}
	}
	if !displayed {
		return out.AppendCString("0")
	}
	return nil
}
func displayUnsigned(out unumeral.RationalSourceStream, n uint32) error {
	// std::ostream's machine unsigned insertion is transported as fixed decimal
	// bytes; no source dynamic numeral or buffer is constructed.
	var bytes [10]byte
	i := len(bytes)
	for {
		i--
		bytes[i] = '0' + byte(n%10)
		n /= 10
		if n == 0 {
			break
		}
	}
	return out.AppendCString(string(bytes[i:]))
}
func (m *UPolynomialManager) DisplaySMT2Into(p UPolynomialView, out unumeral.RationalSourceStream, variable string) (cause error) {
	return m.displaySMT2IntoWithReceipt(p, out, variable, nil)
}
func (m *UPolynomialManager) displaySMT2IntoWithReceipt(p UPolynomialView, out unumeral.RationalSourceStream, variable string, receipt *upolyDisplayInvocationReceipt) (cause error) {
	q, e := m.beginDisplay(true)
	if e != nil {
		return e
	}
	if receipt != nil {
		*receipt = upolyDisplayInvocationReceipt{m, q, q.invocation}
	}
	defer func() { q.executing = false; cause = q.finish(cause); q.endCall() }()
	if out == nil {
		return arena.ErrConfig
	}
	out = upolyDisplayStream{scope: q, out: out}
	if p.count == 0 {
		return out.AppendCString("0")
	}
	if p.count == 1 {
		a, e := m.read(p, 0)
		if e != nil {
			return e
		}
		return q.smtNumeral(out, a)
	}
	index, count := uint32(^uint32(0)), uint32(0)
	for i := uint32(0); i < p.count; i++ {
		a, e := m.read(p, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if !zero {
			index = i
			count++
		}
	}
	if count == 1 {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && (index == ^uint32(0) || index < 1) {
			if e := m.scalarDivisionAssertionFailure(1207, "non_zero_idx != UINT_MAX && non_zero_idx >= 1"); e != nil {
				return e
			}
		}
		a, e := m.read(p, index)
		if e != nil {
			return e
		}
		if e = q.smtMonomial(out, a, index, variable); e != nil {
			return e
		}
	}
	// Literal source fallthrough is intentional: the single monomial is emitted
	// once above and then again inside (+ ...), with no inserted separator.
	if e = out.AppendCString("(+"); e != nil {
		return e
	}
	for i := p.count; i > 0; {
		i--
		a, e := m.read(p, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(a)
		if e != nil {
			return e
		}
		if zero {
			continue
		}
		if e = out.AppendCString(" "); e != nil {
			return e
		}
		a, e = m.read(p, i)
		if e != nil {
			return e
		}
		if e = q.smtMonomial(out, a, i, variable); e != nil {
			return e
		}
	}
	return out.AppendCString(")")
}
func (q *upolyDisplayScope) smtNumeral(out unumeral.RationalSourceStream, a *unumeral.Int) error {
	m := q.owner
	negative, e := m.modular.IsNeg(a)
	if e != nil {
		return e
	}
	if !negative {
		return q.insert(out, a)
	}
	if e = out.AppendCString("(- "); e != nil {
		return e
	}
	q.constructed = true
	q.numeral = unumeral.Int{} // RAW abs_n has no automatic DT.
	if e = numericIntegerModularManager_copyWithDisplayContext(m.modular, &q.numeral, a, q.context()); e != nil {
		return e
	}
	if e = numericIntegerModularManager_negWithDisplayContext(m.modular, &q.numeral, q.context()); e != nil {
		return e
	}
	if e = q.insert(out, &q.numeral); e != nil {
		return e
	}
	if _, e = upolynomialSourceNumeralDel(m.config.Integer, &q.numeral); e != nil {
		q.sourceCause = e
		return e
	}
	q.constructed = false
	return out.AppendCString(")")
}
func (q *upolyDisplayScope) smtMonomial(out unumeral.RationalSourceStream, a *unumeral.Int, k uint32, variable string) error {
	if k == 0 {
		return q.smtNumeral(out, a)
	}
	one, e := q.owner.modular.IsOne(a)
	if e != nil {
		return e
	}
	power := func() error {
		if k == 1 {
			return out.AppendCString(variable)
		}
		if e := out.AppendCString("(^ "); e != nil {
			return e
		}
		if e := out.AppendCString(variable); e != nil {
			return e
		}
		if e := out.AppendCString(" "); e != nil {
			return e
		}
		if e := displayUnsigned(out, k); e != nil {
			return e
		}
		return out.AppendCString(")")
	}
	if one {
		return power()
	}
	if e = out.AppendCString("(* "); e != nil {
		return e
	}
	if e = q.smtNumeral(out, a); e != nil {
		return e
	}
	if e = out.AppendCString(" "); e != nil {
		return e
	}
	if e = power(); e != nil {
		return e
	}
	return out.AppendCString(")")
}
func (m *UPolynomialManager) DisplaySequenceInto(seq *UPolynomialSequence, out unumeral.RationalSourceStream, variable string) error {
	return m.displaySequenceIntoWithReceipt(seq, out, variable, nil)
}
func (m *UPolynomialManager) displaySequenceIntoWithReceipt(seq *UPolynomialSequence, out unumeral.RationalSourceStream, variable string, receipt *upolyDisplayInvocationReceipt) error {
	if m == nil || m.self != m || seq == nil || seq.self != seq || seq.manager != m {
		return ErrUPolynomialOwner
	}
	if e := seq.check(); e != nil {
		return e
	}
	if seq.displaying || m.displaySequence != nil {
		return ErrUPolynomialCleanupPending
	}
	if out == nil {
		return arena.ErrConfig
	}
	seq.displaying = true
	m.displaySequence = seq
	previous := upolyDisplaySequenceCaller(m.config.Integer)
	setUpolyDisplaySequenceCaller(m.config.Integer, seq)
	defer func() {
		setUpolyDisplaySequenceCaller(m.config.Integer, previous)
		m.displaySequence = nil
		seq.displaying = false
	}()
	for i := uint32(0); ; i++ {
		n, e := seq.Size()
		if e != nil {
			return e
		}
		if i >= n {
			return nil
		}
		row, e := seq.RowView(i)
		if e != nil {
			return e
		}
		if e = m.displayIntoWithReceipt(row, out, variable, false, receipt); e != nil {
			return e
		}
		if e = out.AppendCString("\n"); e != nil {
			return e
		}
	}
}

// Error captures diagnostic causes independently of the mutable owner frame.
func upolyDisplayJoin(a, b error) error {
	if a == nil {
		return b
	}
	if b == nil {
		return a
	}
	return errors.Join(a, b)
}
