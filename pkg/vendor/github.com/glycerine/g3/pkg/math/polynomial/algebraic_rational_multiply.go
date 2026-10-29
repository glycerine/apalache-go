package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1490-1563.

import (
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// MulRational is the concrete rational-operand multiplication route. It shares
// the source m_add_tmp polynomial with mixed addition and subtraction.
func (m *AlgebraicRootCore) MulRational(a *Algebraic, b *unumeral.Rational, out *Algebraic) (cause error) {
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if _, e = m.location(out); e != nil {
		return e
	}
	// Source short-circuits before reading the second operand when a is zero.
	if l == nil {
		return m.Del(out)
	}
	qm, bm, p := m.config.Rationals, m.config.Binary, m.config.Polynomials
	zero, e := qm.IsZero(b)
	if e != nil {
		return e
	}
	if zero {
		return m.Del(out)
	}
	s := &m.arithmetic
	if s.active || m.localActive || m.negation.active {
		return ErrAlgebraicPending
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	*s = algebraicRationalArithmeticScope{polynomialCaller: m.polynomialCaller, active: true, executing: true}
	defer func() { cause = m.finishRationalArithmeticSource(cause) }()
	if l.kind == 1 {
		if e = m.traceRationalArithmeticInput(&r, b, true); e != nil {
			return e
		}
	}
	if l.kind == 1 && polynomialSourceDebug {
		d := diagnostic.BorrowProcessDebugContext()
		enabled, e := diagnostic.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled {
			v, e := m.sourcePolynomial(&r)
			if e != nil {
				return e
			}
			sl, e := p.evalSignBinaryWithNegScopeAndReceipt(v, r.interval.BorrowLower(), nil, &s.evaluations[2])
			if e != nil {
				return e
			}
			su, e := p.evalSignBinaryWithNegScopeAndReceipt(v, r.interval.BorrowUpper(), nil, &s.evaluations[3])
			if e != nil {
				return e
			}
			if sl != -su {
				if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1501, "upm().eval_sign_at(a->m_p_sz, a->m_p, lower(a)) == -upm().eval_sign_at(a->m_p_sz, a->m_p, upper(a))"); e != nil {
					return e
				}
			}
		}
	}
	s.q[0], s.qCount = unumeral.NewRational(), 1
	if l.kind == 0 {
		if e = qm.Mul(&r.value, b, &s.q[0]); e != nil {
			return e
		}
		s.executing = false
		if e = m.SetTake(out, &s.q[0]); e != nil {
			return e
		}
		return m.Normalize(out)
	}
	if e = qm.Copy(&s.q[0], b); e != nil {
		return e
	}
	if e = qm.InvInPlace(&s.q[0]); e != nil {
		return e
	}
	if m.addTmp.slot == 0 {
		m.addTmp, e = p.NewPolynomial()
		if e != nil {
			return e
		}
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	if e = p.SetView(m.addTmp, v); e != nil {
		return e
	}
	scaled, e := p.wholeView(m.addTmp)
	if e != nil {
		return e
	}
	if e = p.ComposeRationalXViewInPlace(scaled, &s.q[0]); e != nil {
		return e
	}
	s.bCount = 2
	if e = qm.InvInPlace(&s.q[0]); e != nil {
		return e
	}
	negative, e := qm.IsNeg(&s.q[0])
	if e != nil {
		return e
	}
	exact, e := bm.FromRational(qm, &s.q[0], &s.b[0])
	if e != nil {
		return e
	}
	if exact {
		if e = bm.Mul(r.interval.BorrowUpper(), &s.b[0], &s.b[1]); e != nil {
			return e
		}
		if e = bm.Mul(r.interval.BorrowLower(), &s.b[0], &s.b[0]); e != nil {
			return e
		}
		if negative {
			if e = bm.Swap(&s.b[0], &s.b[1]); e != nil {
				return e
			}
		}
	} else {
		s.q[1], s.q[2] = unumeral.NewRational(), unumeral.NewRational()
		s.qCount = 3
		if e = bm.ToRational(qm, r.interval.BorrowLower(), &s.q[1]); e != nil {
			return e
		}
		if e = bm.ToRational(qm, r.interval.BorrowUpper(), &s.q[2]); e != nil {
			return e
		}
		if e = m.traceRationalArithmeticBounds(true); e != nil {
			return e
		}
		if e = qm.Mul(&s.q[1], &s.q[0], &s.q[1]); e != nil {
			return e
		}
		if e = qm.Mul(&s.q[2], &s.q[0], &s.q[2]); e != nil {
			return e
		}
		if negative {
			if e = qm.Swap(&s.q[1], &s.q[2]); e != nil {
				return e
			}
		}
		if _, e = p.convertRationalToBinaryIntervalWithEntryReceipt(scaled, qm, &s.q[1], &s.q[2], bm, &s.b[0], &s.b[1], &s.convertEntry); e != nil {
			return e
		}
		for s.qCount > 1 {
			if e = qm.Del(&s.q[s.qCount-1]); e != nil {
				s.sourceStop = e
				return e
			}
			s.qCount--
		}
	}
	if e = m.traceRationalArithmeticResult(scaled, true); e != nil {
		return e
	}
	s.executing = false
	if e = m.SetRoot(out, scaled, &s.b[0], &s.b[1], r.flags&1 != 0); e != nil {
		return e
	}
	return m.Normalize(out)
}
