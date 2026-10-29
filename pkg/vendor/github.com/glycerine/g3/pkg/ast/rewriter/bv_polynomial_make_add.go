package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:149-156.

import "github.com/glycerine/g3/pkg/ast"

// MakeAddition is the original void mk_add convenience caller. A FAILED core
// result constructs a sum from the original arguments after core scope exit.
func (p *BVPolynomialAdditionCore) MakeAddition(args ast.AppArguments, out ast.ASTRef, flat bool, o BVPolynomialAdditionOptions) error {
	_, e := p.rewrite(ast.ASTHandle{}, args, out, o, flat, true, true)
	return e
}

// MakeAdditionPair publishes the source two-element initializer left to right.
// Its inline scalar array borrows both operands throughout the nested call.
func (p *BVPolynomialAdditionCore) MakeAdditionPair(left, right ast.ASTHandle, out ast.ASTRef, flat bool, o BVPolynomialAdditionOptions) error {
	if p == nil || p.self != p || p.closed {
		return ast.ErrASTHandle
	}
	if p.started || p.running {
		return ast.ErrASTPending
	}
	p.pair[0] = left
	p.pair[1] = right
	return p.MakeAddition(ast.SliceArguments(p.pair[:]), out, flat, o)
}
