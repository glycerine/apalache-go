package polynomial

// Source: src/math/polynomial/polynomial.cpp:2217-2255,2280-2299.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type polynomialCheapAppendScope struct {
	buffer                                        *polynomialSOM
	coefficientClaims, monomialClaims             uint32
	compactionStage                               uint8
	retirePhase                                   uint8
	retireAs, retireMs                            uint32
	compactIndex, compactTarget                   uint32
	dropConsumed, dropLast                        bool
	pendingDropConsumed, pendingDropLast          bool
	invocation                                    uint64
	active, stopped, foreignMixed                 bool
	coefficients, monomials                       uint32
	pending                                       Monomial
	coefficientCursor, monomialCursor             uint32
	positionCursor                                uint32
	numeric                                       polynomialNumericRoots
	factoryInvocation                             uint64
	transferredCoefficients, transferredMonomials uint32
	completedTransfer                             bool
	movingCoefficient                             unumeral.Int
}
type polynomialCheapAppendInvocationReceipt struct {
	manager    *MultivariatePolynomialCore
	frame      *polynomialCheapAppendScope
	invocation uint64
}

func (b *polynomialSOM) matchesAppendInvocation(r *polynomialCheapAppendInvocationReceipt) bool {
	return b != nil && b.owner != nil && b.owner.self == b.owner && b.owner.ownsArithmeticBuffer(b) && b.appendScope.buffer == b && r != nil && r.manager == b.owner && r.frame == &b.appendScope && r.invocation != 0 && r.invocation == b.appendScope.invocation && b.appendScope.active
}
func (b *polynomialSOM) acquireAppendInvocation(r *polynomialCheapAppendInvocationReceipt, n uint32) error {
	if b == nil || b.owner == nil || b.owner.self != b.owner || !b.owner.ownsArithmeticBuffer(b) {
		return ErrPolynomialOwner
	}
	s := &b.appendScope
	as, e := b.as.size(b.owner)
	if e != nil {
		return e
	}
	if s.active {
		if !b.matchesAppendInvocation(r) || s.stopped || as != n || b.hasPendingReference || s.retirePhase != 0 {
			return ErrPolynomialPending
		}
		return nil
	}
	if as != n || b.hasPendingReference || s.invocation == ^uint64(0) {
		return ErrPolynomialPending
	}
	// Existing source rows precede this invocation and remain borrowed. Only
	// actual subsequent slot-construction hooks stamp the new invocation.
	*s = polynomialCheapAppendScope{buffer: b, active: true, invocation: s.invocation + 1, foreignMixed: n != 0, coefficients: n, monomials: n}
	*r = polynomialCheapAppendInvocationReceipt{manager: b.owner, frame: s, invocation: s.invocation}
	return nil
}
func (b *polynomialSOM) addWithAppendReceipt(a *unumeral.Int, term Monomial, take bool, r *polynomialCheapAppendInvocationReceipt) error {
	return b.addConsumedReceipts(a, term, take, nil, nil, r)
}

// Explicit owning retirement consumes only this invocation's acquired rows.
// Mixed rows and partial factory transfers use actual per-slot acquisition and
// transfer tags; unknown provenance refuses before any payload is consumed.
func (b *polynomialSOM) cleanupAppendInvocation(r *polynomialCheapAppendInvocationReceipt, abort bool) error {
	if !abort || !b.matchesAppendInvocation(r) {
		return nil
	}
	s := &b.appendScope
	m := b.owner
	as, e := b.as.size(m)
	if e != nil {
		return e
	}
	n, e := b.ms.size(m)
	if e != nil {
		return e
	}
	if b.resetActive || b.removeActive {
		return ErrPolynomialPending
	}
	if s.foreignMixed || (as == 0 && n == 0 && s.coefficientClaims == 0 && s.monomialClaims == 0) {
		return b.cleanupTaggedAppendInvocation(r)
	}
	if s.completedTransfer {
		if as != 0 || n != 0 || s.transferredCoefficients != s.coefficients || s.transferredMonomials != s.monomials {
			return ErrPolynomialPending
		}
	} else if as > s.coefficients || n != s.monomials || b.transferred != s.transferredMonomials {
		return ErrPolynomialPending
	}
	if b.hasPendingReference && (s.pending == (Monomial{}) || b.pendingMonomial != s.pending) {
		return ErrPolynomialPending
	}
	if e = m.deleteNumeral(&s.movingCoefficient); e != nil {
		return e
	}
	if s.coefficientCursor < s.transferredCoefficients {
		s.coefficientCursor = s.transferredCoefficients
	}
	if s.monomialCursor < s.transferredMonomials {
		s.monomialCursor = s.transferredMonomials
	}
	for s.coefficientCursor < s.coefficients {
		i := s.coefficientCursor
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
		s.coefficientCursor++
	}
	if s.pending != (Monomial{}) {
		if e = m.monomials.DecRef(s.pending); e != nil {
			return e
		}
		s.pending = Monomial{}
		b.pendingMonomial = Monomial{}
		b.hasPendingReference = false
	}
	for s.monomialCursor < s.monomials {
		term, e := b.ms.monomial(m, s.monomialCursor)
		if e != nil {
			return e
		}
		// A regular source buffer also owns this term's position publication.
		// Record the actual reset frontier before DEC, so a consumed-last
		// cleanup retry never rereads the already released term for reset.
		if !b.cheap && s.positionCursor <= s.monomialCursor {
			if e = b.positions.Reset(term); e != nil {
				return e
			}
			s.positionCursor = s.monomialCursor + 1
		}
		if e = m.monomials.DecRef(term); e != nil {
			return e
		}
		s.monomialCursor++
	}
	if e = b.as.reset(m); e != nil {
		return e
	}
	if e = b.ms.reset(m); e != nil {
		return e
	}
	if e = s.numeric.cleanup(m.integer); e != nil {
		return e
	}
	*s = polynomialCheapAppendScope{invocation: s.invocation}
	return nil
}

// At the real returned factory's incoming INC, the source vectors have already
// transferred every row and reset their logical sizes. Acknowledge only that
// matching completed adapter cohort before the older DEC can call user code.
// No source RESET, DEL, DEC, backing release or numeric sweep occurs here.
func (b *polynomialSOM) acknowledgeFactoryAppendTransfer(invocation uint64) error {
	s := &b.appendScope
	if !s.active || s.factoryInvocation != invocation {
		return nil
	}
	if !s.completedTransfer || s.foreignMixed || s.stopped || s.transferredCoefficients != s.coefficients || s.transferredMonomials != s.monomials || b.hasPendingReference || s.pending != (Monomial{}) || s.movingCoefficient.NativeCellToken() != 0 || s.numeric.PendingObject() != (umemory.ObjectRef{}) || s.numeric.PendingAllocatorBlocks() != ([3]umemory.Block{}) {
		return ErrPolynomialPending
	}
	as, e := b.as.size(b.owner)
	if e != nil {
		return e
	}
	ms, e := b.ms.size(b.owner)
	if e != nil {
		return e
	}
	if as != 0 || ms != 0 {
		return ErrPolynomialPending
	}
	*s = polynomialCheapAppendScope{invocation: s.invocation}
	return nil
}
