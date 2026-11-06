package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mk_{le,ge}_tot,
// bounded_addition and tot_adder.

import (
	"math"
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

type pb2BVTotalizerLayer struct {
	formulas     []ast.ASTHandle
	coefficients []uint32
}

func (r *PB2BVRewriter) totalizer(mode pb2BVComparison, args []ast.ASTHandle, coefficients []*big.Rat, bound *big.Rat) (ast.ASTHandle, bool, error) {
	weights, k, ok := pb2BVUnsignedVector(coefficients, bound)
	if !ok || len(args) == 0 || k > 20 || mode == pb2BVEQ {
		return ast.ASTHandle{}, false, nil
	}
	var sum uint32
	for _, weight := range weights {
		if weight > math.MaxUint32-sum {
			return ast.ASTHandle{}, false, nil
		}
		sum += weight
	}
	if mode == pb2BVLE {
		if k >= sum {
			h, e := r.manager.MkTrue()
			return h, true, e
		}
		return r.totalizerLE(args, weights, k)
	}
	if k == 0 {
		h, e := r.manager.MkTrue()
		return h, true, e
	}
	if k > sum {
		h, e := r.manager.MkFalse()
		return h, true, e
	}
	return r.totalizerGE(args, weights, k)
}

func (r *PB2BVRewriter) totalizerFlip(args []ast.ASTHandle, weights []uint32, k uint32) ([]ast.ASTHandle, uint32, error) {
	flipped := make([]ast.ASTHandle, len(args))
	bound := -k
	for i, h := range args {
		var e error
		flipped[i], e = r.negate(h)
		if e != nil {
			return nil, 0, e
		}
		bound += weights[i]
	}
	return flipped, bound, nil
}

func (r *PB2BVRewriter) totalizerLE(args []ast.ASTHandle, weights []uint32, k uint32) (ast.ASTHandle, bool, error) {
	flipped, dual, e := r.totalizerFlip(args, weights, k)
	if e != nil {
		return ast.ASTHandle{}, true, e
	}
	if dual < k {
		return r.totalizerGE(flipped, weights, dual)
	}
	if k > 20 {
		return ast.ASTHandle{}, false, nil
	}
	h, e := r.boundedAddition(args, weights, k+1)
	if e != nil {
		return ast.ASTHandle{}, true, e
	}
	h, e = r.negate(h)
	return h, true, e
}

func (r *PB2BVRewriter) totalizerGE(args []ast.ASTHandle, weights []uint32, k uint32) (ast.ASTHandle, bool, error) {
	flipped, dual, e := r.totalizerFlip(args, weights, k)
	if e != nil {
		return ast.ASTHandle{}, true, e
	}
	if dual < k {
		return r.totalizerLE(flipped, weights, dual)
	}
	if k > 20 {
		return ast.ASTHandle{}, false, nil
	}
	h, e := r.boundedAddition(args, weights, k)
	return h, true, e
}

func (r *PB2BVRewriter) boundedAddition(args []ast.ASTHandle, weights []uint32, k uint32) (ast.ASTHandle, error) {
	layers := make([]pb2BVTotalizerLayer, len(args))
	for i, h := range args {
		c := weights[i]
		if c > k {
			c = k
		}
		layers[i] = pb2BVTotalizerLayer{formulas: []ast.ASTHandle{h}, coefficients: []uint32{c}}
	}
	for len(layers) > 1 {
		next := make([]pb2BVTotalizerLayer, 0, (len(layers)+1)/2)
		for i := 0; i+1 < len(layers); i += 2 {
			layer, e := r.totalizerAdder(layers[i], layers[i+1], k)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			next = append(next, layer)
		}
		if len(layers)%2 != 0 {
			next = append(next, layers[len(layers)-1])
		}
		layers = next
	}
	if len(layers) == 0 || layers[0].coefficients[len(layers[0].coefficients)-1] != k {
		return r.manager.MkFalse()
	}
	return layers[0].formulas[len(layers[0].formulas)-1], nil
}

func (r *PB2BVRewriter) totalizerAdder(left, right pb2BVTotalizerLayer, k uint32) (pb2BVTotalizerLayer, error) {
	terms := make(map[uint32][]ast.ASTHandle)
	for i := 0; i <= len(left.formulas); i++ {
		for j := 0; j <= len(right.formulas); j++ {
			if i == 0 && j == 0 || i != 0 && j != 0 && (left.coefficients[i-1] >= k || right.coefficients[j-1] >= k) {
				continue
			}
			var sum uint32
			var conjunction []ast.ASTHandle
			if i != 0 {
				sum += left.coefficients[i-1]
				conjunction = append(conjunction, left.formulas[i-1])
			}
			if j != 0 {
				sum += right.coefficients[j-1]
				conjunction = append(conjunction, right.formulas[j-1])
			}
			if sum > k {
				sum = k
			}
			h, e := r.manager.MkAndArray(conjunction)
			if e != nil {
				return pb2BVTotalizerLayer{}, e
			}
			terms[sum] = append(terms[sum], h)
		}
	}
	keys := make([]uint32, 0, len(terms))
	for key := range terms {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	result := pb2BVTotalizerLayer{coefficients: keys}
	for _, key := range keys {
		h, e := r.manager.MkOrArray(terms[key])
		if e != nil {
			return pb2BVTotalizerLayer{}, e
		}
		result.formulas = append(result.formulas, h)
	}
	return result, nil
}
