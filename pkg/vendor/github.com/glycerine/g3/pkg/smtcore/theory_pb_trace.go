package smtcore

// Source: src/smt/theory_pb.cpp retained pb TRACE records and arg_t::display.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/util/diagnostic"
)

func (t *SMTPseudoBooleanTheory) pbTraceTags() *diagnostic.TraceTags {
	if t != nil && t.trace != nil {
		return t.trace
	}
	return diagnostic.BorrowProcessTraceTags()
}

func (t *SMTPseudoBooleanTheory) emitPB(function string, line int, body func(io.Writer) error) error {
	return t.emitPBTag("pb", function, line, body)
}

func (t *SMTPseudoBooleanTheory) emitPBTag(tag, function string, line int, body func(io.Writer) error) error {
	tags := t.pbTraceTags()
	var operandErr error
	err := tags.EmitConditional(tag, function, "src/smt/theory_pb.cpp", line, func() bool { return true }, func(out io.Writer) error {
		// Keep evaluating later source operands after ostream badbit. The
		// diagnostic stream retains the write failure independently.
		operandErr = body(assertedTraceWriter{out})
		return operandErr
	})
	if operandErr != nil {
		return operandErr
	}
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}

func smtPBWriteLiterals(out io.Writer, ls []SMTLiteral, trailing bool) error {
	for i, l := range ls {
		if i != 0 {
			if _, err := io.WriteString(out, " "); err != nil {
				return err
			}
		}
		if err := smtPBWriteLiteral(out, l); err != nil {
			return err
		}
	}
	if trailing && len(ls) != 0 {
		_, err := io.WriteString(out, " ")
		return err
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) traceConstraint(function string, line int, c *smtPBConstraint, values bool) error {
	return t.emitPB(function, line, func(out io.Writer) error { return t.displayInequality(out, c, values) })
}

func (t *SMTPseudoBooleanTheory) traceClause(function string, line int, ls []SMTLiteral) error {
	return t.emitPB(function, line, func(out io.Writer) error {
		if err := smtPBWriteLiterals(out, ls, false); err != nil {
			return err
		}
		_, err := io.WriteString(out, "\n")
		return err
	})
}

func (t *SMTPseudoBooleanTheory) traceAssignment(function string, line int, heading string, l SMTLiteral, c *smtPBConstraint, values bool) error {
	return t.emitPB(function, line, func(out io.Writer) error {
		_, _ = io.WriteString(out, heading)
		if err := smtPBWriteLiteral(out, l); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		if c != nil {
			return t.displayInequality(out, c, values)
		}
		return nil
	})
}

func (t *SMTPseudoBooleanTheory) traceReason(function string, line int, c *smtPBConstraint, ls []SMTLiteral, consequent SMTLiteral) error {
	return t.emitPB(function, line, func(out io.Writer) error {
		_, _ = fmt.Fprintf(out, "#prop:%d - ", c.propagations)
		if err := smtPBWriteLiterals(out, ls, true); err != nil {
			return err
		}
		if consequent != SMTNullLiteral {
			_, _ = io.WriteString(out, "=> ")
			if err := smtPBWriteLiteral(out, consequent); err != nil {
				return err
			}
		}
		_, _ = io.WriteString(out, "\n")
		return t.displayInequality(out, c, true)
	})
}

func (t *SMTPseudoBooleanTheory) displayPBArgs(out io.Writer, a *smtPBArgs, values bool) error {
	for i, term := range a.terms {
		if term.coeff.Cmp(big.NewRat(1, 1)) != 0 {
			if _, err := fmt.Fprintf(out, "%s*", term.coeff.RatString()); err != nil {
				return err
			}
		}
		if err := smtPBWriteLiteral(out, term.literal); err != nil {
			return err
		}
		if values {
			if err := t.displayLiteralValue(out, term.literal); err != nil {
				return err
			}
		}
		if i+1 < len(a.terms) {
			if _, err := io.WriteString(out, " + "); err != nil {
				return err
			}
		}
	}
	_, err := fmt.Fprintf(out, " ~ %s\n", a.bound.RatString())
	return err
}
