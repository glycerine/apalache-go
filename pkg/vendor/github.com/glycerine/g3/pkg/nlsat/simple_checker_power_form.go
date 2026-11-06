package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:404-441,584-610.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type simplePowerForm struct {
	variable, degree uint32
	sign             simpleSign
	a, c, boundary   polynomial.Algebraic
	thresholdSign    int
}

func (f *simplePowerForm) close(am *polynomial.AlgebraicManager) error {
	return errors.Join(am.Del(&f.boundary), am.Del(&f.c), am.Del(&f.a))
}

func (s *Solver) simplePowerLiteral(literal sat.Literal) (form simplePowerForm, supported bool, err error) {
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
	var ai, ci unumeral.Int
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
			ci = coefficient
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
		if power.Degree <= 1 {
			return form, false, nil
		}
		form.variable, form.degree = power.Var, power.Degree
		ai = coefficient
		aSet = true
	}
	if !aSet {
		return form, false, nil
	}
	if !cSet {
		ci = unumeral.NewInt(0)
	}
	form.sign, supported = simpleLiteralSign(atom.kind, literal.Sign())
	if !supported {
		return form, false, nil
	}
	z, err := core.IntegerManager()
	if err != nil {
		return form, false, err
	}
	neg, err := z.IsNeg(&ai)
	if err != nil {
		return form, false, err
	}
	if neg {
		form.sign = form.sign.flipped()
	}
	am := s.config.Algebraic
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, form.close(am))
		}
	}()
	if err = am.SetInteger(&form.a, &ai); err != nil {
		return form, false, err
	}
	if err = am.SetInteger(&form.c, &ci); err != nil {
		return form, false, err
	}
	if err = am.Div(&form.c, &form.a, &form.boundary); err != nil {
		return form, false, err
	}
	if err = am.Neg(&form.boundary); err != nil {
		return form, false, err
	}
	zero := polynomial.Algebraic{}
	order, err := am.Compare(&form.boundary, &zero)
	if err != nil {
		return form, false, err
	}
	form.thresholdSign = order
	if form.degree&1 == 1 || order >= 0 {
		if err = am.Root(&form.boundary, form.degree, &form.boundary); err != nil {
			return form, false, err
		}
	}
	keep = true
	return form, true, nil
}
