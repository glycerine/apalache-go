package polynomial

// Source: src/math/polynomial/linear_eq_solver.h:140-150;
// src/util/mpq.cpp display; src/util/mpzzp.h display.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

func finishLinearDisplayInsertion(body error, receipt *unumeral.NumeralInsertionReceipt, integer *unumeral.IntegerManager) error {
	if !receipt.Active() || receipt.SourceCause() != nil {
		return body
	}
	cleanup := receipt.Complete(integer.Naturals(), true)
	if body != nil && cleanup != nil {
		return errors.Join(body, cleanup)
	}
	if body != nil {
		return body
	}
	return cleanup
}

func cleanupLinearDisplayInsertion(receipt *unumeral.NumeralInsertionReceipt, integer *unumeral.IntegerManager) error {
	if !receipt.Active() {
		return nil
	}
	if e := receipt.Complete(integer.Naturals(), false); e != nil {
		return e
	}
	return receipt.Complete(integer.Naturals(), true)
}

func (s *RationalLinearSolver) displayRational(out io.Writer, a *unumeral.Rational) error {
	body := s.manager.DisplayToTracked(out, a, &s.displayInsertion)
	return finishLinearDisplayInsertion(body, &s.displayInsertion, s.integer)
}
func (s *ModularLinearSolver) displayInteger(out io.Writer, a *unumeral.Int) error {
	body := s.integer.DisplayToTracked(out, a, &s.displayInsertion)
	return finishLinearDisplayInsertion(body, &s.displayInsertion, s.integer)
}

// io.Writer is a cold host-output adapter. Read and format each actual source
// field at its reached position; no matrix snapshot or numerical mirror.
func linearWriteText(out io.Writer, text string) error {
	if out == nil {
		return arena.ErrConfig
	}
	n, e := io.WriteString(out, text)
	if e != nil {
		return e
	}
	if n != len(text) {
		return io.ErrShortWrite
	}
	return nil
}
func (s *RationalLinearSolver) Display(out io.Writer) error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}
	if e := s.open(); e != nil {
		return e
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	return s.displaySource(out)
}
func (s *RationalLinearSolver) displaySource(out io.Writer) error {
	if e := s.open(); e != nil {
		return e
	}
	size := uint32(len(s.A))
	for i := uint32(0); i < size; i++ {
		row, e := s.row(i)
		if e != nil {
			return e
		}
		if polynomialSourceDebug {
			size := uint32(len(*row))
			if e := linearSourceAssert(size == s.n, 141, "A[i].size() == n"); e != nil {
				return e
			}
		}
		for j := uint32(0); j < s.n; j++ {
			a, e := s.load(row, j)
			if e != nil {
				return e
			}
			if e = s.displayRational(out, &a); e != nil {
				return e
			}
			if e = linearWriteText(out, " "); e != nil {
				return e
			}
		}
		b, e := s.load(&s.b, i)
		if e != nil {
			return e
		}
		if e = s.displayRational(out, &b); e != nil {
			return e
		}
		if e = linearWriteText(out, "\n"); e != nil {
			return e
		}
	}
	return nil
}
func (s *ModularLinearSolver) Display(out io.Writer) error {
	if s != nil && s.self == s && s.running {
		return ErrPolynomialPending
	}
	if e := s.open(); e != nil {
		return e
	}
	if e := s.beginExecution(); e != nil {
		return e
	}
	defer s.endExecution()
	return s.displaySource(out)
}
func (s *ModularLinearSolver) displaySource(out io.Writer) error {
	if e := s.open(); e != nil {
		return e
	}
	size := uint32(len(s.A))
	for i := uint32(0); i < size; i++ {
		row, e := s.row(i)
		if e != nil {
			return e
		}
		if polynomialSourceDebug {
			size := uint32(len(*row))
			if e := linearSourceAssert(size == s.n, 141, "A[i].size() == n"); e != nil {
				return e
			}
		}
		for j := uint32(0); j < s.n; j++ {
			a, e := s.load(row, j)
			if e != nil {
				return e
			}
			// mpzzp.h display calls the integer manager directly, without the
			// normalization assertion of the separate modular String API.
			if e = s.displayInteger(out, &a); e != nil {
				return e
			}
			if e = linearWriteText(out, " "); e != nil {
				return e
			}
		}
		b, e := s.load(&s.b, i)
		if e != nil {
			return e
		}
		if e = s.displayInteger(out, &b); e != nil {
			return e
		}
		if e = linearWriteText(out, "\n"); e != nil {
			return e
		}
	}
	return nil
}
