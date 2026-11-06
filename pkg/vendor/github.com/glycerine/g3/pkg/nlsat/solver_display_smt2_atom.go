package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2652-2835.

import (
	"errors"
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
	"github.com/glycerine/g3/pkg/sat"
)

// ErrSMT2RootAtom preserves the source display_smt2(root_atom) stop. The C++
// implementation reaches NOT_IMPLEMENTED_YET before writing any root text.
var ErrSMT2RootAtom = errors.New("nlsat: SMT2 root-atom display is not implemented")

func (s *Solver) displaySMT2Polynomial(out io.Writer, p polynomial.Polynomial) error {
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	sink, err := newSolverDisplaySink(out, displayDefaultVariable)
	if err != nil {
		return err
	}
	if err = core.DisplaySMT2Into(p, sink); err != nil {
		return err
	}
	return sink.err
}

func (s *Solver) displaySMT2Inequality(out io.Writer, atom *InequalityAtom) error {
	if atom == nil || !atom.IsInequality() {
		return arena.ErrConfig
	}
	var prefix string
	switch atom.Kind() {
	case AtomLT:
		prefix = "(< "
	case AtomGT:
		prefix = "(> "
	case AtomEQ:
		prefix = "(= "
	default:
		return arena.ErrConfig
	}
	if err := displayWriteString(out, prefix); err != nil {
		return err
	}
	if atom.Size() > 1 {
		if err := displayWriteString(out, "(* "); err != nil {
			return err
		}
	}
	for i := uint32(0); i < atom.Size(); i++ {
		if i != 0 {
			if err := displayWriteString(out, " "); err != nil {
				return err
			}
		}
		even, err := atom.IsEven(i)
		if err != nil {
			return err
		}
		p, err := atom.Polynomial(i)
		if err != nil {
			return err
		}
		if even {
			if err = displayWriteString(out, "(* "); err != nil {
				return err
			}
			if err = s.displaySMT2Polynomial(out, p); err != nil {
				return err
			}
			if err = displayWriteString(out, " "); err != nil {
				return err
			}
			if err = s.displaySMT2Polynomial(out, p); err != nil {
				return err
			}
			if err = displayWriteString(out, ")"); err != nil {
				return err
			}
		} else if err = s.displaySMT2Polynomial(out, p); err != nil {
			return err
		}
	}
	if atom.Size() > 1 {
		if err := displayWriteString(out, ")"); err != nil {
			return err
		}
	}
	return displayWriteString(out, " 0)")
}

func (s *Solver) displaySMT2Atom(out io.Writer, atom ArithmeticAtom) error {
	switch atom := atom.(type) {
	case *InequalityAtom:
		return s.displaySMT2Inequality(out, atom)
	case *RootAtom:
		if atom == nil {
			return arena.ErrConfig
		}
		return ErrSMT2RootAtom
	default:
		return arena.ErrConfig
	}
}

func (s *Solver) DisplaySMT2Atom(out io.Writer, atom ArithmeticAtom) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displaySMT2Atom(out, atom)
}

func (s *Solver) displaySMT2BooleanAtom(out io.Writer, variable sat.BoolVar) error {
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
	return s.displaySMT2Atom(out, atom)
}

func (s *Solver) displaySMT2Literal(out io.Writer, literal sat.Literal) error {
	variable := literal.Variable()
	if uint64(variable) >= uint64(len(s.atoms.bools.atoms)) || s.atoms.bools.dead[variable] {
		return arena.ErrBounds
	}
	if literal.Sign() {
		if err := displayWriteString(out, "(not "); err != nil {
			return err
		}
		if err := s.displaySMT2BooleanAtom(out, variable); err != nil {
			return err
		}
		return displayWriteString(out, ")")
	}
	return s.displaySMT2BooleanAtom(out, variable)
}

func (s *Solver) DisplaySMT2Literal(out io.Writer, literal sat.Literal) error {
	if err := s.open(); err != nil {
		return err
	}
	if out == nil {
		return arena.ErrConfig
	}
	return s.displaySMT2Literal(out, literal)
}
