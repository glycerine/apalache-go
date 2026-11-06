package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp binary_merge.

import (
	"errors"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func pb2BVUnsignedVector(coefficients []*big.Rat, bound *big.Rat) ([]uint32, uint32, bool) {
	if bound == nil || bound.Sign() < 0 || !bound.IsInt() || !bound.Num().IsUint64() || bound.Num().Uint64() > math.MaxUint32 {
		return nil, 0, false
	}
	weights := make([]uint32, len(coefficients))
	for i, c := range coefficients {
		if c.Sign() < 0 || !c.IsInt() || !c.Num().IsUint64() || c.Num().Uint64() > math.MaxUint32 {
			return nil, 0, false
		}
		weights[i] = uint32(c.Num().Uint64())
	}
	return weights, uint32(bound.Num().Uint64()), true
}

func (r *PB2BVRewriter) binaryMerge(mode pb2BVComparison, args []ast.ASTHandle, coefficients []*big.Rat, bound *big.Rat) (result ast.ASTHandle, matched bool, err error) {
	weights, k, ok := pb2BVUnsignedVector(coefficients, bound)
	if !ok {
		return result, false, nil
	}
	var sum uint32
	for _, weight := range weights {
		if weight > math.MaxUint32-sum {
			return result, false, nil
		}
		sum += weight
	}
	v, err := ast.NewASTLiteralVector(r.manager.Budget())
	if err != nil {
		return result, true, err
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	for _, h := range args {
		if err = v.Push(h); err != nil {
			return result, true, err
		}
	}
	span, err := v.Span()
	if err != nil {
		return result, true, err
	}
	switch mode {
	case pb2BVLE:
		result, err = r.network.WeightedLe(k, weights, span)
	case pb2BVGE:
		result, err = r.network.WeightedGe(k, weights, span)
	case pb2BVEQ:
		result, err = r.network.WeightedEq(k, weights, span)
	}
	if err == nil {
		err = r.syncFresh()
	}
	return result, true, err
}
