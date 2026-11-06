package api

// Source: src/api/api_datalog.cpp:197-277, 443-479.

import "github.com/glycerine/g3/pkg/ast"

func (f *APIFixedpoint) AddRule(formula ast.ASTHandle, name string) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointRule(formula, name)
}

func (f *APIFixedpoint) AddRuleWithBound(formula ast.ASTHandle, name string, bound uint32) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointRuleWithBound(formula, name, bound)
}

func (f *APIFixedpoint) UpdateRule(formula ast.ASTHandle, name string) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.UpdateFixedpointRule(formula, name)
}

func (f *APIFixedpoint) AddFact(declaration ast.ASTHandle, values []ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointFact(declaration, values)
}

func (f *APIFixedpoint) AddTableFact(declaration ast.ASTHandle, values []uint32) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AddFixedpointTableFact(declaration, values)
}

func (f *APIFixedpoint) Assert(formula ast.ASTHandle) error {
	if err := f.open(); err != nil {
		return err
	}
	return f.command.AssertFixedpoint(formula)
}
