package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/math/polynomial/upolynomial.cpp:1298-1321.
// Source: src/math/polynomial/upolynomial.cpp:1734-1764.
// Source: src/math/polynomial/upolynomial.cpp:138-157,187,1259-1265; src/util/mpzzp.h:97-105.

import (
	"errors"
	"github.com/glycerine/g3/pkg/arena"
)

// SourceFinalize runs only the source private-member destructor sequence.
// Caller-owned polynomial descriptors and all cold ownership metadata survive
// for explicit owning cleanup. No descriptor-directory scan acquires ownership.

func (m *UPolynomialManager) SourceFinalize() error { return m.sourceFinalizeWithHensel(nil) }
func (m *UPolynomialManager) sourceFinalizeWithHensel(r *upolyHenselPairInvocationReceipt) error {
	return m.sourceFinalizeWithHenselOwners(r, nil)
}
func (m *UPolynomialManager) sourceFinalizeWithHenselOwners(r *upolyHenselPairInvocationReceipt, c *upolyHenselCallerScope) error {
	return m.sourceFinalizeWithCallerOwners(r, c, nil)
}
func (m *UPolynomialManager) sourceFinalizeWithCallerOwners(r *upolyHenselPairInvocationReceipt, c *upolyHenselCallerScope, p *upolyFactorPrimePreparationInvocationReceipt) error {
	if m != nil && m.self == m && m.sourceFactors != nil && m.sourceFactors.NumericExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.squareFreeCall.active {
		if e := m.squareFreeCall.sourceStop; e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.factorField != nil {
		q := m.factorField
		if q.executing || q.sourceExecuting || q.ownExecuting {
			return ErrUPolynomialCleanupPending
		}
		if q.sourceCause != nil {
			return q.sourceCause
		}
		if e := q.traceBlocker(); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.intervalBounds.active {
		if e := m.intervalBounds.sourceCause; e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.convertEntry.active {
		if e := m.convertEntry.sourceCause; e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.refinable.active {
		if e := m.refinable.sourceCause; e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.defaultNormalization.active {
		if e := m.defaultNormalization.sourceCause; e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.pendingFactorScalarTrace != nil && m.pendingFactorScalarTrace.scalarTracePending() {
		if e := m.pendingFactorScalarTrace.sourceCause; e != nil {
			return e
		}
		if e := m.pendingFactorScalarTrace.scalarTraceBlocker(); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingFactorCoreTrace != nil {
		if e := m.pendingFactorCoreTrace.sourceBlocker(); e != nil {
			return e
		}
	}
	if m != nil && m.self == m && m.pendingEuclidTrace != nil {
		if e := m.pendingEuclidTrace.sourceBlocker(); e != nil {
			return e
		}
	}
	if m != nil && m.modGCDScalars.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.modGCDScalars.ownRemaining != 0 {
		if m.modGCDScalars.sourceCause != nil {
			return m.modGCDScalars.sourceCause
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingModGCDTrace != nil {
		if e := m.pendingModGCDTrace.sourceBlocker(); e != nil {
			return e
		}
	}
	if m != nil && m.pendingCRATrace != nil && m.pendingCRATrace.pending() {
		if e := m.pendingCRATrace.sourceFailure(); e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.pendingExactDivTrace != nil {
		if e := m.pendingExactDivTrace.sourceBlocker(); e != nil {
			return e
		}
	}
	if m != nil && m.self == m && m.pendingBerlekamp != nil {
		if e := m.pendingBerlekamp.closeCause; e != nil {
			return e
		}
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.factorTrace != nil && m.factorTrace.executing {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.factorsDisplay != nil && m.factorsDisplay.NumericExecuting() || m.pendingFactorsDisplay != nil && m.pendingFactorsDisplay.pending()) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.sexprArithmeticExecuting() || m.pendingSexprArithmetic != nil) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && (m.rationalInput.running || m.rationalInput.pending(m)) {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.algebraicCopyBorrowers != 0 {
		return ErrUPolynomialCleanupPending
	}
	if m.sturmIsolationExecuting() || m.sequenceSturmExecuting() || m.rootIDExecuting() || m.removeOneHalfRootExecuting() || m.oneHalfRootExecuting() || m.knuthExecuting() || m.rootUpperBoundExecuting() || m.integerEvaluationExecuting() || m.rationalEvaluationExecuting() || m.binaryEvaluationExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m != nil && m.self == m && m.algebraicNegCaller != nil && m.algebraicNegCaller.negationExecuting() {
		return ErrUPolynomialCleanupPending
	}
	if m == nil || m.self != m || m.config.Integer == nil {
		return ErrUPolynomialOwner
	}
	if e := m.sturmIsolationSourceCause(); e != nil {
		return e
	}
	if e := m.pendingSturmSourceCause(); e != nil {
		return e
	}
	if m.rootID.active && (m.rootID.sourceStop != nil || m.rootID.childStop != nil) {
		return errors.Join(m.rootID.sourceStop, m.rootID.childStop)
	}
	if m.oneHalfRoot.active && m.oneHalfRoot.sourceCause != nil {
		return m.oneHalfRoot.sourceCause
	}
	if m.knuth.active && m.knuth.sourceCause != nil {
		return m.knuth.sourceCause
	}
	if m.removeOneHalfRoot.active && m.removeOneHalfRoot.sourceCause != nil {
		return m.removeOneHalfRoot.sourceCause
	}
	if m.rootUpperBound.active && m.rootUpperBound.sourceCause != nil {
		return m.rootUpperBound.sourceCause
	}

	if m.integerEvaluation.active && m.integerEvaluation.sourceCause != nil {
		return m.integerEvaluation.sourceCause
	}

	if m.rationalEvaluation.active && m.rationalEvaluation.sourceCause != nil {
		return m.rationalEvaluation.sourceCause
	}

	if m.binaryEvaluation.active && m.binaryEvaluation.sourceCause != nil {
		return m.binaryEvaluation.sourceCause
	}
	if m.mignotteCallPending() && !m.henselDestructorAuthorized(r, false) && !m.henselCallerDestructorAuthorized(c, false) && !m.primePreparationDestructorAuthorized(p, false) {
		return ErrUPolynomialCleanupPending
	}
	if m.sourceFinalized {
		return nil
	}
	if m.closed || m.config.Integer.Closed() || (m.closing && !m.sourceFinalizing) {
		return arena.ErrClosed
	}
	// A stopped source invocation cannot acknowledge an owning release on
	// reentry. Explicit Close performs that continuation and keeps this history.
	if m.sourceCause != nil {
		return m.sourceCause
	}
	// Complete the actual display source suffix before private older members;
	// failed SMT RAW bodies remain unjoined for explicit owning cleanup.
	if q := &m.display; q.active {
		q.enterCall()
		e := q.finish(nil)
		q.leaveCall()
		if e != nil {
			return e
		}
	}
	for q := m.displayFrames; q != nil; q = q.next {
		if !q.active {
			continue
		}
		q.enterCall()
		e := q.finish(nil)
		q.leaveCall()
		if e != nil {
			return e
		}
	}
	m.sourceFinalizing = true
	derived := [5]*upolyVectorState{&m.db, &m.dbab1, &m.dbab2, &m.tr, &m.push}
	core := [16]*upolyVectorState{&m.basic, &m.div1, &m.div2, &m.exactDiv, &m.gcd1, &m.gcd2, &m.cra, &m.mgcd[0], &m.mgcd[1], &m.mgcd[2], &m.mgcd[3], &m.mgcd[4], &m.mgcd[5], &m.sqf1, &m.sqf2, &m.pw}
	for m.sourceStage < 5 {
		switch m.sourceStage {
		case 0, 2:
			states := derived[:]
			if m.sourceStage == 2 {
				states = core[:]
			}
			for _, s := range states { // sourceIndex is per reached vector, not an inferred live prefix.
				// Earlier vectors in this body have completed logical size zero.
				if s.size == 0 {
					continue
				}
				if s.backing == 0 {
					m.sourceGuard = true
					m.sourceCause = arena.ErrBounds
					return m.sourceCause
				}
				a := m.values(s)
				for m.sourceIndex < s.size {
					if uint64(m.sourceIndex) >= uint64(len(a)) {
						m.sourceGuard = true
						m.sourceCause = arena.ErrBounds
						return m.sourceCause
					}
					guard, e := upolynomialSourceNumeralDel(m.config.Integer, &a[m.sourceIndex])
					if e != nil {
						m.sourceGuard = guard
						m.sourceCause = e
						return e
					}
					m.sourceIndex++
				}
				s.size = 0
				m.sourceIndex = 0
			}
		case 1:
			for i := len(derived) - 1; i >= 0; i-- {
				m.freeBacking(derived[i])
			}
		case 3:
			for i := len(core) - 1; i >= 0; i-- {
				m.freeBacking(core[i])
			}
		case 4:
			for m.sourceIndex < 7 {
				guard, e := m.modular.FinalizeSourceMember(m.sourceIndex)
				if e != nil {
					m.sourceGuard = guard
					m.sourceCause = e
					return e
				}
				m.sourceIndex++
			}
		}
		m.sourceStage++
		m.sourceIndex = 0
	}
	m.sourceFinalized = true
	m.sourceCause = nil
	return nil
}

// CleanupPolynomial is an owning-abort continuation for the actual provided
// caller field, including after source modular destruction. It does not revive
// the owner, reconstruct source fields, or discover caller roots in a registry.
func (m *UPolynomialManager) CleanupPolynomial(p *UPolynomial) error {
	if m == nil || m.self != m || m.config.Integer == nil {
		return ErrUPolynomialOwner
	}
	if m.closed || m.config.Integer.Closed() {
		return arena.ErrClosed
	}
	if p == nil {
		return ErrUPolynomialOwner
	}
	if *p == (UPolynomial{}) {
		return nil
	}
	if p.origin != m.origin || p.slot == 0 {
		return ErrUPolynomialOwner
	}
	i := p.slot - 1
	if uint64(i/32) >= uint64(len(m.pages)) {
		return ErrUPolynomialOwner
	}
	d := &m.pages[i/32].rows[i%32]
	if !d.live || d.generation != p.generation {
		return ErrUPolynomialOwner
	}
	if d.state.backing != 0 {
		a := m.values(&d.state)
		for j := uint32(0); uint64(j) < uint64(len(a)); j++ {
			if e := m.config.Integer.Del(&a[j]); e != nil {
				return e
			}
		}
	}
	m.freeBacking(&d.state)
	d.live = false
	if d.generation != ^uint32(0) {
		d.generation++
	}
	m.live--
	*p = UPolynomial{}
	return nil
}
