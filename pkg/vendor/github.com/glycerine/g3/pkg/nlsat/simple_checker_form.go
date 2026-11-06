package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:584-610.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type simpleLinearForm struct {
	variable uint32
	sign     simpleSign
	boundary polynomial.Algebraic
	active   bool
}

func (f *simpleLinearForm) close(am *polynomial.AlgebraicManager) error { return am.Del(&f.boundary) }

func (s *Solver) simpleLinearLiteral(literal sat.Literal) (form simpleLinearForm, supported bool, err error) {
	raw, err := s.clauses.literalAtom(literal)
	if err != nil || raw == nil {
		return form, false, err
	}
	atom, ok := raw.(*InequalityAtom)
	if !ok || atom.Size() != 1 {
		return form, false, nil
	}
	p, err := atom.Polynomial(0)
	if err != nil {
		return form, false, err
	}
	core := s.atoms.ineq.core
	info, err := core.Info(p)
	if err != nil {
		return form, false, err
	}
	if info.Size < 1 || info.Size > 2 {
		return form, false, nil
	}
	mm, err := core.MonomialManager()
	if err != nil {
		return form, false, err
	}
	var a, c unumeral.Int
	aSet, cSet := false, false
	for i := uint32(0); i < info.Size; i++ {
		term, cause := core.TermMonomial(p, i)
		if cause != nil {
			return form, false, cause
		}
		ti, cause := mm.Info(term)
		if cause != nil {
			return form, false, cause
		}
		coefficient, cause := core.Coefficient(p, i)
		if cause != nil {
			return form, false, cause
		}
		if ti.Size == 0 {
			c = coefficient
			cSet = true
			continue
		}
		if ti.Size != 1 || aSet {
			return form, false, nil
		}
		power, cause := mm.ReadPower(term, 0)
		if cause != nil {
			return form, false, cause
		}
		if power.Degree != 1 {
			return form, false, nil
		}
		form.variable = power.Var
		a = coefficient
		aSet = true
	}
	if !aSet {
		return form, false, nil
	}
	if !cSet {
		c = unumeral.NewInt(0)
	}
	form.sign, supported = simpleLiteralSign(atom.kind, literal.Sign())
	if !supported {
		return form, false, nil
	}
	z, err := core.IntegerManager()
	if err != nil {
		return form, false, err
	}
	negative, err := z.IsNeg(&a)
	if err != nil {
		return form, false, err
	}
	if negative {
		form.sign = form.sign.flipped()
	}
	am := s.config.Algebraic
	var av, cv polynomial.Algebraic
	defer func() {
		err = errors.Join(err, am.Del(&cv), am.Del(&av))
		if err != nil {
			err = errors.Join(err, form.close(am))
		}
	}()
	if err = am.SetInteger(&av, &a); err != nil {
		return form, false, err
	}
	if err = am.SetInteger(&cv, &c); err != nil {
		return form, false, err
	}
	if err = am.Div(&cv, &av, &form.boundary); err != nil {
		return form, false, err
	}
	if err = am.Neg(&form.boundary); err != nil {
		return form, false, err
	}
	form.active = true
	return form, true, nil
}
