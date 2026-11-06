package polynomial

// Source: src/math/polynomial/polynomial.cpp:7439-7476; src/util/buffer.h;
// src/util/scoped_numeral_buffer.h.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	// The native monomial slice borrows handles; numeral elements own their values.
	// orphans retains only newly created zero-reference monomials for owning abort.
)

type polynomialGlobalConversionScope struct {
	algebraicCaller                      *algebraicSignResultantScope
	isolationCaller                      *algebraicMultivariateIsolationFrame
	ms                                   []Monomial
	as                                   unumeral.ScopedIntegerModularBuffer128
	orphans                              []Monomial
	result                               Polynomial
	monomialRaw                          [3]umemory.Block
	factoryGeneration, calleeGeneration  uint64
	active, executing, ownsResult        bool
	bodyCause, sourceCause, cleanupCause error
}

func ConvertPolynomial(sm *MultivariatePolynomialManager, p Polynomial, tm *MultivariatePolynomialManager, x, maxD uint32) (Polynomial, error) {
	return convertPolynomialForAlgebraic(sm, p, tm, x, maxD, nil)
}
func convertPolynomialForAlgebraic(sm *MultivariatePolynomialManager, p Polynomial, tm *MultivariatePolynomialManager, x, maxD uint32, caller *algebraicSignResultantScope) (out Polynomial, cause error) {
	return convertPolynomialForIsolation(sm, p, tm, x, maxD, caller, nil)
}
func convertPolynomialForIsolation(sm *MultivariatePolynomialManager, p Polynomial, tm *MultivariatePolynomialManager, x, maxD uint32, caller *algebraicSignResultantScope, isolation *algebraicMultivariateIsolationFrame) (out Polynomial, cause error) {
	if e := sm.ready(); e != nil {
		return Polynomial{}, e
	}
	if e := tm.ready(); e != nil {
		return Polynomial{}, e
	}
	if e := tm.beginConversion(); e != nil {
		return Polynomial{}, e
	}
	s := &tm.conversion
	s.algebraicCaller = caller
	s.isolationCaller = isolation
	if isolation != nil {
		isolation.conversionManager = tm
	}
	if caller != nil {
		caller.conversionManager = tm
	}
	defer func() {
		cause = tm.finishConversion(cause)
		if cause != nil {
			out = Polynomial{}
			return
		}
		s.result = Polynomial{}
		s.ownsResult = false
		s.active = false
	}()
	n, e := sm.core.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	if sm == tm {
		return p, nil
	}
	for i := uint32(0); i < n; i++ {
		term, e := sm.core.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if x != ^uint32(0) {
			degree, e := sm.core.monomials.DegreeOf(term, x)
			if e != nil {
				return Polynomial{}, e
			}
			if degree > maxD {
				continue
			}
		}
		if sm.core.monomials != tm.core.monomials {
			term, e = tm.conversionMonomial(sm.core.monomials, term)
			if e != nil {
				return Polynomial{}, e
			}
		}
		s.ms = append(s.ms, term)
		zero := unumeral.Int{}
		if e = s.as.Push(&zero); e != nil {
			return Polynomial{}, e
		}
		coefficient, e := sm.core.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = tm.conversionSet(s.as.Count()-1, sm.core.integer, &coefficient); e != nil {
			return Polynomial{}, e
		}
	}
	av, e := s.as.View()
	if e != nil {
		return Polynomial{}, e
	}
	mv := monomialArrayView{values: s.ms, count: uint32(len(s.ms))}
	if e = tm.core.som.reset(); e != nil {
		return Polynomial{}, e
	}
	return tm.core.mkTermsSource(s.as.Count(), polynomialUnivariateCoefficients{array: av, indexed: true}, mv)
}
func (m *MultivariatePolynomialManager) beginConversion() error {
	s := &m.conversion
	if s.active || m.conversionHasOrphans() {
		return ErrPolynomialPending
	}
	old := s.orphans
	*s = polynomialGlobalConversionScope{orphans: old[:0], active: true, executing: true}
	s.as.Construct(m.core.modular) // second source constructor
	m.core.conversionOwner = s
	return nil
}
func (m *MultivariatePolynomialManager) conversionHasOrphans() bool {
	for _, a := range m.conversion.orphans {
		if a != (Monomial{}) {
			return true
		}
	}
	return false
}
func (m *MultivariatePolynomialManager) conversionSet(i uint32, source *unumeral.IntegerManager, v *unumeral.Int) (err error) {
	b := &m.conversion.as
	a, e := b.Physical(i)
	if e != nil {
		return e
	}
	before, e := unumeral.CaptureNumericAllocationRoots(m.core.integer)
	if e != nil {
		return e
	}
	err = m.core.modular.CopyFromOwner(&a, source, v)
	err = polynomialNewtonJoin(err, polynomialNewtonJoin(b.AllocationRoots().After(m.core.integer, before), b.Store(i, a)))
	if err != nil && b.BodyCause() == nil {
		b.RecordBodyCause(err)
	}
	return err
}
func (m *MultivariatePolynomialManager) conversionMonomial(source *MonomialManager, a Monomial) (Monomial, error) {
	s := &m.conversion
	c := m.core
	before, e := c.monomials.allocator.PendingRoots()
	if e != nil {
		return Monomial{}, e
	}
	outcome := monomialFactoryOutcome{}
	term, e := c.monomials.convertCreated(source, a, &outcome)
	if outcome.started && c.monomials.creation.generation == outcome.generation {
		s.calleeGeneration = outcome.generation
		c.monomials.creation.parent = c
	}
	after, probe := c.monomials.allocator.PendingRoots()
	if before == ([3]umemory.Block{}) {
		s.monomialRaw = after
	}
	if e != nil || probe != nil {
		return Monomial{}, polynomialNewtonJoin(e, probe)
	}
	if outcome.created {
		i := len(s.orphans)
		s.orphans = append(s.orphans, term)
		l, e := c.monomials.location(term)
		if e != nil {
			return Monomial{}, e
		}
		l.conversionCohort = s
		l.conversionSlot = uint32(i)
	}
	return term, nil
}
func (m *MultivariatePolynomialManager) finishConversion(body error) error {
	s := &m.conversion
	s.executing = false
	if s.bodyCause == nil {
		s.bodyCause = body
	}
	if s.sourceCause != nil {
		return polynomialNewtonJoin(body, s.sourceCause)
	}
	e := s.as.SourceClose()
	if e == nil {
		s.ms = nil
	}
	if e != nil {
		s.sourceCause = e
	}
	return polynomialNewtonJoin(body, e)
}
func (m *MultivariatePolynomialManager) CleanupConversion() error {
	if e := m.identity(); e != nil {
		return e
	}
	s := &m.conversion
	c := m.core
	if s.executing {
		return ErrPolynomialPending
	}
	fail := func(e error) error { s.cleanupCause = e; return e }
	if s.factoryGeneration != 0 {
		if c.pending.generation == s.factoryGeneration && c.pending.conversionParent == s {
			if e := c.cleanupPendingCreation(); e != nil {
				return fail(e)
			}
		}
		s.factoryGeneration = 0
	}
	if s.calleeGeneration != 0 {
		if c.monomials.creation.generation == s.calleeGeneration && c.monomials.creation.parent == c {
			if e := c.monomials.CleanupCreation(); e != nil {
				return fail(e)
			}
		}
		s.calleeGeneration = 0
	}
	if s.ownsResult && s.result != (Polynomial{}) {
		if e := c.CleanupValue(&s.result); e != nil {
			return fail(e)
		}
		s.ownsResult = false
	}
	if e := s.as.Cleanup(); e != nil {
		return fail(e)
	}
	s.ms = nil
	for i := range s.orphans {
		if s.orphans[i] != (Monomial{}) {
			if e := c.monomials.CleanupValue(&s.orphans[i]); e != nil {
				return fail(e)
			}
		}
	}
	if s.monomialRaw != ([3]umemory.Block{}) {
		now, e := c.monomials.allocator.PendingRoots()
		if e != nil {
			return fail(e)
		}
		if now == s.monomialRaw {
			e = c.monomials.allocator.CleanupPending()
			after, x := c.monomials.allocator.PendingRoots()
			for i := range s.monomialRaw {
				if after[i] == (umemory.Block{}) {
					s.monomialRaw[i] = umemory.Block{}
				}
			}
			if e != nil || x != nil {
				return fail(polynomialNewtonJoin(e, x))
			}
		}
		s.monomialRaw = [3]umemory.Block{}
	}
	s.orphans = nil
	s.active = false
	s.cleanupCause = nil
	return nil
}
