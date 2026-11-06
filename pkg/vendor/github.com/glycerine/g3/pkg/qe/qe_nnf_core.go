package qe

import "github.com/glycerine/g3/pkg/ast"

func (n *QENNFNormalizer) toNNF(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	key := qeNNFKey{expression, positive}
	if value, found := n.memo[key]; found {
		return value, nil
	}
	relevant, err := n.relevance.IsRelevant(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !relevant {
		value := expression
		if !positive {
			value, err = n.gate(ast.ASTOpNot, expression)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		return n.remember(key, value)
	}
	kind, err := n.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	family, err := n.manager.GetFamilyID(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	operation, err := n.manager.GetDeclKind(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var value ast.ASTHandle
	if family == n.manager.BasicFamilyID() {
		switch operation {
		case ast.ASTOpAnd, ast.ASTOpOr:
			value, err = n.andOr(expression, positive, operation == ast.ASTOpAnd)
		case ast.ASTOpNot:
			var argument ast.ASTHandle
			argument, err = n.manager.GetArg(expression, 0)
			if err == nil {
				value, err = n.toNNF(argument, !positive)
			}
		case ast.ASTOpIte:
			value, err = n.conditional(expression, positive)
		case ast.ASTOpIff:
			value, err = n.equivalence(expression, positive)
		case ast.ASTOpXor:
			value, err = n.equivalence(expression, !positive)
		case ast.ASTOpImplies:
			value, err = n.implication(expression, positive)
		case ast.ASTOpEq:
			var boolean bool
			boolean, err = n.booleanEquality(expression)
			if err == nil && boolean {
				value, err = n.equivalence(expression, positive)
			} else if err == nil {
				value, err = n.atomic(expression, positive)
			}
		default:
			value, err = n.atomic(expression, positive)
		}
	} else {
		value, err = n.atomic(expression, positive)
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return n.remember(key, value)
}

func (n *QENNFNormalizer) implication(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	left, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := n.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	l, err := n.toNNF(left, !positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := n.toNNF(right, positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	operation := int32(ast.ASTOpAnd)
	if positive {
		operation = ast.ASTOpOr
	}
	return n.gate(operation, l, r)
}

func (n *QENNFNormalizer) booleanEquality(expression ast.ASTHandle) (bool, error) {
	argument, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return false, err
	}
	sort, err := n.manager.GetSort(argument)
	if err != nil {
		return false, err
	}
	boolean, err := n.manager.MkBoolSort()
	return sort == boolean, err
}

func (n *QENNFNormalizer) atomic(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	if lifted, found, err := n.liftITE(expression); err != nil {
		return ast.ASTHandle{}, err
	} else if found {
		return n.toNNF(lifted, positive)
	}
	if positive {
		return expression, nil
	}
	return n.gate(ast.ASTOpNot, expression)
}
