package polynomial

// Source: src/math/polynomial/polynomial.cpp:6832-6841,2582-2594;
// src/util/scoped_numeral_buffer.h; src/util/buffer.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// One actual per-invocation lexical buffer embedded in the reflected charged
// class header. Successful destruction permits reuse; stopped roots retain
// their exact source history until explicit owning abort.

type polynomialToPolynomialScope struct {
	calleeGeneration                     uint64
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	monomialAllocator                    monomialAllocatorInvocationReceipt
	invocation                           uint64
	direct                               polynomialToPolynomialInvocationReceipt
	numeric                              polynomialNumericRoots
	normalize                            unumeral.IntegerNormalizeInvocationReceipt
	finishing                            bool
	transport                            unumeral.Int
	transportBound                       bool
	buffer                               unumeral.ScopedIntegerModularBuffer128
	generated                            Monomial
	active, executing                    bool
	bodyCause, sourceCause, cleanupCause error
}

func (m *MultivariatePolynomialManager) ToPolynomial(sz uint32, as []unumeral.Int, x uint32) (out Polynomial, cause error) {
	return m.toPolynomialWithReceipt(sz, as, x, nil)
}
func (m *MultivariatePolynomialManager) toPolynomialWithReceipt(sz uint32, as []unumeral.Int, x uint32, receipt *polynomialToPolynomialInvocationReceipt) (out Polynomial, cause error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	c := m.core
	if sz == 0 {
		return c.zero, nil
	}
	if e := m.beginToPolynomial(); e != nil {
		return Polynomial{}, e
	}
	s := &m.toPolynomial
	if receipt != nil {
		*receipt = s.direct
	}
	defer func() {
		s.executing = false
		cause = m.finishToPolynomial(cause)
		if cause != nil {
			out = Polynomial{}
			return
		}
		if c.matchesFactoryInvocation(&s.factory) {
			if c.cheap.matchesAppendInvocation(&s.append) {
				if e := c.cheap.acknowledgeFactoryAppendTransfer(s.factory.invocation); e != nil {
					cause = e
					out = Polynomial{}
					return
				}
			}
			if e := c.transferFactoryResult(&s.factory, out); e != nil {
				cause = e
				out = Polynomial{}
				return
			}
		}
		s.active = false
	}()
	for i := uint32(0); i < sz; i++ {
		zero := unumeral.Int{}
		if cause = numericScopedIntegerModularBuffer128_pushToPolynomial(&s.buffer, &zero, &s.direct); cause != nil {
			return Polynomial{}, cause
		}
		if uint64(i) >= uint64(len(as)) {
			return Polynomial{}, arena.ErrBounds
		}
		if cause = numericScopedIntegerModularBuffer128_setToPolynomial(&s.buffer, i, &as[i], &s.direct); cause != nil {
			return Polynomial{}, cause
		}
	}
	view, e := s.buffer.View()
	if e != nil {
		return Polynomial{}, e
	}
	p, e := c.mkUnivariateToPolynomial(x, sz-1, polynomialUnivariateCoefficients{array: view, indexed: true}, &s.direct)
	if e != nil {
		return Polynomial{}, e
	}
	return p, nil
}
func (m *MultivariatePolynomialManager) beginToPolynomial() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.toPolynomial
	if s.active || s.finishing || s.invocation == ^uint64(0) || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if e := m.archiveToPolynomial(); e != nil {
		return e
	}
	*s = polynomialToPolynomialScope{active: true, executing: true, invocation: s.invocation + 1}
	s.direct = polynomialToPolynomialInvocationReceipt{m, s, s.invocation}
	m.core.toPolynomialCaller = m
	s.buffer.Construct(m.core.modular)
	return nil
}
func (m *MultivariatePolynomialManager) finishToPolynomial(body error) error {
	if m == nil || m.self != m || !m.matchesToPolynomialInvocation(&m.toPolynomial.direct) {
		return ErrPolynomialOwner
	}
	s := &m.toPolynomial
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if s.bodyCause == nil {
		s.bodyCause = body
	}
	e := s.buffer.SourceClose()
	if e != nil && s.sourceCause == nil {
		s.sourceCause = e
	}
	return polynomialNewtonJoin(body, e)
}
func (m *MultivariatePolynomialManager) CleanupToPolynomial() error {
	if e := m.identity(); e != nil {
		return e
	}
	return m.cleanupToPolynomialInvocation(&m.toPolynomial.direct, true)
}
