package polynomial

// Source: src/math/polynomial/polynomial.cpp:5154-5194.

import (
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Divides returns a boolean after its NUM/ref suffix. Its R rows remain in
// the actual persistent SOM member. This handoff transfers only the tags of
// that exact invocation; it performs no source RESET, SIZE write, DEL or DEC.
func (b *polynomialSOM) transferArithmeticRowsToMember(r *polynomialArithmeticInvocationReceipt) error {
	if b == nil || b.owner == nil || !b.owner.matchesArithmeticInvocation(r) || r.frame.buffer != b {
		return nil
	}
	m, s := b.owner, r.frame
	if s.executing {
		return ErrPolynomialPending
	}
	if e := m.cleanupArithmeticInvocation(r, false); e != nil {
		return e
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.transportBound || s.singleton != (Monomial{}) || s.generated != (Monomial{}) || s.calleeGeneration != 0 || (m.monomials.matchesMonomialAllocatorInvocation(&s.monomialAllocator) && s.monomialAllocator.outcome.Pending != (umemory.Block{})) || s.numeric.PendingObject().OwnerID() != 0 || s.numeric.PendingAllocatorBlocks() != ([3]umemory.Block{}) || m.matchesFactoryInvocation(&s.factory) || m.modular.MatchesPNormalizeInvocation(&s.normalize) || m.integer.MatchesAddMulInvocation(&s.addMul) {
		return ErrPolynomialPending
	}
	a := &b.appendScope
	if a.active {
		if !b.matchesAppendInvocation(&s.append) || a.pending != (Monomial{}) || a.movingCoefficient != (unumeral.Int{}) || a.numeric.PendingObject().OwnerID() != 0 || a.numeric.PendingAllocatorBlocks() != ([3]umemory.Block{}) || a.retirePhase != 0 || a.stopped {
			return ErrPolynomialPending
		}
		// The per-slot acquisition tags are the authority. Claim counters are
		// checked only before changing metadata, to refuse a malformed adapter.
		var coefficients, monomials uint32
		for _, tag := range b.coefficientOwners {
			if tag == a.invocation {
				coefficients++
			}
		}
		for _, tag := range b.monomialOwners {
			if tag == a.invocation {
				monomials++
			}
		}
		if coefficients != a.coefficientClaims || monomials != a.monomialClaims {
			return ErrPolynomialPending
		}
		for i, tag := range b.coefficientOwners {
			if tag == a.invocation {
				b.coefficientOwners[i] = 0
			}
		}
		for i, tag := range b.monomialOwners {
			if tag == a.invocation {
				b.monomialOwners[i] = 0
			}
		}
		for i, tag := range b.coefficientTransferred {
			if tag == a.invocation {
				b.coefficientTransferred[i] = 0
			}
		}
		for i, tag := range b.monomialTransferred {
			if tag == a.invocation {
				b.monomialTransferred[i] = 0
			}
		}
		a.coefficientClaims, a.monomialClaims = 0, 0
		a.active = false
	}
	s.active = false
	return nil
}
