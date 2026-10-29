package polynomial

// Source: src/util/ref_buffer.h:38-99; src/util/buffer.h:29-46,146-151;
// src/math/polynomial/polynomial.cpp:5704.

import "github.com/glycerine/g3/pkg/arena"

// Native storage owns the actual references acquired by each completed push.
// The cursor and drainDec retain continuation of a stopped real destructor.
type polynomialPSCRefBuffer struct {
	nextFrame                                   *polynomialPSCNextScope
	nextInvocation, invocation                  uint64
	self                                        *polynomialPSCRefBuffer
	owner                                       *MultivariatePolynomialCore
	values                                      []Polynomial
	cursor                                      uint32
	drainDec, constructed, finalized, finishing bool
	cleanupCause                                error
}

func (b *polynomialPSCRefBuffer) construct(m *MultivariatePolynomialCore) error {
	return b.constructWithNextReceipt(m, nil)
}
func (b *polynomialPSCRefBuffer) constructWithNextReceipt(m *MultivariatePolynomialCore, r *polynomialPSCNextInvocationReceipt) error {
	if e := m.check(); e != nil {
		return e
	}
	if r != nil && (b != &m.pscBuffer || !m.matchesPSCNextInvocation(r) || !r.frame.executing) {
		return ErrPolynomialOwner
	}
	if b.invocation == ^uint64(0) || b.constructed && !b.finalized {
		return ErrPolynomialPending
	}
	*b = polynomialPSCRefBuffer{owner: m, constructed: true, invocation: b.invocation + 1}
	b.self = b
	if r != nil {
		b.nextFrame = r.frame
		b.nextInvocation = r.invocation
	}
	return nil
}
func (b *polynomialPSCRefBuffer) check(m *MultivariatePolynomialCore) error {
	if b == nil || b.self != b || b.owner != m || !b.constructed {
		return ErrPolynomialOwner
	}
	if b.finalized {
		return arena.ErrClosed
	}
	return nil
}
func (b *polynomialPSCRefBuffer) physical(m *MultivariatePolynomialCore, i uint32) (Polynomial, error) {
	if uint64(i) >= uint64(len(b.values)) {
		return Polynomial{}, arena.ErrBounds
	}
	return b.values[i], nil
}
func (b *polynomialPSCRefBuffer) get(m *MultivariatePolynomialCore, i uint32) (Polynomial, error) {
	if e := b.check(m); e != nil {
		return Polynomial{}, e
	}
	if polynomialSourceDebug && uint64(i) >= uint64(len(b.values)) {
		return Polynomial{}, ErrPolynomialAssertion
	}
	return b.physical(m, i)
}
func (b *polynomialPSCRefBuffer) push(m *MultivariatePolynomialCore, p Polynomial) error {
	return b.pushWithNextReceipt(m, p, nil)
}
func (b *polynomialPSCRefBuffer) pushWithNextReceipt(m *MultivariatePolynomialCore, p Polynomial, r *polynomialPSCNextInvocationReceipt) error {
	if e := b.check(m); e != nil {
		return e
	}
	if b.nextFrame != nil {
		if b != &m.pscBuffer || r == nil || !m.matchesPSCNextInvocation(r) || r.frame != b.nextFrame || r.invocation != b.nextInvocation || !r.frame.executing {
			return ErrPolynomialPending
		}
	}

	if p != (Polynomial{}) {
		if e := m.incRefCore(p); e != nil {
			return e
		}
	}
	// No fallible container operation separates INC from publication.
	b.values = append(b.values, p)
	return nil
}
func (b *polynomialPSCRefBuffer) finalize(m *MultivariatePolynomialCore, owning bool) error {
	return b.finalizePhase(m, owning)
}
func (b *polynomialPSCRefBuffer) finalizePhase(m *MultivariatePolynomialCore, owning bool) error {
	if b == nil || b.self != b || b.owner != m {
		return ErrPolynomialOwner
	}
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if b.finishing || m.callbackDepth != 0 {
		return ErrPolynomialPending
	}
	b.finishing = true
	defer func() { b.finishing = false }()
	if b.nextFrame != nil {
		if m == nil || m.self != m || b != &m.pscBuffer || b.nextFrame != &m.pscNext || b.nextInvocation != m.pscNext.invocation {
			return ErrPolynomialOwner
		}
		if b.nextFrame.executing {
			return ErrPolynomialPending
		}
	}
	if !b.finalized {
		if b.cleanupCause != nil && !owning {
			return b.cleanupCause
		}
		for uint64(b.cursor) < uint64(len(b.values)) {
			if b.drainDec && !owning {
				return b.cleanupCause
			}
			if e := polynomialReferenceDrop(m, &b.values[b.cursor], &b.drainDec); e != nil {
				b.cleanupCause = e
				return e
			}
			b.drainDec = false
			b.cursor++
		}
		b.values = nil
		b.finalized = true
		if b.nextFrame != nil && b.nextFrame == &m.pscNext && b.nextInvocation == m.pscNext.invocation && b.invocation == m.pscNext.bufferInvocation {
			m.pscNext.bufferDestroyed = true
		}
		b.cleanupCause = nil
	}
	return nil
}
