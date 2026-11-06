package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:63-84,104-127.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

// SimplifyLiteralContent canonicalizes the integer content of single-factor
// inequality atoms, then rewrites original clauses through the canonical atom.
// Learned clauses are deliberately outside this source phase.
func (s *Solver) SimplifyLiteralContent() (count uint32, err error) {
	if err = s.open(); err != nil {
		return 0, err
	}
	replacements := make(map[sat.BoolVar]sat.Literal)
	// scoped_literal_vector in the source protects every recreated atom until
	// all old clauses have been replaced. Retain the same temporary ownership.
	var retained []sat.BoolVar
	defer func() {
		for i := len(retained) - 1; i >= 0; i-- {
			err = errors.Join(err, s.atoms.decRef(retained[i]))
		}
	}()
	atoms := append([]ArithmeticAtom(nil), s.atoms.bools.atoms...)
	for _, raw := range atoms {
		atom, ok := raw.(*InequalityAtom)
		if !ok || atom.Size() != 1 {
			continue
		}
		replacement, changed, cause := s.simplifyInequalityContent(atom)
		if cause != nil {
			return 0, cause
		}
		if cause = s.atoms.incRef(replacement.Variable()); cause != nil {
			return 0, cause
		}
		retained = append(retained, replacement.Variable())
		if changed {
			replacements[atom.boolVar] = replacement
		}
	}
	count, err = s.rewriteContentClauses(replacements)
	if err == nil {
		s.search.simplifications += count
	}
	return count, err
}

func (s *Solver) simplifyInequalityContent(atom *InequalityAtom) (literal sat.Literal, changed bool, err error) {
	p, err := atom.Polynomial(0)
	if err != nil {
		return literal, false, err
	}
	allInteger, err := s.polynomialVariablesAreInteger(p)
	if err != nil {
		return literal, false, err
	}
	relation := polynomial.GCDSimplifyEquality
	if allInteger {
		if atom.kind == AtomLT {
			relation = polynomial.GCDSimplifyLess
		}
		if atom.kind == AtomGT {
			relation = polynomial.GCDSimplifyGreater
		}
	}
	core := s.atoms.ineq.core
	ref, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return literal, false, err
	}
	defer func() { err = errors.Join(err, ref.Close()) }()
	if err = core.GCDSimplifyInto(p, relation, ref); err != nil {
		return literal, false, err
	}
	simplified, err := ref.Get()
	if err != nil {
		return literal, false, err
	}
	even, err := atom.IsEven(0)
	if err != nil {
		return literal, false, err
	}
	literal, err = s.atoms.mkInequalityLiteral(atom.kind, []polynomial.Polynomial{simplified}, []bool{even})
	if err != nil {
		return literal, false, err
	}
	return literal, literal.Variable() != atom.boolVar, nil
}
