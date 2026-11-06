package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2629-2815.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) displayMathematicaInequality(out io.Writer, atom *InequalityAtom) error {
	if atom == nil || !atom.IsInequality() {
		return arena.ErrConfig
	}
	for i := uint32(0); i < atom.Size(); i++ {
		if i != 0 {
			if err := displayWriteString(out, "*"); err != nil {
				return err
			}
		}
		even, err := atom.IsEven(i)
		if err != nil {
			return err
		}
		if atom.Size() > 1 {
			if err = displayWriteString(out, "("); err != nil {
				return err
			}
		}
		if even {
			if err = displayWriteString(out, "("); err != nil {
				return err
			}
		}
		p, err := atom.Polynomial(i)
		if err != nil {
			return err
		}
		if err = s.displayMathematicaPolynomial(out, p, nil); err != nil {
			return err
		}
		if even {
			if err = displayWriteString(out, "^2)"); err != nil {
				return err
			}
		}
		if atom.Size() > 1 {
			if err = displayWriteString(out, ")"); err != nil {
				return err
			}
		}
	}
	var relation string
	switch atom.Kind() {
	case AtomLT:
		relation = " < 0"
	case AtomGT:
		relation = " > 0"
	case AtomEQ:
		relation = " == 0"
	default:
		return arena.ErrConfig
	}
	return displayWriteString(out, relation)
}

func (s *Solver) displayMathematicaRoot(out io.Writer, atom *RootAtom) error {
	if atom == nil || !atom.IsRoot() {
		return arena.ErrConfig
	}
	if err := displayDefaultVariable(out, atom.Variable()); err != nil {
		return err
	}
	var relation string
	switch atom.Kind() {
	case AtomRootLT:
		relation = " < "
	case AtomRootGT:
		relation = " > "
	case AtomRootLE:
		relation = " <= "
	case AtomRootGE:
		relation = " >= "
	case AtomRootEQ:
		relation = " == "
	default:
		return arena.ErrConfig
	}
	if err := displayWriteString(out, relation+"Root["); err != nil {
		return err
	}
	variable := atom.Variable()
	if err := s.displayMathematicaPolynomial(out, atom.Polynomial(), &variable); err != nil {
		return err
	}
	return displayWriteString(out, " &, "+strconv.FormatUint(uint64(atom.Index()), 10)+"]")
}

func (s *Solver) displayMathematicaAtom(out io.Writer, atom ArithmeticAtom) error {
	switch atom := atom.(type) {
	case *InequalityAtom:
		return s.displayMathematicaInequality(out, atom)
	case *RootAtom:
		return s.displayMathematicaRoot(out, atom)
	default:
		return arena.ErrConfig
	}
}

func (s *Solver) DisplayMathematicaAtom(out io.Writer, atom ArithmeticAtom) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayMathematicaAtom(out, atom)
}

func (s *Solver) displayMathematicaBooleanAtom(out io.Writer, variable sat.BoolVar) error {
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return arena.ErrBounds
	}
	if variable == TrueBooleanVariable {
		return displayWriteString(out, "(0 < 1)")
	}
	atom := s.atoms.bools.atoms[variable]
	if atom == nil {
		return displayWriteString(out, "b"+strconv.FormatUint(uint64(variable), 10))
	}
	return s.displayMathematicaAtom(out, atom)
}

func (s *Solver) displayMathematicaLiteral(out io.Writer, literal sat.Literal) error {
	variable := literal.Variable()
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return arena.ErrBounds
	}
	if !literal.Sign() {
		return s.displayMathematicaBooleanAtom(out, variable)
	}
	if err := displayWriteString(out, "!"); err != nil {
		return err
	}
	interpreted := s.atoms.bools.atoms[variable] != nil
	if interpreted {
		if err := displayWriteString(out, "("); err != nil {
			return err
		}
	}
	if err := s.displayMathematicaBooleanAtom(out, variable); err != nil {
		return err
	}
	if interpreted {
		return displayWriteString(out, ")")
	}
	return nil
}

func (s *Solver) DisplayMathematicaLiteral(out io.Writer, literal sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayMathematicaLiteral(out, literal)
}
