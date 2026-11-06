package nlsat

// Source: src/nlsat/nlsat_solver.cpp:578-587; nlsat_simplify.cpp:711-727.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) fourierMotzkinSubstitutedLiteral(kind AtomKind, p polynomial.Polynomial, even bool) (literal sat.Literal, err error) {
	core := s.atoms.ineq.core
	ref, err := polynomial.NewPolynomialRef(core)
	if err != nil {
		return literal, err
	}
	defer func() { err = errors.Join(err, ref.Close()) }()
	relation := polynomial.GCDSimplifyEquality
	allInteger, err := s.polynomialVariablesAreInteger(p)
	if err != nil {
		return literal, err
	}
	if allInteger {
		if kind == AtomLT {
			relation = polynomial.GCDSimplifyLess
		}
		if kind == AtomGT {
			relation = polynomial.GCDSimplifyGreater
		}
	}
	if err = core.GCDSimplifyInto(p, relation, ref); err != nil {
		return literal, err
	}
	p, err = ref.Get()
	if err != nil {
		return literal, err
	}
	return s.MkInequality(kind, []polynomial.Polynomial{p}, []bool{even}, false)
}
