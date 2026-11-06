package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2585-2603,2911-2962.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) displayMathematicaAssignment(out io.Writer) (bool, error) {
	first := true
	for variable := range s.clauses.vars.isInt {
		assigned, err := s.assignment.Contains(uint32(variable))
		if err != nil {
			return !first, err
		}
		if !assigned {
			continue
		}
		if !first {
			if err = displayWriteString(out, " && "); err != nil {
				return true, err
			}
		}
		first = false
		if err = displayWriteString(out, "x"+strconv.Itoa(variable)+" == "); err != nil {
			return true, err
		}
		value, err := s.assignment.Value(uint32(variable))
		if err != nil {
			return true, err
		}
		sink, err := newSolverMathematicaSink(out, nil)
		if err != nil {
			return true, err
		}
		if err = s.config.Algebraic.DisplayMathematicaInto(sink, value); err != nil {
			return true, err
		}
		if sink.err != nil {
			return true, sink.err
		}
	}
	return !first, nil
}

// DisplayMathematicaAssignment emits only assigned arithmetic variables and
// reports whether at least one conjunct was written.
func (s *Solver) DisplayMathematicaAssignment(out io.Writer) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if out == nil {
		return false, arena.ErrConfig
	}
	return s.displayMathematicaAssignment(out)
}

// DisplayMathematicaLemma emits the source Resolve/ForAll debugging formula.
// When requested, the current arithmetic assignment guards the disjunction.
func (s *Solver) DisplayMathematicaLemma(out io.Writer, literals []sat.Literal, includeAssignment bool) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if err := displayWriteString(out, "Resolve[ForAll[{"); err != nil {
		return err
	}
	for variable := range s.clauses.vars.isInt {
		if variable != 0 {
			if err := displayWriteString(out, ", "); err != nil {
				return err
			}
		}
		if err := displayWriteString(out, "x"+strconv.Itoa(variable)); err != nil {
			return err
		}
	}
	if err := displayWriteString(out, "}, "); err != nil {
		return err
	}
	if includeAssignment {
		if err := displayWriteString(out, "!("); err != nil {
			return err
		}
		wrote, err := s.displayMathematicaAssignment(out)
		if err != nil {
			return err
		}
		if !wrote {
			if err = displayWriteString(out, "0 < 1"); err != nil {
				return err
			}
		}
		if err = displayWriteString(out, ") || "); err != nil {
			return err
		}
	}
	for i, literal := range literals {
		if i != 0 {
			if err := displayWriteString(out, " || "); err != nil {
				return err
			}
		}
		if err := s.displayMathematicaLiteral(out, literal); err != nil {
			return err
		}
	}
	return displayWriteString(out, "], Reals]\n")
}
