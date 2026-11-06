package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mod_ge.

import "github.com/glycerine/g3/pkg/ast"

func (r *PB2BVRewriter) sortingModGE(output []ast.ASTHandle, radix, limit uint32) (ast.ASTHandle, error) {
	if limit == radix {
		return r.manager.MkFalse()
	}
	if limit == 0 {
		return r.manager.MkTrue()
	}
	terms := make([]ast.ASTHandle, 0, (len(output)+int(radix)-1)/int(radix))
	for start := uint32(0); uint64(start)+uint64(limit)-1 < uint64(len(output)); start += radix {
		term := output[start+limit-1]
		if uint64(start)+uint64(radix)-1 < uint64(len(output)) {
			not, e := r.negate(output[start+radix-1])
			if e != nil {
				return ast.ASTHandle{}, e
			}
			term, e = r.manager.MkAnd(term, not)
			if e != nil {
				return ast.ASTHandle{}, e
			}
		}
		terms = append(terms, term)
	}
	return r.manager.MkOrArray(terms)
}
