package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:685-697,1378-1494.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicRationalArithmeticScope struct {
	convertEntry      upolyConvertEntryReceipt
	evaluations       [4]upolyBinaryEvaluationInvocationReceipt // diagnostic l/u, then debug assertion l/u
	polynomialCaller  *polynomialAlgebraicEvaluationScope
	integerAddition   *algebraicIntegerAddScope
	q                 [3]unumeral.Rational       // nbv (or basic result), il, iu
	b                 [2]unumeral.BinaryRational // l, u
	qCount, bCount    uint8
	stage             uint8
	active, executing bool
	sourceStop        error
}

func (m *AlgebraicRootCore) rationalArithmeticEvaluationPending() bool {
	if m == nil || m.self != m || !m.arithmetic.active {
		return false
	}
	p := m.config.Polynomials
	for i := range m.arithmetic.evaluations {
		if p.matchesBinaryEvaluationReceipt(&m.arithmetic.evaluations[i]) && p.binaryEvaluation.active {
			return true
		}
	}
	return false
}

func (m *AlgebraicRootCore) cleanupRationalArithmetic() error {
	return m.finishRationalArithmetic(true)
}

func (m *AlgebraicRootCore) finishRationalArithmetic(owning bool) error {
	s := &m.arithmetic
	if s.executing {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	s.executing = true
	defer func() { s.executing = false }()
	// A completed Horner source scope can still own unjoined RAW. Only
	// acknowledgement consumes it; ordinary source unwinding reaches the
	// older arithmetic destructors without acquiring that ownership.
	if owning {
		for i := len(s.evaluations) - 1; i >= 0; i-- {
			if e := m.config.Polynomials.cleanupBinaryEvaluationWithReceipt(&s.evaluations[i], false); e != nil {
				return e
			}
		}
	}
	// The nonbinary branch's iu/il are younger than u/l, then comes nbv.
	for s.stage < 5 {
		var e error
		switch s.stage {
		case 0, 1:
			i := uint8(2) - s.stage
			if s.qCount > i {
				e = m.config.Rationals.Del(&s.q[i])
			}
		case 2, 3:
			i := uint8(3) - s.stage
			if s.bCount > i {
				e = m.config.Binary.Del(&s.b[i])
			}
		case 4:
			if s.qCount != 0 {
				e = m.config.Rationals.Del(&s.q[0])
			}
		}
		if e != nil {
			return e
		}
		s.stage++
	}
	for i := len(s.evaluations) - 1; i >= 0; i-- {
		p := m.config.Polynomials
		if owning {
			if e := p.cleanupBinaryEvaluationWithReceipt(&s.evaluations[i], true); e != nil {
				return e
			}
		} else if p.matchesBinaryEvaluationReceipt(&s.evaluations[i]) && p.binaryEvaluation.active {
			return nil
		}
	}
	s.active = false
	return nil
}

// Normalize follows the source's interval normalization, including turning a
// root cell into null when its represented root is zero.
func (m *AlgebraicRootCore) Normalize(a *Algebraic) error {
	l, e := m.location(a)
	if e != nil || l == nil {
		return e
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	if l.kind == 0 {
		zero, e := m.config.Rationals.IsZero(&r.value)
		if e != nil || !zero {
			return e
		}
		return m.Del(a)
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	sign := 1
	if r.flags&2 != 0 {
		sign = -1
	}
	interval, e := m.config.Polynomials.NormalizeIntervalCore(v, sign, m.config.Binary, r.interval.BorrowLower(), r.interval.BorrowUpper())
	if e = m.save(l, &r, e); e != nil {
		return e
	}
	if !interval {
		return m.Del(a)
	}
	return nil
}

// AddRational and SubRational are the concrete rational-operand source routes.
// The rational must belong to the core's borrowed rational manager. Inputs and
// output may alias; an algebraic input is translated before output replacement.
func (m *AlgebraicRootCore) AddRational(a *Algebraic, b *unumeral.Rational, out *Algebraic) error {
	return m.addSubRational(a, b, out, false)
}
func (m *AlgebraicRootCore) SubRational(a *Algebraic, b *unumeral.Rational, out *Algebraic) error {
	return m.addSubRational(a, b, out, true)
}

func (m *AlgebraicRootCore) addSubRational(a *Algebraic, b *unumeral.Rational, out *Algebraic, subtract bool) (cause error) {
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if _, e = m.location(out); e != nil {
		return e
	}
	qm, bm, p := m.config.Rationals, m.config.Binary, m.config.Polynomials
	zero, e := qm.IsZero(b)
	if e != nil {
		return e
	}
	if zero {
		return m.Copy(out, a)
	}
	if l == nil {
		// sub(0,b,c) publishes b before negating c. In particular a stopped
		// minimum-integer negation must leave that already-published value.
		if e = m.SetRational(out, b); e != nil {
			return e
		}
		if subtract {
			return m.Neg(out)
		}
		return nil
	}
	s := &m.arithmetic
	if s.active || m.localActive || m.negation.active {
		return ErrAlgebraicPending
	}
	*s = algebraicRationalArithmeticScope{polynomialCaller: m.polynomialCaller, active: true}
	if m.integerAdd.running {
		s.integerAddition = &m.integerAdd
	}
	defer func() { cause = m.finishRationalArithmeticSource(cause) }()
	if l.kind == 0 {
		s.q[0], s.qCount = unumeral.NewRational(), 1
		r, e := m.load(l)
		if e != nil {
			return e
		}
		s.executing = true
		if subtract {
			e = qm.Sub(&r.value, b, &s.q[0])
		} else {
			e = qm.Add(&r.value, b, &s.q[0])
		}
		s.executing = false
		if e != nil {
			return e
		}
		if e = m.SetTake(out, &s.q[0]); e != nil {
			return e
		}
		return m.Normalize(out)
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	s.executing = true
	if e = m.traceRationalArithmeticInput(&r, b, false); e != nil {
		return e
	}
	s.q[0], s.qCount = unumeral.NewRational(), 1
	if e = qm.Copy(&s.q[0], b); e != nil {
		return e
	}
	if !subtract {
		if e = qm.Neg(&s.q[0]); e != nil {
			return e
		}
	}
	if m.addTmp.slot == 0 {
		m.addTmp, e = p.NewPolynomial()
		if e != nil {
			return e
		}
	}
	if e = p.Reset(m.addTmp); e != nil {
		return e
	}
	v, e := m.sourcePolynomial(&r)
	if e != nil {
		return e
	}
	if e = p.TranslateRational(v, &s.q[0], m.addTmp); e != nil {
		return e
	}
	s.bCount = 2
	if e = qm.Neg(&s.q[0]); e != nil {
		return e
	}
	exact, e := bm.FromRational(qm, &s.q[0], &s.b[0])
	if e != nil {
		return e
	}
	if exact {
		if e = bm.Add(r.interval.BorrowUpper(), &s.b[0], &s.b[1]); e != nil {
			return e
		}
		if e = bm.Add(r.interval.BorrowLower(), &s.b[0], &s.b[0]); e != nil {
			return e
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
		if e = m.traceRationalArithmeticBounds(false); e != nil {
			return e
		}
		if e = qm.Add(&s.q[1], &s.q[0], &s.q[1]); e != nil {
			return e
		}
		if e = qm.Add(&s.q[2], &s.q[0], &s.q[2]); e != nil {
			return e
		}
		translated, e := p.wholeView(m.addTmp)
		if e != nil {
			return e
		}
		if _, e = p.convertRationalToBinaryIntervalWithEntryReceipt(translated, qm, &s.q[1], &s.q[2], bm, &s.b[0], &s.b[1], &s.convertEntry); e != nil {
			return e
		}
		// il and iu leave their inner source scope before set(c, ...).
		for s.qCount > 1 {
			if e = qm.Del(&s.q[s.qCount-1]); e != nil {
				s.sourceStop = e
				return e
			}
			s.qCount--
		}
	}
	translated, e := p.wholeView(m.addTmp)
	if e != nil {
		return e
	}
	if e = m.traceRationalArithmeticResult(translated, false); e != nil {
		return e
	}
	s.executing = false
	if e = m.SetRoot(out, translated, &s.b[0], &s.b[1], r.flags&1 != 0); e != nil {
		return e
	}
	return m.Normalize(out)
}

// finishRationalArithmeticSource observes only accepted children before older
// source destructors. Owning acknowledgement remains a separate continuation.
func (m *AlgebraicRootCore) finishRationalArithmeticSource(cause error) error {
	s, p := &m.arithmetic, m.config.Polynomials

	s.executing = false
	if s.sourceStop == nil {
		s.sourceStop = m.rootTraceSourceCause()
	}
	if s.sourceStop == nil {
		for i := range s.evaluations {
			if p.matchesBinaryEvaluationReceipt(&s.evaluations[i]) {
				s.sourceStop = p.binaryEvaluation.sourceCause
				if s.sourceStop != nil {
					break
				}
			}
		}
	}
	if s.sourceStop == nil {
		s.sourceStop = p.convertEntryReceiptSourceCause(&s.convertEntry)
	}
	if s.sourceStop == nil {
		cleanup := m.finishRationalArithmetic(false)
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}
	return cause
}
