package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:541-563.

import (
	"errors"
	"math"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// algebraicIsolationNumericScope transports an allocation failure from the
// retained univariate isolation call into its owning algebraic invocation.
// Nested polynomial scopes temporarily shadow these caller bindings and keep
// their own receipts; an allocation made before such a child is accepted stays
// with this exact caller.
type algebraicIsolationNumericScope struct {
	owner             *AlgebraicRootCore
	hooks             unumeral.NumericInvocationHooks
	before            unumeral.NumericAllocationRoots
	roots             unumeral.NumericAllocationRoots
	integer, natural  unumeral.NumericCallerBinding
	active, executing bool
}

func (s *algebraicIsolationNumericScope) valid() bool {
	m := s.owner
	return m != nil && m.self == m && !m.closed && s == &m.isolationNumeric
}

func (s *algebraicIsolationNumericScope) NumericBound(executing bool) bool {
	return s.valid() && s.active && (!executing || s.executing)
}

func (s *algebraicIsolationNumericScope) NumericExecuting() bool {
	return s.NumericBound(true)
}

func (s *algebraicIsolationNumericScope) NumericRoots(z *unumeral.IntegerManager, local *unumeral.NumericAllocationRoots) *unumeral.NumericAllocationRoots {
	if !s.NumericBound(true) || s.owner.config.Rationals.Integers() != z {
		return nil
	}
	if local != nil {
		return local
	}
	return &s.roots
}

func (s *algebraicIsolationNumericScope) bind() error {
	z := s.owner.config.Rationals.Integers()
	if e := z.BindCaller(&s.integer, s, 0); e != nil {
		return e
	}
	if e := z.Naturals().BindCaller(&s.natural, s, 0); e != nil {
		return errors.Join(e, s.integer.Unbind())
	}
	return nil
}

func (s *algebraicIsolationNumericScope) unbind() error {
	return errors.Join(s.natural.Unbind(), s.integer.Unbind())
}

func (m *AlgebraicRootCore) beginIsolationNumeric() error {
	s := &m.isolationNumeric
	if s.active || s.executing || s.hooks.Generation == math.MaxUint64 {
		return ErrAlgebraicPending
	}
	generation := s.hooks.Generation + 1
	*s = algebraicIsolationNumericScope{owner: m, active: true, executing: true}
	s.hooks = unumeral.NumericInvocationHooks{Generation: generation, Policy: s}
	before, e := unumeral.CaptureNumericAllocationRoots(m.config.Rationals.Integers())
	if e != nil {
		s.active = false
		s.executing = false
		return e
	}
	s.before = before
	if e = s.bind(); e != nil {
		s.active = false
		s.executing = false
		return e
	}
	return nil
}

func (m *AlgebraicRootCore) finishIsolationNumeric(cause error) error {
	s := &m.isolationNumeric
	captured := s.roots.After(m.config.Rationals.Integers(), s.before)
	s.executing = false
	unbound := s.unbind()
	if !unumeral.NumericAllocationPending(&s.roots) {
		s.active = false
	}
	return errors.Join(cause, captured, unbound)
}

func (m *AlgebraicRootCore) cleanupIsolationNumeric() error {
	s := &m.isolationNumeric
	if s.executing {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	s.executing = true
	defer func() { s.executing = false }()
	if e := s.roots.Cleanup(m.config.Rationals.Integers()); e != nil {
		return e
	}
	s.active = false
	return nil
}
