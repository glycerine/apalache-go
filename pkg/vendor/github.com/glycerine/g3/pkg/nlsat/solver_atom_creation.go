package nlsat

// Source: src/nlsat/nlsat_solver.cpp:548-599,3178-3188;
// src/nlsat/nlsat_solver.h:79-101.

import (
	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func validInequalityKind(kind AtomKind) bool {
	return kind == AtomLT || kind == AtomGT || kind == AtomEQ
}

// MkInequalityAtom creates or retrieves the canonical arithmetic atom and
// returns its Boolean variable. Unlike MkInequality, constant polynomials are
// retained as atoms, exactly as the source mk_ineq_atom entry point does.
func (s *Solver) MkInequalityAtom(kind AtomKind, factors []polynomial.Polynomial, even []bool) (sat.BoolVar, error) {
	if err := s.open(); err != nil {
		return sat.NullBoolVar, err
	}
	if !validInequalityKind(kind) {
		return sat.NullBoolVar, arena.ErrConfig
	}
	if len(factors) == 0 || len(factors) != len(even) {
		return sat.NullBoolVar, arena.ErrBounds
	}
	return s.atoms.mkInequality(kind, factors, even)
}

// MkInequalityLiteral is the literal-level source factory. An empty product is
// one, yielding false for LT/EQ and true for GT without allocating an atom.
func (s *Solver) MkInequalityLiteral(kind AtomKind, factors []polynomial.Polynomial, even []bool) (sat.Literal, error) {
	if err := s.open(); err != nil {
		return sat.DefaultLiteral(), err
	}
	if !validInequalityKind(kind) {
		return sat.DefaultLiteral(), arena.ErrConfig
	}
	if len(factors) != len(even) {
		return sat.DefaultLiteral(), arena.ErrBounds
	}
	return s.atoms.mkInequalityLiteral(kind, factors, even)
}

// MkRootAtom creates or retrieves the canonical root atom and returns its
// Boolean variable without applying a literal sign.
func (s *Solver) MkRootAtom(kind AtomKind, variable, index uint32, p polynomial.Polynomial) (sat.BoolVar, error) {
	if err := s.open(); err != nil {
		return sat.NullBoolVar, err
	}
	return s.atoms.mkRoot(kind, variable, index, p)
}
