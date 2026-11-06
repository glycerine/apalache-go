package nlsat

// Source: src/nlsat/nlsat_solver.cpp:3016-3038.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
)

func (s *Solver) displaySMT2ArithmeticDeclarations(out io.Writer) error {
	for variable, integer := range s.clauses.vars.isInt {
		sort := "Real"
		if integer {
			sort = "Int"
		}
		if err := displayWriteString(out, "(declare-fun x"+strconv.Itoa(variable)+" () "+sort+")\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) DisplaySMT2ArithmeticDeclarations(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displaySMT2ArithmeticDeclarations(out)
}

func (s *Solver) displaySMT2BooleanDeclarations(out io.Writer) error {
	for variable, atom := range s.atoms.bools.atoms {
		if atom != nil {
			continue
		}
		if err := displayWriteString(out, "(declare-fun b"+strconv.Itoa(variable)+" () Bool)\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) DisplaySMT2BooleanDeclarations(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displaySMT2BooleanDeclarations(out)
}

func (s *Solver) DisplaySMT2Declarations(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if err := s.displaySMT2BooleanDeclarations(out); err != nil {
		return err
	}
	return s.displaySMT2ArithmeticDeclarations(out)
}
