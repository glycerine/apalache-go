package lp

// Source: src/util/lp/lar_solver.h:1309-1337,1359-1362;
// column_namer.h:73-97; lar_constraints.h:35-45.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/util/numeral"
)

func writeConstraintText(out io.Writer, text string) error {
	n, err := io.WriteString(out, text)
	if err == nil && n != len(text) {
		return io.ErrShortWrite
	}
	return err
}

func flushConstraintOutput(out io.Writer) error {
	if f, ok := out.(interface{ Flush() error }); ok {
		return f.Flush()
	}
	return nil
}

// PrintConstraints is lar_solver::print_constraints. Each constraint retains
// print_constraint's endl flush, including the last constraint.
func (s *LARSolver) PrintConstraints(out io.Writer) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	for _, constraint := range s.Constraints {
		if err = s.WriteConstraint(constraint, out); err != nil {
			return err
		}
	}
	return nil
}

// PrintTerms is lar_solver::print_terms. It prints current terms in publication
// order; unlike print_constraints, its added newlines do not flush the writer.
func (s *LARSolver) PrintTerms(out io.Writer) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	for _, term := range s.Terms {
		if err = s.PrintTerm(term, out); err != nil {
			return err
		}
		if err = writeConstraintText(out, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func (s *LARSolver) PrintConstraint(index uint32, out io.Writer) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if uint64(index) >= uint64(len(s.Constraints)) {
		if err = writeConstraintText(out, fmt.Sprintf("constraint %d is not found\n", index)); err != nil {
			return err
		}
		return flushConstraintOutput(out)
	}
	return s.WriteConstraint(s.Constraints[index], out)
}

func (s *LARSolver) WriteConstraint(c LARStoredConstraint, out io.Writer) (err error) {
	if err = s.ready(); err != nil {
		return err
	}
	defer func() { s.stopped = err != nil }()
	if out == nil {
		return ErrBounds
	}
	if c == nil || c.Base().manager != s.manager {
		return ErrExactVectorOwner
	}
	if err = c.Base().ready(); err != nil {
		return err
	}
	if err = s.printConstraintLeftSide(c, out); err != nil {
		return err
	}
	if err = writeConstraintText(out, " "); err != nil {
		return err
	}
	kind, e := LConstraintKindString(c.Base().Kind)
	if e != nil {
		return e
	}
	if err = writeConstraintText(out, kind+" "); err != nil {
		return err
	}
	text, e := s.manager.String(&c.Base().RightSide)
	if e != nil {
		return e
	}
	if err = writeConstraintText(out, text+"\n"); err != nil {
		return err
	}
	return flushConstraintOutput(out)
}

func (s *LARSolver) printConstraintLeftSide(c LARStoredConstraint, out io.Writer) (err error) {
	s.validationCoefficients, err = c.CoefficientsAsVector()
	if err != nil {
		return err
	}
	s.rationalWork = NewIndexedRationalVector(s.manager, 3)
	w := s.rationalWork.Data // val, unary negation temporary, free coefficient.
	if err = s.printColumnCombination(s.validationCoefficients.Data, out); err != nil {
		return err
	}
	if err = s.validationCoefficients.Close(); err != nil {
		return err
	}
	s.validationCoefficients = nil
	if err = c.FreeCoefficient(&w[2]); err != nil {
		return err
	}
	zero, e := s.manager.IsZero(&w[2])
	if e != nil {
		return e
	}
	if !zero {
		if err = writeConstraintText(out, " + "); err != nil {
			return err
		}
		text, e := s.manager.String(&w[2])
		if e != nil {
			return e
		}
		if err = writeConstraintText(out, text); err != nil {
			return err
		}
	}
	return s.closeConstraintValue()
}

// printColumnCombination borrows the source vector while the caller owns its
// copied coefficient and unary-minus lexical values in rationalWork.
func (s *LARSolver) printColumnCombination(coefficients []RationalSparseEntry, out io.Writer) (err error) {
	w := s.rationalWork.Data
	for i := range coefficients {
		cell := &coefficients[i]
		if err = s.rationalWork.copySlot(&w[0], &cell.Value); err != nil {
			return err
		}
		if i != 0 {
			positive, e := s.manager.IsPos(&w[0])
			if e != nil {
				return e
			}
			if positive {
				if err = writeConstraintText(out, " + "); err != nil {
					return err
				}
			} else {
				if err = writeConstraintText(out, " - "); err != nil {
					return err
				}
				if err = s.rationalWork.copySlot(&w[1], &w[0]); err != nil {
					return err
				}
				before, e := s.calls.begin(s.manager)
				if e != nil {
					return e
				}
				e = s.manager.Neg(&w[1])
				if err = s.calls.after(s.manager, before, e); err != nil {
					return err
				}
				if err = s.rationalWork.copySlot(&w[0], &w[1]); err != nil {
					return err
				}
				if err = s.manager.Del(&w[1]); err != nil {
					return err
				}
				w[1] = numeral.NewRational()
			}
		}
		minusOne, e := s.manager.IsMinusOne(&w[0])
		if e != nil {
			return e
		}
		if minusOne {
			if err = writeConstraintText(out, " - "); err != nil {
				return err
			}
		} else {
			one, e := s.manager.IsOne(&w[0])
			if e != nil {
				return e
			}
			if !one {
				text, e := s.manager.String(&w[0])
				if e != nil {
					return e
				}
				if err = writeConstraintText(out, text); err != nil {
					return err
				}
			}
		}
		if err = writeConstraintText(out, s.ColumnName(cell.Index)); err != nil {
			return err
		}
		if err = s.manager.Del(&w[0]); err != nil {
			return err
		}
		w[0] = numeral.NewRational()
	}
	return nil
}
