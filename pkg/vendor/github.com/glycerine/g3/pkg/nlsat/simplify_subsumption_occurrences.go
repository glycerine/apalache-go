package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:304-335.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

func (s *subsumptionSimplifier) collectPolynomialVariables(p polynomial.Polynomial, variables map[uint32]struct{}) error {
	core, err := s.solver.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	monomials, err := core.MonomialManager()
	if err != nil {
		return err
	}
	info, err := core.Info(p)
	if err != nil {
		return err
	}
	for i := uint32(0); i < info.Size; i++ {
		monomial, cause := core.TermMonomial(p, i)
		if cause != nil {
			return cause
		}
		monomialInfo, cause := monomials.Info(monomial)
		if cause != nil {
			return cause
		}
		for j := uint32(0); j < monomialInfo.Size; j++ {
			power, cause := monomials.ReadPower(monomial, j)
			if cause != nil {
				return cause
			}
			if uint64(power.Var) >= uint64(len(s.solver.clauses.vars.isInt)) {
				return arena.ErrBounds
			}
			variables[power.Var] = struct{}{}
		}
	}
	return nil
}

func (s *subsumptionSimplifier) computeOccurrences(clause *Clause) error {
	variables := make(map[uint32]struct{})
	for _, literal := range clause.Literals() {
		atom, err := s.solver.clauses.literalAtom(literal)
		if err != nil {
			return err
		}
		inequality, ok := atom.(*InequalityAtom)
		if !ok || inequality == nil {
			continue
		}
		for _, factor := range inequality.factors {
			if err = s.collectPolynomialVariables(factor.Polynomial, variables); err != nil {
				return err
			}
		}
	}
	var hash uint32
	for variable := range variables {
		for len(s.occurrences) <= int(variable) {
			s.occurrences = append(s.occurrences, nil)
		}
		s.occurrences[variable] = append(s.occurrences[variable], clause)
		hash |= uint32(1) << (variable % 32)
	}
	s.hashes[clause] = hash
	return nil
}
