package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2636-2655.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteMkBV(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	if !p.polynomial.config.bitVector.MkBVToNumeral {
		return
	}
	m := p.manager
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return st, e
		}
		if !m.IsTrue(arg) && !m.IsFalse(arg) {
			return
		}
	}
	n, e := ast.NewBitVectorMkBVNumerals(m)
	p.mkbv = n
	if e != nil {
		return st, e
	}
	for i := args.Count(); i > 0; {
		i--
		if e = n.MultiplyTwo(); e != nil {
			return st, e
		}
		arg, e := args.At(i)
		if e != nil {
			return st, e
		}
		if m.IsTrue(arg) {
			if e = n.Increment(); e != nil {
				return st, e
			}
		}
	}
	h, e := n.Constant(0, args.Count())
	if e != nil {
		return st, e
	}
	st = RewriteDone
	if err = m.SetRef(out, h); err != nil {
		return
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.mkbv = nil
	return
}
