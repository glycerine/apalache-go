package tactic

// Source: src/tactic/goal_util.cpp; src/tactic/goal_util.h.

import "github.com/glycerine/g3/pkg/ast"

// HasTermITE reports whether a goal contains an if-then-else expression whose
// result is not Boolean.
func HasTermITE(g *Goal) (bool, error) {
	if g == nil {
		return false, ast.ErrASTHandle
	}
	if err := g.open(); err != nil {
		return false, err
	}
	return walkGoalExpressions(g, func(h ast.ASTHandle, kind ast.ASTKind) (bool, error) {
		if kind != ast.ASTApp {
			return false, nil
		}
		family, err := g.manager.GetFamilyID(h)
		if err != nil || family != g.manager.BasicFamilyID() {
			return false, err
		}
		op, err := g.manager.GetDeclKind(h)
		if err != nil || op != ast.ASTOpIte {
			return false, err
		}
		boolean, err := probeIsBool(g.manager, h)
		return !boolean, err
	})
}
