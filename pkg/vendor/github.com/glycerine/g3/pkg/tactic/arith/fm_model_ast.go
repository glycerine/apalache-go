package arith

import "github.com/glycerine/g3/pkg/ast"

func (c *FMModelConverter) appInfo(h ast.ASTHandle) (ast.FamilyID, int32, uint32, error) {
	m := c.manager
	k, e := m.GetKind(h)
	if e != nil {
		return 0, 0, 0, e
	}
	if k != ast.ASTApp {
		return 0, 0, 0, ast.ErrASTDomain
	}
	f, e := m.GetFamilyID(h)
	if e != nil {
		return 0, 0, 0, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return 0, 0, 0, e
	}
	n, e := m.GetNumArgs(h)
	return f, op, n, e
}
func (c *FMModelConverter) notArg(h ast.ASTHandle) (bool, ast.ASTHandle, error) {
	return c.manager.IsNot(h)
}
func (c *FMModelConverter) uninterpConst(h ast.ASTHandle) (bool, error) {
	f, _, n, e := c.appInfo(h)
	if e != nil {
		return false, nil
	}
	return f == ast.NullFamilyID && n == 0, nil
}
func (c *FMModelConverter) isVariable(h, decl ast.ASTHandle) (bool, error) {
	yes, e := c.uninterpConst(h)
	if e != nil || !yes {
		return false, e
	}
	d, e := c.manager.GetDecl(h)
	return d == decl, e
}
