package polynomial

// Source: src/math/polynomial/polynomial.cpp:4507-4528,7228-7229;
// src/util/scoped_numeral.h.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// These are the actual two lexical numeral fields and known creation receipts.
// Orphans retain only genuinely newly-created zero-coefficient results which
// the source cheap buffer never INCs. This is owning Go metadata, not source
// arithmetic storage or a source destructor which repairs native leaks.
type polynomialDerivativeScope struct {
	invocation                           uint64
	n, a                                 unumeral.Int
	constructed                          uint8
	stage                                uint8
	numericPhase                         uint8
	active, executing, stopped           bool
	generated                            Monomial
	calleeGeneration                     uint64
	monomialAllocator                    monomialAllocatorInvocationReceipt
	factoryGeneration                    uint64
	numeric                              polynomialNumericRoots
	append                               polynomialCheapAppendInvocationReceipt
	factory                              polynomialFactoryInvocationReceipt
	normalize                            unumeral.IntegerNormalizeInvocationReceipt
	orphans                              []Monomial
	orphanInvocations                    []uint64
	drain                                uint32
	bodyCause, sourceCause, cleanupCause error
}

func (s *polynomialDerivativeScope) Error() string {
	return "polynomial derivative retained lexical owner"
}
func (s *polynomialDerivativeScope) Unwrap() error {
	if s.bodyCause != nil {
		return s.bodyCause
	}
	return s.sourceCause
}

func (m *MonomialManager) derivativeCreated(a Monomial, x uint32, out *monomialFactoryOutcome) (Monomial, error) {
	return m.variableTransformCreated(a, x, 0, 2, out)
}
func (m *MultivariatePolynomialManager) finishDerivativeNumerals() error {
	s := &m.derivative
	if s.stopped {
		return s.sourceCause
	}

	for s.stage < 2 {
		a := &s.a
		mask := uint8(2)
		if s.stage == 1 {
			a = &s.n
			mask = 1
		}
		if s.constructed&mask != 0 {
			_, e := upolynomialSourceNumeralDel(m.core.integer, a)
			if e != nil {
				s.stopped = true
				s.sourceCause = e
				return e
			}
			s.constructed &^= mask
		}
		s.stage++
	}
	return nil
}
func (m *MultivariatePolynomialManager) retainDerivativeGenerated() {
	s := &m.derivative
	if s.generated != (Monomial{}) {
		slot := uint32(len(s.orphans))
		l := &m.core.monomials.locations[s.generated.slot-1]
		l.derivativeCohort = s
		l.derivativeSlot = slot
		old := s.orphans
		s.orphans = append(s.orphans, s.generated)
		if len(old) != 0 && &old[0] != &s.orphans[0] {
			clear(old)
		}
		s.orphanInvocations = append(s.orphanInvocations, s.invocation)
		s.generated = Monomial{}
	}
}
func (m *MultivariatePolynomialManager) derivativeHasOrphans() bool {
	for _, a := range m.derivative.orphans {
		if a != (Monomial{}) {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) transferDerivativeCreated(a Monomial) {
	s := &m.derivative
	if s.generated == a {
		s.generated = Monomial{}
	}
	for i, b := range s.orphans {
		if a == b {
			s.orphans[i] = Monomial{}
			s.orphanInvocations[i] = 0
		}
	}
}
func (m *MultivariatePolynomialManager) Derivative(p Polynomial, x uint32) (Polynomial, error) {
	return m.derivativeWithReceipt(p, x, nil)
}
func (m *MultivariatePolynomialManager) derivativeWithReceipt(p Polynomial, x uint32, receipt *polynomialDerivativeInvocationReceipt) (result Polynomial, cause error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	s := &m.derivative
	if s.active {
		return Polynomial{}, ErrPolynomialPending
	}
	c := m.core
	if e := polynomialDerivativeSourceAssert(4512, "is_valid(x)", func() (bool, error) {
		return c.monomials.IsValidVar(x)
	}); e != nil {
		return Polynomial{}, e
	}
	if e := polynomialDerivativeSourceAssert(4513, "m_cheap_som_buffer.empty()", func() (bool, error) {
		size, e := c.cheap.ms.size(c)
		return size == 0, e
	}); e != nil {
		return Polynomial{}, e
	}
	size, e := c.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if s.invocation == ^uint64(0) {
		return Polynomial{}, ErrPolynomialPending
	}
	s.invocation++
	// This is a new guarded source invocation, not a retry of the older child.
	// Its historical standalone receipt remains valid on the modular owner.
	s.normalize = unumeral.IntegerNormalizeInvocationReceipt{}
	s.append = polynomialCheapAppendInvocationReceipt{}
	s.factory = polynomialFactoryInvocationReceipt{}
	s.monomialAllocator = monomialAllocatorInvocationReceipt{}
	s.numericPhase = 0
	directReceipt := polynomialDerivativeInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
	if receipt != nil {
		*receipt = directReceipt
	}
	s.active = true
	s.executing = true
	c.derivativeOwner = s
	s.bodyCause = nil
	s.sourceCause = nil
	s.cleanupCause = nil
	s.stopped = false
	defer func() {
		s.executing = false
		s.bodyCause = cause
		if e := m.finishDerivativeNumerals(); e != nil {
			// Return the captured exception cause, never the mutable owning
			// frame. Its first body/source diagnostics remain on the owner.
			if s.bodyCause != nil {
				cause = s.bodyCause
			} else {
				cause = e
			}
			return
		}
		if cause == nil && s.generated == (Monomial{}) && s.calleeGeneration == 0 {
			s.active = false
		}
	}()
	for i := uint32(0); i < size; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		d, e := c.monomials.DegreeOf(term, x)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.traceDerivativeTerm(term, x, d); e != nil {
			return Polynomial{}, e
		}
		if d == 0 {
			continue
		}
		// Source constructs n, sets its degree, then constructs a.
		s.n = unumeral.Int{}
		s.a = unumeral.Int{}
		s.constructed = 1
		s.stage = 0
		s.numericPhase = 1
		e = numericIntegerModularManager_setUint32WithDerivativeReceipt(c.modular, &s.n, d, &directReceipt)
		if e != nil {
			return Polynomial{}, e
		}
		s.constructed |= 2
		coefficient, e := c.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		s.numericPhase = 2
		e = numericIntegerModularManager_mulWithDerivativeReceipt(c.modular, &coefficient, &s.n, &s.a, &directReceipt)
		if e != nil {
			return Polynomial{}, e
		}
		outcome := monomialFactoryOutcome{}
		derivative, e := c.monomials.derivativeCreated(term, x, &outcome)
		if outcome.allocatorInvocation.invocation != 0 {
			s.monomialAllocator = outcome.allocatorInvocation
		}
		if e != nil {
			if outcome.started {
				c.recordGeneratedCallee(outcome)
				s.calleeGeneration = outcome.generation
			}
			return Polynomial{}, e
		}
		if outcome.created {
			s.generated = derivative
		}
		consumed := false
		e = c.cheap.addConsumedReceipts(&s.a, derivative, true, &consumed, nil, &s.append)
		if consumed {
			m.transferDerivativeCreated(derivative)
		}
		if e != nil {
			return Polynomial{}, e
		}
		m.retainDerivativeGenerated()
		if e = m.finishDerivativeNumerals(); e != nil {
			return Polynomial{}, e
		}
	}
	p, e = c.cheap.mkWithAppendReceipt(false, &s.append, &s.factory)
	if e != nil {
		return Polynomial{}, e
	}
	// Every per-term lexical NUM destructor has already completed. Preserve
	// the exact constructed child until metadata acknowledgement succeeds,
	// then hand this returned pointer to the caller without another INC.
	if c.matchesFactoryInvocation(&s.factory) {
		if c.cheap.matchesAppendInvocation(&s.append) {
			if e = c.cheap.acknowledgeFactoryAppendTransfer(s.factory.invocation); e != nil {
				return Polynomial{}, e
			}
		}
		if e = c.transferFactoryResult(&s.factory, p); e != nil {
			return Polynomial{}, e
		}
	}
	return p, nil
}

// Owning abort: only actual fields and matching factory continuation identities.
// No source destructor retry, no caller directory scan, no source counter repair.
func (m *MultivariatePolynomialManager) CleanupDerivative() error {
	return m.cleanupDerivativeMode(true)
}
func (m *MultivariatePolynomialManager) cleanupDerivativeMode(abort bool) error {
	return m.cleanupDerivativeModeOwned(abort, 0, &m.derivative.drain)
}
func (m *MultivariatePolynomialManager) cleanupDerivativeModeOwned(abort bool, invocation uint64, cursor *uint32) error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.derivative
	if !s.active && !m.derivativeHasOrphans() && len(s.orphans) == 0 {
		return nil
	}
	if s.executing {
		return ErrPolynomialPending
	}
	if e := m.core.modular.CleanupPNormalizeInvocation(&s.normalize, false); e != nil {
		s.cleanupCause = e
		return e
	}
	for _, a := range [2]*unumeral.Int{&s.a, &s.n} {
		if e := m.core.integer.Del(a); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	s.constructed = 0
	if e := m.core.cleanupFactoryInvocation(&s.factory, false); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := m.core.cheap.cleanupAppendInvocation(&s.append, false); e != nil {
		s.cleanupCause = e
		return e
	}
	if !abort {
		return nil
	}
	if e := m.core.cleanupFactoryInvocation(&s.factory, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := m.core.cheap.cleanupAppendInvocation(&s.append, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := m.core.modular.CleanupPNormalizeInvocation(&s.normalize, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if e := s.numeric.cleanup(m.core.integer); e != nil {
		s.cleanupCause = e
		return e
	}
	if s.generated != (Monomial{}) {
		if e := m.core.monomials.CleanupValue(&s.generated); e != nil {
			s.cleanupCause = e
			return e
		}
	}
	if e := m.cleanupDerivativeOrphanInvocation(invocation, cursor); e != nil {
		return e
	}
	if s.calleeGeneration != 0 {
		c := m.core
		creation := &c.monomials.creation
		if creation.generation == s.calleeGeneration && creation.parent == c {
			if e := c.monomials.CleanupCreation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		if c.generatedCalleeGeneration == s.calleeGeneration {
			c.generatedCalleePending = false
		}
		s.calleeGeneration = 0
	}
	if e := m.core.monomials.cleanupMonomialAllocatorInvocation(&s.monomialAllocator, true); e != nil {
		s.cleanupCause = e
		return e
	}
	if s.factoryGeneration != 0 {
		p := &m.core.pending
		if p.generation == s.factoryGeneration && p.derivativeParent == s {
			if e := m.core.cleanupPendingCreation(); e != nil {
				s.cleanupCause = e
				return e
			}
		}
		s.factoryGeneration = 0
	}

	s.active = false
	m.releaseDerivativeRetirementIfEmpty()
	s.cleanupCause = nil
	return nil
}

// The tag is published with the actual created root at its append and cleared
// by the actual successful INC. It carries no additional authoritative token.
func (m *MultivariatePolynomialManager) cleanupDerivativeOrphanInvocation(invocation uint64, cursor *uint32) error {
	s := &m.derivative
	for *cursor < uint32(len(s.orphans)) {
		i := *cursor
		if (invocation == 0 || s.orphanInvocations[i] == invocation) && s.orphans[i] != (Monomial{}) {
			if e := m.core.monomials.CleanupValue(&s.orphans[i]); e != nil {
				s.cleanupCause = e
				return e
			}
			s.orphanInvocations[i] = 0
		}
		*cursor++
	}
	return nil
}
func (m *MultivariatePolynomialManager) releaseDerivativeRetirementIfEmpty() {
	s := &m.derivative
	if s.active || m.derivativeHasOrphans() {
		return
	}
	s.orphans = nil
	s.orphanInvocations = nil
	s.drain = 0
}

func (m *MultivariatePolynomialManager) derivativeInvocationHasOrphans(r *polynomialDerivativeInvocationReceipt) bool {
	if r == nil || r.manager != m || r.frame != &m.derivative || r.invocation == 0 {
		return false
	}
	for i, p := range m.derivative.orphans {
		if p != (Monomial{}) && m.derivative.orphanInvocations[i] == r.invocation {
			return true
		}
	}
	return false
}

// Source: polynomial.cpp:4518, before the d>0 branch and its two numerals.
type polynomialDerivativeTraceBorrow struct{ executing bool }

func (b *polynomialDerivativeTraceBorrow) NumericExecuting() bool { return b.executing }
func (m *MultivariatePolynomialManager) traceDerivativeTerm(term Monomial, x, d uint32) error {
	c := m.core
	if !c.monomials.monomialTraceTags().ShouldTrace("polynomial") {
		return nil
	}
	borrow := polynomialDerivativeTraceBorrow{executing: true}
	var integer, natural unumeral.NumericCallerBinding
	if e := c.integer.BindCaller(&integer, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := integer.Unbind(); e != nil {
			panic(e)
		}
	}()
	if e := c.integer.Natural.BindCaller(&natural, &borrow, 0); e != nil {
		return e
	}
	defer func() {
		if e := natural.Unbind(); e != nil {
			panic(e)
		}
	}()
	return m.emitFactorTraceTag(nil, "polynomial", "derivative", 4518, func(out *polynomialTraceSink) error {
		if e := c.monomials.displayIntoCore(term, out, false); e != nil {
			return e
		}
		_ = out.WriteLiteral(" degree_of x")
		_ = out.WriteUnsigned(x)
		_ = out.WriteLiteral(": ")
		_ = out.WriteUnsigned(d)
		return out.WriteLiteral("\n")
	})
}
