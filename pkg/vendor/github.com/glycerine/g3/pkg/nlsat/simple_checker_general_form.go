package nlsat

// Source: src/nlsat/nlsat_simple_checker.cpp:1384-1474.

import "github.com/glycerine/g3/pkg/sat"

type simpleGeneralForm struct {
	atom    *InequalityAtom
	literal sat.Literal
	sign    simpleSign
}

func (s *Solver) simpleGeneralLiteral(literal sat.Literal) (form simpleGeneralForm, supported bool, err error) {
	raw, err := s.clauses.literalAtom(literal)
	if err != nil || raw == nil {
		return form, false, err
	}
	atom, ok := raw.(*InequalityAtom)
	if !ok {
		return form, false, nil
	}
	form.atom, form.literal = atom, literal
	form.sign, supported = simpleLiteralSign(atom.kind, literal.Sign())
	if literal.Sign() && atom.kind == AtomEQ {
		return form, true, nil
	}
	return form, supported, nil
}
