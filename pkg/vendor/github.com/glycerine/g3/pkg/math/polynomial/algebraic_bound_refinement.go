package polynomial

// Source: src/math/polynomial/upolynomial.cpp:2764-2778,2797-2807, reached
// by src/math/polynomial/algebraic_numbers.cpp:730,743,773,2721-2767.
// Width outlives each midpoint. MPBQ_DEBUG snapshots retain their separate
// diagnostic service ownership; this policy covers the numerical body.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	"math"
)

const (
	boundRefinementWidth = 0
	boundRefinementMid   = 1
)

type algebraicRefinementScope struct {
	invocation uint64
	evaluation upolyBinaryEvaluationInvocationReceipt
	owner      *AlgebraicRootCore
	values     [2]unumeral.BinaryRational
	live       uint8
	hooks      unumeral.NumericInvocationHooks
	roots      unumeral.NumericAllocationRoots
	stop       error
	executing  bool
}

func (s *algebraicRefinementScope) NumericBound(executing bool) bool {
	m := s.owner
	if m == nil || m.self != m || m.closed || s.invocation == 0 || s.hooks.Generation != s.invocation {
		return false
	}
	if s == &m.bounds.refinement {
		return !executing || s.executing && m.bounds.executing
	}
	if s == &m.decimalDisplay.refinement {
		return !executing || s.executing && m.decimalDisplayRunning
	}
	return s == &m.refinement && (!executing || s.executing)
}
func (s *algebraicRefinementScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}
func (s *algebraicRefinementScope) pending(m *AlgebraicRootCore) bool {
	if s.owner != m {
		return false
	}
	if s.live != 0 || s.stop != nil || unumeral.NumericAllocationPending(&s.roots) || s.evaluationPending(m) {
		return true
	}

	return false
}
func (s *algebraicRefinementScope) context() unumeral.NumericAllocationContext {
	return unumeral.NumericAllocationContext{}.WithCaller(&s.hooks)
}
func (s *algebraicRefinementScope) construct(which uint8) (*unumeral.BinaryRational, error) {
	if !s.NumericBound(true) || s.live&(1<<which) != 0 {
		return nil, ErrAlgebraicPending
	}
	s.values[which] = unumeral.BinaryRational{}
	s.live |= 1 << which
	return &s.values[which], nil
}
func (s *algebraicRefinementScope) sourceFinish(which uint8) error {
	if s.stop != nil {
		return s.stop
	}
	m := s.owner
	if e := s.evaluationSourceCause(m); e != nil {
		s.stop = e
		return e
	}

	if s.live&(1<<which) == 0 {
		return nil
	}
	_, e := unumeral.SourceNumeralDel(m.config.Rationals.Integers(), s.values[which].BorrowNumerator())
	if e != nil {
		s.stop = e
		return e
	}
	s.live &^= 1 << which
	return nil
}
func (s *algebraicRefinementScope) cleanup() error {
	if s.owner == nil {
		return nil
	}
	if !s.NumericBound(false) {
		return ErrAlgebraicOwner
	}
	if s.executing {
		return ErrAlgebraicPending
	}
	if e := s.cleanupEvaluation(s.owner); e != nil {
		return e
	}
	z := s.owner.config.Rationals.Integers()
	if e := s.roots.Cleanup(z); e != nil {
		return e
	}

	for i := uint8(2); i != 0; {
		i--
		if s.live&(1<<i) != 0 {
			if e := z.CleanupRawNumeral(s.values[i].BorrowNumerator()); e != nil {
				return e
			}
			s.live &^= 1 << i
		}
	}
	s.stop = nil
	return nil
}

func (s *algebraicRefinementScope) begin(m *AlgebraicRootCore) error {
	if m == nil || m.self != m || (s != &m.bounds.refinement && s != &m.refinement && s != &m.decimalDisplay.refinement) {
		return ErrAlgebraicOwner
	}
	if s.executing || s.pending(m) {
		return ErrAlgebraicPending
	}
	if s.invocation == math.MaxUint64 {
		return ErrAlgebraicPending
	}
	s.invocation++
	s.owner = m
	s.hooks = unumeral.NumericInvocationHooks{Generation: s.invocation, Policy: s}
	s.evaluation = upolyBinaryEvaluationInvocationReceipt{}
	s.executing = true
	return nil
}
