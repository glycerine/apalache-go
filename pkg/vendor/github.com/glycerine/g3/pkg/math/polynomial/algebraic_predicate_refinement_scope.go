package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:291-308,897-929.

import unumeral "github.com/glycerine/g3/pkg/util/numeral"

// A receipt is stamped only after the actual in-place refinement entry accepts
// its frame. Refusal before that entry cannot adopt another caller's stop.
type algebraicRefinementInvocation struct {
	owner      *AlgebraicRootCore
	frame      *algebraicRefinementScope
	invocation uint64
}

func (m *AlgebraicRootCore) matchesRefinement(r *algebraicRefinementInvocation) bool {
	return m != nil && m.self == m && r != nil && r.owner == m && r.frame == &m.refinement && m.refinement.owner == m && r.invocation != 0 && r.invocation == m.refinement.invocation
}
func (m *AlgebraicRootCore) refinementSourceStop(r *algebraicRefinementInvocation) error {
	if !m.matchesRefinement(r) {
		return nil
	}
	if e := m.refinement.stop; e != nil {
		return e
	}
	if e := m.refinement.evaluationSourceCause(m); e != nil {
		return e
	}
	if m.refinementQ.active {
		if e := m.refinementQ.childStop; e != nil {
			return e
		}
		return m.refinementQ.sourceStop
	}
	return nil
}

// This is an owning continuation, not a replay of the stopped source finalizer.
// Younger refinement/promotion children must be acknowledged first.
func (m *AlgebraicRootCore) cleanupPredicate() error {
	p := &m.predicate
	if !p.active {
		return nil
	}
	if p.finishing || p.arithmeticExecuting || m.predicateDiagnosticRunning || m.predicateTracePending(&p.trace) || m.predicateQ.active || m.integerSetterPending(&p.setter) {
		return ErrAlgebraicPending
	}
	if m.matchesRefinement(&p.refinement) && (m.refinement.pending(m) || m.refinementConversion.pending(m) || m.refinementQ.active) {
		return ErrAlgebraicPending
	}
	p.finishing = true
	defer func() { p.finishing = false }()
	z := m.config.Rationals.Integers()
	if e := p.cleanupIntegerEvaluation(m); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&p.integerComparison); e != nil {
		return e
	}
	if e := p.numericRoots.Cleanup(z); e != nil {
		if p.ownCause == nil {
			p.ownCause = e
		}
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&p.candidateCopy); e != nil {
		if p.ownCause == nil {
			p.ownCause = e
		}
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&p.leadingCopy); e != nil {
		if p.ownCause == nil {
			p.ownCause = e
		}
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&p.savedCopy); e != nil {
		if p.ownCause == nil {
			p.ownCause = e
		}
		return e
	}
	for p.ownStage < 5 {
		var e error
		switch p.ownStage {
		case 0:
			if p.candidateActive {
				e = z.CleanupRawNumeral(&p.candidate)
			}
		case 1:
			if p.upperActive {
				e = z.CleanupRawNumeral(p.upper.BorrowNumerator())
			}
		case 2:
			if p.lowerActive {
				e = z.CleanupRawNumeral(p.lower.BorrowNumerator())
			}
		case 3:
			e = m.restoreSaved()
		case 4:
			if p.savedActive {
				fields := [2]*unumeral.Int{p.saved.BorrowLower().BorrowNumerator(), p.saved.BorrowUpper().BorrowNumerator()}
				for p.savedOwnCursor < 2 {
					e = z.CleanupRawNumeral(fields[p.savedOwnCursor])
					if e != nil {
						break
					}
					p.savedOwnCursor++
				}
			}
		}
		if e != nil {
			if p.ownCause == nil {
				p.ownCause = e
			}
			return e
		}
		p.ownStage++
	}
	p.active = false
	return nil
}

// The actual predicate arithmetic frame grants numerical allocation ownership.
func (p *algebraicPredicateScope) NumericBound(executing bool) bool {
	m := p.numericOwner
	return m != nil && m.self == m && !m.closed && p == &m.predicate && p.active && (!executing || p.arithmeticExecuting)
}
func (p *algebraicPredicateScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !p.NumericBound(true) || p.numericOwner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &p.numericRoots
}
func (p *algebraicPredicateScope) numericContext() unumeral.NumericAllocationContext {
	return unumeral.NumericAllocationContext{}.WithCaller(&p.numericHooks)
}
func (p *algebraicPredicateScope) beginArithmetic(m *AlgebraicRootCore) error {
	if m == nil || m.self != m || p != &m.predicate {
		return ErrAlgebraicOwner
	}
	if !p.active || p.finishing || p.arithmeticExecuting || p.integerEvaluationPending(m) || m.config.Rationals.Integers().MatchesCopyAllocationInvocation(&p.integerComparison) || unumeral.NumericAllocationPending(&p.numericRoots) {
		return ErrAlgebraicPending
	}
	p.numericOwner = m
	p.numericHooks.Policy = p
	p.arithmeticExecuting = true
	return nil
}
