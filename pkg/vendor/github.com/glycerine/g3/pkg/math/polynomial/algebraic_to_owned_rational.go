package polynomial

// Source: src/math/polynomial/algebraic_numbers.cpp:343-348,2835-2837.

import (
	"errors"

	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Embedded in the concrete core, with no separate per-call adapter allocation.
type algebraicOwnedRationalScope struct {
	rawConversion                     bool
	boundInvocation                   uint64
	service                           *unumeral.RationalService
	tmp                               unumeral.Rational
	tmp2                              unumeral.OwnedRational
	constructor, assignment           unumeral.RationalServiceInvocation
	integerBinding, naturalBinding    unumeral.NumericCallerBinding
	active, running, executing        bool
	constructed, tmpDone, tmp2Done    bool
	constructorEnded, assignmentEnded bool
	bodyCause, sourceCause            error
}

func (s *algebraicOwnedRationalScope) NumericExecuting() bool { return s.running || s.executing }

func (s *algebraicOwnedRationalScope) bind(z *unumeral.IntegerManager) error {
	if e := z.BindCaller(&s.integerBinding, s, 0); e != nil {
		return e
	}
	if e := z.Natural.BindCaller(&s.naturalBinding, s, 0); e != nil {
		_ = s.integerBinding.Unbind()
		return e
	}
	return nil
}

func (s *algebraicOwnedRationalScope) unbind() {
	if e := s.naturalBinding.Unbind(); e != nil {
		panic(e)
	}
	if e := s.integerBinding.Unbind(); e != nil {
		panic(e)
	}
}

// ToOwnedRational implements the rational overload with a private scoped_mpq,
// a separately constructed global rational, and the original assignment into
// caller output. The service is the explicit Go owner of the global rational.
func (m *AlgebraicRootCore) ToOwnedRational(a *Algebraic, service *unumeral.RationalService, out *unumeral.OwnedRational) error {
	return m.ownedRationalResult(a, service, out, 0, 0)
}

// Source cpp2998–3002,3008–3012,3025–3029,3035–3039. Each wrapper
// declares its raw scoped_mpq before constructing and assigning rational(q).
// These wrappers share CleanupToOwnedRational with ToOwnedRational.
func (m *AlgebraicRootCore) GetLowerOwnedRational(a *Algebraic, service *unumeral.RationalService, out *unumeral.OwnedRational) error {
	return m.ownedRationalResult(a, service, out, 1, 0)
}
func (m *AlgebraicRootCore) GetUpperOwnedRational(a *Algebraic, service *unumeral.RationalService, out *unumeral.OwnedRational) error {
	return m.ownedRationalResult(a, service, out, 2, 0)
}
func (m *AlgebraicRootCore) GetLowerOwnedRationalAtPrecision(a *Algebraic, service *unumeral.RationalService, out *unumeral.OwnedRational, precision uint32) error {
	return m.ownedRationalResult(a, service, out, 3, precision)
}
func (m *AlgebraicRootCore) GetUpperOwnedRationalAtPrecision(a *Algebraic, service *unumeral.RationalService, out *unumeral.OwnedRational, precision uint32) error {
	return m.ownedRationalResult(a, service, out, 4, precision)
}

// Concrete dispatch shares the identical raw/global local destruction sequence.
func (m *AlgebraicRootCore) ownedRationalResult(a *Algebraic, service *unumeral.RationalService, out *unumeral.OwnedRational, operation uint8, precision uint32) (e error) {
	if e = m.check(); e != nil {
		return e
	}
	s := &m.toOwnedRational
	z := m.config.Rationals.Integers()
	if s.active || z.MatchesCopyAllocationInvocation(&m.toRationalCopy) {
		return ErrAlgebraicPending
	}
	*s = algebraicOwnedRationalScope{service: service, tmp: unumeral.NewRational(), active: true, rawConversion: operation == 0}
	if e = s.bind(z); e != nil {
		*s = algebraicOwnedRationalScope{}
		return e
	}
	s.running = true
	defer func() {
		s.unbind()
		s.running = false
		if e == nil {
			e = m.cleanupToOwnedRational()
		}
	}()
	// The real raw overload owns its own predicate and copy receipts. This
	// wrapper rejected a pre-existing receipt before entering that child.
	if operation == 0 {
		e = m.ToRational(a, &s.tmp)
	} else {
		previous := m.bounds.invocation
		e = m.rationalBound(a, &s.tmp, operation == 2 || operation == 4, operation >= 3, precision)
		if m.bounds.invocation != previous {
			s.boundInvocation = m.bounds.invocation
		}
	}
	if e == nil {
		s.executing = true
		s.tmp2, e = service.NewFromRationalOwnerWithInvocation(m.config.Rationals, &s.tmp, &s.constructor)
		s.executing = false
		if e == nil {
			s.constructed = true
			s.executing = true
			e = service.CopyWithInvocation(out, &s.tmp2, &s.assignment)
			s.executing = false
		}
	}
	s.bodyCause = e
	if stop := m.ownedRationalChildSourceCause(s); stop != nil {
		s.sourceCause = stop
		if errors.Is(e, stop) {
			return e
		}
		return errors.Join(e, stop)
	}
	// Constructed locals receive source destruction in reverse declaration
	// order even when the body failed. A stopped suffix is never replayed.
	s.executing = true
	defer func() { s.executing = false }()
	if s.constructed {
		if cause := service.SourceDelValue(&s.tmp2); cause != nil {
			s.sourceCause = cause
			return errors.Join(e, cause)
		}
		s.tmp2Done = true
	}
	if cause := unumeral.SourceRationalDel(m.config.Rationals, &s.tmp); cause != nil {
		s.sourceCause = cause
		return errors.Join(e, cause)
	}
	s.tmpDone = true
	return e
}

// CleanupToOwnedRational releases conversion or bound wrapper locals and issued
// numerical receipts. Predicate work retains the raw overload's existing core
// continuation. Caller output remains independently owned.
func (m *AlgebraicRootCore) CleanupToOwnedRational() error {
	if m == nil || m.self != m || m.closed {
		return ErrAlgebraicOwner
	}
	return m.cleanupToOwnedRational()
}

func (m *AlgebraicRootCore) cleanupToOwnedRational() error {
	s := &m.toOwnedRational
	if s.running || s.executing {
		return ErrAlgebraicPending
	}
	if !s.active {
		return nil
	}
	if e := s.bind(m.config.Rationals.Integers()); e != nil {
		return e
	}
	complete := false
	s.executing = true
	defer func() {
		s.unbind()
		s.executing = false
		if complete {
			*s = algebraicOwnedRationalScope{}
		}
	}()
	if !s.assignmentEnded {
		if e := s.assignment.Cleanup(false); e != nil {
			return e
		}
	}
	if !s.constructorEnded {
		if e := s.constructor.Cleanup(false); e != nil {
			return e
		}
	}
	if s.constructed && !s.tmp2Done {
		if e := s.service.CleanupValue(&s.tmp2); e != nil {
			return e
		}
		s.tmp2Done = true
	}
	if s.boundInvocation != 0 && s.boundInvocation == m.bounds.invocation {
		if e := m.cleanupBounds(); e != nil {
			return e
		}
	}
	if s.rawConversion {
		if e := m.config.Rationals.Integers().CleanupCopyAllocationInvocation(&m.toRationalCopy); e != nil {
			return e
		}
	}
	if !s.tmpDone {
		if e := m.config.Rationals.Integers().CleanupRawNumeral(&s.tmp.Num); e != nil {
			return e
		}
		if e := m.config.Rationals.Integers().CleanupRawNumeral(&s.tmp.Den); e != nil {
			return e
		}
		s.tmpDone = true
	}
	if !s.assignmentEnded {
		if e := s.assignment.Cleanup(true); e != nil {
			return e
		}
		if e := s.assignment.EndContext(); e != nil {
			return e
		}
		s.assignmentEnded = true
	}
	if !s.constructorEnded {
		if e := s.constructor.Cleanup(true); e != nil {
			return e
		}
		if e := s.constructor.EndContext(); e != nil {
			return e
		}
		s.constructorEnded = true
	}
	complete = true
	return nil
}

// A stopped accepted child precedes the wrapper's older global/raw locals.
// Body failures with no source stop still execute those source destructors.
func (m *AlgebraicRootCore) ownedRationalChildSourceCause(s *algebraicOwnedRationalScope) error {
	if e := s.assignment.SourceReady(); e != nil {
		return e
	}
	if e := s.constructor.SourceReady(); e != nil {
		return e
	}
	if s.boundInvocation != 0 && s.boundInvocation == m.bounds.invocation {
		return m.bounds.sourceStop
	}
	return nil
}
