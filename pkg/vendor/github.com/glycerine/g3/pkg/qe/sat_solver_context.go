package qe

// Source: src/qe/qe_sat_tactic.cpp, default NNF callbacks and the concrete
// solver_context formula initialization and constraint assertion.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/smtcore"
)

type satNNFDefault struct{ manager *ast.ASTManager }

func (d *satNNFDefault) IsRelevant(ast.ASTHandle) (bool, error) { return true, nil }
func (d *satNNFDefault) NormalizeAtom(h ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if positive {
		return h, nil
	}
	return d.manager.MkNot(h)
}

// satSolverContext borrows the enclosing tactic's SMT solver. Its formula and
// atom index are owned, and refreshed together by initializeFormula.
type satSolverContext struct {
	manager    *ast.ASTManager
	solver     *smtcore.SMTSolver
	normalizer *QENNFNormalizer
	formula    ast.ASTRef
	atoms      *BoolAtomIndex
	variables  []ast.ASTRef
	contains   []*ast.ContainsApp
	closed     bool
}

func newSATSolverContext(manager *ast.ASTManager, solver *smtcore.SMTSolver, config Config) (_ *satSolverContext, result error) {
	if manager == nil || solver == nil {
		return nil, ast.ErrASTHandle
	}
	owner, err := solver.Manager()
	if err != nil {
		return nil, err
	}
	if owner != manager {
		return nil, ast.ErrASTHandle
	}
	defaults := &satNNFDefault{manager: manager}
	normalizer, err := NewQENNFNormalizer(manager, defaults, defaults, config)
	if err != nil {
		return nil, err
	}
	c := &satSolverContext{manager: manager, solver: solver, normalizer: normalizer}
	c.formula, err = manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, errors.Join(err, normalizer.Close())
	}
	return c, nil
}

func (c *satSolverContext) initializeFormula(formula ast.ASTHandle) (result error) {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	next, err := c.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, c.manager.DelRef(&next)) }()
	atoms, err := c.normalizer.Normalize(next)
	if err != nil {
		return err
	}
	if c.atoms != nil {
		if err = c.atoms.Close(); err != nil {
			return errors.Join(err, atoms.Close())
		}
		c.atoms = nil
	}
	if err = c.manager.AssignRef(c.formula, next); err != nil {
		return errors.Join(err, atoms.Close())
	}
	c.atoms = atoms
	return nil
}

// addVariable stores both the application and the source contains-app helper.
func (c *satSolverContext) addVariable(variable ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	contains, err := ast.NewContainsApp(c.manager, variable)
	if err != nil {
		return err
	}
	ref, err := c.manager.NewRef(variable)
	if err != nil {
		return errors.Join(err, contains.Close())
	}
	c.variables = append(c.variables, ref)
	c.contains = append(c.contains, contains)
	return nil
}

func (c *satSolverContext) variable(index uint32) (ast.ASTHandle, error) {
	if c == nil || c.closed {
		return ast.ASTHandle{}, ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return c.manager.RefGet(c.variables[index])
}

func (c *satSolverContext) isVariable(expr ast.ASTHandle) (uint32, bool, error) {
	if c == nil || c.closed {
		return 0, false, ast.ErrASTHandle
	}
	for i := range c.variables {
		h, err := c.manager.RefGet(c.variables[i])
		if err != nil {
			return 0, false, err
		}
		if h == expr {
			return uint32(i), true, nil
		}
	}
	return 0, false, nil
}

// eliminateVariable recomputes NNF then erases the selected entry while
// preserving the order of the remaining variables. The definition is unused
// by the source solver context.
func (c *satSolverContext) eliminateVariable(index uint32, formula, definition ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	if index >= uint32(len(c.variables)) {
		return arena.ErrBounds
	}
	if err := c.initializeFormula(formula); err != nil {
		return err
	}
	if c.contains[index] != nil {
		if err := c.contains[index].Close(); err != nil {
			return err
		}
		c.contains[index] = nil
	}
	if err := c.manager.DelRef(&c.variables[index]); err != nil {
		return err
	}
	copy(c.variables[index:], c.variables[index+1:])
	c.variables = c.variables[:len(c.variables)-1]
	copy(c.contains[index:], c.contains[index+1:])
	c.contains = c.contains[:len(c.contains)-1]
	return nil
}

// addConstraint intentionally ignores useVar, matching the source callback.
func (c *satSolverContext) addConstraint(useVar bool, literals ...ast.ASTHandle) error {
	if c == nil || c.closed {
		return ast.ErrASTHandle
	}
	if len(literals) > 3 {
		return ast.ErrASTDomain
	}
	args := make([]ast.ASTHandle, 0, len(literals))
	for _, h := range literals {
		if !h.IsNull() {
			args = append(args, h)
		}
	}
	clause, err := c.manager.MkOrArgs(args)
	if err != nil {
		return err
	}
	ref, err := c.manager.NewRef(clause)
	if err != nil {
		return err
	}
	err = c.solver.AssertFormula(clause)
	return errors.Join(err, c.manager.DelRef(&ref))
}

func (c *satSolverContext) Close() (result error) {
	if c == nil || c.closed {
		return nil
	}
	for len(c.variables) != 0 {
		i := len(c.variables) - 1
		if c.contains[i] != nil {
			if err := c.contains[i].Close(); err != nil {
				return err
			}
			c.contains[i] = nil
		}
		if c.variables[i] != (ast.ASTRef{}) {
			if err := c.manager.DelRef(&c.variables[i]); err != nil {
				return err
			}
		}
		c.contains = c.contains[:i]
		c.variables = c.variables[:i]
	}
	if c.atoms != nil {
		if err := c.atoms.Close(); err != nil {
			return err
		}
		c.atoms = nil
	}
	if c.formula != (ast.ASTRef{}) {
		if err := c.manager.DelRef(&c.formula); err != nil {
			return err
		}
	}
	if c.normalizer != nil {
		if err := c.normalizer.Close(); err != nil {
			return err
		}
		c.normalizer = nil
	}
	c.closed = true
	return nil
}
