package tab

// Source: src/muz/tab/tab_context.cpp, clause get_body and to_formula.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Body returns an owning reference to the flattened predicate/constraint conjunction.
func (c *Clause) Body() (result ast.ASTRef, err error) {
	if c == nil || c.closed {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	attempt := &clauseFormulaAttempt{manager: m}
	defer func() { result, err = finishClauseFormula(attempt, result, err) }()
	values, err := c.Predicates()
	if err != nil {
		return result, err
	}
	constraint, err := c.Constraint()
	if err != nil {
		return result, err
	}
	values = append(values, constraint)
	values, err = m.FlattenAnd(values)
	if err != nil {
		return result, err
	}
	result, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return result, err
	}
	attempt.result = result
	if err = rewriter.SimplifyBooleanGateDefault(m, ast.ASTOpAnd, values, result); err != nil {
		return ast.ASTRef{}, err
	}
	return result, nil
}

// Formula returns an owning reference to the universally closed Horn clause.
func (c *Clause) Formula() (result ast.ASTRef, err error) {
	if c == nil || c.closed {
		return result, ast.ErrASTHandle
	}
	m := c.manager
	attempt := &clauseFormulaAttempt{manager: m}
	defer func() { result, err = finishClauseFormula(attempt, result, err) }()
	result, err = c.Body()
	if err != nil {
		return result, err
	}
	attempt.result = result
	head, err := c.Head()
	if err != nil {
		return result, err
	}
	body, err := m.RefGet(result)
	if err != nil {
		return result, err
	}
	formula := head
	if !m.IsTrue(body) {
		formula, err = m.MkImplies(body, head)
		if err != nil {
			return result, err
		}
	}
	if err = m.SetRef(result, formula); err != nil {
		return result, err
	}
	fv, err := c.freeVariables()
	if err != nil {
		return result, err
	}
	if fv.Size() != 0 {
		sorts := make([]ast.ASTHandle, fv.Size())
		names := make([]usymbol.Symbol, fv.Size())
		fresh := model.NewFreshName(m)
		if err = fresh.AddAST(formula); err != nil {
			return result, err
		}
		for i := range sorts {
			sorts[i], err = fv.Get(uint32(len(sorts) - 1 - i))
			if err != nil {
				return result, err
			}
			if sorts[i].IsNull() {
				sorts[i], err = m.MkBoolSort()
				if err != nil {
					return result, err
				}
			}
			names[i], err = fresh.Next()
			if err != nil {
				return result, err
			}
		}
		formula, err = m.MkQuantifier(ast.ASTQuantifierSpec{Forall: true, Sorts: sorts, Names: names, Body: formula})
		if err != nil {
			return result, err
		}
		if err = m.SetRef(result, formula); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (c *Clause) freeVariables() (*rewriter.ExprFreeVars, error) {
	if c == nil || c.closed {
		return nil, ast.ErrASTHandle
	}
	fv, err := rewriter.NewExprFreeVars(c.manager)
	if err != nil {
		return nil, err
	}
	head, err := c.Head()
	if err != nil {
		return nil, err
	}
	predicates, err := c.Predicates()
	if err != nil {
		return nil, err
	}
	constraint, err := c.Constraint()
	if err != nil {
		return nil, err
	}
	for _, h := range append(append([]ast.ASTHandle{head}, predicates...), constraint) {
		if err = fv.Accumulate(h); err != nil {
			return nil, err
		}
	}
	return fv, nil
}
