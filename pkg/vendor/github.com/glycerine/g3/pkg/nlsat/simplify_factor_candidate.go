package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:134-151.

import "github.com/glycerine/g3/pkg/math/polynomial"

func (s *factorSimplifier) candidate(clause *Clause) (factorSimplificationCandidate, bool, error) {
	if clause == nil || clause.Size() != 1 || clause.IsLearned() {
		return factorSimplificationCandidate{}, false, nil
	}
	literal := clause.Literals()[0]
	atom, err := s.solver.clauses.literalAtom(literal)
	if err != nil {
		return factorSimplificationCandidate{}, false, err
	}
	inequality, ok := atom.(*InequalityAtom)
	if !ok || inequality == nil || inequality.Size() != 2 {
		return factorSimplificationCandidate{}, false, nil
	}
	first, err := inequality.Polynomial(0)
	if err != nil {
		return factorSimplificationCandidate{}, false, err
	}
	second, err := inequality.Polynomial(1)
	if err != nil {
		return factorSimplificationCandidate{}, false, err
	}
	firstEven, err := inequality.IsEven(0)
	if err != nil {
		return factorSimplificationCandidate{}, false, err
	}
	secondEven, err := inequality.IsEven(1)
	if err != nil {
		return factorSimplificationCandidate{}, false, err
	}
	return factorSimplificationCandidate{
		clause: clause, literal: literal, kind: inequality.Kind(),
		polynomial: [2]polynomial.Polynomial{first, second}, even: [2]bool{firstEven, secondEven},
	}, true, nil
}
