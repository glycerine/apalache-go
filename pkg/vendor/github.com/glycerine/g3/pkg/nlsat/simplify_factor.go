package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:128-243.

import (
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

type factorSimplificationCandidate struct {
	clause     *Clause
	literal    sat.Literal
	kind       AtomKind
	polynomial [2]polynomial.Polynomial
	even       [2]bool
}

type factorSimplifier struct{ solver *Solver }

// SimplifyUnitFactors runs the retained split_factors phase over the original
// clause prefix. Newly published clauses are not revisited in the same phase.
func (s *Solver) SimplifyUnitFactors() (uint32, error) {
	if err := s.open(); err != nil {
		return 0, err
	}
	if err := s.search.initializeSearch(); err != nil {
		return 0, err
	}
	return (&factorSimplifier{solver: s}).run()
}

func (s *factorSimplifier) run() (uint32, error) {
	prefix := append([]*Clause(nil), s.solver.clauses.originals...)
	var count uint32
	for _, clause := range prefix {
		candidate, found, err := s.candidate(clause)
		if err != nil {
			return count, err
		}
		if !found {
			continue
		}
		clauses, err := s.replacement(candidate)
		if err != nil {
			return count, err
		}
		if err = s.publish(candidate, clauses); err != nil {
			return count, err
		}
		count++
		s.solver.search.simplifications++
	}
	return count, nil
}
