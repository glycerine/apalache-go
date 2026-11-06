package model

// Source: src/model/model.cpp:93-109; src/model/model_core.h:65;
// src/ast/ast.cpp:2561-2582.

import "github.com/glycerine/g3/pkg/ast"

// modelValueProc borrows native universe storage while the AST manager is
// locked. Handle identity lookup does not reenter the manager.
type modelValueProc struct{ model *Model }

func (p modelValueProc) SomeValue(s ast.ASTHandle, _ uint32) (ast.ASTHandle, error) {
	if u, found := p.model.Universes.Get2(s); found && len(u.values) > 0 {
		return u.values[0], nil
	}
	return ast.ASTHandle{}, nil
}

// GetSomeValue borrows the first universe value when present, then delegates
// to the original AST plugin/fallback path under a scoped universe callback.
func (md *Model) GetSomeValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if e := md.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	return md.CoreField.manager.GetSomeValueWithProc(s, modelValueProc{md})
}
func (md *ModelCore) GetSomeValue(s ast.ASTHandle) (ast.ASTHandle, error) {
	if e := md.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	// The source base method is abstract; a bare core has no value provider.
	if md.valueProvider == nil {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return md.valueProvider.GetSomeValue(s)
}
