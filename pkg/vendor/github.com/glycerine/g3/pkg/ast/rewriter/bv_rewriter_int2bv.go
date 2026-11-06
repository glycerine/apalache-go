package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1343-1361;
// src/ast/rewriter/bv_rewriter.h:131.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteInt2BV(width uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	n, e := ast.NewBitVectorInt2BVNumerals(p.manager)
	p.int2bv = n
	if e != nil {
		return st, e
	}
	var integer bool
	matched, e := n.Read(p.arithFamily, arg, &integer)
	if e != nil {
		return st, e
	}
	var result ast.ASTHandle
	if matched {
		// The original reads is_int but does not test it before normalization.
		if e = n.Normalize(width); e != nil {
			return st, e
		}
		result, e = n.Constant(0, width)
		if e != nil {
			return st, e
		}
		st = RewriteDone
	} else {
		var base ast.ASTHandle
		matched, e = p.manager.BitVectorToIntValue(arg, &base)
		if e != nil {
			return st, e
		}
		if matched {
			size, e := p.manager.BitVectorSize(base)
			if e != nil {
				return st, e
			}
			if size == width {
				result = base
				st = RewriteDone
			}
		}
	}
	if st != RewriteFailed {
		if err = p.manager.SetRef(out, result); err != nil {
			return
		}
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.int2bv = nil
	return
}
