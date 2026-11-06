package polynomial

// Source: src/math/polynomial/polynomial.h:1040-1047;
// src/math/polynomial/polynomial.cpp:3769-3865.

import (
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// This is an embedded lexical local, not a persistent modular manager.
// A failed constructor unwinds its completed member without restoring mode.

type polynomialScopedSetZP struct {
	owner                                      *MultivariatePolynomialManager
	p                                          unumeral.Int
	numeric                                    polynomialNumericRoots
	active, completed, memberEnded, oldModular bool
	stage                                      uint8
	bodyCause, sourceCause                     error
}

func (s *polynomialScopedSetZP) construct(m *MultivariatePolynomialManager, p *unumeral.Int) (err error) {
	if e := m.ready(); e != nil {
		return e
	}
	if s.active {
		return ErrPolynomialPending
	}
	old, e := m.core.modular.Modular()
	if e != nil {
		return e
	}
	*s = polynomialScopedSetZP{owner: m, active: true, oldModular: old}
	z := m.core.integer
	defer func() {
		if err == nil {
			s.completed = true
			return
		}
		s.bodyCause = err
		err = polynomialNewtonJoin(err, s.endMember())
	}()
	before, e := polynomialNumericBefore(z)
	if e != nil {
		return e
	}
	e = z.Copy(&s.p, m.core.modular.BorrowModulus())
	e = polynomialNewtonJoin(e, s.numeric.after(z, before))
	if e != nil {
		return e
	}
	before, e = polynomialNumericBefore(z)
	if e != nil {
		return e
	}
	e = m.core.modular.SetZP(p)
	return polynomialNewtonJoin(e, s.numeric.after(z, before))
}

func (s *polynomialScopedSetZP) endMember() error {
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if s.memberEnded {
		return nil
	}
	_, e := unumeral.SourceNumeralDel(s.owner.core.integer, &s.p)
	if e != nil {
		s.sourceCause = e
		return e
	}
	s.memberEnded = true
	return nil
}

func (s *polynomialScopedSetZP) sourceEnd() error {
	if !s.active {
		return nil
	}
	if s.sourceCause != nil {
		return s.sourceCause
	}
	if !s.completed {
		return s.endMember()
	}
	if s.stage == 0 {
		var e error
		if s.oldModular {
			z := s.owner.core.integer
			before, x := polynomialNumericBefore(z)
			if x != nil {
				e = x
			} else {
				e = s.owner.core.modular.SetZP(&s.p)
				e = polynomialNewtonJoin(e, s.numeric.after(z, before))
			}
		} else {
			e = s.owner.core.modular.SetZ()
		}
		if e != nil {
			s.sourceCause = e
			return e
		}
		s.stage = 1
	}
	if e := s.endMember(); e != nil {
		return e
	}
	pending := s.numeric.PendingObject().OwnerID() != 0
	for _, raw := range s.numeric.PendingAllocatorBlocks() {
		pending = pending || raw.OwnerID() != 0
	}
	if !pending {
		*s = polynomialScopedSetZP{}
	}
	return nil
}

// Owning abort acknowledges retained storage; it never repeats the class body.
func (s *polynomialScopedSetZP) cleanup() error {
	if !s.active {
		return nil
	}
	if s.owner == nil || s.owner.self != s.owner {
		return ErrPolynomialOwner
	}
	c := s.owner.core
	if !s.memberEnded {
		if e := c.deleteNumeral(&s.p); e != nil {
			return e
		}
		s.memberEnded = true
	}
	if e := s.numeric.cleanup(c.integer); e != nil {
		return e
	}
	*s = polynomialScopedSetZP{}
	return nil
}
