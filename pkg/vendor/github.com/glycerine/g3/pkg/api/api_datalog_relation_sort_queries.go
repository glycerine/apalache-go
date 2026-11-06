package api

// Source: src/api/api_datalog.cpp:154-189.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) RelationArity(sort ast.ASTHandle) (uint32, error) {
	columns, err := c.RelationColumns(sort)
	if err != nil {
		return 0, err
	}
	return uint32(len(columns)), nil
}

func (c *SMTContext) RelationColumn(sort ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	columns, err := c.RelationColumns(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(index) >= uint64(len(columns)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return columns[index], nil
}

func (c *SMTContext) RelationColumns(sort ast.ASTHandle) ([]ast.ASTHandle, error) {
	manager, err := c.datalogManager()
	if err != nil {
		return nil, err
	}
	return manager.DatalogRelationColumns(sort)
}
