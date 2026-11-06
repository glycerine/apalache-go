package smtcore

// Source: src/smt/theory_pb.cpp display(ineq), display_watch and display.

import (
	"errors"
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
	utext "github.com/glycerine/g3/pkg/util/text"
)

func smtPBWriteLiteral(out io.Writer, l SMTLiteral) error {
	var err error
	switch l {
	case SMTTrueLiteral:
		_, err = io.WriteString(out, "true")
	case SMTFalseLiteral:
		_, err = io.WriteString(out, "false")
	default:
		if l.Sign() {
			_, err = fmt.Fprintf(out, "(not p%d)", l.Var())
		} else {
			_, err = fmt.Fprintf(out, "p%d", l.Var())
		}
	}
	return err
}

func (t *SMTPseudoBooleanTheory) displayLiteralValue(out io.Writer, l SMTLiteral) error {
	value, err := t.state.Assignment(l)
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(out, "@(%s", value.String()); err != nil {
		return err
	}
	if value != lbool.Undef {
		if _, err = fmt.Fprintf(out, ":%d", t.state.booleanData[l.Var()].ScopeLevel); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, ")")
	return err
}

func (t *SMTPseudoBooleanTheory) displayPBExpression(out io.Writer, l SMTLiteral) (result error) {
	expressions, err := t.state.clauseLiteralExpressions([]SMTLiteral{l})
	if err != nil {
		return err
	}
	return t.displayPBApplication(out, expressions[0], false)
}

func (t *SMTPseudoBooleanTheory) displayPBApplication(out io.Writer, expression ast.ASTHandle, newline bool) (result error) {
	format, err := t.state.manager.SMT2PPExprTree(expression, nil)
	if err != nil {
		return err
	}
	stream, err := utext.NewNumericTextStream(utext.NumericTextStreamConfig{Budget: t.state.manager.Budget()})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, stream.Close()) }()
	if err = t.state.manager.PrintFormat(stream, format, nil, nil, nil); err != nil {
		return err
	}
	body, err := stream.String()
	if err != nil {
		return err
	}
	_, err = io.WriteString(out, body)
	if err == nil && newline {
		_, err = io.WriteString(out, "\n")
	}
	return err
}

func (t *SMTPseudoBooleanTheory) displayInequality(out io.Writer, c *smtPBConstraint, values bool) error {
	if err := smtPBWriteLiteral(out, c.lit()); err != nil {
		return err
	}
	if c.lit() != SMTNullLiteral {
		if values {
			if err := t.displayLiteralValue(out, c.lit()); err != nil {
				return err
			}
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
		if err := t.displayPBExpression(out, c.lit()); err != nil {
			return err
		}
		if _, err := io.WriteString(out, "\n"); err != nil {
			return err
		}
	} else {
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
	}
	a := c.current()
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
		if i+1 == c.watchSize {
			if _, err := io.WriteString(out, " .w "); err != nil {
				return err
			}
		}
		if i+1 < len(a.terms) {
			if _, err := io.WriteString(out, " + "); err != nil {
				return err
			}
		}
	}
	op := " >= "
	if c.equality {
		op = " = "
	}
	if _, err := fmt.Fprintf(out, "%s%s\n", op, a.bound.RatString()); err != nil {
		return err
	}
	if c.propagations != 0 {
		if _, err := fmt.Fprintf(out, "propagations: %d ", c.propagations); err != nil {
			return err
		}
	}
	if c.maxWatch.Sign() > 0 {
		if _, err := fmt.Fprintf(out, "max_watch: %s ", c.maxWatch.RatString()); err != nil {
			return err
		}
	}
	if c.watchSize != 0 {
		if _, err := fmt.Fprintf(out, "watch size: %d ", c.watchSize); err != nil {
			return err
		}
	}
	if c.watchSum.Sign() > 0 {
		if _, err := fmt.Fprintf(out, "watch-sum: %s ", c.watchSum.RatString()); err != nil {
			return err
		}
	}
	if c.maxSum.Sign() != 0 {
		if _, err := fmt.Fprintf(out, "sum: [%s:%s] ", c.minSum.RatString(), c.maxSum.RatString()); err != nil {
			return err
		}
	}
	if c.propagations != 0 || c.maxWatch.Sign() > 0 || c.watchSize != 0 || c.watchSum.Sign() > 0 || c.maxSum.Sign() != 0 {
		_, err := io.WriteString(out, "\n")
		return err
	}
	return nil
}

func smtPBDisplayWatch(out io.Writer, variable int32, sign bool, list []*smtPBConstraint, variableWatch bool) error {
	if list == nil {
		return nil
	}
	label := "watch: "
	if variableWatch {
		label = "watch (v): "
	}
	if _, err := io.WriteString(out, label); err != nil {
		return err
	}
	if err := smtPBWriteLiteral(out, NewSMTLiteral(variable, sign)); err != nil {
		return err
	}
	if _, err := io.WriteString(out, " |-> "); err != nil {
		return err
	}
	for _, c := range list {
		if err := smtPBWriteLiteral(out, c.lit()); err != nil {
			return err
		}
		if _, err := io.WriteString(out, " "); err != nil {
			return err
		}
	}
	_, err := io.WriteString(out, "\n")
	return err
}

func (t *SMTPseudoBooleanTheory) Display(out io.Writer) error {
	if _, err := t.State(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	// Map keys include high-water records for popped Boolean IDs. Source
	// retains allocated empty watch lists and prints them in numeric order.
	variables := t.watchVariables()
	for _, v := range variables {
		for sign, list := range t.watches[v].literals {
			if err := smtPBDisplayWatch(out, v, sign != 0, list, false); err != nil {
				return err
			}
		}
	}
	for _, v := range variables {
		if err := smtPBDisplayWatch(out, v, false, t.watches[v].variables, true); err != nil {
			return err
		}
	}
	for _, v := range variables {
		if c := t.watches[v].constraint; c != nil {
			if err := t.displayInequality(out, c, true); err != nil {
				return err
			}
		}
	}
	return nil
}
