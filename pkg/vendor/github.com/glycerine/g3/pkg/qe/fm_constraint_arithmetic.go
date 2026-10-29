package qe

// Source: src/qe/qe_lite.cpp fm::fm::mk_int / normalize_coeffs.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (s *fmStore) makeIntegral(scope *fmScope, as []*numeral.OwnedRational, c *numeral.OwnedRational) error {
	n := s.recognizer.manager.NumeralService()
	l, err := scope.keep(n.Denominator(c))
	if err != nil {
		return err
	}
	for _, a := range as {
		d, err := scope.keep(n.Denominator(a))
		if err != nil {
			return err
		}
		l, err = scope.keep(n.LCM(l, d))
		if err != nil {
			return err
		}
	}
	one, err := n.IsOne(l)
	if err != nil || one {
		return err
	}
	if err = n.MulAssign(c, l); err != nil {
		return err
	}
	for _, a := range as {
		if err = n.MulAssign(a, l); err != nil {
			return err
		}
	}
	return nil
}

func (s *fmStore) normalize(c *fmConstraint) (err error) {
	scope, err := s.begin()
	if err != nil {
		return err
	}
	defer s.finish(scope, &err)
	if c == nil || c.owner != s || c.closed {
		return ast.ErrASTDomain
	}
	return s.normalizeCore(scope, c)
}
func (s *fmStore) normalizeCore(scope *fmScope, c *fmConstraint) error {
	if len(c.variables) == 0 {
		return nil
	}
	n := s.recognizer.manager.NumeralService()
	g, err := scope.keep(n.Abs(&c.constant))
	if err != nil {
		return err
	}
	for i := range c.coefficients {
		one, err := n.IsOne(g)
		if err != nil {
			return err
		}
		if one {
			break
		}
		a := &c.coefficients[i]
		positive, err := n.IsPos(a)
		if err != nil {
			return err
		}
		if !positive {
			a, err = scope.keep(n.Neg(a))
			if err != nil {
				return err
			}
		}
		g, err = scope.keep(n.GCD(a, g))
		if err != nil {
			return err
		}
	}
	one, err := n.IsOne(g)
	if err != nil || one {
		return err
	}
	if err = n.DivAssign(&c.constant, g); err != nil {
		return err
	}
	for i := range c.coefficients {
		if err = n.DivAssign(&c.coefficients[i], g); err != nil {
			return err
		}
	}
	return nil
}

func (s *fmStore) integer(h ast.ASTHandle) (bool, error) {
	m := s.recognizer.manager
	sort, err := m.GetSort(h)
	if err != nil {
		return false, err
	}
	i, err := m.MkFamilySort(s.recognizer.arithmetic, 1, nil)
	return sort == i, err
}
func (s *fmStore) allInteger(c *fmConstraint) bool {
	for _, x := range c.variables {
		if !s.variables[x].integer {
			return false
		}
	}
	return true
}
func (s *fmStore) falseConstraint(c *fmConstraint) (bool, error) {
	if len(c.literals) != 0 || len(c.variables) != 0 {
		return false, nil
	}
	n := s.recognizer.manager.NumeralService()
	negative, err := n.IsNeg(&c.constant)
	if err != nil || negative {
		return negative, err
	}
	if !c.strict {
		return false, nil
	}
	return n.IsZero(&c.constant)
}
