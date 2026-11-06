package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:688-720,954-1004.

import (
	"errors"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

type simpleSumTerm struct {
	variable, degree uint32
	coefficient      polynomial.Algebraic
}
type simpleSumForm struct {
	sign     simpleSign
	terms    []simpleSumTerm
	constant polynomial.Algebraic
}

func (f *simpleSumForm) close(am *polynomial.AlgebraicManager) (err error) {
	for i := len(f.terms) - 1; i >= 0; i-- {
		err = errors.Join(err, am.Del(&f.terms[i].coefficient))
	}
	return errors.Join(err, am.Del(&f.constant))
}
func (s *Solver) simpleSumLiteral(literal sat.Literal) (form simpleSumForm, supported bool, err error) {
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
	mm, err := core.MonomialManager()
	if err != nil {
		return form, false, err
	}
	am := s.config.Algebraic
	keep := false
	defer func() {
		if !keep {
			err = errors.Join(err, form.close(am))
		}
	}()
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
			if cause = am.SetInteger(&form.constant, &coefficient); cause != nil {
				return form, false, cause
			}
			continue
		}
		if ti.Size != 1 {
			return form, false, nil
		}
		power, cause := mm.ReadPower(term, 0)
		if cause != nil {
			return form, false, cause
		}
		entry := simpleSumTerm{variable: power.Var, degree: power.Degree}
		if cause = am.SetInteger(&entry.coefficient, &coefficient); cause != nil {
			return form, false, cause
		}
		form.terms = append(form.terms, entry)
	}
	if len(form.terms) < 2 {
		return form, false, nil
	}
	form.sign, supported = simpleLiteralSign(atom.kind, literal.Sign())
	if !supported {
		return form, false, nil
	}
	keep = true
	return form, true, nil
}
