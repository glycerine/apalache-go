package polynomial

// Source: src/math/polynomial/polynomial.cpp:5654-5810; src/util/obj_ref.h.

// Generation identifies a concrete invocation of these source local scopes.
// The subordinate continuation itself records acquisition AFTER its own
// foreign-pending guard. No allocator registry or refcount creates authority.
type polynomialPSCReceipts struct {
	generation, depth, factory uint64
	singleton                  Monomial
}

func (m *MultivariatePolynomialCore) pscExecutingOwner() uint64 {
	if m.pscReceipts.depth == 0 || m.callbackDepth != 0 {
		return 0
	}
	return m.pscReceipts.generation
}
func (m *MultivariatePolynomialCore) pscEnter() error {
	r := &m.pscReceipts
	if r.depth == 0 {
		if m.pending.pscOwner != 0 || m.powerLocal.pscOwner != 0 || m.pseudoDivision.pscOwner != 0 || m.exactDivision.pscOwner != 0 || m.generatedPSC != 0 || r.factory != 0 {
			return ErrPolynomialPending
		}
		if r.generation == ^uint64(0) {
			return ErrPolynomialPending
		}
		r.generation++
	}
	r.depth++
	return nil
}
func (m *MultivariatePolynomialCore) pscLeave() { m.pscReceipts.depth-- }

func (m *MultivariatePolynomialCore) pscSourceStopped() error {
	g := m.pscReceipts.generation
	if g == 0 {
		return nil
	}
	if m.contentLocal.active && m.contentLocal.generation == g && m.contentLocal.cleanupCause != nil {
		return m.contentLocal.cleanupCause
	}
	if m.powerLocal.pscOwner == g && m.powerLocal.cleanupCause != nil {
		return m.powerLocal.cleanupCause
	}
	if m.pseudoDivision.pscOwner == g && m.pseudoDivision.cleanupCause != nil {
		return m.pseudoDivision.cleanupCause
	}
	if m.exactDivision.pscOwner == g && m.exactDivision.cleanupCause != nil {
		return m.exactDivision.cleanupCause
	}
	return nil
}

// All completed younger source locals resume via their owning physical
// protocol first. Only afterward are unjoined INCs/failed constructors aborted.
func (m *MultivariatePolynomialCore) CleanupPSC() error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	r := &m.pscReceipts
	if r.depth != 0 {
		return ErrPolynomialPending
	}
	g := r.generation
	if e := m.cleanupPSCCompleted(g); e != nil {
		return e
	}
	return m.cleanupPSCAborts(g)
}

// An outer lexical owner may unwind its completed locals between these
// phases. Only the exact invocation may acknowledge failed constructors.
func (m *MultivariatePolynomialCore) cleanupPSCCompleted(g uint64) error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.pscReceipts.depth != 0 {
		return ErrPolynomialPending
	}
	if g != m.pscReceipts.generation {
		return nil
	}
	// Compatibility generation selects a known actual parent frame. Its concrete
	// child tuples, rather than the generation, grant every new callee authority.
	if g != 0 && m.pscLocal.sourceGeneration == g {
		return m.finishPSCPhase(true, false)
	}
	if g != 0 && m.pscNext.sourceGeneration == g {
		return m.finishPSCNextPhase(true, false)
	}
	if g != 0 && m.lazardLocal.sourceGeneration == g {
		return m.finishLazardPhase(true, false)
	}

	if m.powerLocal.pscOwner == g && g != 0 {
		if e := m.cleanupPolynomialPowerPhase(true, false); e != nil {
			return e
		}
	}
	if m.pseudoDivision.pscOwner == g && g != 0 {
		if e := m.pseudoCleanupPhase(true, false); e != nil {
			return e
		}
	}
	if m.exactDivision.pscOwner == g && g != 0 {
		if e := m.exactDivisionCleanupPhase(true, false); e != nil {
			return e
		}
	}
	if m.lazardLocal.sourceGeneration == g {
		if e := m.finishLazardPhase(true, false); e != nil {
			return e
		}
	}
	if m.pscNext.sourceGeneration == g {
		if e := m.finishPSCNextPhase(true, false); e != nil {
			return e
		}
	}
	if m.pscLocal.sourceGeneration == g {
		if e := m.finishPSCPhase(true, false); e != nil {
			return e
		}
	}
	return nil
}

func (m *MultivariatePolynomialCore) cleanupPSCAborts(g uint64) error {
	if m == nil || m.self != m {
		return ErrPolynomialOwner
	}
	if m.pscReceipts.depth != 0 {
		return ErrPolynomialPending
	}
	if g != m.pscReceipts.generation {
		return nil
	}
	// Compatibility generation selects a known actual parent frame. Its concrete
	// child tuples, rather than the generation, grant every new callee authority.
	if g != 0 && m.pscLocal.sourceGeneration == g {
		if e := m.finishPSCPhase(true, true); e != nil {
			return e
		}
	} else if g != 0 && m.pscNext.sourceGeneration == g {
		if e := m.finishPSCNextPhase(true, true); e != nil {
			return e
		}
	} else if g != 0 && m.lazardLocal.sourceGeneration == g {
		if e := m.finishLazardPhase(true, true); e != nil {
			return e
		}
	}

	r := &m.pscReceipts
	if m.lazardLocal.sourceGeneration == g {
		if e := m.finishLazardPhase(true, true); e != nil {
			return e
		}
	}
	if m.pscNext.sourceGeneration == g {
		if e := m.finishPSCNextPhase(true, true); e != nil {
			return e
		}
	}
	if m.pscLocal.sourceGeneration == g {
		if e := m.finishPSCPhase(true, true); e != nil {
			return e
		}
	}
	if m.powerLocal.pscOwner == g && g != 0 {
		if e := m.CleanupPolynomialPower(); e != nil {
			return e
		}
	}
	if m.pseudoDivision.pscOwner == g && g != 0 {
		if e := m.CleanupPseudoDivision(); e != nil {
			return e
		}
	}
	if m.exactDivision.pscOwner == g && g != 0 {
		if e := m.CleanupExactDivision(); e != nil {
			return e
		}
	}
	if m.pending.pscOwner == g && g != 0 {
		if e := m.cleanupPendingCreation(); e != nil {
			return e
		}
	}
	if r.singleton != (Monomial{}) {
		if m.singletonReference && m.singletonMonomial == r.singleton {
			if e := m.monomials.DecRef(r.singleton); e != nil {
				return e
			}
			m.singletonReference = false
			m.singletonMonomial = Monomial{}
		}
		r.singleton = Monomial{}
	}
	if m.generatedPSC == g && g != 0 {
		if e := m.cleanupGeneratedMonomial(); e != nil {
			return e
		}
	}
	if r.factory != 0 {
		c := &m.monomials.creation
		if c.generation == r.factory && c.parent == m {
			if e := m.monomials.CleanupCreation(); e != nil {
				return e
			}
		}
		r.factory = 0
	}
	return nil
}

// Pending describes actual retained source scopes and their acquired continuations.
func (m *MultivariatePolynomialCore) pscPending() bool {
	g := m.pscReceipts.generation
	factory := m.pscReceipts.factory != 0 && m.monomials.creation.generation == m.pscReceipts.factory && m.monomials.creation.parent == m
	return m.pscLocal.active || m.pscNext.active || m.lazardLocal.active ||
		(m.pscBuffer.constructed && !m.pscBuffer.finalized) ||
		(g != 0 && (m.pending.pscOwner == g || m.powerLocal.pscOwner == g || m.pseudoDivision.pscOwner == g || m.exactDivision.pscOwner == g || m.generatedPSC == g)) || factory || (m.pscReceipts.singleton != (Monomial{}) && m.singletonReference && m.singletonMonomial == m.pscReceipts.singleton)
}

// A compatibility caller may observe only the source cohort it entered. This
// read-only test conveys no authority over a different retained source frame.
func (m *MultivariatePolynomialCore) pscPendingForGeneration(g uint64) bool {
	if g == 0 || g != m.pscReceipts.generation {
		return false
	}
	r := &m.pscReceipts
	factory := r.factory != 0 && m.monomials.creation.generation == r.factory && m.monomials.creation.parent == m
	h := &m.pscBuffer
	buffer := h.constructed && !h.finalized && h.nextFrame == &m.pscNext && h.nextInvocation == m.pscNext.invocation && m.pscNext.sourceGeneration == g
	return m.pscLocal.active && m.pscLocal.sourceGeneration == g || m.pscNext.active && m.pscNext.sourceGeneration == g || m.lazardLocal.active && m.lazardLocal.sourceGeneration == g || buffer || m.pending.pscOwner == g || m.powerLocal.pscOwner == g || m.pseudoDivision.pscOwner == g || m.exactDivision.pscOwner == g || m.generatedPSC == g || factory || (r.singleton != (Monomial{}) && m.singletonReference && m.singletonMonomial == r.singleton)
}
