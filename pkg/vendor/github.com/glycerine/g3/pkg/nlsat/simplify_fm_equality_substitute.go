package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:679-727,755-802.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) substituteFourierMotzkinEquality(clauses []*Clause, lower, upper *fourierMotzkinBound) (err error) {
	joined, err := s.assumptions.mkJoin(lower.clause.Assumptions(), upper.clause.Assumptions())
	if err != nil {
		return err
	}
	if err = s.assumptions.incRef(joined); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.assumptions.decRef(joined)) }()
	a, b, err := lower.values()
	if err != nil {
		return err
	}
	positive, err := s.fourierMotzkinCoefficientPositive(a)
	if err != nil {
		return err
	}
	core := s.atoms.ineq.core
	numerator, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return err
	}
	denominator, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		_ = numerator.Close()
		return err
	}
	defer func() { err = errors.Join(err, denominator.Close(), numerator.Close()) }()
	if positive {
		if err = denominator.Assign(a); err != nil {
			return err
		}
		negative, cause := core.Neg(b)
		if cause != nil {
			return cause
		}
		if err = numerator.Assign(negative); err != nil {
			return err
		}
	} else {
		negative, cause := core.Neg(a)
		if cause != nil {
			return cause
		}
		if err = denominator.Assign(negative); err != nil {
			return err
		}
		if err = numerator.Assign(b); err != nil {
			return err
		}
	}
	n, err := numerator.Get()
	if err != nil {
		return err
	}
	d, err := denominator.Get()
	if err != nil {
		return err
	}
	var replacements []struct {
		old         *Clause
		literals    []sat.Literal
		assumptions *AssumptionSet
		tautology   bool
	}
	seen := map[*Clause]bool{}
	for _, clause := range clauses {
		if seen[clause] || s.clauses.owned[clause] == nil {
			continue
		}
		seen[clause] = true
		if clause == lower.clause || clause == upper.clause {
			replacements = append(replacements, struct {
				old         *Clause
				literals    []sat.Literal
				assumptions *AssumptionSet
				tautology   bool
			}{old: clause})
			continue
		}
		entry := struct {
			old         *Clause
			literals    []sat.Literal
			assumptions *AssumptionSet
			tautology   bool
		}{old: clause}
		for _, literal := range clause.Literals() {
			replacement, cause := s.substituteFourierMotzkinLiteral(lower.variable, n, d, literal)
			if cause != nil {
				return cause
			}
			entry.literals = append(entry.literals, replacement)
			if replacement == TrueLiteral {
				entry.tautology = true
			}
		}
		entry.assumptions, err = s.assumptions.mkJoin(clause.Assumptions(), joined)
		if err != nil {
			return err
		}
		replacements = append(replacements, entry)
	}
	for _, entry := range replacements {
		if !entry.tautology && entry.old != lower.clause && entry.old != upper.clause {
			if len(entry.literals) == 0 {
				entry.literals = append(entry.literals, FalseLiteral)
			}
			if _, err = s.clauses.mkClause(entry.literals, false, entry.assumptions); err != nil {
				return err
			}
		}
	}
	for _, entry := range replacements {
		if s.clauses.owned[entry.old] != nil {
			if err = s.clauses.deleteClause(entry.old); err != nil {
				return err
			}
		}
	}
	return nil
}
