package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:318-334;
// src/util/scoped_numeral.h, mpq.h::del (NUM before DEN).

import (
	"errors"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Payload is the existing core.localQ, constructed only at accepted beginQ.
// Source destruction and explicit owning acknowledgement have separate cursors.
type algebraicPredicateQScope struct {
	comparisonCopy                                 unumeral.IntegerCopyAllocationReceipt
	evaluation                                     upolyRationalEvaluationInvocationReceipt
	owner                                          *AlgebraicRootCore
	hooks                                          unumeral.NumericInvocationHooks
	roots                                          unumeral.NumericAllocationRoots
	parts                                          unumeral.RationalPartsInvocation
	raw                                            []unumeral.IntegerRawInvocationReceipt
	sourceCursor, ownCursor                        uint8
	active, executing                              bool
	bodyCause, childStop, sourceStop, cleanupCause error
}

func (s *algebraicPredicateQScope) NumericBound(executing bool) bool {
	m := s.owner
	return m != nil && m.self == m && !m.closed && s == &m.predicateQ && s.active && m.localActive && (!executing || s.executing)
}
func (s *algebraicPredicateQScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicPredicateQScope) NumericRecord(z *unumeral.IntegerManager, _ string, r unumeral.NumericInvocationReceipt) {
	if s.NumericBound(true) && s.owner.config.Rationals.Integers() == z && r.Raw != nil {
		s.raw = append(s.raw, *r.Raw)
	}
}
func (m *AlgebraicRootCore) beginPredicateQ() error {
	s := &m.predicateQ
	if s.active || s.executing || !m.predicate.active {
		return ErrAlgebraicPending
	}
	if e := m.beginQ(); e != nil {
		return e
	}
	*s = algebraicPredicateQScope{owner: m, active: true}
	s.hooks.Policy = s
	return nil
}
func (m *AlgebraicRootCore) setPredicateQParts(n, d *unumeral.Int) error {
	s := &m.predicateQ
	if !s.NumericBound(false) {
		return ErrAlgebraicOwner
	}
	if s.executing {
		return ErrAlgebraicPending
	}
	s.executing = true
	defer func() { s.executing = false }()
	context := unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
	e := m.config.Rationals.SetPartsWithInvocation(&m.localQ, n, d, context, &s.parts)
	z := m.config.Rationals.Integers()
	for i := len(s.raw) - 1; i >= 0 && s.childStop == nil; i-- {
		s.childStop = z.RawInvocationSourceCause(&s.raw[i])
	}
	if s.childStop == nil {
		s.childStop = m.config.Rationals.RawNormalizeInvocationSourceCause(&s.parts.Normalize)
	}
	return e
}
func (s *algebraicPredicateQScope) childrenPending() bool {
	m := s.owner
	z := m.config.Rationals.Integers()
	if z.MatchesCopyAllocationInvocation(&s.comparisonCopy) || s.evaluationPending() || unumeral.NumericAllocationPending(&s.roots) || z.MatchesCopyAllocationInvocation(&s.parts.Numerator) || z.MatchesCopyAllocationInvocation(&s.parts.Denominator) || m.config.Rationals.MatchesRawNormalizeInvocation(&s.parts.Normalize) {
		return true
	}
	for i := range s.raw {
		if z.MatchesRawInvocation(&s.raw[i]) {
			return true
		}
	}
	return false
}
func (m *AlgebraicRootCore) finishPredicateQ(cause error) error {
	s := &m.predicateQ
	if !s.active {
		return cause
	}
	if !s.NumericBound(false) {
		return errors.Join(cause, ErrAlgebraicOwner)
	}
	if s.executing {
		return errors.Join(cause, ErrAlgebraicPending)
	}
	s.executing = true
	defer func() { s.executing = false }()
	if s.bodyCause == nil {
		s.bodyCause = cause
	}
	if s.childStop != nil || s.sourceStop != nil {
		if m.predicate.childStop == nil {
			m.predicate.childStop = errors.Join(s.childStop, s.sourceStop)
		}
		return errors.Join(cause, s.childStop, s.sourceStop)
	}
	z := m.config.Rationals.Integers()
	fields := [2]*unumeral.Int{m.localQ.BorrowNumerator(), m.localQ.BorrowDenominator()}
	for s.sourceCursor < 2 {
		if e := unumeral.SourceIntegerDel(z, fields[s.sourceCursor]); e != nil {
			s.sourceStop = e
			m.localOriginal, m.localCleanup = cause, e
			if m.predicate.childStop == nil {
				m.predicate.childStop = e
			}
			return errors.Join(cause, e)
		}
		s.sourceCursor++
	}
	if !s.childrenPending() {
		s.active = false
		m.localActive = false
		m.localOriginal = nil
		m.localCleanup = nil
	}
	return cause
}
func (m *AlgebraicRootCore) cleanupPredicateQ() error {
	s := &m.predicateQ
	if !s.active {
		return nil
	}
	if !s.NumericBound(false) {
		return ErrAlgebraicOwner
	}
	if s.executing {
		return ErrAlgebraicPending
	}
	s.executing = true
	defer func() { s.executing = false }()
	e := m.cleanupPredicateQBody()
	if e != nil && s.cleanupCause == nil {
		s.cleanupCause = e
	}
	return e
}
func (m *AlgebraicRootCore) cleanupPredicateQBody() error {
	s := &m.predicateQ
	z := m.config.Rationals.Integers()
	if e := s.cleanupEvaluation(); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.comparisonCopy); e != nil {
		return e
	}
	if e := s.roots.Cleanup(z); e != nil {
		return e
	}
	for i := len(s.raw) - 1; i >= 0; i-- {
		if e := z.CleanupRawInvocation(&s.raw[i], true); e != nil {
			return e
		}
	}
	if e := m.config.Rationals.CleanupRawNormalizeInvocation(&s.parts.Normalize); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.parts.Denominator); e != nil {
		return e
	}
	if e := z.CleanupCopyAllocationInvocation(&s.parts.Numerator); e != nil {
		return e
	}
	fields := [2]*unumeral.Int{m.localQ.BorrowNumerator(), m.localQ.BorrowDenominator()}
	for s.ownCursor < 2 {
		if e := z.CleanupRawNumeral(fields[s.ownCursor]); e != nil {
			return e
		}
		s.ownCursor++
	}
	s.active = false
	m.localActive = false
	m.localOriginal = nil
	m.localCleanup = nil
	return nil
}

func (m *UPolynomialManager) matchesRationalEvaluationReceipt(r *upolyRationalEvaluationInvocationReceipt) bool {
	if m == nil || m.self != m || r == nil {
		return false
	}
	q := &m.rationalEvaluation
	return r.owner == m && r.frame == q && q.owner == m && r.invocation != 0 && r.invocation == q.invocation && r.history == q.history
}
func (s *algebraicPredicateQScope) evaluationPending() bool {
	p := s.owner.config.Polynomials
	return p.matchesRationalEvaluationReceipt(&s.evaluation) && p.rationalEvaluation.active
}
func (s *algebraicPredicateQScope) cleanupEvaluation() error {
	if s.evaluation.owner == nil {
		return nil
	}
	p := s.owner.config.Polynomials
	if e := p.cleanupRationalEvaluationWithReceipt(&s.evaluation, false); e != nil {
		return e
	}
	return p.cleanupRationalEvaluationWithReceipt(&s.evaluation, true)
}
func (m *AlgebraicRootCore) evalPredicateRational(v UPolynomialView) (int, error) {
	s := &m.predicateQ
	if !s.NumericBound(false) {
		return 0, ErrAlgebraicOwner
	}
	if s.executing || s.evaluationPending() {
		return 0, ErrAlgebraicPending
	}
	s.executing = true
	defer func() { s.executing = false }()
	p := m.config.Polynomials
	sign, e := p.evalSignRationalWithReceipt(v, &m.localQ, &s.evaluation)
	if p.matchesRationalEvaluationReceipt(&s.evaluation) && s.childStop == nil {
		s.childStop = p.rationalEvaluation.sourceCause
	}
	return sign, e
}

func (m *AlgebraicRootCore) lessPredicateRational(a *unumeral.BinaryRational) (bool, error) {
	s := &m.predicateQ
	if !s.NumericBound(false) {
		return false, ErrAlgebraicOwner
	}
	if s.executing || s.childrenPending() {
		return false, ErrAlgebraicPending
	}
	s.executing = true
	defer func() { s.executing = false }()
	context := unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
	return m.config.Binary.WithAllocationContext(context).LessRationalWithCopyAllocationReceipt(a, m.config.Rationals, &m.localQ, &s.comparisonCopy)
}
