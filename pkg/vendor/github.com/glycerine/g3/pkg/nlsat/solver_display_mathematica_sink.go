package nlsat

// Source: src/nlsat/nlsat_solver.cpp:2585-2750.

import (
	"io"
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/math/polynomial"
)

// solverMathematicaSink supplies both the polynomial and exact-numeral stream
// callbacks used by the two native Mathematica display paths.
type solverMathematicaSink struct{ *solverDisplaySink }

func newSolverMathematicaSink(out io.Writer, variable func(io.Writer, uint32) error) (*solverMathematicaSink, error) {
	sink, err := newSolverDisplaySink(out, variable)
	if err != nil {
		return nil, err
	}
	return &solverMathematicaSink{solverDisplaySink: sink}, nil
}

func (s *solverMathematicaSink) AppendInt32(value int32) error {
	if s == nil || s.solverDisplaySink == nil {
		return arena.ErrConfig
	}
	return s.WriteLiteral(strconv.FormatInt(int64(value), 10))
}

func (s *solverMathematicaSink) AppendCString(value string) error {
	if s == nil || s.solverDisplaySink == nil {
		return arena.ErrConfig
	}
	return s.WriteLiteral(value)
}

func (s *solverMathematicaSink) AppendNumeralText(value string) error {
	if s == nil || s.solverDisplaySink == nil {
		return arena.ErrConfig
	}
	return s.WriteLiteral(value)
}

func (s *Solver) displayMathematicaPolynomial(out io.Writer, p polynomial.Polynomial, rootVariable *uint32) error {
	core, err := s.config.Polynomial.ArithmeticCore()
	if err != nil {
		return err
	}
	variable := displayDefaultVariable
	if rootVariable != nil {
		x := *rootVariable
		variable = func(out io.Writer, current uint32) error {
			if current == x {
				return displayWriteString(out, "#1")
			}
			return displayDefaultVariable(out, current)
		}
	}
	sink, err := newSolverMathematicaSink(out, variable)
	if err != nil {
		return err
	}
	if err = core.DisplayInto(p, sink, true); err != nil {
		return err
	}
	return sink.err
}
