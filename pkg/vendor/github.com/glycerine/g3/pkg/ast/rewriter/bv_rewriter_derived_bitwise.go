package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1971-1998.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) RewriteAND(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteNegatedBitwise(args, out, true, true, false)
}
func (p *BVRewriter) RewriteNAND(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteNegatedBitwise(args, out, true, false, false)
}
func (p *BVRewriter) RewriteNOR(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteNegatedBitwise(args, out, false, true, false)
}
func (p *BVRewriter) RewriteXNOR(args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return p.rewriteNegatedBitwise(args, out, false, true, true)
}

// Source ptr_buffer elements are borrowed. A stopped body retains the actual
// buffer in its owning BV object; only successful return runs its source end.
func (p *BVRewriter) rewriteNegatedBitwise(args ast.AppArguments, out ast.ASTRef, negateChildren, negateResult, xor bool) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	if negateChildren {
		for i := uint32(0); i < args.Count(); i++ {
			arg, e := args.At(i)
			if e != nil {
				return st, e
			}
			child, e := m.MkBitVectorNot(arg)
			if e != nil {
				return st, e
			}
			p.bitwiseArgs = append(p.bitwiseArgs, child)
		}
		if ast.ASTSourceDebug && negateResult && args.Count() != uint32(len(p.bitwiseArgs)) {
			return st, ast.ErrASTDomain
		}
		args = ast.SliceArguments(p.bitwiseArgs)
	}
	var result ast.ASTHandle
	if xor {
		result, err = m.MkBitVectorXorArguments(args)
	} else {
		result, err = m.MkBitVectorOrArguments(args)
	}
	if err != nil {
		return
	}
	if negateResult {
		result, err = m.MkBitVectorNot(result)
		if err != nil {
			return
		}
	}
	st = Rewrite2
	if negateChildren && negateResult {
		st = Rewrite3
	}
	if err = m.SetRef(out, result); err != nil {
		return
	}
	if negateChildren {
		clear(p.bitwiseArgs)
		p.bitwiseArgs = nil
	}
	return
}
