package nlsat

// Source: src/nlsat/nlsat_simplify.cpp:651-659; nlsat_solver.cpp:578-587.

import (
	"errors"

	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) fourierMotzkinLiteral(p polynomial.Polynomial, strict bool) (literal sat.Literal, err error) {
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
		if strict {
			relation = polynomial.GCDSimplifyGreater
		} else {
			relation = polynomial.GCDSimplifyLess
		}
	}
	if err = core.GCDSimplifyInto(p, relation, ref); err != nil {
		return literal, err
	}
	p, err = ref.Get()
	if err != nil {
		return literal, err
	}
	kind := AtomLT
	if strict {
		kind = AtomGT
	}
	literal, err = s.MkInequality(kind, []polynomial.Polynomial{p}, []bool{false}, false)
	if err == nil && !strict {
		literal = literal.Negated()
	}
	return literal, err
}
