package polynomial

// Source: src/math/polynomial/polynomial.cpp:2217-2255,2280-2299.

import (
	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Native scalar tags identify actual slot acquisitions. Payload authority stays
// in the original numeral/monomial fields; tags do not reserve backing storage.
func (b *polynomialSOM) prepareAppendTags(need uint32) error {
	if uint64(need) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	for _, slot := range [](*[]uint64){&b.coefficientOwners, &b.monomialOwners, &b.coefficientTransferred, &b.monomialTransferred} {
		for len(*slot) < int(need) {
			*slot = append(*slot, 0)
		}
	}
	return nil
}
func (b *polynomialSOM) cleanupTaggedAppendInvocation(r *polynomialCheapAppendInvocationReceipt) error {
	s := &b.appendScope
	m := b.owner
	if b.hasPendingReference && (s.pending == (Monomial{}) || b.pendingMonomial != s.pending) {
		return ErrPolynomialPending
	}
	var as, ms uint32
	var e error
	if s.retirePhase == 0 {
		as, e = b.as.size(m)
		if e != nil {
			return e
		}
		ms, e = b.ms.size(m)
		if e != nil {
			return e
		}
		// An invocation with no owned-slot acquisition has no member payload to
		// retire. Exact leaf numeric roots remain separately owned.
		if s.coefficientClaims == 0 && s.monomialClaims == 0 && s.pending == (Monomial{}) && !b.hasPendingReference && s.compactionStage == 0 && s.compactIndex == 0 && s.retirePhase == 0 {
			if e = s.numeric.cleanup(m.integer); e != nil {
				return e
			}
			*s = polynomialCheapAppendScope{invocation: s.invocation}
			return nil
		}
		// Unsupported/unknown provenance refuses before any payload is consumed.
		if (as != ms && !(as == ms+1 && uint64(ms) < uint64(len(b.coefficientOwners)) && b.coefficientOwners[ms] == r.invocation)) || uint64(as) > uint64(len(b.coefficientOwners)) || uint64(ms) > uint64(len(b.monomialOwners)) || uint64(as) > uint64(len(b.coefficientTransferred)) || uint64(ms) > uint64(len(b.monomialTransferred)) || (b.hasPendingReference && (s.pending == (Monomial{}) || b.pendingMonomial != s.pending)) || (!b.hasPendingReference && s.pending != (Monomial{})) {
			return ErrPolynomialPending
		}
		var coefficientClaims, monomialClaims uint32
		for i := uint32(0); i < as; i++ {
			if b.coefficientOwners[i] == r.invocation {
				coefficientClaims++
			}
			if b.monomialOwners[i] == r.invocation {
				monomialClaims++
			}
			if i < ms && (b.coefficientOwners[i] == r.invocation) != (b.monomialOwners[i] == r.invocation) &&
				!((b.coefficientOwners[i] == r.invocation && b.monomialTransferred[i] == r.invocation) || (b.monomialOwners[i] == r.invocation && b.coefficientTransferred[i] == r.invocation)) {
				return ErrPolynomialPending
			}
		}
		if coefficientClaims != s.coefficientClaims || monomialClaims != s.monomialClaims {
			return ErrPolynomialPending
		}
		s.retireAs, s.retireMs = as, ms
		s.retirePhase = 1
	} else {
		as, ms = s.retireAs, s.retireMs
	}
	for s.retirePhase == 1 && s.coefficientCursor < as {
		i := s.coefficientCursor
		if b.coefficientOwners[i] == r.invocation {
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
		s.coefficientCursor++
	}
	if s.retirePhase == 1 {
		s.retirePhase = 2
	}
	if s.retirePhase == 2 && s.pending != (Monomial{}) {
		if s.pendingDropConsumed {
			if s.pendingDropLast {
				term := s.pending
				e = m.monomials.CleanupValue(&term)
				if e != nil {
					return e
				}
			}
		} else {
			outcome := monomialDecrementOutcome{}
			if e = m.monomials.check(); e == nil {
				e = m.monomials.decRefCoreOutcome(s.pending, &outcome)
			}
			if outcome.written {
				s.pendingDropConsumed, s.pendingDropLast = true, outcome.last
			}
			if e != nil {
				return e
			}
		}
		b.pendingMonomial, b.hasPendingReference = Monomial{}, false
		s.pending, s.pendingDropConsumed, s.pendingDropLast = Monomial{}, false, false
	}
	for s.retirePhase == 2 && s.monomialCursor < ms {
		i := s.monomialCursor
		if b.monomialOwners[i] == r.invocation {
			term, e := b.ms.monomial(m, i)
			if e != nil {
				return e
			}
			if !b.cheap && s.positionCursor <= i {
				if e = b.positions.Reset(term); e != nil {
					return e
				}
				s.positionCursor = i + 1
			}
			if s.dropConsumed {
				if s.dropLast {
					e = m.monomials.CleanupValue(&term)
					if w := b.ms.putMonomial(m, i, term); w != nil {
						return w
					}
					if e != nil {
						return e
					}
				}
			} else {
				outcome := monomialDecrementOutcome{}
				if e = m.monomials.check(); e == nil {
					e = m.monomials.decRefCoreOutcome(term, &outcome)
				}
				if outcome.written {
					s.dropConsumed = true
					s.dropLast = outcome.last
				}
				if e != nil {
					return e
				}
			}

			s.dropConsumed = false
			s.dropLast = false
		}
		s.monomialCursor++
	}

	if s.retirePhase == 2 {
		s.retirePhase = 3
	}
	// The exact acquisition tags remain until compaction; the consumed cursors
	// prevent repeated leaf actions after a stopped owning cleanup.
	for s.retirePhase == 3 && s.compactIndex < as {
		i, target := s.compactIndex, s.compactTarget
		if b.coefficientOwners[i] == r.invocation || b.monomialOwners[i] == r.invocation || b.coefficientTransferred[i] == r.invocation || b.monomialTransferred[i] == r.invocation {
			s.compactIndex++
			continue
		}
		if i != target {
			if s.compactionStage == 0 {
				a, e := b.as.coefficient(m, i)
				if e != nil {
					return e
				}
				if e = b.as.putCoefficient(m, i, unumeral.Int{}); e != nil {
					return e
				}
				b.cohortMovingCoefficient = a
				s.compactionStage = 1
			}
			if s.compactionStage == 1 {
				if e = b.as.putCoefficient(m, target, b.cohortMovingCoefficient); e != nil {
					return e
				}
				b.cohortMovingCoefficient = unumeral.Int{}
				s.compactionStage = 2
			}
			if s.compactionStage == 2 {
				term, e := b.ms.monomial(m, i)
				if e != nil {
					return e
				}
				if e = b.ms.putMonomial(m, i, Monomial{}); e != nil {
					return e
				}
				b.cohortMovingMonomial = term
				s.compactionStage = 3
			}
			if s.compactionStage == 3 {
				if e = b.ms.putMonomial(m, target, b.cohortMovingMonomial); e != nil {
					return e
				}
				b.cohortMovingMonomial = Monomial{}
				s.compactionStage = 4
			}
			if !b.cheap && s.compactionStage == 4 {
				term, e := b.ms.monomial(m, target)
				if e != nil {
					return e
				}
				if e = b.positions.Reset(term); e != nil {
					return e
				}
				if e = b.positions.Set(term, target); e != nil {
					return e
				}
			}
			b.coefficientOwners[target] = b.coefficientOwners[i]
			b.monomialOwners[target] = b.monomialOwners[i]
			b.coefficientTransferred[target], b.monomialTransferred[target] = b.coefficientTransferred[i], b.monomialTransferred[i]
			b.coefficientOwners[i] = 0
			b.monomialOwners[i] = 0
			b.coefficientTransferred[i], b.monomialTransferred[i] = 0, 0
		}
		s.compactionStage = 0
		s.compactIndex++
		s.compactTarget++
	}
	if s.retirePhase == 3 {
		s.retirePhase = 4
	}
	if s.retirePhase == 4 {
		if s.compactTarget == 0 {
			e = b.as.reset(m)
		} else {
			e = b.as.publishSize(m, s.compactTarget)
		}
		if e != nil {
			return e
		}
		s.retirePhase = 5
	}
	if s.retirePhase == 5 {
		if s.compactTarget == 0 {
			e = b.ms.reset(m)
		} else {
			e = b.ms.publishSize(m, s.compactTarget)
		}
		if e != nil {
			return e
		}
		s.retirePhase = 6
	}
	if e = s.numeric.cleanup(m.integer); e != nil {
		return e
	}
	*s = polynomialCheapAppendScope{invocation: s.invocation}
	return nil
}

// A later unreceipted source write uses the existing accumulator row as a
// persistent member. Capture its actual prior tags before the scalar action;
// acknowledge only that exact frame and those tags after successful publication.
type polynomialAccumulatorRowTransfer struct {
	buffer                            *polynomialSOM
	owner                             *MultivariatePolynomialCore
	frame                             *polynomialCheapAppendScope
	invocation, coefficient, monomial uint64
	coefficientValue                  unumeral.Int
	monomialValue                     Monomial
}

func (b *polynomialSOM) accumulatorRowTransfer(i uint32) polynomialAccumulatorRowTransfer {
	s := &b.appendScope
	if b.owner == nil || b.owner.self != b.owner || b.owner.callbackDepth != 0 || !b.owner.ownsArithmeticBuffer(b) || !s.active || uint64(i) >= uint64(len(b.coefficientOwners)) || uint64(i) >= uint64(len(b.monomialOwners)) {
		return polynomialAccumulatorRowTransfer{}
	}
	return polynomialAccumulatorRowTransfer{buffer: b, owner: b.owner, frame: s, invocation: s.invocation, coefficient: b.coefficientOwners[i], monomial: b.monomialOwners[i]}
}
func (b *polynomialSOM) transferAccumulatorRowToMember(i uint32, r polynomialAccumulatorRowTransfer) {
	s := &b.appendScope
	if r.buffer != b || r.owner != b.owner || b.owner == nil || b.owner.self != b.owner || b.owner.callbackDepth != 0 || r.frame != s || !s.active || s.invocation != r.invocation || r.invocation == 0 || uint64(i) >= uint64(len(b.coefficientOwners)) || uint64(i) >= uint64(len(b.monomialOwners)) || b.coefficientOwners[i] != r.coefficient || b.monomialOwners[i] != r.monomial {
		return
	}
	if r.coefficient == s.invocation {
		b.coefficientOwners[i] = 0
		s.coefficientClaims--
	}
	if r.monomial == s.invocation {
		b.monomialOwners[i] = 0
		s.monomialClaims--
	}
	s.foreignMixed = true
}
