package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:157-164.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVPolynomialMultiplicationCore) MakeMultiplication(args ast.AppArguments, out ast.ASTRef, flat bool, o BVPolynomialMultiplicationOptions) error {
	_, e := p.rewrite(ast.ASTHandle{}, args, out, o, flat, true, true)
	return e
}

// The two-element source initializer borrows the operands through the call.
func (p *BVPolynomialMultiplicationCore) MakeMultiplicationPair(left, right ast.ASTHandle, out ast.ASTRef, flat bool, o BVPolynomialMultiplicationOptions) error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if p.started || p.running {
		return ast.ErrASTPending
	}
	p.pair[0] = left
	p.pair[1] = right
	return p.MakeMultiplication(ast.SliceArguments(p.pair[:]), out, flat, o)
}
