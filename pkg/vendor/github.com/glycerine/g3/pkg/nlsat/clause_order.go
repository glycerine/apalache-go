package nlsat

// Source: src/nlsat/nlsat_solver.cpp:300-387,660-694.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *clauseStore) literalAtom(l sat.Literal) (ArithmeticAtom, error) {
	b := l.Variable()
	if uint64(b) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[b] {
		return nil, arena.ErrBounds
	}
	return s.atoms.bools.atoms[b], nil
}
func (s *clauseStore) maxVar(lits []sat.Literal) (uint32, error) {
	x := uint32(NullVariable)
	for _, l := range lits {
		a, err := s.literalAtom(l)
		if err != nil {
			return NullVariable, err
		}
		if h := arithmeticAtomHeader(a); h != nil {
			y := h.maxVar
			if x == NullVariable || y > x {
				x = y
			}
		}
	}
	return x, nil
}
func clauseMaxBoolVar(lits []sat.Literal) sat.BoolVar {
	b := sat.NullBoolVar
	for _, l := range lits {
		if b == sat.NullBoolVar || l.Variable() > b {
			b = l.Variable()
		}
	}
	return b
}
func (s *clauseStore) atomDegree(a ArithmeticAtom) (uint32, error) {
	core := s.atoms.ineq.core
	switch a := a.(type) {
	case *InequalityAtom:
		max := uint32(0)
		for _, factor := range a.factors {
			d, err := core.Degree(factor.Polynomial, a.maxVar)
			if err != nil {
				return 0, err
			}
			if d > max {
				max = d
			}
		}
		return max, nil
	case *RootAtom:
		return core.Degree(a.polynomial, a.maxVar)
	}
	return 0, arena.ErrConfig
}
func (s *clauseStore) clauseDegree(lits []sat.Literal) (uint32, error) {
	x, err := s.maxVar(lits)
	if err != nil || x == NullVariable {
		return 0, err
	}
	max := uint32(0)
	for _, l := range lits {
		a, err := s.literalAtom(l)
		if err != nil {
			return 0, err
		}
		if a == nil {
			continue
		}
		// The source takes each atom's own maximal-variable degree; it
		// does not filter atoms to the clause's overall maximal variable.
		d, err := s.atomDegree(a)
		if err != nil {
			return 0, err
		}
		if d > max {
			max = d
		}
	}
	return max, nil
}
func (s *clauseStore) literalLess(l, r sat.Literal) (bool, error) {
	a1, err := s.literalAtom(l)
	if err != nil {
		return false, err
	}
	a2, err := s.literalAtom(r)
	if err != nil {
		return false, err
	}
	if a1 == nil && a2 == nil {
		return uint32(l) < uint32(r), nil
	}
	if a1 == nil {
		return true, nil
	}
	if a2 == nil {
		return false, nil
	}
	h1, h2 := arithmeticAtomHeader(a1), arithmeticAtomHeader(a2)
	if h1.maxVar != h2.maxVar {
		return h1.maxVar < h2.maxVar, nil
	}
	if err := atomAssert("nlsat_solver.cpp", 679, "x1 == x2", h1.maxVar == h2.maxVar); err != nil {
		return false, err
	}
	d1, err := s.atomDegree(a1)
	if err != nil {
		return false, err
	}
	d2, err := s.atomDegree(a2)
	if err != nil {
		return false, err
	}
	if d1 != d2 {
		return d1 < d2, nil
	}
	if h1.IsEquality() != h2.IsEquality() {
		return !h1.IsEquality(), nil
	}
	return uint32(l) < uint32(r), nil
}
