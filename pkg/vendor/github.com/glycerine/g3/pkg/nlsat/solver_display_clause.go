package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2837-3000.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) displayLiterals(out io.Writer, literals []sat.Literal, separator string) error {
	for i, literal := range literals {
		if i != 0 {
			if err := displayWriteString(out, separator); err != nil {
				return err
			}
		}
		if err := s.displayLiteral(out, literal); err != nil {
			return err
		}
	}
	return nil
}

// DisplayDisjunction prints literals in the syntax used inside native NLSAT
// clauses. An empty slice produces no bytes, as in the internal source overload.
func (s *Solver) DisplayDisjunction(out io.Writer, literals []sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayLiterals(out, literals, " or ")
}

// DisplayLiterals implements solver::display(out, n, ls): each independently
// displayed literal is followed by the source debug separator.
func (s *Solver) DisplayLiterals(out io.Writer, literals []sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	for _, literal := range literals {
		if err := s.displayLiteral(out, literal); err != nil {
			return err
		}
		if err := displayWriteString(out, ";  "); err != nil {
			return err
		}
	}
	return nil
}

func (s *Solver) displayClause(out io.Writer, clause *Clause) error {
	if clause == nil || s.clauses.owned[clause] == nil {
		return ErrAtomCacheOwner
	}
	if clause.Assumptions() != nil {
		// The source deliberately leaves the opaque assumption set blank.
		if err := displayWriteString(out, " |- "); err != nil {
			return err
		}
	}
	return s.displayLiterals(out, clause.Literals(), " or ")
}

func (s *Solver) DisplayClause(out io.Writer, clause *Clause) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayClause(out, clause)
}

func (s *Solver) displayClauseVector(out io.Writer, clauses []*Clause) error {
	for _, clause := range clauses {
		if err := s.displayClause(out, clause); err != nil {
			return err
		}
		if err := displayWriteString(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

// DisplayClauses prints original clauses followed by the learned-clause
// section. It excludes assignments; Display appends those too.
func (s *Solver) DisplayClauses(out io.Writer) error {
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
		return s.displayClauseVector(out, s.clauses.learned)
	}
	return nil
}

func (s *Solver) displayAbstractLiteral(out io.Writer, literal sat.Literal) error {
	variable := literal.Variable()
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return arena.ErrBounds
	}
	if literal.Sign() {
		if err := displayWriteString(out, "!"); err != nil {
			return err
		}
		if variable == TrueBooleanVariable {
			return displayWriteString(out, "true")
		}
	}
	return displayWriteString(out, "b"+strconv.FormatUint(uint64(variable), 10))
}

func (s *Solver) DisplayAbstractLiteral(out io.Writer, literal sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayAbstractLiteral(out, literal)
}

func (s *Solver) DisplayAbstractLiterals(out io.Writer, literals []sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	for i, literal := range literals {
		if i != 0 {
			if err := displayWriteString(out, " or "); err != nil {
				return err
			}
		}
		if err := s.displayAbstractLiteral(out, literal); err != nil {
			return err
		}
	}
	return nil
}
