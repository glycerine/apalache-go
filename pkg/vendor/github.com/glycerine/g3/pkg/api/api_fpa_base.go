package api

// Source: src/api/api_fpa.cpp:25-39.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) floatingPointManager() (*ast.ASTManager, ast.FamilyID, error) {
	m, err := c.Manager()
	if err != nil {
		return nil, ast.NullFamilyID, err
	}
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		return nil, ast.NullFamilyID, err
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		return nil, ast.NullFamilyID, err
	}
	family, err := m.RegisterFloatingPointPlugin()
	return m, family, err
}

func (c *SMTContext) floatingPointApp(kind int32, arguments ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, family, err := c.floatingPointManager()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return m.MkFamilyApp(family, kind, nil, arguments)
}
