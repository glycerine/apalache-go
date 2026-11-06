package api

// Source: src/api/api_datalog.cpp:431-452; retained declare-var command.

import "github.com/glycerine/g3/pkg/ast"

func (f *APIFixedpoint) DeclareRelation(name string, domain []ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return f.command.DeclareRelation(name, domain)
}

func (f *APIFixedpoint) RegisterRelation(declaration ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.RegisterFixedpointRelation(declaration)
}

func (f *APIFixedpoint) SetPredicateRepresentation(declaration ast.ASTHandle, kinds ...string) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.SetFixedpointPredicateRepresentation(declaration, kinds)
}

func (f *APIFixedpoint) DeclareVariable(name string, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := f.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	if err := f.command.DeclareFixedpointVariable(name, sort); err != nil {
		return ast.ASTHandle{}, err
	}
	declaration, err := f.command.FindFunctionDeclaration(name, nil, nil, ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	manager, err := f.command.Manager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkApp(declaration, nil)
}
