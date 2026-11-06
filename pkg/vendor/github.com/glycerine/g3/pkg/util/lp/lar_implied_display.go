package lp

// Source: src/util/lp/lar_solver.h:141-159,1340-1345.

import (
	"fmt"
	"io"
)

// PrintTerm borrows the original term; copied coefficients and numerical
// printing temporaries belong to the solver until successful cleanup or Close.
func (s *LARSolver) PrintTerm(term *LARTerm, out io.Writer) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if term == nil || term.manager != s.manager {
		return ErrExactVectorOwner
	}
	if err = term.ready(); err != nil {
		return err
	}
	zero, err := s.manager.IsZero(&term.Value)
	if err != nil {
		return err
	}
	if !zero {
		text, e := s.manager.String(&term.Value)
		if e != nil {
			return e
		}
		if err = writeConstraintText(out, text+" + "); err != nil {
			return err
		}
	}
	s.validationCoefficients, err = term.CoefficientsAsVector()
	if err != nil {
		return err
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 2)
	if err = s.printColumnCombination(s.validationCoefficients.Data, out); err != nil {
		return err
	}
	if err = s.validationCoefficients.Close(); err != nil {
		return err
	}
	s.validationCoefficients = nil
	return s.closeConstraintValue()
}

// PrintImpliedBound preserves the source newline/endl distinction, including
// the separate flushed term-number line and original (unsubstituted) term.
func (s *LARSolver) PrintImpliedBound(bound *ImpliedBound, out io.Writer) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if bound == nil || bound.manager != s.manager {
		return ErrExactVectorOwner
	}
	if bound.stopped || bound.calls.pending() {
		return ErrExactVectorPending
	}
	if err = writeConstraintText(out, "implied bound\n"); err != nil {
		return err
	}
	if s.IsTerm(bound.Column) {
		if err = writeConstraintText(out, fmt.Sprintf("it is a term number %d\n", bound.Column)); err != nil {
			return err
		}
		if err = flushConstraintOutput(out); err != nil {
			return err
		}
		index := bound.Column - LARTermStartIndex
		if uint64(index) >= uint64(len(s.OriginalTerms)) {
			return ErrBounds
		}
		if err = s.PrintTerm(s.OriginalTerms[index], out); err != nil {
			return err
		}
	} else if err = writeConstraintText(out, s.ColumnName(bound.Column)); err != nil {
		return err
	}
	kind, err := LConstraintKindString(bound.Kind())
	if err != nil {
		return err
	}
	if err = writeConstraintText(out, " "+kind+" "); err != nil {
		return err
	}
	text, err := s.manager.String(&bound.Bound)
	if err != nil {
		return err
	}
	if err = writeConstraintText(out, text+"\n"); err != nil {
		return err
	}
	if err = flushConstraintOutput(out); err != nil {
		return err
	}
	if err = writeConstraintText(out, "end of implied bound\n"); err != nil {
		return err
	}
	return flushConstraintOutput(out)
}
