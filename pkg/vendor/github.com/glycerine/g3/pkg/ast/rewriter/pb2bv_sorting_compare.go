package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mk_ge/mk_le/mk_eq.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *PB2BVRewriter) sortingProfile(mode pb2BVComparison, args []ast.ASTHandle, coefficients []*big.Rat, bound *big.Rat) (ast.ASTHandle, bool, error) {
	weights, k, ok := pb2BVUnsignedVector(coefficients, bound)
	if !ok {
		return ast.ASTHandle{}, false, nil
	}
	var sum uint32
	for _, weight := range weights {
		if weight > math.MaxUint32-sum {
			return ast.ASTHandle{}, false, nil
		}
		sum += weight
	}
	switch mode {
	case pb2BVGE:
		return r.sortingGE(args, weights, k)
	case pb2BVLE:
		negated := make([]ast.ASTHandle, len(args))
		for i, h := range args {
			var e error
			negated[i], e = r.negate(h)
			if e != nil {
				return ast.ASTHandle{}, true, e
			}
		}
		if k > sum {
			// Source computes the negative dual bound and rejects its unsigned
			// mixed-radix precondition, allowing the BV encoding fallback.
			return ast.ASTHandle{}, false, nil
		}
		return r.sortingGE(negated, weights, sum-k)
	case pb2BVEQ:
		ge, matched, e := r.sortingGE(args, weights, k)
		if e != nil || !matched {
			return ast.ASTHandle{}, matched, e
		}
		le, matched, e := r.sortingProfile(pb2BVLE, args, coefficients, bound)
		if e != nil || !matched {
			return ast.ASTHandle{}, matched, e
		}
		result, e := r.manager.MkAnd(ge, le)
		return result, true, e
	}
	return ast.ASTHandle{}, false, nil
}
