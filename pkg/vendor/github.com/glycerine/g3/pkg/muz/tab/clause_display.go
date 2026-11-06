package tab

// Source: src/muz/tab/tab_context.cpp tb::clause::display.

import (
	"fmt"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

// Display prints the unquantified clause. False-head goals display their body,
// while other clauses display their head or the implication from their body.
func (c *Clause) Display(out io.Writer) (err error) {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	if out == nil {
		return io.ErrClosedPipe
	}
	m := c.manager
	attempt := &clauseFormulaAttempt{manager: m}
	defer func() {
		if err == nil {
			err = attempt.Close()
		}
		if err != nil {
			_, err = finishClauseFormula(attempt, ast.ASTRef{}, err)
		}
	}()
	values, err := c.Predicates()
	if err != nil {
		return err
	}
	constraint, err := c.Constraint()
	if err != nil {
		return err
	}
	values = append(values, constraint)
	attempt.result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, values, attempt.result); err != nil {
		return err
	}
	formula, err := m.RefGet(attempt.result)
	if err != nil {
		return err
	}
	head, err := c.Head()
	if err != nil {
		return err
	}
	if !m.IsFalse(head) {
		if m.IsTrue(formula) {
			formula = head
		} else {
			formula, err = m.MkImplies(formula, head)
			if err != nil {
				return err
			}
		}
		if err = m.SetRef(attempt.result, formula); err != nil {
			return err
		}
	}
	if err = ast.DisplayLegacySMT(out, m, formula, 0, nil); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out)
	return err
}
