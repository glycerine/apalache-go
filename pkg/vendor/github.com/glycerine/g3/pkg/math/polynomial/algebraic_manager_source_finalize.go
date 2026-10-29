package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:64-125,2782-2787.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func (m *AlgebraicManager) NumericExecuting() bool { return m.sourceRunning }
func (m *AlgebraicManager) NumericCallbackActive() bool {
	z := m.config.Numerals.Integers()
	return z.RuntimeOwner().FailureTransportActive() || z.BorrowTraceTags().OutputCallbackActive()
}
func (m *AlgebraicManager) bindSourceDestructor() error {
	z := m.config.Numerals.Integers()
	if e := z.BindCaller(&m.sourceBindings[0], m, 0); e != nil {
		return e
	}
	m.sourceBound++
	if e := z.Natural.BindCaller(&m.sourceBindings[1], m, 0); e != nil {
		m.unbindSourceDestructor()
		return e
	}
	m.sourceBound++
	return nil
}
func (m *AlgebraicManager) unbindSourceDestructor() {
	for m.sourceBound != 0 {
		m.sourceBound--
		if e := m.sourceBindings[m.sourceBound].Unbind(); e != nil {
			panic(e)
		}
	}
}

// sourceDestructorAdmission borrows no ownership from a directory. Retained
// lexical invocations must end through their own receipts before destruction.
func (m *AlgebraicManager) sourceDestructorAdmission() error {
	c := m.AlgebraicRootCore
	if e := m.ready(); e != nil {
		return e
	}
	if e := m.univariate.check(); e != nil {
		return e
	}
	if e := m.polynomial.ready(); e != nil {
		return e
	}
	if e := m.binary.Open(); e != nil {
		return e
	}
	if m.binary.DisplayState().Pending {
		return ErrAlgebraicPending
	}
	if m.params.Helpers != nil {
		return ErrAlgebraicPending
	}
	if c.callers != 0 || c.vectors != 0 {
		return ErrAlgebraicLive
	}
	if c.rootSelection != nil || c.compareSequence != nil || c.pending.slot != 0 || c.localActive || c.localSelection != nil || c.localIntegerSet != nil || c.localIntegerAdd != nil {
		return ErrAlgebraicPending
	}
	if c.foreignCopy.active || c.toOwnedRational.active || c.rootConstruction.active || c.isolationSigns.active || c.isolationConversion.active || c.binary.active || c.arithmetic.active || c.division.active || c.selection.active || c.basicRootScope.active || c.integerSet.active || c.integerAdd.active || c.constQ.active || c.predicateQ.active || c.refinementQ.active || c.rootDisplay.active || c.predicateTrace.active || c.rationalSign.active || c.signSubstitution.active || c.signInterval.active || c.signResultant.active || c.resultant.active || c.predicate.active || c.negation.active {
		return ErrAlgebraicPending
	}
	if c.multivariateIsolationDepth != 0 || c.copyCall.pending() || c.cellBoundsCall.pending() || c.cellNeg.pending() || c.cellEvaluationPending() || c.decimalDisplay.pending(c) || c.bounds.remaining != 0 || c.bounds.refinement.pending(c) || c.bounds.conversion.pending(c) || c.bounds.evaluationPending(c) || c.refinement.pending(c) || c.refinementConversion.pending(c) || c.inversion.remaining != 0 || c.inversion.sourceRemaining != 0 || c.inversion.numericPending(c) || c.inversion.traceDisplayPending {
		return ErrAlgebraicPending
	}
	for i := range c.inversion.conversions {
		if c.inversion.conversions[i].pending(c) {
			return ErrAlgebraicPending
		}
	}
	z := m.config.Numerals.Integers()
	for _, r := range []*unumeral.IntegerCopyAllocationReceipt{&c.toRationalCopy, &c.representationCopy, &c.rationalCopy, &c.coefficientCopy, &c.bounds.copy} {
		if z.MatchesCopyAllocationInvocation(r) {
			return ErrAlgebraicPending
		}
	}
	if unumeral.NumericAllocationPending(&c.representationNeg.roots) || m.univariate.modular.MatchesPNormalizeInvocation(&c.representationNormalize) {
		return ErrAlgebraicPending
	}
	for _, v := range []*BinaryRationalVector{c.uppers, c.lowers, c.roots} {
		if v != nil && (v.state.dropPending || v.state.managerResetPending) {
			return ErrAlgebraicPending
		}
	}
	s := m.factorService
	if s.scope != nil || s.frontend.active || s.preparation.active || s.diagnostic.active || s.constantCallerExecuting() || s.entryTraceBorrow.executing || s.manager.displayReceiptPending(&s.entryTrace) || s.Stats().RetainedScopes != 0 {
		return ErrAlgebraicPending
	}
	for q := s.frontendConstantCalls; q != nil; q = q.next {
		if !q.retired {
			return ErrAlgebraicPending
		}
	}
	return nil
}

// SourceFinalize runs empty ~imp followed by reverse declaration-order member
// destruction, then the outer owned allocator destructor. Go cold owner/header
// retirement remains the separate Close path. A first source stop is sticky.
func (m *AlgebraicManager) SourceFinalize() error {
	if m == nil || m.self != m || m.AlgebraicRootCore == nil {
		return ErrAlgebraicOwner
	}
	if m.sourceRunning || m.finishing || m.constructing {
		return ErrAlgebraicPending
	}
	if e := m.AlgebraicRootCore.checkDestructorExecution(); e != nil {
		return e
	}
	if m.sourceStarted {
		return m.sourceCause
	}
	if e := m.sourceDestructorAdmission(); e != nil {
		return e
	}
	m.sourceStarted, m.sourceRunning = true, true
	defer func() { m.sourceRunning = false }()
	if e := m.bindSourceDestructor(); e != nil {
		m.sourceCause = e
		return e
	}
	defer m.unbindSourceDestructor()
	fail := func(e error) error { m.sourceCause = e; return e }
	for m.sourceStage < 15 {
		var e error
		if m.sourceStage < 14 {
			e = m.sourceMember(m.sourceStage, &m.sourceIndex, false)
		} else if m.ownAllocator {
			e = m.allocator.Close()
		}
		if e != nil {
			return fail(e)
		}
		m.sourceStage++
		m.sourceIndex = 0
	}
	m.sourceFinalized = true
	return nil
}
func (m *AlgebraicManager) sourceScopedPolynomial(p *UPolynomial, index *uint32) error {
	if p.slot == 0 {
		return nil
	}
	d, e := m.univariate.descriptor(*p)
	if e != nil {
		return e
	}
	return upolynomialScopedVectorSourceDel(m.univariate, &d.state, index)
}

// These are the five actual scoped parent fields, never caller polynomials.
// Their cold descriptors survive source destruction for exact owning cleanup.
func (m *AlgebraicManager) cleanupSourceMembers() error {
	c := m.AlgebraicRootCore
	if e := m.univariate.CleanupPolynomial(&c.addTmp); e != nil {
		return e
	}
	for _, v := range []**BinaryRationalVector{&c.uppers, &c.lowers, &c.roots} {
		if *v != nil {
			if e := (*v).CleanupManagerReset(); e != nil {
				return e
			}
			if e := (*v).CleanupDrop(); e != nil {
				return e
			}
			if e := (*v).Close(); e != nil {
				return e
			}
			*v = nil
		}
	}
	if c.factors != nil {
		if e := c.factors.Close(); e != nil {
			return e
		}
		c.factors = nil
	}
	for _, p := range []*UPolynomial{&c.evalSignTmp, &c.tmp3, &c.tmp2, &c.isolateTmp1} {
		if e := m.univariate.CleanupPolynomial(p); e != nil {
			return e
		}
	}
	if m.rationalTemporaryConstructed {
		return m.config.Numerals.Integers().Del(&c.isRationalTmp)
	}
	return nil
}

func (m *AlgebraicManager) sourceRationalTemporary() error {
	return unumeral.SourceIntegerDel(m.config.Numerals.Integers(), &m.AlgebraicRootCore.isRationalTmp)
}
