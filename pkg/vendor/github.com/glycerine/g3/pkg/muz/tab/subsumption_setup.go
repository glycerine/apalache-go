package tab

// Source: src/muz/tab/tab_context.cpp, index::setup.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type groundClause struct {
	manager            *ast.ASTManager
	head, precondition ast.ASTRef
	predicates         []ast.ASTRef
}

func (g *groundClause) Close() error {
	if g == nil || g.manager == nil {
		return nil
	}
	for i := range g.predicates {
		if g.predicates[i] != (ast.ASTRef{}) {
			if err := g.manager.DelRef(&g.predicates[i]); err != nil {
				return err
			}
		}
	}
	for _, ref := range []*ast.ASTRef{&g.head, &g.precondition} {
		if *ref != (ast.ASTRef{}) {
			if err := g.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	g.predicates = nil
	g.manager = nil
	return nil
}

func setupGroundClause(c *Clause) (result *groundClause, err error) {
	if c == nil || c.closed {
		return nil, ast.ErrASTHandle
	}
	m := c.manager
	fv, err := c.freeVariables()
	if err != nil {
		return nil, err
	}
	result = &groundClause{manager: m}
	attempt := &groundClauseConstructionAttempt{manager: m, result: result}
	defer func() { result, err = finishGroundClauseConstruction(attempt, err) }()
	vars := make([]ast.ASTHandle, fv.Size())
	for i := range vars {
		sort, e := fv.Get(uint32(i))
		if e != nil {
			return result, e
		}
		if sort.IsNull() {
			sort, e = m.MkBoolSort()
			if e != nil {
				return result, e
			}
		}
		vars[i], e = m.MkConst(usymbol.Numerical(uint32(i)), sort)
		if e != nil {
			return result, e
		}
		ref, e := m.NewRef(vars[i])
		if e != nil {
			return result, e
		}
		attempt.variables = append(attempt.variables, ref)
	}
	subst, err := rewriter.NewVarSubstitution(m, false)
	if err != nil {
		return result, err
	}
	attempt.subst = subst
	apply := func(h ast.ASTHandle) (out ast.ASTRef, e error) {
		out, e = m.NewRef(ast.ASTHandle{})
		if e != nil {
			return out, e
		}
		if e = subst.Substitute(h, vars, out); e != nil {
			attempt.temporary = out
			return ast.ASTRef{}, e
		}
		return out, e
	}
	head, err := c.Head()
	if err != nil {
		return result, err
	}
	result.head, err = apply(head)
	if err != nil {
		return result, err
	}
	predicates, err := c.Predicates()
	if err != nil {
		return result, err
	}
	for _, p := range predicates {
		ref, e := apply(p)
		if e != nil {
			return result, e
		}
		result.predicates = append(result.predicates, ref)
	}
	constraint, err := c.Constraint()
	if err != nil {
		return result, err
	}
	reduced, err := apply(constraint)
	if err != nil {
		return result, err
	}
	attempt.temporary = reduced
	h, err := m.RefGet(reduced)
	if err != nil {
		return result, err
	}
	precondition, err := m.MkAndArgs([]ast.ASTHandle{h})
	if err != nil {
		return result, err
	}
	result.precondition, err = m.NewRef(precondition)
	return result, err
}
