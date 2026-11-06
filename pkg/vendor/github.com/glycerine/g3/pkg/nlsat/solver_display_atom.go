package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2605-2840.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

func (s *Solver) displayPolynomial(out io.Writer, p polynomial.Polynomial, star bool) error {
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	sink, err := newSolverDisplaySink(out, displayDefaultVariable)
	if err != nil {
		return err
	}
	if err = core.DisplayInto(p, sink, star); err != nil {
		return err
	}
	return sink.err
}

func (s *Solver) displayInequalityAtom(out io.Writer, atom *InequalityAtom, star bool) error {
	if atom == nil || !atom.IsInequality() {
		return arena.ErrConfig
	}
	for i := uint32(0); i < atom.Size(); i++ {
		if star && i != 0 {
			if err := displayWriteString(out, "*"); err != nil {
				return err
			}
		}
		even, err := atom.IsEven(i)
		if err != nil {
			return err
		}
		parentheses := even || atom.Size() > 1
		if parentheses {
			if err = displayWriteString(out, "("); err != nil {
				return err
			}
		}
		p, err := atom.Polynomial(i)
		if err != nil {
			return err
		}
		if err = s.displayPolynomial(out, p, star); err != nil {
			return err
		}
		if parentheses {
			if err = displayWriteString(out, ")"); err != nil {
				return err
			}
		}
		if even {
			if err = displayWriteString(out, "^2"); err != nil {
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
		relation = " = 0"
	default:
		return arena.ErrConfig
	}
	return displayWriteString(out, relation)
}

func (s *Solver) displayRootAtom(out io.Writer, atom *RootAtom) error {
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
		relation = " = "
	default:
		return arena.ErrConfig
	}
	if err := displayWriteString(out, relation+"root["+strconv.FormatUint(uint64(atom.Index()), 10)+"]("); err != nil {
		return err
	}
	if err := s.displayPolynomial(out, atom.Polynomial(), false); err != nil {
		return err
	}
	return displayWriteString(out, ")")
}

// DisplayArithmeticAtom prints an inequality or root atom using the source's
// ordinary NLSAT syntax. The atom and its polynomials remain solver-owned.
func (s *Solver) DisplayArithmeticAtom(out io.Writer, atom ArithmeticAtom) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	switch atom := atom.(type) {
	case *InequalityAtom:
		return s.displayInequalityAtom(out, atom, false)
	case *RootAtom:
		return s.displayRootAtom(out, atom)
	default:
		return arena.ErrConfig
	}
}

func (s *Solver) displayBooleanAtom(out io.Writer, variable sat.BoolVar) error {
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return arena.ErrBounds
	}
	if variable == TrueBooleanVariable {
		return displayWriteString(out, "true")
	}
	atom := s.atoms.bools.atoms[variable]
	if atom == nil {
		return displayWriteString(out, "b"+strconv.FormatUint(uint64(variable), 10))
	}
	switch atom := atom.(type) {
	case *InequalityAtom:
		return s.displayInequalityAtom(out, atom, false)
	case *RootAtom:
		return s.displayRootAtom(out, atom)
	default:
		return arena.ErrConfig
	}
}

func (s *Solver) DisplayBooleanAtom(out io.Writer, variable sat.BoolVar) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayBooleanAtom(out, variable)
}

func (s *Solver) displayLiteral(out io.Writer, literal sat.Literal) error {
	variable := literal.Variable()
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return arena.ErrBounds
	}
	if !literal.Sign() {
		return s.displayBooleanAtom(out, variable)
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
	if err := s.displayBooleanAtom(out, variable); err != nil {
		return err
	}
	if interpreted {
		return displayWriteString(out, ")")
	}
	return nil
}

func (s *Solver) DisplayLiteral(out io.Writer, literal sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displayLiteral(out, literal)
}
