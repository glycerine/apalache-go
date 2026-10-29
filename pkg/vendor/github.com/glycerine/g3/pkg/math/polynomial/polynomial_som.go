package polynomial

// Source: src/math/polynomial/polynomial.cpp:1885-2156,2217-2300.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialSOM struct {
	normalizeScope                              polynomialSOMNormalizeScope
	cohortMovingCoefficient                     unumeral.Int
	cohortMovingMonomial                        Monomial
	coefficientOwners, monomialOwners           []uint64
	coefficientTransferred, monomialTransferred []uint64
	arithmeticScope                             polynomialArithmeticScope
	appendScope                                 polynomialCheapAppendScope
	factorsParent                               *PolynomialFactors
	factorsGeneration                           uint64
	retirementOwner                             *PolynomialSOMBuffer
	borrowedAppend                              bool
	borrowedAppendIndex                         uint32
	unreferencedConsumed                        bool
	unreferencedIndex                           uint32
	removeActive, removeStopped                 bool
	removePrefix, removeSuffix                  uint32
	owner                                       *MultivariatePolynomialCore
	positions                                   MonomialPositions
	as                                          polynomialNumerals
	ms                                          polynomialMonomials
	gcd                                         unumeral.Int
	gcdActive                                   bool
	gcdEnded                                    bool
	gcdGeneration                               uint64
	gcdInvocation                               uint64
	gcdSubstitution                             *polynomialSubstitutionScope
	gcdNormalization                            *polynomialNormalizeScope
	gcdNumeric                                  polynomialNumericRoots
	gcdBodyCause, gcdCleanupCause               error
	pendingMonomial                             Monomial
	transferred, cleanupIndex                   uint32
	cleanupDropConsumed, cleanupDropLast        bool
	cheap, hasPendingReference                  bool
	resetActive                                 bool
	resetIndex                                  uint32
	resetStage                                  uint8
}

func (b *polynomialSOM) init(m *MultivariatePolynomialCore, cheap bool) {
	b.owner = m
	b.cheap = cheap

	b.positions.manager = m.monomials
	b.positions.self = &b.positions
}
func (b *polynomialSOM) add(a *unumeral.Int, term Monomial, take bool) error {
	return b.addConsumed(a, term, take, nil)
}
func (b *polynomialSOM) addConsumed(a *unumeral.Int, term Monomial, take bool, consumed *bool) error {
	return b.addConsumedNumeric(a, term, take, consumed, nil)
}
func (b *polynomialSOM) addConsumedNumeric(a *unumeral.Int, term Monomial, take bool, consumed *bool, receipt *polynomialNumericRoots) error {
	return b.addConsumedReceipts(a, term, take, consumed, receipt, nil)
}
func (b *polynomialSOM) addConsumedReceipts(a *unumeral.Int, term Monomial, take bool, consumed *bool, receipt *polynomialNumericRoots, appendReceipt *polynomialCheapAppendInvocationReceipt) (cause error) {
	return b.addConsumedCoefficientReceipts(a, term, take, consumed, receipt, appendReceipt, nil)
}
func (b *polynomialSOM) addConsumedCoefficientReceipts(a *unumeral.Int, term Monomial, take bool, consumed *bool, receipt *polynomialNumericRoots, appendReceipt *polynomialCheapAppendInvocationReceipt, coefficientReceipt *polynomialCoefficientInvocationReceipt) (cause error) {
	return b.addConsumedScalarDivEngine(a, term, take, consumed, receipt, appendReceipt, coefficientReceipt, nil, nil)
}
func (b *polynomialSOM) addConsumedScalarDivReceipts(a *unumeral.Int, term Monomial, take bool, consumed *bool, appendReceipt *polynomialCheapAppendInvocationReceipt, scalarReceipt *polynomialScalarDivInvocationReceipt) error {
	return b.addConsumedScalarDivEngine(a, term, take, consumed, nil, appendReceipt, nil, scalarReceipt, nil)
}
func (b *polynomialSOM) addConsumedNegReceipts(a *unumeral.Int, term Monomial, take bool, consumed *bool, appendReceipt *polynomialCheapAppendInvocationReceipt, negReceipt *polynomialNegInvocationReceipt) error {
	return b.addConsumedScalarNegEngine(a, term, take, consumed, nil, appendReceipt, nil, nil, nil, negReceipt)
}
func (b *polynomialSOM) addConsumedScalarDivEngine(a *unumeral.Int, term Monomial, take bool, consumed *bool, receipt *polynomialNumericRoots, appendReceipt *polynomialCheapAppendInvocationReceipt, coefficientReceipt *polynomialCoefficientInvocationReceipt, scalarReceipt *polynomialScalarDivInvocationReceipt, arithmeticReceipt *polynomialArithmeticInvocationReceipt) error {
	return b.addConsumedScalarNegEngine(a, term, take, consumed, receipt, appendReceipt, coefficientReceipt, scalarReceipt, arithmeticReceipt, nil)
}
func (b *polynomialSOM) addConsumedScalarNegEngine(a *unumeral.Int, term Monomial, take bool, consumed *bool, receipt *polynomialNumericRoots, appendReceipt *polynomialCheapAppendInvocationReceipt, coefficientReceipt *polynomialCoefficientInvocationReceipt, scalarReceipt *polynomialScalarDivInvocationReceipt, arithmeticReceipt *polynomialArithmeticInvocationReceipt, negReceipt *polynomialNegInvocationReceipt) (cause error) {
	if consumed != nil {
		*consumed = false
	}
	m := b.owner
	if b.cheap {
		line := int32(2248)
		if take {
			line = 2234
		}
		if e := b.assertUniqueMonomial(term, line); e != nil {
			return e
		}
	}
	zero, e := m.modular.IsZero(a)
	if e != nil || zero {
		return e
	}
	if arithmeticReceipt != nil {
		if !m.matchesArithmeticInvocation(arithmeticReceipt) || arithmeticReceipt.frame.buffer != b || arithmeticReceipt.frame.executing {
			return ErrPolynomialOwner
		}
		arithmeticReceipt.frame.executing = true
		defer func() { arithmeticReceipt.frame.executing = false; arithmeticReceipt.frame.bodyCause = cause }()
		appendReceipt = &arithmeticReceipt.frame.append
	}
	// An owning compaction has already changed the source columns. Ordinary
	// append may extend an earlier deletion prefix, but cannot interpret or
	// overwrite a partially compacted member layout.
	if b.appendScope.active && b.appendScope.retirePhase >= 3 && b.appendScope.retirePhase <= 5 {
		return ErrPolynomialPending
	}
	pos := ^uint32(0)
	if !b.cheap {
		pos, e = b.positions.Get(term)
		if e != nil {
			return e
		}
	}
	if pos != ^uint32(0) {
		priorTransfer := b.accumulatorRowTransfer(pos)
		old, e := b.as.coefficient(m, pos)
		if e != nil {
			return e
		}
		priorTransfer.coefficientValue, priorTransfer.monomialValue = old, term
		if arithmeticReceipt != nil {
			e = b.arithmeticNumericCall(1, &old, a, nil, &old, arithmeticReceipt)
		} else {
			e = b.addNumericLeaf(receipt, 1, &old, a, nil, &old)
		}
		w := b.as.putCoefficient(m, pos, old)
		if e == nil && w == nil && arithmeticReceipt == nil && appendReceipt == nil {
			b.transferAccumulatorRowToMember(pos, priorTransfer)
		}
		if e != nil {
			return e
		}
		return w
	}
	n, e := b.ms.size(m)
	if e != nil {
		return e
	}
	var appendScope *polynomialCheapAppendScope
	if appendReceipt != nil {
		if e = b.acquireAppendInvocation(appendReceipt, n); e != nil {
			return e
		}
		appendScope = &b.appendScope
		receipt = &appendScope.numeric
		defer func() {
			if cause != nil {
				appendScope.stopped = true
			}
		}()
	}
	if b.appendScope.active {
		if e = b.prepareAppendTags(n + 1); e != nil {
			return e
		}
	}

	if !b.cheap {
		if e = b.positions.Set(term, n); e != nil {
			return e
		}
	}
	if b.cheap {
		observer := appendScope
		if observer == nil && b.appendScope.active {
			observer = &b.appendScope
		}
		if e = b.as.appendCoefficientWithAppendScope(m, unumeral.Int{}, observer, appendScope != nil); e != nil {
			return e
		}
		value, e := b.as.coefficient(m, n)
		if e != nil {
			return e
		}
		var transport *unumeral.Int
		if scalarReceipt != nil {
			if numericIntegerManager_scalarDivAllocationRoots(m.integer, scalarReceipt) == nil || b != &m.cheap || !m.matchesScalarDivInvocation(scalarReceipt) || scalarReceipt.frame.transportBound {
				return ErrPolynomialPending
			}
			s := scalarReceipt.frame
			// The actual source default NUM already exists at this slot. This
			// adapter holder becomes the SET destination until codec publication.
			s.transport, s.transportBuffer, s.transportSlot, s.transportBound = value, b, n, true
			transport = &s.transport
		} else {
			transport = &value
		}
		if negReceipt != nil {
			if b != &m.cheap || numericIntegerManager_negAllocationRoots(m.integer, negReceipt) == nil || !m.matchesNegInvocation(negReceipt) || negReceipt.frame.transportBound {
				return ErrPolynomialOwner
			}
			s := negReceipt.frame
			s.transport = value
			s.transportBuffer, s.transportSlot, s.transportBound = b, n, true
			transport = &s.transport
		}
		if take {
			e = m.modular.Swap(transport, a)
		} else if scalarReceipt != nil {
			e = numericIntegerModularManager_copyWithScalarDivReceipt(m.modular, transport, a, scalarReceipt)
		} else if negReceipt != nil {
			e = numericIntegerModularManager_copyWithNegReceipt(m.modular, transport, a, negReceipt)
		} else if arithmeticReceipt != nil {
			e = b.arithmeticNumericCall(0, a, nil, nil, transport, arithmeticReceipt)
		} else {
			e = b.addNumericLeaf(receipt, 0, a, nil, nil, transport)
		}
		if w := b.as.putCoefficient(m, n, *transport); w != nil {
			return w
		}
		if scalarReceipt != nil {
			s := scalarReceipt.frame
			s.transport, s.transportBuffer, s.transportSlot, s.transportBound = unumeral.Int{}, nil, 0, false
		}
		if negReceipt != nil {
			s := negReceipt.frame
			s.transport, s.transportBuffer, s.transportSlot, s.transportBound = unumeral.Int{}, nil, 0, false
		}
		if e != nil {
			return e
		}

	}
	if e = m.monomials.IncRef(term); e != nil {
		return e
	}
	if arithmeticReceipt != nil && arithmeticReceipt.frame.generated == term {
		arithmeticReceipt.frame.generated = Monomial{}
	}
	m.transferCoefficientGenerated(coefficientReceipt, term)
	if appendScope != nil {
		appendScope.pending = term
	}
	if consumed != nil {
		*consumed = true
	}
	b.pendingMonomial = term
	b.hasPendingReference = true
	if m.generatedActive && m.generatedMonomial == term {
		m.transferGeneratedMonomial()
	}
	observer := appendScope
	if observer == nil && b.appendScope.active {
		observer = &b.appendScope
	}
	if e = b.ms.appendMonomialWithAppendScope(m, term, observer, appendScope != nil); e != nil {
		return e
	}
	if appendScope != nil {
		appendScope.pending = Monomial{}
		appendScope.monomials = n + 1
	}
	b.hasPendingReference = false
	b.pendingMonomial = Monomial{}
	if !b.cheap {
		if e = b.as.appendCoefficientWithAppendScope(m, unumeral.Int{}, observer, appendScope != nil); e != nil {
			return e
		}
		value, e := b.as.coefficient(m, n)
		if e != nil {
			return e
		}
		var target *unumeral.Int = &value
		if arithmeticReceipt != nil {
			s := arithmeticReceipt.frame
			s.transport = value
			s.transportSlot = n
			s.transportBound = true
			target = &s.transport
		}
		if take {
			e = m.modular.Swap(target, a)
		} else {
			if arithmeticReceipt != nil {
				e = b.arithmeticNumericCall(0, a, nil, nil, target, arithmeticReceipt)
			} else {
				e = b.addNumericLeaf(receipt, 0, a, nil, nil, target)
			}
		}
		if w := b.as.putCoefficient(m, n, *target); w != nil {
			return w
		}
		if arithmeticReceipt != nil {
			arithmeticReceipt.frame.transport = unumeral.Int{}
			arithmeticReceipt.frame.transportBound = false
		}
		return e
	}
	return nil
}

type polynomialSOMInput struct {
	polynomial Polynomial
	buffer     *PolynomialSOMBuffer
	fromBuffer bool
}

func (p polynomialSOMInput) size(m *MultivariatePolynomialCore) (uint32, error) {
	if !p.fromBuffer {
		return m.readWord(p.polynomial, 8)
	}
	if p.buffer == nil || p.buffer.self != p.buffer || p.buffer.storage != m {
		return 0, ErrPolynomialOwner
	}
	if p.buffer.closed || p.buffer.closing || p.buffer.sourceStarted {
		return 0, arena.ErrClosed
	}
	return p.buffer.buffer.ms.size(m)
}
func (p polynomialSOMInput) monomial(m *MultivariatePolynomialCore, i uint32) (Monomial, error) {
	if !p.fromBuffer {
		return m.monomialCore(p.polynomial, i)
	}
	if polynomialSourceDebug {
		n, e := p.buffer.buffer.ms.size(m)
		if e != nil {
			return Monomial{}, e
		}
		if i >= n {
			return Monomial{}, ErrPolynomialAssertion
		}
	}
	return p.buffer.buffer.ms.monomial(m, i)
}
func (p polynomialSOMInput) coefficient(m *MultivariatePolynomialCore, i uint32) (unumeral.Int, error) {
	if !p.fromBuffer {
		return m.coefficientCore(p.polynomial, i)
	}
	if polynomialSourceDebug {
		n, e := p.buffer.buffer.as.size(m)
		if e != nil {
			return unumeral.Int{}, e
		}
		if i >= n {
			return unumeral.Int{}, ErrPolynomialAssertion
		}
	}
	return p.buffer.buffer.as.coefficient(m, i)
}
func (b *polynomialSOM) addMul(a *unumeral.Int, term Monomial, p Polynomial) error {
	return b.addSourceInput(a, term, polynomialSOMInput{polynomial: p}, true)
}
func (b *polynomialSOM) addMulWithArithmeticReceipt(a *unumeral.Int, term Monomial, p Polynomial, receipt *polynomialArithmeticInvocationReceipt) error {
	return b.addSourceInputWithArithmeticReceipt(a, term, polynomialSOMInput{polynomial: p}, true, receipt)
}
func (b *polynomialSOM) addSourceInput(a *unumeral.Int, term Monomial, p polynomialSOMInput, multiply bool) error {
	return b.addSourceInputWithArithmeticReceipt(a, term, p, multiply, nil)
}
func (b *polynomialSOM) addSourceInputWithArithmeticReceipt(a *unumeral.Int, term Monomial, p polynomialSOMInput, multiply bool, receipt *polynomialArithmeticInvocationReceipt) (cause error) {
	m := b.owner
	if multiply {
		z, e := m.modular.IsZero(a)
		if e != nil || z {
			return e
		}
	}
	if b.appendScope.active && b.appendScope.retirePhase >= 3 && b.appendScope.retirePhase <= 5 {
		return ErrPolynomialPending
	}
	// The erased zero route above never acquires or inspects this invocation.
	// The source body uses only the actual buffer named by its guarded caller.
	if receipt != nil {
		if !m.matchesArithmeticInvocation(receipt) || receipt.frame.buffer != b || m.callbackDepth != 0 {
			return ErrPolynomialOwner
		}
		frame := receipt.frame
		frame.executing = true
		defer func() {
			frame.executing = false
			if frame.bodyCause == nil {
				frame.bodyCause = cause
			}
			if frame.sourceCause != nil {
				frame.stopped = true
			}
		}()
	}

	n, e := p.size(m)
	if e != nil {
		return e
	}
	if n > 0 {
		if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
			b.factorsParent = f
			b.factorsGeneration = f.generation
			if b.cheap {
				f.cheapOwned = true
			} else {
				f.somOwned = true
			}
		}
	}
	for i := uint32(0); i < n; i++ {
		other, e := p.monomial(m, i)
		if e != nil {
			return e
		}
		prod := other
		if multiply {
			prod, e = b.arithmeticProduct(term, other, receipt)
			if e != nil {
				return e
			}
		}
		pos := ^uint32(0)
		if !b.cheap {
			pos, e = b.positions.Get(prod)
			if e != nil {
				return e
			}
		}
		if pos == ^uint32(0) {
			n, e := b.ms.size(m)
			if e != nil {
				return e
			}
			if b.cheap {
				if e := b.assertUniqueMonomial(prod, 2270); e != nil {
					return e
				}
			}
			var appendScope *polynomialCheapAppendScope
			if receipt != nil {
				if e = b.acquireAppendInvocation(&receipt.frame.append, n); e != nil {
					return e
				}
				appendScope = &b.appendScope
			}
			if b.appendScope.active {
				if e = b.prepareAppendTags(n + 1); e != nil {
					return e
				}
			}
			if !b.cheap {
				if e = b.positions.Set(prod, n); e != nil {
					return e
				}
				// Source regular push precedes INC. This publication is a
				// borrowed pointer, so the cheap post-INC hook cannot stamp
				// monomial ownership here. Authority is stamped below only
				// after the actual successful INC.
				if e = b.ms.appendMonomial(m, prod); e != nil {
					return e
				}
				if b.appendScope.active {
					b.monomialOwners[n], b.monomialTransferred[n] = 0, 0
				}

				b.borrowedAppend = true
				b.borrowedAppendIndex = n
				if multiply && m.generatedActive {
					m.generatedBuffer = b
					m.generatedIndex = n
				}
			}
			if e = m.monomials.IncRef(prod); e != nil {
				return e
			}
			// Actual successful INC transfers this newly created product into
			// the source buffer. No RC lookup or later registry inspection.
			if receipt != nil && receipt.frame.generated == prod {
				receipt.frame.generated = Monomial{}
			}

			if !b.cheap {
				b.borrowedAppend = false
				if appendScope != nil {
					b.monomialOwners[n] = appendScope.invocation
					b.monomialTransferred[n] = 0
					appendScope.monomialClaims++
					appendScope.monomials = n + 1
				} else if b.appendScope.active {
					b.monomialOwners[n], b.monomialTransferred[n] = 0, 0
					b.appendScope.foreignMixed = true
				}
			}
			if b.cheap {
				b.pendingMonomial = prod
				b.hasPendingReference = true
				if appendScope != nil {
					appendScope.pending = prod
				}
				if multiply {
					m.transferGeneratedMonomial()
				}
				observer := appendScope
				if observer == nil && b.appendScope.active {
					observer = &b.appendScope
				}
				if e = b.ms.appendMonomialWithAppendScope(m, prod, observer, appendScope != nil); e != nil {
					return e
				}
				if appendScope != nil {
					appendScope.pending = Monomial{}
					appendScope.monomials = n + 1
				}
				b.hasPendingReference = false
				b.pendingMonomial = Monomial{}
			}
			if !b.cheap {
				if multiply {
					m.transferGeneratedMonomial()
				}
			}
			observer := appendScope
			if observer == nil && b.appendScope.active {
				observer = &b.appendScope
			}
			if e = b.as.appendCoefficientWithAppendScope(m, unumeral.Int{}, observer, appendScope != nil); e != nil {
				return e
			}
			out, e := b.as.coefficient(m, n)
			if e != nil {
				return e
			}
			source, e := p.coefficient(m, i)
			if e != nil {
				return e
			}
			if multiply {
				e = b.arithmeticNumericCall(2, a, &source, nil, &out, receipt)
			} else {
				e = b.arithmeticNumericCall(0, &source, nil, nil, &out, receipt)
			}
			if w := b.as.putCoefficient(m, n, out); w != nil {
				return w
			}
			if e != nil {
				return e
			}
		} else {
			priorTransfer := b.accumulatorRowTransfer(pos)
			if multiply {
				m.transferGeneratedMonomial()
			}
			out, e := b.as.coefficient(m, pos)
			if e != nil {
				return e
			}
			source, e := p.coefficient(m, i)
			if e != nil {
				return e
			}
			priorTransfer.coefficientValue, priorTransfer.monomialValue = out, prod
			if multiply {
				e = b.arithmeticNumericCall(3, &out, a, &source, &out, receipt)
			} else {
				e = b.arithmeticNumericCall(1, &out, &source, nil, &out, receipt)
			}
			if w := b.as.putCoefficient(m, pos, out); w != nil {
				return w
			}
			if e == nil && receipt == nil {
				b.transferAccumulatorRowToMember(pos, priorTransfer)
			}
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func (b *polynomialSOM) reset() error { return b.resetWithNormalizationReceipt(nil) }
func (b *polynomialSOM) resetWithNormalizationReceipt(receipt *polynomialNormalizeInvocationReceipt) error {
	m := b.owner
	if !m.actualSOMNormalizeBuffer(b) {
		return ErrPolynomialOwner
	}
	if b.normalizeScope.active || b.normalizeScope.finishing {
		return ErrPolynomialPending
	}
	owner := &m.normalizationOwner
	if owner.manager != nil && owner.manager.hasNormalizeInvocationHistory(owner) && owner.manager.core == m && owner.frame.active {
		if numericIntegerManager_normalizationAllocationRoots(m.integer, receipt) == nil || receipt.manager != owner.manager || receipt.frame != owner.frame || receipt.invocation != owner.invocation || b != &m.cheap {
			return ErrPolynomialPending
		}
	}
	if b.resetActive {
		return ErrPolynomialPending
	}
	resetReceipt := polynomialArithmeticInvocationReceipt{manager: m, frame: &b.arithmeticScope, invocation: b.arithmeticScope.invocation}
	n, e := b.ms.size(m)
	if e != nil {
		return e
	}
	if n == 0 {
		b.factorsParent = nil
		b.factorsGeneration = 0
		b.acknowledgeArithmeticSourceReset(&resetReceipt)
		return nil
	}
	sz, e := b.as.size(m)
	if e != nil {
		return e
	}
	if e = polynomialSOMSourceAssert(1936, "m_tmp_ms.size() == m_tmp_as.size()", sz == n); e != nil {
		return e
	}
	b.resetActive = true
	for i := uint32(0); i < n; i++ {
		b.resetIndex = i
		b.resetStage = 0
		term, e := b.ms.monomial(m, i)
		if e != nil {
			return e
		}
		if !b.cheap {
			if e = b.positions.Reset(term); e != nil {
				return e
			}
		}
		b.resetStage = 1
		if i >= sz {
			return arena.ErrBounds
		}
		a, e := b.as.coefficient(m, i)
		if e != nil {
			return e
		}
		if b.cheap {
			e = m.deleteNumeral(&a)
		} else {
			e = m.modular.Reset(&a)
		}
		if w := b.as.putCoefficient(m, i, a); w != nil {
			return w
		}
		if e != nil {
			return e
		}
		if b.appendScope.active && uint64(i) < uint64(len(b.coefficientOwners)) && b.coefficientOwners[i] == b.appendScope.invocation {
			b.coefficientOwners[i] = 0
			b.appendScope.coefficientClaims--
		}
		b.resetStage = 3
		outcome := monomialDecrementOutcome{}
		if e = m.monomials.check(); e == nil {
			e = m.monomials.decRefCoreOutcome(term, &outcome)
		}
		if outcome.written && b.appendScope.active && uint64(i) < uint64(len(b.monomialOwners)) && b.monomialOwners[i] == b.appendScope.invocation {
			b.monomialOwners[i] = 0
			b.appendScope.monomialClaims--
		}
		if e != nil {
			return e
		}
		b.resetIndex = i + 1
		b.resetStage = 0
	}
	if e = b.as.reset(m); e != nil {
		return e
	}
	if e = b.ms.reset(m); e != nil {
		return e
	}
	b.resetActive = false
	b.resetIndex = 0
	b.resetStage = 0
	b.factorsParent = nil
	b.factorsGeneration = 0
	b.acknowledgeArithmeticSourceReset(&resetReceipt)
	return nil
}
func (b *polynomialSOM) removeZeros(normalize bool) error {
	m := b.owner
	n, e := b.ms.size(m)
	if e != nil {
		return e
	}
	sz, e := b.as.size(m)
	if e != nil {
		return e
	}
	if e = polynomialSOMSourceAssert(1898, "m_tmp_ms.size() == m_tmp_as.size()", sz == n); e != nil {
		return e
	}
	if b.removeActive {
		return ErrPolynomialPending
	}
	b.removeActive = true
	b.removeStopped = false
	b.removePrefix = 0
	b.removeSuffix = 0
	j := uint32(0)
	for i := uint32(0); i < n; i++ {
		b.removeSuffix = i
		b.removeStopped = false
		term, e := b.ms.monomial(m, i)
		if e != nil {
			return e
		}
		if e = b.positions.Reset(term); e != nil {
			return e
		}
		a, e := b.as.coefficient(m, i)
		if e != nil {
			return e
		}
		zero, e := m.modular.IsZero(&a)
		if e != nil {
			return e
		}
		if zero {
			e = m.modular.Reset(&a)
			if w := b.as.putCoefficient(m, i, a); w != nil {
				return w
			}
			if e != nil {
				return e
			}
			if b.appendScope.active && uint64(i) < uint64(len(b.coefficientOwners)) && b.coefficientOwners[i] == b.appendScope.invocation {
				b.coefficientOwners[i] = 0
				b.appendScope.coefficientClaims--
			}
			b.removeStopped = true
			outcome := monomialDecrementOutcome{}
			if e = m.monomials.check(); e == nil {
				e = m.monomials.decRefCoreOutcome(term, &outcome)
			}
			if outcome.written && b.appendScope.active && uint64(i) < uint64(len(b.monomialOwners)) && b.monomialOwners[i] == b.appendScope.invocation {
				b.monomialOwners[i] = 0
				b.appendScope.monomialClaims--
			}
			if e != nil {
				return e
			}
			b.removeStopped = false
			b.removeSuffix = i + 1
		} else {
			if i != j {
				if e := b.assertCompactionMonomial(j, term); e != nil {
					return e
				}
				if e = b.ms.putMonomial(m, j, term); e != nil {
					return e
				}
				if b.appendScope.active && uint64(i) < uint64(len(b.monomialOwners)) && uint64(j) < uint64(len(b.monomialOwners)) {
					b.monomialOwners[j], b.monomialTransferred[j] = b.monomialOwners[i], b.monomialTransferred[i]
					b.monomialOwners[i], b.monomialTransferred[i] = 0, 0
				}
				// The source raw pointer copy does not increment refs. Ownership
				// follows that one real destination; the old source slot is a
				// borrowed alias until the source shrink removes it.
				b.removePrefix = j + 1
				b.removeSuffix = i + 1
				old, e := b.as.coefficient(m, j)
				if e != nil {
					return e
				}
				e = m.modular.Swap(&old, &a)
				if w := b.as.putCoefficient(m, j, old); w != nil {
					return w
				}
				if w := b.as.putCoefficient(m, i, a); w != nil {
					return w
				}
				if e != nil {
					return e
				}
				if b.appendScope.active && uint64(i) < uint64(len(b.coefficientOwners)) && uint64(j) < uint64(len(b.coefficientOwners)) {
					b.coefficientOwners[j], b.coefficientOwners[i] = b.coefficientOwners[i], b.coefficientOwners[j]
					b.coefficientTransferred[j], b.coefficientTransferred[i] = b.coefficientTransferred[i], b.coefficientTransferred[j]
				}
			}
			j++
			b.removePrefix = j
			b.removeSuffix = i + 1
		}
	}
	if polynomialSourceDebug {
		for i := j; i < n; i++ {
			if e := b.assertDiscardedCoefficientZero(i); e != nil {
				return e
			}
		}
	}
	if n != 0 {
		if e = b.as.publishSize(m, j); e != nil {
			return e
		}
		if e = b.ms.publishSize(m, j); e != nil {
			return e
		}
	}
	if b.appendScope.active {
		b.appendScope.coefficients, b.appendScope.monomials = j, j
	}
	b.removeActive = false
	b.removePrefix = 0
	b.removeSuffix = 0
	b.removeStopped = false
	if normalize {
		return b.normalize()
	}
	return nil
}
func (b *polynomialSOM) normalize() error { return b.normalizeWithOwner(nil) }
func (b *polynomialSOM) normalizeWithOwner(parent *polynomialNormalizeScope) (result error) {
	m := b.owner
	n, e := b.as.size(m)
	if e != nil {
		return e
	}
	if n == 0 {
		return nil
	}
	// A fresh completed default scoped_numeral is constructed only after the
	// source empty early return. An interrupted previous local is not repaired.
	if e := b.beginNormalizeNumeral(parent); e != nil {
		return e
	}
	b.normalizeScope.executing = true
	defer func() {
		b.gcdBodyCause = result
		b.normalizeScope.executing = false
		if cleanup := b.finishNormalizeNumeral(false); cleanup != nil {
			result = errors.Join(result, cleanup)
		}
	}()
	e = numericIntegerModularManager_gcdManyIndexedWithSOMNormalizeReceipt(m.modular, unumeral.IntegerArrayView{Values: b.as.values, Count: n}, &b.gcd, &b.normalizeScope.receipt)
	if e != nil {
		return e
	}
	one, e := m.modular.IsOne(&b.gcd)
	if e != nil {
		return e
	}
	if !one {
		if e := polynomialSOMPredicateSourceAssert(1871, "m.is_pos(g)", func() (bool, error) {
			return m.modular.IsPos(&b.gcd)
		}); e != nil {
			return e
		}
		for i := uint32(0); i < n; i++ {
			a, e := b.as.coefficient(m, i)
			if e != nil {
				return e
			}
			e = numericIntegerModularManager_divWithSOMNormalizeReceipt(m.modular, &a, &b.gcd, &a, &b.normalizeScope.receipt)
			if w := b.as.putCoefficient(m, i, a); w != nil {
				return w
			}
			if e != nil {
				return e
			}
		}
	}
	return nil
}
func (b *polynomialSOM) mk(normalize bool) (Polynomial, error) {
	return b.mkWithFactoryReceipt(normalize, nil)
}
func (b *polynomialSOM) mkWithFactoryReceipt(normalize bool, receipt *polynomialFactoryInvocationReceipt) (Polynomial, error) {
	return b.mkWithAppendReceipt(normalize, nil, receipt)
}
func (b *polynomialSOM) mkWithAppendReceipt(normalize bool, appendReceipt *polynomialCheapAppendInvocationReceipt, receipt *polynomialFactoryInvocationReceipt) (Polynomial, error) {
	if !b.cheap {
		if e := b.removeZeros(normalize); e != nil {
			return Polynomial{}, e
		}
	}
	n, e := b.ms.size(b.owner)
	if e != nil {
		return Polynomial{}, e
	}
	p, e := b.owner.mkCellWithReceipts(n, nil, nil, b, nil, receipt, appendReceipt)
	if e != nil {
		return Polynomial{}, e
	}
	if e = b.as.reset(b.owner); e != nil {
		return Polynomial{}, e
	}
	if e = b.ms.reset(b.owner); e != nil {
		return Polynomial{}, e
	}
	if b.matchesAppendInvocation(appendReceipt) {
		b.appendScope.completedTransfer = true
	}
	b.transferred = 0
	return p, nil
}
func (b *polynomialSOM) close() error {
	if b.owner == nil {
		return nil
	}
	m := b.owner
	// No normalize_numerals constructor exists on an untouched standalone
	// source buffer. Invoke its exact child continuation only if constructed.
	if b.gcdActive {
		if e := b.finishNormalizeNumeral(true); e != nil {
			return e
		}
	}
	// A previous explicit owning phase can have completed earlier row actions.
	// Finish that exact actual frame before the whole class visits its remaining
	// member fields, so completed references are not decremented a second time.
	s := &b.appendScope
	if s.active && (s.retirePhase != 0 || s.coefficientCursor != 0 || s.monomialCursor != 0) {
		r := polynomialCheapAppendInvocationReceipt{manager: m, frame: s, invocation: s.invocation}
		if b.matchesAppendInvocation(&r) {
			if e := b.cleanupAppendInvocation(&r, true); e != nil {
				return e
			}
		}
	}
	if b.hasPendingReference {
		if e := m.monomials.DecRef(b.pendingMonomial); e != nil {
			return e
		}
		b.hasPendingReference = false
		b.pendingMonomial = Monomial{}
	}
	n, e := b.ms.size(m)
	if e != nil {
		return e
	}
	as, e := b.as.size(m)
	if e != nil {
		return e
	}
	// An interrupted source reset has already consumed the preceding actual
	// monomial fields. Owning cleanup resumes known fields, never replays reset.
	if b.resetActive && b.cleanupIndex < b.resetIndex {
		b.cleanupIndex = b.resetIndex
	}
	for b.cleanupIndex < n || b.cleanupIndex < as {
		i := b.cleanupIndex
		if i < as {
			a, e := b.as.coefficient(m, i)
			if e != nil {
				return e
			}
			e = m.deleteNumeral(&a)
			if w := b.as.putCoefficient(m, i, a); w != nil {
				return w
			}
			if e != nil {
				return e
			}
		}
		if i < n && !(b.borrowedAppend && i == b.borrowedAppendIndex) && !(b.unreferencedConsumed && i == b.unreferencedIndex) && i >= b.transferred && (!b.removeActive || i < b.removePrefix || i >= b.removeSuffix) {
			x, e := b.ms.monomial(m, i)
			if e != nil {
				return e
			}
			if !b.cheap && !b.cleanupDropConsumed && !(b.resetActive && i == b.resetIndex && b.resetStage >= 1) && !(b.removeActive && b.removeStopped && i == b.removeSuffix) {
				if e = b.positions.Reset(x); e != nil {
					return e
				}
			}
			if b.cleanupDropConsumed {
				// The actual preceding DEC wrote zero before its deletion refused.
				// Resume only that caller-reachable slot; never DEC it again.
				if b.cleanupDropLast {
					e = m.monomials.CleanupValue(&x)
					if w := b.ms.putMonomial(m, i, x); w != nil {
						return w
					}
					if e != nil {
						return e
					}
				}
			} else if (b.resetActive && i == b.resetIndex && b.resetStage == 3) || (b.removeActive && b.removeStopped && i == b.removeSuffix) {
				l, e := m.monomials.location(x)
				if e != nil {
					return e
				}
				if l.stage != 0 {
					if e = m.monomials.CleanupValue(&x); e != nil {
						return e
					}
				} else if e = m.monomials.DecRef(x); e != nil {
					return e
				}
			} else {
				outcome := monomialDecrementOutcome{}
				if e = m.monomials.check(); e == nil {
					e = m.monomials.decRefCoreOutcome(x, &outcome)
				}
				if outcome.written {
					b.cleanupDropConsumed = true
					b.cleanupDropLast = outcome.last
				}
				if e != nil {
					return e
				}
			}
		}
		b.cleanupDropConsumed = false
		b.cleanupDropLast = false
		b.cleanupIndex++
	}
	if e = b.ms.close(m); e != nil {
		return e
	}
	if e = b.as.close(m); e != nil {
		return e
	}
	b.coefficientOwners = nil
	b.monomialOwners = nil
	b.coefficientTransferred, b.monomialTransferred = nil, nil
	if !b.cheap && !b.positions.closed {
		if e = b.positions.CloseChecked(); e != nil {
			return e
		}
	}
	return nil
}

func (b *polynomialSOM) addNumericLeaf(receipt *polynomialNumericRoots, op uint8, a, c, d, out *unumeral.Int) error {
	if receipt == nil {
		return b.owner.compositionNumericCall(op, a, c, d, out)
	}
	if receipt.caller.squareRoot != nil {
		context := receipt.caller
		if context.roots(b.owner.integer) != receipt {
			return ErrPolynomialOwner
		}
		m := b.owner.modular.WithAllocationContext(context.utility(b.owner.integer))
		switch op {
		case 0:
			return m.Copy(out, a)
		case 1:
			return m.Add(a, c, out)
		}
		return ErrPolynomialAssertion
	}
	before, e := polynomialNumericBefore(b.owner.integer)
	if e != nil {
		return e
	}
	e = b.owner.compositionNumericCall(op, a, c, d, out)
	return errors.Join(e, receipt.after(b.owner.integer, before))
}

func (b *polynomialSOM) addWithArithmeticReceipt(a *unumeral.Int, term Monomial, receipt *polynomialArithmeticInvocationReceipt) error {
	return b.addConsumedScalarDivEngine(a, term, false, nil, nil, nil, nil, nil, receipt)
}
