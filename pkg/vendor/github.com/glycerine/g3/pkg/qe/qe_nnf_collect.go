package qe

import "github.com/glycerine/g3/pkg/ast"

func (n *QENNFNormalizer) collectAtoms(root ast.ASTHandle) (positive, negative []ast.ASTHandle, err error) {
	todo := []ast.ASTHandle{root}
	visited := make(map[ast.ASTHandle]bool)
	positiveSeen := make(map[ast.ASTHandle]bool)
	negativeSeen := make(map[ast.ASTHandle]bool)
	for len(todo) > 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		relevant, cause := n.relevance.IsRelevant(current)
		if cause != nil {
			return nil, nil, cause
		}
		if !relevant {
			continue
		}
		kind, cause := n.manager.GetKind(current)
		if cause != nil || kind != ast.ASTApp {
			continue
		}
		family, cause := n.manager.GetFamilyID(current)
		if cause != nil {
			return nil, nil, cause
		}
		operation, cause := n.manager.GetDeclKind(current)
		if cause != nil {
			return nil, nil, cause
		}
		if family == n.manager.BasicFamilyID() && (operation == ast.ASTOpAnd || operation == ast.ASTOpOr) {
			count, cause := n.manager.GetNumArgs(current)
			if cause != nil {
				return nil, nil, cause
			}
			for index := uint32(0); index < count; index++ {
				argument, cause := n.manager.GetArg(current, index)
				if cause != nil {
					return nil, nil, cause
				}
				todo = append(todo, argument)
			}
			continue
		}
		if family == n.manager.BasicFamilyID() && operation == ast.ASTOpNot {
			atom, cause := n.manager.GetArg(current, 0)
			if cause != nil {
				return nil, nil, cause
			}
			kind, cause = n.manager.GetKind(atom)
			if cause != nil {
				return nil, nil, cause
			}
			if kind == ast.ASTApp && !negativeSeen[atom] {
				negativeSeen[atom] = true
				negative = append(negative, atom)
			}
			continue
		}
		if !positiveSeen[current] {
			positiveSeen[current] = true
			positive = append(positive, current)
		}
	}
	return positive, negative, nil
}
