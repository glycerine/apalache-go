package base

// Source: src/muz/base/dl_rule.cpp rule_manager::is_forall.

import "github.com/glycerine/g3/pkg/ast"

// ForallQuantifier recognizes the source's accepted universal-formula shape.
// Besides a direct universal quantifier, it accepts an iff with true on either
// side. The returned handle is borrowed from expression.
func (m *RuleManager) ForallQuantifier(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := m.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	kind, err := m.manager.GetKind(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	if kind == ast.ASTApp {
		op, getErr := m.manager.GetDeclKind(expression)
		if getErr != nil {
			return ast.ASTHandle{}, false, getErr
		}
		if op == ast.ASTOpIff {
			count, countErr := m.manager.GetNumArgs(expression)
			if countErr != nil {
				return ast.ASTHandle{}, false, countErr
			}
			if count == 2 {
				left, leftErr := m.manager.GetArg(expression, 0)
				if leftErr != nil {
					return ast.ASTHandle{}, false, leftErr
				}
				right, rightErr := m.manager.GetArg(expression, 1)
				if rightErr != nil {
					return ast.ASTHandle{}, false, rightErr
				}
				switch {
				case m.manager.IsTrue(right):
					expression = left
				case m.manager.IsTrue(left):
					expression = right
				}
				kind, err = m.manager.GetKind(expression)
				if err != nil {
					return ast.ASTHandle{}, false, err
				}
			}
		}
	}
	if kind != ast.ASTQuantifier {
		return ast.ASTHandle{}, false, nil
	}
	forall, err := m.manager.IsForall(expression)
	if err != nil || !forall {
		return ast.ASTHandle{}, false, err
	}
	return expression, true, nil
}
