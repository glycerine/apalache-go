package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp mixed-radix mk_ge loop.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
)

func (r *PB2BVRewriter) sortingNetwork(values []ast.ASTHandle) (output []ast.ASTHandle, err error) {
	v, e := ast.NewASTLiteralVector(r.manager.Budget())
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, v.Close()) }()
	out, e := ast.NewASTLiteralVector(r.manager.Budget())
	if e != nil {
		return nil, e
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	for _, h := range values {
		if e = v.Push(h); e != nil {
			return nil, e
		}
	}
	span, e := v.Span()
	if e != nil {
		return nil, e
	}
	if e = r.network.SortingWeightedGE(uint32(len(values)), span, out); e != nil {
		return nil, e
	}
	for i := uint32(0); i < uint32(len(values)); i++ {
		h, x := out.At(i)
		if x != nil {
			return nil, x
		}
		output = append(output, h)
	}
	return output, nil
}

func (r *PB2BVRewriter) sortingGE(args []ast.ASTHandle, coefficients []uint32, bound uint32) (result ast.ASTHandle, matched bool, err error) {
	// The final mixed-radix base is bound+1 and must fit in uint32.
	if bound == math.MaxUint32 {
		return result, false, nil
	}
	basis, ok := pb2BVCreateSortingBasis(coefficients)
	if !ok {
		return result, false, nil
	}
	workCoefficients := append([]uint32(nil), coefficients...)
	result, err = r.manager.MkTrue()
	if err != nil {
		return result, true, err
	}
	carry := []ast.ASTHandle(nil)
	basis = append(basis, bound+1)
	for _, radix := range basis {
		digit := bound % radix
		bound /= radix
		for i, coefficient := range workCoefficients {
			for count := uint32(0); count < coefficient%radix; count++ {
				carry = append(carry, args[i])
			}
			workCoefficients[i] = coefficient / radix
		}
		output, e := r.sortingNetwork(carry)
		if e != nil {
			return result, true, e
		}
		greater, e := r.sortingModGE(output, radix, digit+1)
		if e != nil {
			return result, true, e
		}
		greaterEqual, e := r.sortingModGE(output, radix, digit)
		if e != nil {
			return result, true, e
		}
		result, e = r.sortingAnd(greaterEqual, result)
		if e != nil {
			return result, true, e
		}
		result, e = r.sortingOr(greater, result)
		if e != nil {
			return result, true, e
		}
		carry = carry[:0]
		for i := radix - 1; uint64(i) < uint64(len(output)); i += radix {
			carry = append(carry, output[i])
		}
	}
	if err = r.syncFresh(); err != nil {
		return result, true, err
	}
	return result, true, nil
}
