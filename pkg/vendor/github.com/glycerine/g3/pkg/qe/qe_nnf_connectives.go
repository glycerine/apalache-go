package qe

import (
	"github.com/glycerine/g3/pkg/ast"
	astrewriter "github.com/glycerine/g3/pkg/ast/rewriter"
)

func (n *QENNFNormalizer) gate(operation int32, arguments ...ast.ASTHandle) (ast.ASTHandle, error) {
	out, err := n.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = astrewriter.SimplifyBooleanGateDefault(n.manager, operation, arguments, out); err != nil {
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

func (n *QENNFNormalizer) andOr(expression ast.ASTHandle, positive bool, conjunction bool) (ast.ASTHandle, error) {
	count, err := n.manager.GetNumArgs(expression)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	arguments := make([]ast.ASTHandle, count)
	for index := uint32(0); index < count; index++ {
		argument, cause := n.manager.GetArg(expression, index)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		arguments[index], cause = n.toNNF(argument, positive)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
	}
	operation := int32(ast.ASTOpOr)
	if positive == conjunction {
		operation = ast.ASTOpAnd
	}
	return n.gate(operation, arguments...)
}

func (n *QENNFNormalizer) equivalence(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	left, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := n.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	lp, err := n.toNNF(left, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ln, err := n.toNNF(left, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rp, err := n.toNNF(right, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rn, err := n.toNNF(right, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if positive {
		bothTrue, cause := n.gate(ast.ASTOpAnd, lp, rp)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		bothFalse, cause := n.gate(ast.ASTOpAnd, ln, rn)
		if cause != nil {
			return ast.ASTHandle{}, cause
		}
		return n.gate(ast.ASTOpOr, bothTrue, bothFalse)
	}
	leftCase, err := n.gate(ast.ASTOpOr, lp, rp)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	rightCase, err := n.gate(ast.ASTOpOr, ln, rn)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return n.gate(ast.ASTOpAnd, leftCase, rightCase)
}

func (n *QENNFNormalizer) conditional(expression ast.ASTHandle, positive bool) (ast.ASTHandle, error) {
	condition, err := n.manager.GetArg(expression, 0)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	thenValue, err := n.manager.GetArg(expression, 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	elseValue, err := n.manager.GetArg(expression, 2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	cp, err := n.toNNF(condition, true)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	cn, err := n.toNNF(condition, false)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	th, err := n.toNNF(thenValue, positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	el, err := n.toNNF(elseValue, positive)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	left, err := n.gate(ast.ASTOpAnd, cp, th)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	right, err := n.gate(ast.ASTOpAnd, cn, el)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return n.gate(ast.ASTOpOr, left, right)
}
