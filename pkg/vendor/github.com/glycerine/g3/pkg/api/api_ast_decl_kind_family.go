package api

import "github.com/glycerine/g3/pkg/ast"

// apiDeclarationFamilyOperation returns the declaration behind either a
// declaration or application view and its family-local operation code.
func (c *SMTContext) apiDeclarationFamilyOperation(value ast.ASTHandle) (*ast.ASTManager, ast.FamilyID, int32, error) {
	manager, err := c.Manager()
	if err != nil {
		return nil, ast.NullFamilyID, 0, err
	}
	kind, err := manager.GetKind(value)
	if err != nil {
		return nil, ast.NullFamilyID, 0, err
	}
	if kind == ast.ASTApp {
		value, err = manager.GetDecl(value)
		if err != nil {
			return nil, ast.NullFamilyID, 0, err
		}
	} else if kind != ast.ASTFuncDecl {
		return nil, ast.NullFamilyID, 0, ast.ErrASTDomain
	}
	family, err := manager.GetFamilyID(value)
	if err != nil {
		return nil, ast.NullFamilyID, 0, err
	}
	operation, err := manager.GetDeclKind(value)
	return manager, family, operation, err
}
