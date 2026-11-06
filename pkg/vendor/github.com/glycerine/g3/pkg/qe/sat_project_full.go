package qe

// Source: src/qe/qe_sat_tactic.cpp, solver_context::project_var_full.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
)

// projectFull existentially abstracts one variable and invokes the available
// quantifier elimination traversal. Unsupported inputs leave the context intact.
func (c *satSolverContext) projectFull(index uint32, rw *rewriter.TheoryRewriter, config Config) (err error) {
	if c == nil || c.closed || rw == nil {
		return ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return arena.ErrBounds
	}
	m := c.manager
	variable, err := m.RefGet(c.variables[index])
	if err != nil {
		return err
	}
	formula, err := m.RefGet(c.formula)
	if err != nil {
		return err
	}
	abstractor, err := ast.NewExprAbstractor(m)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, abstractor.Close()) }()
	quantified, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&quantified)) }()
	if err = abstractor.MkExists([]ast.ASTHandle{variable}, formula, quantified); err != nil {
		return err
	}
	q, err := m.RefGet(quantified)
	if err != nil {
		return err
	}
	projected, complete, err := rewriteQuantifierScalar(m, q, config)
	if err != nil {
		return err
	}
	if !complete {
		return tactic.NewTacticException("qe-sat full projection could not eliminate variable")
	}
	defer func() { err = errors.Join(err, m.DelRef(&projected)) }()
	value, err := m.RefGet(projected)
	if err != nil {
		return err
	}
	if err = rw.Rewrite(value, projected); err != nil {
		return err
	}
	value, err = m.RefGet(projected)
	if err != nil {
		return err
	}
	return c.eliminateVariable(index, value, ast.ASTHandle{})
}
