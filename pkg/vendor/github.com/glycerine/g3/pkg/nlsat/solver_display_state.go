package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2554-2583,2965-3014.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (s *Solver) displayBooleanAssignment(out io.Writer) error {
	for variable, atom := range s.atoms.bools.atoms {
		if atom != nil || s.atoms.bools.dead[variable] {
			continue
		}
		value := s.atoms.bools.values[variable]
		if value == lbool.Undef {
			continue
		}
		text := "false"
		if value == lbool.True {
			text = "true"
		}
		if err := displayWriteString(out, "b"+strconv.Itoa(variable)+" -> "+text+"\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) displayNumericAssignment(out io.Writer) error {
	for variable := range s.clauses.vars.isInt {
		assigned, err := s.assignment.Contains(uint32(variable))
		if err != nil {
			return err
		}
		if !assigned {
			continue
		}
		if err = displayDefaultVariable(out, uint32(variable)); err != nil {
			return err
		}
		if err = displayWriteString(out, " -> "); err != nil {
			return err
		}
		value, err := s.assignment.Value(uint32(variable))
		if err != nil {
			return err
		}
		sink, err := newSolverDisplaySink(out, nil)
		if err != nil {
			return err
		}
		if err = s.config.Algebraic.DisplayDecimal(sink, value, 10); err != nil {
			return err
		}
		if sink.err != nil {
			return sink.err
		}
		if err = displayWriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) DisplayAssignment(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if err := s.displayBooleanAssignment(out); err != nil {
		return err
	}
	return s.displayNumericAssignment(out)
}

// DisplayVars prints the internal-to-display variable view. With the default
// source display procedure both sides are the current internal identifier.
func (s *Solver) DisplayVars(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	for variable := range s.clauses.vars.isInt {
		if err := displayWriteString(out, strconv.Itoa(variable)+" -> "); err != nil {
			return err
		}
		if err := displayDefaultVariable(out, uint32(variable)); err != nil {
			return err
		}
		if err := displayWriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// DisplayVariable implements solver::display(out, var) using the active
// default display procedure.
func (s *Solver) DisplayVariable(out io.Writer, variable uint32) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if uint64(variable) >= uint64(len(s.clauses.vars.isInt)) {
		return arena.ErrBounds
	}
	return displayDefaultVariable(out, variable)
}

// Display prints clauses, learned clauses, and the current Boolean/numerical
// assignment in the same order as solver::imp::display.
func (s *Solver) Display(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if err := s.displayClauseVector(out, s.clauses.originals); err != nil {
		return err
	}
	if len(s.clauses.learned) != 0 {
		if err := displayWriteString(out, "Lemmas:\n"); err != nil {
			return err
		}
		if err := s.displayClauseVector(out, s.clauses.learned); err != nil {
			return err
		}
	}
	if err := s.displayBooleanAssignment(out); err != nil {
		return err
	}
	return s.displayNumericAssignment(out)
}
