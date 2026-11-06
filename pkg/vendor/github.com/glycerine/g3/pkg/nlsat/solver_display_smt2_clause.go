package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2850-2879.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) displaySMT2Literals(out io.Writer, literals []sat.Literal) error {
	switch len(literals) {
	case 0:
		return displayWriteString(out, "false")
	case 1:
		return s.displaySMT2Literal(out, literals[0])
	default:
		if err := displayWriteString(out, "(or"); err != nil {
			return err
		}
		for _, literal := range literals {
			if err := displayWriteString(out, " "); err != nil {
				return err
			}
			if err := s.displaySMT2Literal(out, literal); err != nil {
				return err
			}
		}
		return displayWriteString(out, ")")
	}
}

func (s *Solver) DisplaySMT2Literals(out io.Writer, literals []sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displaySMT2Literals(out, literals)
}

func (s *Solver) displaySMT2Clause(out io.Writer, clause *Clause) error {
	if clause == nil || s.clauses.owned[clause] == nil {
		return ErrAtomCacheOwner
	}
	return s.displaySMT2Literals(out, clause.Literals())
}

func (s *Solver) DisplaySMT2Clause(out io.Writer, clause *Clause) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displaySMT2Clause(out, clause)
}
