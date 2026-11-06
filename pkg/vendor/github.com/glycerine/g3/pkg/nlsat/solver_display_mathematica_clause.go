package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2898-2991.

import (
	"io"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) displayMathematicaClause(out io.Writer, clause *Clause) error {
	if clause == nil || s.clauses.owned[clause] == nil {
		return ErrAtomCacheOwner
	}
	if err := displayWriteString(out, "("); err != nil {
		return err
	}
	for i, literal := range clause.Literals() {
		if i != 0 {
			if err := displayWriteString(out, " || "); err != nil {
				return err
			}
		}
		if err := s.displayMathematicaLiteral(out, literal); err != nil {
			return err
		}
	}
	return displayWriteString(out, ")")
}

func (s *Solver) DisplayMathematicaClause(out io.Writer, clause *Clause) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayMathematicaClause(out, clause)
}

func (s *Solver) displayMathematicaClauses(out io.Writer, clauses []*Clause) error {
	for i, clause := range clauses {
		if i != 0 {
			if err := displayWriteString(out, ",\n"); err != nil {
				return err
			}
		}
		if err := displayWriteString(out, " "); err != nil {
			return err
		}
		if err := s.displayMathematicaClause(out, clause); err != nil {
			return err
		}
	}
	return nil
}

// DisplayMathematica emits the original-clause collection. Learned clauses are
// intentionally absent, matching the source debugging view.
func (s *Solver) DisplayMathematica(out io.Writer) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	if err := displayWriteString(out, "{\n"); err != nil {
		return err
	}
	if err := s.displayMathematicaClauses(out, s.clauses.originals); err != nil {
		return err
	}
	return displayWriteString(out, "}\n")
}

// DisplayMathematicaLiterals prints the clause-body disjunction without an
// outer clause pair; this is the body used by lemma generation.
func (s *Solver) DisplayMathematicaLiterals(out io.Writer, literals []sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
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
	return nil
}
