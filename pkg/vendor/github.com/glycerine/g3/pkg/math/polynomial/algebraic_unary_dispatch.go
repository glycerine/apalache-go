package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:1271-1381.

import (
	"errors"

	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

var ErrAlgebraicZeroPower error = newAlgebraicException("0^0 is indeterminate")
var ErrAlgebraicZeroRoot error = newAlgebraicException("0-th root is indeterminate")
var ErrAlgebraicEvenNegativeRoot error = newAlgebraicException("even root of negative number is not real")

// Power and Root may refine the input and allow the destination to alias it.
func (m *AlgebraicRootCore) Power(a *Algebraic, k uint32, out *Algebraic) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.binary.active || m.basicRootScope.active {
		return ErrAlgebraicPending
	}
	return m.dispatchUnary(a, k, out, false)
}
func (m *AlgebraicRootCore) Root(a *Algebraic, k uint32, out *Algebraic) error {
	if e := m.check(); e != nil {
		return e
	}
	if m.binary.active || m.basicRootScope.active {
		return ErrAlgebraicPending
	}
	return m.dispatchUnary(a, k, out, true)
}
func (m *AlgebraicRootCore) dispatchUnary(a *Algebraic, k uint32, out *Algebraic, root bool) error {
	if root && k == 0 {
		return ErrAlgebraicZeroRoot
	}
	l, e := m.location(a)
	if e != nil {
		return e
	}
	if !root && k == 0 {
		if l == nil {
			return ErrAlgebraicZeroPower
		}
		return m.basicPower(nil, 0, out)
	}
	if k == 1 || root && l == nil {
		return m.Copy(out, a)
	}
	if l == nil {
		return m.Del(out)
	}
	if root {
		negative, e := m.IsNeg(a)
		if e != nil {
			return e
		}
		if negative && k%2 == 0 {
			return ErrAlgebraicEvenNegativeRoot
		}
	}
	if l.kind == 1 {
		return m.makeUnary(a, out, k, root)
	}
	r, e := m.load(l)
	if e != nil {
		return e
	}
	if root {
		return m.basicRoot(&r.value, k, out)
	}
	return m.basicPower(&r.value, k, out)
}

type algebraicBasicRootScope struct {
	power            unumeral.RationalPowerInvocation
	adjust           unumeral.BinaryIntegerDebugInvocationReceipt
	convert          unumeral.BinaryConvertInvocationReceipt
	root             unumeral.RationalRootInvocation
	qConstructed     bool
	polynomialCaller *polynomialAlgebraicEvaluationScope
	sourceStop       error
	q                unumeral.Rational
	p                UPolynomial
	bounds           [2]unumeral.BinaryRational
	constructed      uint8
	active, running  bool
}

func (s *algebraicBasicRootScope) pending(m *AlgebraicRootCore) bool {
	return m.rootTracePending() || s.root.Pending() || s.power.Pending() || m.config.Binary.ConvertInvocationPending(&s.convert) || m.config.Binary.IntegerDebugInvocationPending(&s.adjust)
}

func (m *AlgebraicRootCore) cleanupBasicRoot() error {
	s := &m.basicRootScope
	if s.running {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if e := m.cleanupRootTrace(); e != nil {
		return e
	}
	if e := m.config.Binary.CleanupIntegerDebugInvocation(&s.adjust); e != nil {
		return e
	}
	if e := m.config.Binary.CleanupConvertInvocation(&s.convert); e != nil {
		return e
	}
	if e := s.power.Cleanup(false); e != nil {
		return e
	}
	if e := s.root.Cleanup(false); e != nil {
		return e
	}
	if e := m.finishBasicRootLocals(); e != nil {
		return e
	}
	if e := s.power.Cleanup(true); e != nil {
		return e
	}
	if e := s.root.Cleanup(true); e != nil {
		return e
	}
	s.active = s.pending(m)
	return nil
}
func (m *AlgebraicRootCore) finishBasicRootSource() error {
	s := &m.basicRootScope
	if m.config.Binary.IntegerDebugInvocationPending(&s.adjust) {
		if e := m.config.Binary.IntegerDebugInvocationSourceCause(&s.adjust); e != nil {
			return e
		}
	}
	if m.config.Binary.ConvertInvocationPending(&s.convert) {
		if e := m.config.Binary.ConvertInvocationSourceCause(&s.convert); e != nil {
			return e
		}
	}
	if e := s.power.SourceCause(); e != nil {
		return e
	}
	if e := s.root.SourceCause(); e != nil {
		return e
	}
	p := m.config.Polynomials
	if p.defaultNormalizationReceiptPending(&m.coefficientNormalization) {
		if e := p.defaultNormalization.sourceCause; e != nil {
			return e
		}
		p.defaultNormalization.parentSourceExecuting = true
		defer func() { p.defaultNormalization.parentSourceExecuting = false }()
	}
	if e := m.rootTraceSourceCause(); e != nil {
		return e
	}
	return m.finishBasicRootLocals()
}
func (m *AlgebraicRootCore) finishBasicRootLocals() error {
	s := &m.basicRootScope
	for s.constructed != 0 {
		if e := m.config.Binary.Del(&s.bounds[s.constructed-1]); e != nil {
			return e
		}
		s.constructed--
	}
	if s.p.slot != 0 {
		if e := m.config.Polynomials.DelPolynomial(&s.p); e != nil {
			return e
		}
	}
	if s.qConstructed {
		if e := m.config.Rationals.Del(&s.q); e != nil {
			return e
		}
		s.qConstructed = false
	}
	s.active = s.pending(m)
	return nil
}
func (m *AlgebraicRootCore) basicRoot(value *unumeral.Rational, k uint32, out *Algebraic) (cause error) {
	s := &m.basicRootScope
	if s.active {
		return ErrAlgebraicPending
	}
	d, enabled, e := algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		zero, e := m.config.Rationals.IsZero(value)
		if e != nil {
			return e
		}
		if zero {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1272, "!qm().is_zero(a->m_value)"); e != nil {
				return e
			}
		}
	}
	d, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled && k <= 1 {
		if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1273, "k > 1"); e != nil {
			return e
		}
	}
	*s = algebraicBasicRootScope{polynomialCaller: m.polynomialCaller, q: unumeral.NewRational(), qConstructed: true, active: true, running: true}
	defer func() {
		s.running = false
		cleanup := m.finishBasicRootSource()
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}()
	q, p, b := m.config.Rationals, m.config.Polynomials, m.config.Binary
	exact, e := q.RootWithInvocation(value, k, &s.q, &s.root)
	if e != nil {
		return e
	}
	if exact {
		if e = m.traceBasicRootRational(value); e != nil {
			return e
		}
		return m.SetRational(out, &s.q)
	}
	s.p, e = p.NewPolynomial()
	if e != nil {
		return e
	}
	// Source pushes a zero coefficient, then sets and negates the numerator.
	zero := unumeral.NewInt(0)
	if e = p.PushCoefficient(s.p, &zero); e != nil {
		return e
	}
	poly, e := p.descriptor(s.p)
	if e != nil {
		return e
	}
	if e = q.Integers().Copy(&p.values(&poly.state)[0], &value.Num); e != nil {
		return e
	}
	if e = q.Integers().Neg(&p.values(&poly.state)[0]); e != nil {
		return e
	}
	for i := uint32(0); i < k; i++ {
		if e = p.PushCoefficient(s.p, &zero); e != nil {
			return e
		}
	}
	poly, e = p.descriptor(s.p)
	if e != nil {
		return e
	}
	if e = q.Integers().Copy(&p.values(&poly.state)[k], &value.Den); e != nil {
		return e
	}
	s.constructed = 2
	negative, e := q.IsNeg(value)
	if e != nil {
		return e
	}
	bound := &s.bounds[1]
	if negative {
		bound = &s.bounds[0]
	}
	exact, e = b.FromRationalWithReceipt(q, value, bound, &s.convert)
	if e != nil {
		return e
	}
	if !exact {
		if e = b.Mul2(bound); e != nil {
			return e
		}
	}
	one := unumeral.NewInt(1)
	if negative {
		e = b.SubIntegerWithDebugReceipt(bound, &one, bound, &s.adjust)
	} else {
		e = b.AddIntegerWithDebugReceipt(bound, &one, bound, &s.adjust)
	}
	if e != nil {
		return e
	}
	d, enabled, e = algebraicAssertionSwitch()
	if e != nil {
		return e
	}
	if enabled {
		less, e := b.Less(&s.bounds[0], &s.bounds[1])
		if e != nil {
			return e
		}
		if !less {
			if e = diagnostic.DebugAssertionFailure(d, "src/math/polynomial/algebraic_numbers.cpp", 1319, "bqm().lt(lower, upper)"); e != nil {
				return e
			}
		}
	}
	v, e := p.wholeView(s.p)
	if e != nil {
		return e
	}
	if e = m.traceBasicRootPolynomial(v); e != nil {
		return e
	}
	return m.SetRoot(out, v, &s.bounds[0], &s.bounds[1], false)
}

func (m *AlgebraicRootCore) basicPower(value *unumeral.Rational, k uint32, out *Algebraic) (cause error) {
	s := &m.basicRootScope
	if s.active {
		return ErrAlgebraicPending
	}
	*s = algebraicBasicRootScope{polynomialCaller: m.polynomialCaller, q: unumeral.NewRational(), qConstructed: true, active: true, running: true}
	defer func() {
		s.running = false
		cleanup := m.finishBasicRootSource()
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}()
	var e error
	if value == nil {
		e = m.config.Rationals.SetInt32(&s.q, 1)
	} else {
		e = m.config.Rationals.PowerWithInvocation(value, k, &s.q, &s.power)
	}
	if e != nil {
		return e
	}
	return m.SetRational(out, &s.q)
}
