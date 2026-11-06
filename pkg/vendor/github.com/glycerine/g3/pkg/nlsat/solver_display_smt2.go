package nlsat

// Source: src/nlsat/nlsat_solver.cpp:3040-3055.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
)

// DisplaySMT2 emits the source debugging script for original clauses. If a
// root atom is reached it returns ErrSMT2RootAtom at the native unimplemented
// boundary, retaining all bytes written before that point.
func (s *Solver) DisplaySMT2(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if err := s.displaySMT2BooleanDeclarations(out); err != nil {
		return err
	}
	if err := s.displaySMT2ArithmeticDeclarations(out); err != nil {
		return err
	}
	if err := displayWriteString(out, "(assert (and true\n"); err != nil {
		return err
	}
	for _, clause := range s.clauses.originals {
		if err := s.displaySMT2Clause(out, clause); err != nil {
			return err
		}
		if err := displayWriteString(out, "\n"); err != nil {
			return err
		}
	}
	return displayWriteString(out, "))\n(check-sat)\n")
}
