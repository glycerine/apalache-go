package qe

// Source: src/qe/qe_array_plugin.cpp, solve_eq's direct-variable branch.

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (p *ArrayPlugin) tryDirectSide(left, right, formula ast.ASTHandle) (solved bool, err error) {
	k, e := p.manager.GetKind(left)
	if e != nil {
		return false, e
	}
	if k != ast.ASTApp {
		return false, nil
	}
	idx, found, e := p.context.IsVariable(left)
	if e != nil || !found {
		return false, e
	}
	contains, e := p.context.Contains(idx)
	if e != nil {
		return false, e
	}
	has, e := contains.Check(right)
	if e != nil || has {
		return false, e
	}
	result, e := p.manager.NewRef(formula)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, p.manager.DelRef(&result)) }()
	rw, e := rewriter.NewExprSafeReplace(p.manager)
	if e != nil {
		return false, e
	}
	defer func() { err = errors.Join(err, rw.Close()) }()
	if e = rw.ApplySubstitution(left, right, result); e != nil {
		return false, e
	}
	rewritten, e := p.manager.RefGet(result)
	if e != nil {
		return false, e
	}
	if e = p.context.EliminateVariable(idx, rewritten, right); e != nil {
		return false, e
	}
	return true, nil
}

func (p *ArrayPlugin) equalitySides(eq ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool, error) {
	kind, e := p.manager.GetKind(eq)
	if e != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	family, e := p.manager.GetFamilyID(eq)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	op, e := p.manager.GetDeclKind(eq)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if family != p.manager.BasicFamilyID() || op != ast.ASTOpEq {
		return ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	left, e := p.manager.GetArg(eq, 0)
	if e != nil {
		return left, ast.ASTHandle{}, false, e
	}
	right, e := p.manager.GetArg(eq, 1)
	return left, right, e == nil, e
}

// TryDirectEquality eliminates a directly equated variable in either orientation.
func (p *ArrayPlugin) TryDirectEquality(eq, formula ast.ASTHandle) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	left, right, ok, e := p.equalitySides(eq)
	if e != nil || !ok {
		return false, e
	}
	solved, e := p.tryDirectSide(left, right, formula)
	if e != nil || solved {
		return solved, e
	}
	return p.tryDirectSide(right, left, formula)
}
