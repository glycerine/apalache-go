package qe

import "github.com/glycerine/g3/pkg/ast"

func (n *QENNFNormalizer) normalizeLiterals(expression ast.ASTHandle) (ast.ASTHandle, error) {
	cache := make(map[ast.ASTHandle]ast.ASTHandle)
	var visit func(ast.ASTHandle) (ast.ASTHandle, error)
	visit = func(current ast.ASTHandle) (ast.ASTHandle, error) {
		if result, found := cache[current]; found {
			return result, nil
		}
		relevant, err := n.relevance.IsRelevant(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if !relevant {
			cache[current] = current
			return current, nil
		}
		kind, err := n.manager.GetKind(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if kind != ast.ASTApp {
			cache[current] = current
			return current, nil
		}
		family, err := n.manager.GetFamilyID(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		operation, err := n.manager.GetDeclKind(current)
		if err != nil {
			return ast.ASTHandle{}, err
		}
		var result ast.ASTHandle
		if family == n.manager.BasicFamilyID() && (operation == ast.ASTOpAnd || operation == ast.ASTOpOr) {
			count, cause := n.manager.GetNumArgs(current)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			arguments := make([]ast.ASTHandle, count)
			for index := uint32(0); index < count; index++ {
				argument, cause := n.manager.GetArg(current, index)
				if cause == nil {
					arguments[index], cause = visit(argument)
				}
				if cause != nil {
					return ast.ASTHandle{}, cause
				}
			}
			result, err = n.gate(operation, arguments...)
		} else if family == n.manager.BasicFamilyID() && operation == ast.ASTOpNot {
			atom, cause := n.manager.GetArg(current, 0)
			if cause != nil {
				return ast.ASTHandle{}, cause
			}
			result, err = n.atoms.NormalizeAtom(atom, false)
		} else {
			result, err = n.atoms.NormalizeAtom(current, true)
		}
		if err != nil {
			return ast.ASTHandle{}, err
		}
		if err = n.hold(result); err != nil {
			return ast.ASTHandle{}, err
		}
		cache[current] = result
		return result, nil
	}
	return visit(expression)
}
