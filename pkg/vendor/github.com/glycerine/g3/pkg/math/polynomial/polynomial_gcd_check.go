package polynomial

// Source: src/math/polynomial/polynomial.cpp:4475-4493; src/util/debug.h;
// src/util/trace.h.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func polynomialMGCDCheckEnabled() (bool, error) {
	if !polynomialSourceDebug {
		return false, nil
	}
	return diagnostic.BorrowProcessDebugContext().IsDebugEnabled("mgcd_check")
}

func (m *MultivariatePolynomialManager) traceMGCDCheckMismatch(u, v, first, second Polynomial) error {
	c := m.core
	tags := c.monomials.monomialTraceTags()
	if !tags.ShouldTrace("mgcd_bug") {
		return nil
	}
	equal, e := c.Equal(first, second)
	if e != nil || equal {
		return e
	}
	var formatErr error
	_ = tags.EmitConditional("mgcd_bug", "gcd", "src/math/polynomial/polynomial.cpp", 4491,
		func() bool { return true }, func(w io.Writer) error {
			sink := &polynomialTraceSink{writer: w}
			labels := []string{"u: ", "\nv: ", "\nr1: ", "\nr2: "}
			values := []Polynomial{u, v, first, second}
			for i := range values {
				_ = sink.WriteLiteral(labels[i])
				formatErr = c.displayIntoCore(values[i], sink, false)
				if formatErr != nil {
					return formatErr
				}
			}
			return sink.WriteLiteral("\n")
		})
	return formatErr
}

// The two original references belong to the source try block, including when
// its first debug tag is disabled. They die before sparse-failure fallback.
func (m *MultivariatePolynomialManager) modGCDWithDebugCheck(u, v Polynomial, x uint32, r *polynomialReference, s *polynomialGCDFunctionScope) (err error) {
	frontier := s.depth
	defer func() { err = errors.Join(err, m.finishGCDLocalsTo(s, frontier)) }()
	if polynomialSourceDebug {
		s.constructReference(m.core, 0)
		s.constructReference(m.core, 1)
	}
	check, e := polynomialMGCDCheckEnabled()
	if e != nil {
		return e
	}
	if check {
		if e = s.refs[0].root.assign(m.core, u); e != nil {
			return e
		}
		if e = s.refs[1].root.assign(m.core, v); e != nil {
			return e
		}
	}
	if e = m.modGCD(u, v, &s.powers[0], &s.powers[1], r); e != nil {
		return e
	}
	// The source asks again after mod_gcd; cooperation may change this tag.
	check, e = polynomialMGCDCheckEnabled()
	if e != nil {
		return e
	}
	if !check {
		return nil
	}
	return m.independentModGCDCheck(x, r, s)
}

func (m *MultivariatePolynomialManager) independentModGCDCheck(x uint32, r *polynomialReference, s *polynomialGCDFunctionScope) (err error) {
	frontier := s.depth
	s.constructReference(m.core, 2)
	defer func() { err = errors.Join(err, m.finishGCDLocalsTo(s, frontier)) }()
	previous := m.usePRSGCD
	m.usePRSGCD = false
	// flet is younger than r2: restore the option before r2 destruction.
	defer func() { m.usePRSGCD = previous }()
	if e := m.gcdPRS(s.refs[0].root.value, s.refs[1].root.value, x, &s.refs[2].root); e != nil {
		return e
	}
	if e := m.traceMGCDCheckMismatch(s.refs[0].root.value, s.refs[1].root.value, r.value, s.refs[2].root.value); e != nil {
		return e
	}
	return polynomialGCDDispatchSourceAssert(4492, "eq(r, r2)", func() (bool, error) { return m.core.Equal(r.value, s.refs[2].root.value) })
}
