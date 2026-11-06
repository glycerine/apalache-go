package qe

import "github.com/glycerine/g3/pkg/ast"

func (c *arithmeticAtomCollector) boolean(expression ast.ASTHandle) (kind int32, matched bool, err error) {
	family, e := c.manager.GetFamilyID(expression)
	if e != nil {
		return 0, false, e
	}
	if family != c.manager.BasicFamilyID() {
		return 0, false, nil
	}
	kind, e = c.manager.GetDeclKind(expression)
	if e != nil {
		return 0, false, e
	}
	return kind, kind == ast.ASTOpAnd || kind == ast.ASTOpOr || kind == ast.ASTOpNot, nil
}
