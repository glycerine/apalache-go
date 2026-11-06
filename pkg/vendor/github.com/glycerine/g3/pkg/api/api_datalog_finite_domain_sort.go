package api

// Source: src/api/api_datalog.cpp:192-218.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) datalogManager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	manager, err := c.command.Manager()
	if err != nil {
		return nil, err
	}
	if _, err = manager.RegisterDatalogPlugin(); err != nil {
		return nil, err
	}
	return manager, nil
}

func (c *SMTContext) FiniteDomainSort(name string, size uint64) (ast.ASTHandle, error) {
	manager, err := c.datalogManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := manager.Symbols.Intern(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return manager.MkDatalogFiniteSort(symbol, size)
}

func (c *SMTContext) FiniteDomainSortSize(sort ast.ASTHandle) (uint64, error) {
	manager, err := c.datalogManager()
	if err != nil {
		return 0, err
	}
	size, matched, err := manager.DatalogFiniteSortSize(sort)
	if err != nil {
		return 0, err
	}
	if !matched {
		return 0, ast.ErrASTDomain
	}
	return size, nil
}
