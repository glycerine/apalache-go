package rewriter

// Source: src/ast/rewriter/poly_rewriter_def.h:667-681.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVPolynomialRewriter) RewriteUnaryMinus(arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() {
		p.running = false
		if err != nil {
			p.fail(err)
		}
	}()
	n, e := ast.NewBitVectorPolynomialNegationNumerals(p.manager)
	if e != nil {
		return RewriteFailed, e
	}
	p.negation = n
	// The source default numeral is placed before reading the operand's sort.
	if e = n.Begin(); e != nil {
		return RewriteFailed, e
	}
	sort, e := p.manager.GetSort(arg)
	if e != nil {
		return RewriteFailed, e
	}
	p.currentSort = sort
	yes, e := n.Read(arg)
	if e != nil {
		return RewriteFailed, e
	}
	var node ast.ASTHandle
	if yes {
		if e = n.Negate(); e != nil {
			return RewriteFailed, e
		}
		if e = n.Normalize(p.currentSort); e != nil {
			return RewriteFailed, e
		}
		node, e = n.Constant(p.currentSort)
		st = RewriteDone
	} else {
		node, e = n.Product(p.currentSort, arg)
		st = Rewrite1
	}
	if e != nil {
		return st, e
	}
	if e = p.manager.SetRef(out, node); e != nil {
		return st, e
	}
	if e = n.SourceEnd(); e != nil {
		return st, e
	}
	if e = n.Close(); e != nil {
		return st, e
	}
	p.negation = nil
	return st, nil
}
