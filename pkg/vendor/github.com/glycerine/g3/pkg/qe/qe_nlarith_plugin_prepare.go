package qe

import (
	"errors"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (p *NonlinearArithmeticPlugin) Prepare(variable *ast.ContainsApp, formula ast.ASTHandle, positive, negative []ast.ASTHandle) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if variable == nil {
		return false, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return false, err
	}
	key := nlPluginKey{target, formula}
	if p.cache[key] != nil {
		return true, nil
	}
	literals := append([]ast.ASTHandle(nil), positive...)
	owned := make([]ast.ASTRef, 0, len(negative))
	defer func() {
		for i := range owned {
			_ = p.manager.DelRef(&owned[i])
		}
	}()
	for _, atom := range negative {
		n, cause := p.manager.MkNot(atom)
		if cause != nil {
			return false, cause
		}
		r, cause := p.manager.NewRef(n)
		if cause != nil {
			return false, cause
		}
		owned = append(owned, r)
		literals = append(literals, n)
	}
	branches, err := NewNLBranchConditions(p.manager)
	if err != nil {
		return false, err
	}
	ok, err := p.util.CreateBranches(variable, literals, branches)
	if err != nil || !ok {
		_ = branches.Close()
		return ok, err
	}
	entry := &nlPluginEntry{branches: branches}
	entry.variable, err = p.manager.NewRef(target)
	if err != nil {
		_ = branches.Close()
		return false, err
	}
	entry.formula, err = p.manager.NewRef(formula)
	if err != nil {
		_ = p.manager.DelRef(&entry.variable)
		_ = branches.Close()
		return false, err
	}
	p.cache[key] = entry
	return true, nil
}
func (p *NonlinearArithmeticPlugin) entry(variable *ast.ContainsApp, formula ast.ASTHandle) (*nlPluginEntry, ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, ast.ASTHandle{}, err
	}
	if variable == nil {
		return nil, ast.ASTHandle{}, ast.ErrASTHandle
	}
	target, err := variable.Target()
	if err != nil {
		return nil, target, err
	}
	e := p.cache[nlPluginKey{target, formula}]
	if e == nil {
		return nil, target, ast.ErrASTDomain
	}
	return e, target, nil
}
func (p *NonlinearArithmeticPlugin) GetNumBranches(variable *ast.ContainsApp, formula ast.ASTHandle, count *numeral.OwnedRational) (bool, error) {
	if count == nil {
		return false, ast.ErrASTHandle
	}
	e, _, err := p.entry(variable, formula)
	if err != nil {
		return false, err
	}
	return true, p.manager.NumeralService().SetUnsigned(count, uint32(len(e.branches.Branches)))
}

var _ = errors.Join
