package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:423-478.

import "github.com/glycerine/g3/pkg/sat"

func inequalityRelation(literal sat.Literal, atom *InequalityAtom) (lt, le, gt, ge bool) {
	lt = !literal.Sign() && atom.Kind() == AtomLT
	le = literal.Sign() && atom.Kind() == AtomGT
	gt = !literal.Sign() && atom.Kind() == AtomGT
	ge = literal.Sign() && atom.Kind() == AtomLT
	return
}

func (s *subsumptionSimplifier) literalSubsumes(left, right sat.Literal) (bool, error) {
	if left == right {
		return true, nil
	}
	leftAtom, err := s.solver.clauses.literalAtom(left)
	if err != nil {
		return false, err
	}
	rightAtom, err := s.solver.clauses.literalAtom(right)
	if err != nil {
		return false, err
	}
	first, firstOK := leftAtom.(*InequalityAtom)
	second, secondOK := rightAtom.(*InequalityAtom)
	if !firstOK || !secondOK || first == nil || second == nil || first.Size() != second.Size() {
		return false, nil
	}
	lt1, le1, gt1, ge1 := inequalityRelation(left, first)
	lt2, le2, gt2, ge2 := inequalityRelation(right, second)
	checkGE := lt1 && (lt2 || le2) || le1 && le2
	checkLE := gt1 && (gt2 || ge2) || ge1 && ge2
	if !checkGE && !checkLE {
		return false, nil
	}
	core, err := s.solver.config.Polynomial.ArithmeticCore()
	if err != nil {
		return false, err
	}
	for i := uint32(0); i < first.Size(); i++ {
		leftPolynomial, cause := first.Polynomial(i)
		if cause != nil {
			return false, cause
		}
		rightPolynomial, cause := second.Polynomial(i)
		if cause != nil {
			return false, cause
		}
		if checkLE {
			leftPolynomial, rightPolynomial = rightPolynomial, leftPolynomial
		}
		proved, cause := core.GreaterEqual(leftPolynomial, rightPolynomial)
		if cause != nil || !proved {
			return false, cause
		}
	}
	return true, nil
}
