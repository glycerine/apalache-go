package arith

import (
	"github.com/glycerine/g3/pkg/ast"
)

func (i *forwardTrigInvocation) findUnsafe() error {
	m := i.owner.manager
	_, e := visitArithmeticGoal(i.goal, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return false, nil
		}
		fid, x := m.GetFamilyID(h)
		if x != nil {
			return false, x
		}
		op, x := m.GetDeclKind(h)
		if x != nil {
			return false, x
		}
		if fid == i.owner.arithmetic && (op == forwardTrigSin || op == forwardTrigCos) {
			return false, nil
		}
		n, x := m.GetNumArgs(h)
		if x != nil {
			return false, x
		}
		for j := uint32(0); j < n; j++ {
			a, x := m.GetArg(h, j)
			if x != nil {
				return false, x
			}
			i.unsafe[a] = true
		}
		return false, nil
	})
	return e
}
func (i *forwardTrigInvocation) isUninterpretedConst(h ast.ASTHandle) (bool, error) {
	m := i.owner.manager
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil || fid != ast.NullFamilyID {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	return n == 0, e
}
