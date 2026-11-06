package api

// Source: src/api/api_datatype.cpp:556-615.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) tupleConstructor(sort ast.ASTHandle) (ast.ASTHandle, error) {
	recursive, err := c.DatatypeIsRecursive(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	count, err := c.DatatypeConstructorCount(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if recursive || count != 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return c.DatatypeConstructor(sort, 0)
}

func (c *SMTContext) TupleConstructor(sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.tupleConstructor(sort)
}

func (c *SMTContext) TupleFieldCount(sort ast.ASTHandle) (uint32, error) {
	constructor, err := c.tupleConstructor(sort)
	if err != nil {
		return 0, err
	}
	accessors, err := c.datatypeConstructorAccessors(constructor)
	return uint32(len(accessors)), err
}

func (c *SMTContext) TupleField(sort ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	constructor, err := c.tupleConstructor(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	accessors, err := c.datatypeConstructorAccessors(constructor)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(index) >= uint64(len(accessors)) {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return accessors[index], nil
}
