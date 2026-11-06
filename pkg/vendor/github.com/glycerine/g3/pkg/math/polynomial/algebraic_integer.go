package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:2861-2865,2911-2915.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type algebraicIntegerSetScope struct {
	invocation              uint64
	numeratorCopy           unumeral.IntegerCopyAllocationReceipt
	sourceCursor, ownCursor uint8
	finishing               bool
	selection               *algebraicSelectionScope
	polynomialCaller        *polynomialAlgebraicEvaluationScope
	value                   unumeral.Rational
	addition                *algebraicIntegerAddScope
	active, running         bool
	sourceStop              error
}
type algebraicIntegerAddScope struct {
	polynomialCaller          *polynomialAlgebraicEvaluationScope
	value                     Algebraic
	active, running, cleaning bool
	sourceStop                error
}

func (m *AlgebraicRootCore) cleanupIntegerSet() error {
	s := &m.integerSet
	if s.running || s.finishing {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if m.localIntegerSet == s && m.localActive {
		return ErrAlgebraicPending
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	z := m.config.Rationals.Integers()
	if e := z.CleanupCopyAllocationInvocation(&s.numeratorCopy); e != nil {
		return e
	}
	fields := [2]*unumeral.Int{s.value.BorrowNumerator(), s.value.BorrowDenominator()}
	for s.ownCursor < 2 {
		if e := z.CleanupRawNumeral(fields[s.ownCursor]); e != nil {
			return e
		}
		s.ownCursor++
	}
	s.active = false
	return nil
}
func (m *AlgebraicRootCore) finishIntegerSet(cause error) error {
	s := &m.integerSet
	if !s.active {
		return cause
	}
	if s.running || s.finishing {
		return errors.Join(cause, ErrAlgebraicPending)
	}
	s.finishing = true
	defer func() { s.finishing = false }()
	if s.sourceStop == nil && m.localIntegerSet == s && m.localActive && m.localCleanup != nil {
		s.sourceStop = m.localCleanup
	}
	if s.sourceStop != nil {
		return errors.Join(cause, s.sourceStop)
	}
	z := m.config.Rationals.Integers()
	fields := [2]*unumeral.Int{s.value.BorrowNumerator(), s.value.BorrowDenominator()}
	for s.sourceCursor < 2 {
		if e := unumeral.SourceIntegerDel(z, fields[s.sourceCursor]); e != nil {
			s.sourceStop = e
			return errors.Join(cause, e)
		}
		s.sourceCursor++
	}
	if !z.MatchesCopyAllocationInvocation(&s.numeratorCopy) {
		s.active = false
	}
	return cause
}

func (m *AlgebraicRootCore) cleanupIntegerAdd(owning bool) error {
	s := &m.integerAdd
	if s.running || s.cleaning {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	s.cleaning = true
	defer func() { s.cleaning = false }()
	l, e := m.locationCleanup(&s.value)
	if e != nil {
		return e
	}
	if l != nil {
		if e = m.destroyCell(&s.value, l, owning); e != nil {
			return e
		}
	}
	s.active = false
	return nil
}

// SetInteger preserves the source mpz -> scoped mpq -> algebraic setter path.
func (m *AlgebraicRootCore) SetInteger(out *Algebraic, n *unumeral.Int) (cause error) {
	return m.setIntegerWithInvocation(out, n, nil)
}
func (m *AlgebraicRootCore) setIntegerWithInvocation(out *Algebraic, n *unumeral.Int, receipt *algebraicIntegerSetInvocation) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	s := &m.integerSet
	if s.active || s.finishing || s.invocation == ^uint64(0) {
		return ErrAlgebraicPending
	}
	*s = algebraicIntegerSetScope{invocation: s.invocation + 1, polynomialCaller: m.polynomialCaller, value: unumeral.NewRational(), active: true, running: true}
	if receipt != nil {
		*receipt = algebraicIntegerSetInvocation{m, s, s.invocation}
	}
	if m.selection.running {
		s.selection = &m.selection
	}
	if m.integerAdd.running {
		s.addition = &m.integerAdd
	}
	defer func() { s.running = false; cause = m.finishIntegerSet(cause) }()

	if e := m.config.Rationals.SetIntegerWithCopyAllocationReceipt(&s.value, n, &s.numeratorCopy); e != nil {
		return e
	}
	return m.SetRational(out, &s.value)
}

// AddInteger constructs a real scoped algebraic coefficient before adding it.
// The source overload is not equivalent in lifetime to adding a borrowed mpq.
func (m *AlgebraicRootCore) AddInteger(a *Algebraic, n *unumeral.Int, out *Algebraic) error {
	return m.addIntegerWithManager(a, n, out, nil)
}
func (m *AlgebraicRootCore) addIntegerWithManager(a *Algebraic, n *unumeral.Int, out *Algebraic, wrapper *AlgebraicManager) (cause error) {
	if e := m.check(); e != nil {
		return e
	}
	s := &m.integerAdd
	if s.active {
		return ErrAlgebraicPending
	}
	*s = algebraicIntegerAddScope{polynomialCaller: m.polynomialCaller, active: true, running: true}
	defer func() {
		s.running = false
		stopped := m.rootTraceSourceCause()
		if stopped == nil && m.integerSet.addition == s && m.integerSet.active {
			stopped = m.integerSet.sourceStop
		}
		if stopped == nil && m.localIntegerAdd == s && m.localActive && m.localCleanup != nil {
			stopped = m.localCleanup
		}
		if stopped == nil && m.arithmetic.integerAddition == s && m.arithmetic.active && m.arithmetic.sourceStop != nil {
			stopped = m.arithmetic.sourceStop
		}
		if stopped != nil {
			s.sourceStop = stopped
			cause = errors.Join(cause, stopped)
			return
		}
		cleanup := m.cleanupIntegerAdd(false)
		if cleanup != nil {
			s.sourceStop = cleanup
		}
		cause = errors.Join(cause, cleanup)
	}()
	if e := m.SetInteger(&s.value, n); e != nil {
		return e
	}
	if wrapper != nil {
		return wrapper.Add(a, &s.value, out)
	}
	return m.Add(a, &s.value, out)
}
