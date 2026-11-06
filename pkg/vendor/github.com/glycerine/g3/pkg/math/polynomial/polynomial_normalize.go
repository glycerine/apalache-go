package polynomial

// Source: src/math/polynomial/polynomial.cpp:5904-5940;
// src/math/polynomial/polynomial.h:873.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialNormalizeScope struct {
	converterHistoryParent           polynomialFactorNumeralVectorReceipt
	som                              polynomialSOMNormalizeInvocationReceipt
	directReceipt                    polynomialNormalizeInvocationReceipt
	executing, finishing             bool
	sourceCause                      error
	normalize                        unumeral.IntegerNormalizeInvocationReceipt
	gcdRaw, divisionRaw, extendedRaw unumeral.IntegerRawInvocationReceipt

	append  polynomialCheapAppendInvocationReceipt
	factory polynomialFactoryInvocationReceipt

	invocation                                   uint64
	g, a                                         unumeral.Int
	active, gConstructed, aConstructed, topOwner bool
	generation                                   uint64
	cursor, ownCursor                            uint8
	somInvocation                                uint64
	numeric                                      polynomialNumericRoots
	bodyCause, cleanupCause                      error
}

func (m *MultivariatePolynomialManager) beginNormalization() error {
	if m == nil || m.self != m || m.core == nil || m.core.self != m.core {
		return ErrPolynomialOwner
	}
	if m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	s := &m.normalization
	if s.active || s.finishing {
		return ErrPolynomialPending
	}
	if s.invocation == ^uint64(0) {
		return arena.ErrBounds
	}
	c := m.core
	top := c.pscReceipts.depth == 0
	if e := c.pscEnter(); e != nil {
		return e
	}
	*s = polynomialNormalizeScope{invocation: s.invocation + 1, active: true, topOwner: top, generation: c.pscReceipts.generation}
	s.directReceipt = polynomialNormalizeInvocationReceipt{m, s, s.invocation}
	c.normalizationOwner = s.directReceipt
	return nil
}
func (s *polynomialNormalizeScope) directPending(m *MultivariatePolynomialManager) bool {
	c := m.core
	if polynomialNegNumericPending(&s.numeric) || c.integer.MatchesRawInvocation(&s.gcdRaw) || c.integer.MatchesRawInvocation(&s.divisionRaw) || c.integer.MatchesRawInvocation(&s.extendedRaw) || c.modular.MatchesPNormalizeInvocation(&s.normalize) || c.cheap.matchesAppendInvocation(&s.append) || c.matchesFactoryInvocation(&s.factory) {
		return true
	}

	return c.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.active
}
func (m *MultivariatePolynomialManager) finishNormalizationSource() error {
	if m == nil || m.self != m || m.core == nil || m.core.self != m.core {
		return ErrPolynomialOwner
	}
	s := &m.normalization
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if m.core.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.sourceCause != nil {
		s.sourceCause = s.som.frame.sourceCause
		return s.sourceCause
	}
	if e := numericIntegerManager_normalizationNumericSourceCause(m.core.integer, &s.directReceipt); e != nil {
		s.sourceCause = e
		return e
	}
	if s.cursor == 0 {
		if s.aConstructed {
			if _, e := upolynomialSourceNumeralDel(m.core.integer, &s.a); e != nil {
				s.sourceCause = e
				return e
			}
		}
		s.cursor = 1
	}
	if s.cursor == 1 {
		if s.gConstructed {
			if _, e := upolynomialSourceNumeralDel(m.core.integer, &s.g); e != nil {
				s.sourceCause = e
				return e
			}
		}
		s.cursor = 2
	}
	if !s.directPending(m) {
		s.active = false
	}
	return nil
}
func (m *MultivariatePolynomialManager) CleanupNormalization() error {
	if m == nil || m.self != m || m.core == nil || m.core.self != m.core {
		return ErrPolynomialOwner
	}
	if e := m.cleanupNormalizationPhase(false); e != nil {
		return e
	}
	return m.cleanupNormalizationPhase(true)
}
func (m *MultivariatePolynomialManager) cleanupNormalizationPhase(abort bool) (err error) {
	s := &m.normalization
	if s.executing || s.finishing || m.core.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	if !s.active {
		return nil
	}
	s.finishing = true
	defer func() {
		s.finishing = false
		if err != nil && s.cleanupCause == nil {
			s.cleanupCause = err
		}
	}()
	c := m.core
	b := &c.cheap
	if c.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.active {
		if e := b.finishNormalizeNumeralPhase(true, false); e != nil {
			return e
		}
	}
	if e := numericIntegerManager_cleanupNormalizationNumeric(c.integer, &s.directReceipt, false); e != nil {
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.factory, false); e != nil {
		return e
	}
	if e := b.cleanupAppendInvocation(&s.append, false); e != nil {
		return e
	}
	if s.ownCursor == 0 {
		if s.aConstructed {
			if e := c.deleteNumeral(&s.a); e != nil {
				return e
			}
		}
		s.ownCursor = 1
	}
	if s.ownCursor == 1 {
		if s.gConstructed {
			if e := c.deleteNumeral(&s.g); e != nil {
				return e
			}
		}
		s.ownCursor = 2
	}
	if !abort {
		return nil
	}
	if c.hasSOMNormalizeInvocationHistory(&s.som) && s.som.frame.active {
		if e := b.finishNormalizeNumeralPhase(true, true); e != nil {
			return e
		}
	}
	if e := numericIntegerManager_cleanupNormalizationNumeric(c.integer, &s.directReceipt, true); e != nil {
		return e
	}
	if e := c.cleanupFactoryInvocation(&s.factory, true); e != nil {
		return e
	}
	if e := b.cleanupAppendInvocation(&s.append, true); e != nil {
		return e
	}
	s.active = false
	return nil
}
func (m *MultivariatePolynomialManager) Normalize(p Polynomial) (Polynomial, error) {
	return m.normalizeWithReceipt(p, nil)
}
func (m *MultivariatePolynomialManager) normalizeWithReceipt(p Polynomial, receipt *polynomialNormalizeInvocationReceipt) (Polynomial, error) {
	return m.normalizeWithFactorNumeralVectorHistory(p, receipt, nil)
}
func (m *MultivariatePolynomialManager) normalizeWithFactorNumeralVectorHistory(p Polynomial, receipt *polynomialNormalizeInvocationReceipt, parent *polynomialFactorNumeralVectorReceipt) (result Polynomial, err error) {
	if e := m.ready(); e != nil {
		return Polynomial{}, e
	}
	if parent != nil && (!m.matchesFactorNumeralVector(parent) || !parent.frame.active || !parent.frame.executing || m.core.callbackDepth != 0) {
		return Polynomial{}, ErrPolynomialPending
	}
	c := m.core
	zero, e := c.IsZero(p)
	if e != nil || zero {
		return p, e
	}
	if m.normalization.active || m.normalization.finishing || c.callbackDepth != 0 {
		return Polynomial{}, ErrPolynomialPending
	}
	n, e := c.readWord(p, 8)
	if e != nil {
		return Polynomial{}, e
	}
	field, e := c.modular.Modular()
	if e != nil {
		return Polynomial{}, e
	}
	started := false
	defer func() {
		if !started {
			return
		}
		s := &m.normalization
		s.bodyCause = err
		s.executing = false
		cleanup := m.finishNormalizationSource()
		c.pscLeave()
		if cleanup != nil {
			err = errors.Join(err, cleanup)
		}
		if err == nil && c.matchesFactoryInvocation(&s.factory) {
			if e := c.cheap.cleanupAppendInvocation(&s.append, false); e != nil {
				err = e
				result = Polynomial{}
				if s.cleanupCause == nil {
					s.cleanupCause = e
				}
				return
			}
			if e := c.transferFactoryResult(&s.factory, result); e != nil {
				err = e
				result = Polynomial{}
				return
			}
			s.active = s.directPending(m)
		}
		if err != nil && c.matchesFactoryInvocation(&s.factory) {
			result = Polynomial{}
		}
	}()
	if field {
		i := uint32(0)
		for ; i < n; i++ {
			a, e := c.coefficientCore(p, i)
			if e != nil {
				return Polynomial{}, e
			}
			normal, e := c.modular.IsPNormalized(&a)
			if e != nil {
				return Polynomial{}, e
			}
			if !normal {
				break
			}
		}
		if i < n {
			if e = c.cheap.resetWithNormalizationReceipt(&m.normalization.directReceipt); e != nil {
				return Polynomial{}, e
			}
			if e = m.beginNormalization(); e != nil {
				return Polynomial{}, e
			}
			started = true
			m.normalization.executing = true
			if parent != nil {
				m.normalization.converterHistoryParent = *parent
			}
			if receipt != nil {
				*receipt = polynomialNormalizeInvocationReceipt{m, &m.normalization, m.normalization.invocation}
			}
			s := &m.normalization
			s.aConstructed = true
			for i := uint32(0); i < n; i++ {
				term, e := c.monomialCore(p, i)
				if e != nil {
					return Polynomial{}, e
				}
				a, e := c.coefficientCore(p, i)
				if e != nil {
					return Polynomial{}, e
				}
				e = numericIntegerModularManager_copyWithNormalizeReceipt(c.modular, &s.a, &a, &s.directReceipt)
				if e != nil {
					return Polynomial{}, e
				}
				if e = c.cheap.addConsumedReceipts(&s.a, term, true, nil, nil, &s.append); e != nil {
					return Polynomial{}, e
				}
			}
			if e = c.cheap.normalizeWithOwner(s); e != nil {
				return Polynomial{}, e
			}
			return c.cheap.mkWithAppendReceipt(false, &s.append, &s.factory)
		}
	}
	if e = m.beginNormalization(); e != nil {
		return Polynomial{}, e
	}
	started = true
	m.normalization.executing = true
	if parent != nil {
		m.normalization.converterHistoryParent = *parent
	}
	if receipt != nil {
		*receipt = polynomialNormalizeInvocationReceipt{m, &m.normalization, m.normalization.invocation}
	}
	s := &m.normalization
	s.gConstructed = true
	l, e := c.polynomialLocation(p)
	if e != nil {
		return Polynomial{}, e
	}
	e = numericIntegerModularManager_gcdManyIndexedWithNormalizeReceipt(c.modular, unumeral.IntegerArrayView{Allocator: c.monomials.allocator, Object: l.raw, Count: n, Offset: 12, Stride: 12}, &s.g, &s.directReceipt)
	if e != nil {
		return Polynomial{}, e
	}
	one, e := c.modular.IsOne(&s.g)
	if e != nil {
		return Polynomial{}, e
	}
	if one {
		return p, nil
	}
	if e = c.cheap.resetWithNormalizationReceipt(&m.normalization.directReceipt); e != nil {
		return Polynomial{}, e
	}
	s.aConstructed = true
	for i := uint32(0); i < n; i++ {
		term, e := c.monomialCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		a, e := c.coefficientCore(p, i)
		if e != nil {
			return Polynomial{}, e
		}
		e = numericIntegerModularManager_divWithNormalizeReceipt(c.modular, &a, &s.g, &s.a, &s.directReceipt)
		if e != nil {
			return Polynomial{}, e
		}
		if e = c.cheap.addConsumedReceipts(&s.a, term, true, nil, nil, &s.append); e != nil {
			return Polynomial{}, e
		}
	}
	return c.cheap.mkWithAppendReceipt(false, &s.append, &s.factory)
}
