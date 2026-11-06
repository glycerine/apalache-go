package qe

import "github.com/glycerine/g3/pkg/ast"

func (n *QENNFNormalizer) liftITE(expression ast.ASTHandle) (ast.ASTHandle, bool, error) {
	ite, found, err := n.findRelevantITE(expression)
	if err != nil || !found {
		return ast.ASTHandle{}, found, err
	}
	condition, err := n.manager.GetArg(ite, 0)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	thenValue, err := n.manager.GetArg(ite, 1)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	elseValue, err := n.manager.GetArg(ite, 2)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	thenFormula, err := n.manager.NewRef(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	defer func() { _ = n.manager.DelRef(&thenFormula) }()
	elseFormula, err := n.manager.NewRef(expression)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	defer func() { _ = n.manager.DelRef(&elseFormula) }()
	if err = n.replacer.ApplySubstitution(ite, thenValue, thenFormula); err != nil {
		return ast.ASTHandle{}, false, err
	}
	if err = n.replacer.ApplySubstitution(ite, elseValue, elseFormula); err != nil {
		return ast.ASTHandle{}, false, err
	}
	thenHandle, err := n.manager.RefGet(thenFormula)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	elseHandle, err := n.manager.RefGet(elseFormula)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	raw, err := n.manager.MkIte(condition, thenHandle, elseHandle)
	if err != nil {
		return ast.ASTHandle{}, false, err
	}
	result, err := n.rewriteTheory(raw)
	return result, err == nil, err
}

func (n *QENNFNormalizer) rewriteTheory(expression ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = n.theory.Rewrite(expression, out); err != nil {
		_ = n.manager.DelRef(&out)
		return ast.ASTHandle{}, err
	}
	result, err := n.manager.RefGet(out)
	if err == nil {
		err = n.hold(result)
	}
	closeErr := n.manager.DelRef(&out)
	if err == nil {
		err = closeErr
	}
	return result, err
}

func (n *QENNFNormalizer) findRelevantITE(root ast.ASTHandle) (ast.ASTHandle, bool, error) {
	todo := []ast.ASTHandle{root}
	visited := make(map[ast.ASTHandle]bool)
	for len(todo) > 0 {
		last := len(todo) - 1
		current := todo[last]
		todo = todo[:last]
		if visited[current] {
			continue
		}
		visited[current] = true
		relevant, err := n.relevance.IsRelevant(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if !relevant {
			continue
		}
		kind, err := n.manager.GetKind(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if kind != ast.ASTApp {
			continue
		}
		family, err := n.manager.GetFamilyID(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		operation, err := n.manager.GetDeclKind(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		if family == n.manager.BasicFamilyID() && operation == ast.ASTOpIte {
			return current, true, nil
		}
		count, err := n.manager.GetNumArgs(current)
		if err != nil {
			return ast.ASTHandle{}, false, err
		}
		for index := uint32(0); index < count; index++ {
			argument, cause := n.manager.GetArg(current, index)
			if cause != nil {
				return ast.ASTHandle{}, false, cause
			}
			todo = append(todo, argument)
		}
	}
	return ast.ASTHandle{}, false, nil
}
