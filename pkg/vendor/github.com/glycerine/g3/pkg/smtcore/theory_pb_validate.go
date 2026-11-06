package smtcore

// Source: src/smt/theory_pb.cpp:961-970,1029-1045,1146,1838-1915.

import (
	"fmt"
	"io"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (a *smtPBArgs) validateWellFormed() error {
	if !smtcoreSourceDebug {
		return nil
	}
	if a.bound.Sign() <= 0 {
		return ast.ErrASTDomain
	}
	seen := make(map[int32]bool)
	var sum big.Rat
	for _, term := range a.terms {
		if term.coeff.Cmp(&a.bound) > 0 || term.coeff.Cmp(big.NewRat(1, 1)) < 0 || term.literal == SMTTrueLiteral || term.literal == SMTFalseLiteral || term.literal == SMTNullLiteral || seen[term.literal.Var()] {
			return ast.ErrASTDomain
		}
		seen[term.literal.Var()] = true
		sum.Add(&sum, &term.coeff)
	}
	if sum.Cmp(&a.bound) < 0 {
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) validateWatch(c *smtPBConstraint) error {
	if !smtcoreSourceDebug {
		return nil
	}
	a := c.current()
	if c.watchSize < 0 || c.watchSize > len(a.terms) {
		return ast.ErrASTDomain
	}
	var sum, maximum big.Rat
	for _, term := range a.terms[:c.watchSize] {
		sum.Add(&sum, &term.coeff)
		if term.coeff.Cmp(&maximum) > 0 {
			maximum.Set(&term.coeff)
		}
	}
	if c.watchSum.Cmp(&sum) != 0 || sum.Cmp(&a.bound) < 0 || c.maxWatch.Cmp(&maximum) != 0 {
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) validateAssign(c *smtPBConstraint, premises []SMTLiteral, consequent SMTLiteral) error {
	if !smtcoreSourceDebug {
		return nil
	}
	blocked := make(map[SMTLiteral]bool)
	for _, l := range premises {
		value, err := t.state.Assignment(l)
		if err != nil {
			return err
		}
		if value != lbool.True {
			return ast.ErrASTDomain
		}
		blocked[l.Negated()] = true
	}
	value, err := t.state.Assignment(consequent)
	if err != nil {
		return err
	}
	if value != lbool.Undef {
		return ast.ErrASTDomain
	}
	value, err = t.state.Assignment(c.lit())
	if err != nil {
		return err
	}
	if value != lbool.True {
		return ast.ErrASTDomain
	}
	blocked[consequent] = true
	var sum big.Rat
	for _, term := range c.current().terms {
		if !blocked[term.literal] {
			sum.Add(&sum, &term.coeff)
		}
	}
	if sum.Cmp(&c.current().bound) >= 0 {
		if err = t.emitPB("validate_assign", 1867, func(out io.Writer) error {
			_, _ = io.WriteString(out, "invalid assign")
			if err := t.displayInequality(out, c, true); err != nil {
				return err
			}
			if err := smtPBWriteLiterals(out, premises, true); err != nil {
				return err
			}
			_, _ = io.WriteString(out, " => ")
			if err := smtPBWriteLiteral(out, consequent); err != nil {
				return err
			}
			_, err := io.WriteString(out, "\n")
			return err
		}); err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) validateVariableSums(c *smtPBConstraint) error {
	if !smtcoreSourceDebug {
		return nil
	}
	var sum, maximum big.Rat
	for _, term := range c.current().terms {
		value, err := t.state.Assignment(term.literal)
		if err != nil {
			return err
		}
		if value == lbool.True {
			sum.Add(&sum, &term.coeff)
		}
		if value != lbool.False {
			maximum.Add(&maximum, &term.coeff)
		}
	}
	if maximum.Cmp(&c.maxSum) > 0 {
		if err := t.traceConstraint("assign_watch", 1036, c, true); err != nil {
			return err
		}
	}
	if c.minSum.Cmp(&sum) > 0 || sum.Cmp(&maximum) > 0 || maximum.Cmp(&c.maxSum) > 0 || c.minSum.Cmp(&c.maxSum) > 0 || c.minSum.Sign() < 0 {
		return ast.ErrASTDomain
	}
	return nil
}

func (t *SMTPseudoBooleanTheory) validateFinalConstraint(c *smtPBConstraint) error {
	if !smtcoreSourceDebug {
		return nil
	}
	value, err := t.state.Assignment(c.lit())
	if err != nil {
		return err
	}
	if value == lbool.Undef {
		return nil
	}
	expression, err := t.state.BoolExpression(c.lit().Var())
	if err != nil {
		return err
	}
	if !t.state.isRelevant(expression) {
		return nil
	}
	var sum, maximum big.Rat
	for _, term := range c.current().terms {
		v, err := t.state.Assignment(term.literal)
		if err != nil {
			return err
		}
		if v == lbool.True {
			sum.Add(&sum, &term.coeff)
		}
		if v != lbool.False {
			maximum.Add(&maximum, &term.coeff)
		}
	}
	k := &c.current().bound
	if err = t.emitPB("validate_final_check", 1906, func(out io.Writer) error {
		_, _ = io.WriteString(out, "validate: ")
		if err := t.displayInequality(out, c, true); err != nil {
			return err
		}
		_, err := fmt.Fprintf(out, "sum: %s %s %s\n", sum.RatString(), maximum.RatString(), value.String())
		return err
	}); err != nil {
		return err
	}
	if sum.Cmp(&maximum) > 0 {
		return ast.ErrASTDomain
	}
	if !c.equality {
		if (sum.Cmp(k) >= 0) != (value == lbool.True) || (maximum.Cmp(k) < 0) != (value == lbool.False) {
			return ast.ErrASTDomain
		}
	} else if (sum.Cmp(k) == 0) != (value == lbool.True) {
		return ast.ErrASTDomain
	}
	return nil
}
