package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp has_small_coefficients/mk_pb.

import (
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *PB2BVRewriter) retainWeighted(kind int32, decl ast.ASTHandle, bound *big.Rat) bool {
	if r.pbSolver != "solver" || bound == nil || bound.Sign() < 0 || !bound.IsInt() || !bound.Num().IsUint64() || bound.Num().Uint64() > math.MaxUint32 {
		return false
	}
	if kind != ast.ASTPBOpLE && kind != ast.ASTPBOpGE && kind != ast.ASTPBOpEQ {
		return false
	}
	var sum uint32
	arity, e := r.manager.GetArity(decl)
	if e != nil {
		return false
	}
	for i := uint32(0); i < arity; i++ {
		c, e := r.pseudo.coefficient(decl, i)
		if e != nil || c.Sign() < 0 || !c.IsInt() || !c.Num().IsUint64() || c.Num().Uint64() > math.MaxUint32 {
			return false
		}
		next := sum + uint32(c.Num().Uint64())
		if next < sum {
			return false
		}
		sum = next
	}
	return true
}
