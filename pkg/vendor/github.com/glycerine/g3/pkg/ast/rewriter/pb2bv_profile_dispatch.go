package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mk_le_ge profile dispatch.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *PB2BVRewriter) profileWeighted(mode pb2BVComparison, args []ast.ASTHandle, coefficients []*big.Rat, bound *big.Rat) (ast.ASTHandle, bool, error) {
	switch r.pbSolver {
	case "totalizer":
		return r.totalizer(mode, args, coefficients, bound)
	case "binary_merge":
		return r.binaryMerge(mode, args, coefficients, bound)
	case "sorting":
		return r.sortingProfile(mode, args, coefficients, bound)
	case "segmented":
		return ast.ASTHandle{}, true, ast.ErrASTDomain
	default:
		return ast.ASTHandle{}, false, nil
	}
}
