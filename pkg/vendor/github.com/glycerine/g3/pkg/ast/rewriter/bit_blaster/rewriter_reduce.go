package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:211-321.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

type bitVectorUnary func([]ast.ASTHandle) ([]ast.ASTRef, error)
type bitVectorBinary func([]ast.ASTHandle, []ast.ASTHandle) ([]ast.ASTRef, error)
type bitVectorPredicate func([]ast.ASTHandle, []ast.ASTHandle, ast.ASTRef) error

func (r *BitBlasterRewriter) reduceUnary(term ast.ASTHandle, out ast.ASTRef, operation bitVectorUnary) (result error) {
	bits, err := r.getBits(term)
	if err != nil {
		return err
	}
	var reduced []ast.ASTRef
	defer func() { result = errors.Join(result, releaseGroups(r.manager, bits, reduced)) }()
	handles, err := r.blaster.handles(bits)
	if err != nil {
		return err
	}
	reduced, err = operation(handles)
	if err != nil {
		return err
	}
	return r.setBitVector(reduced, out)
}

func (r *BitBlasterRewriter) reduceBinary(left, right ast.ASTHandle, out ast.ASTRef, operation bitVectorBinary) (result error) {
	leftRefs, err := r.getBits(left)
	if err != nil {
		return err
	}
	var rightRefs, reduced []ast.ASTRef
	defer func() { result = errors.Join(result, releaseGroups(r.manager, leftRefs, rightRefs, reduced)) }()
	rightRefs, err = r.getBits(right)
	if err != nil {
		return err
	}
	leftBits, err := r.blaster.handles(leftRefs)
	if err != nil {
		return err
	}
	rightBits, err := r.blaster.handles(rightRefs)
	if err != nil {
		return err
	}
	reduced, err = operation(leftBits, rightBits)
	if err != nil {
		return err
	}
	return r.setBitVector(reduced, out)
}

func (r *BitBlasterRewriter) reducePredicate(left, right ast.ASTHandle, out ast.ASTRef, operation bitVectorPredicate) (result error) {
	leftRefs, err := r.getBits(left)
	if err != nil {
		return err
	}
	var rightRefs []ast.ASTRef
	defer func() { result = errors.Join(result, releaseGroups(r.manager, leftRefs, rightRefs)) }()
	rightRefs, err = r.getBits(right)
	if err != nil {
		return err
	}
	leftBits, err := r.blaster.handles(leftRefs)
	if err != nil {
		return err
	}
	rightBits, err := r.blaster.handles(rightRefs)
	if err != nil {
		return err
	}
	return operation(leftBits, rightBits, out)
}

func (r *BitBlasterRewriter) reduceReduction(term ast.ASTHandle, out ast.ASTRef, and bool) (result error) {
	bits, err := r.getBits(term)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, releaseRefs(r.manager, bits)) }()
	handles, err := r.blaster.handles(bits)
	if err != nil {
		return err
	}
	bit, err := r.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.manager.DelRef(&bit)) }()
	if and {
		err = r.blaster.MkReductionAnd(handles, bit)
	} else {
		err = r.blaster.MkReductionOr(handles, bit)
	}
	if err != nil {
		return err
	}
	return r.setBitVector([]ast.ASTRef{bit}, out)
}

func (r *BitBlasterRewriter) reduceAC(args ast.AppArguments, out ast.ASTRef, operation bitVectorBinary) (result error) {
	if args.Count() == 0 {
		return ast.ErrASTDomain
	}
	first, err := args.At(0)
	if err != nil {
		return err
	}
	current, err := r.manager.NewRef(first)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, r.manager.DelRef(&current)) }()
	for i := uint32(1); i < args.Count(); i++ {
		right, err := args.At(i)
		if err != nil {
			return err
		}
		left, err := r.manager.RefGet(current)
		if err != nil {
			return err
		}
		next, err := r.manager.NewRef(ast.ASTHandle{})
		if err != nil {
			return err
		}
		if err = r.reduceBinary(left, right, next, operation); err != nil {
			_ = r.manager.DelRef(&next)
			return err
		}
		if err = r.manager.DelRef(&current); err != nil {
			_ = r.manager.DelRef(&next)
			return err
		}
		current = next
	}
	return r.manager.AssignRef(out, current)
}

func (r *BitBlasterRewriter) reduceITE(condition, thenTerm, elseTerm ast.ASTHandle, out ast.ASTRef) (result error) {
	thenRefs, err := r.getBits(thenTerm)
	if err != nil {
		return err
	}
	var elseRefs, reduced []ast.ASTRef
	defer func() { result = errors.Join(result, releaseGroups(r.manager, thenRefs, elseRefs, reduced)) }()
	elseRefs, err = r.getBits(elseTerm)
	if err != nil {
		return err
	}
	thenBits, err := r.blaster.handles(thenRefs)
	if err != nil {
		return err
	}
	elseBits, err := r.blaster.handles(elseRefs)
	if err != nil {
		return err
	}
	reduced, err = r.blaster.MkMultiplexer(condition, thenBits, elseBits)
	if err != nil {
		return err
	}
	return r.setBitVector(reduced, out)
}

func (r *BitBlasterRewriter) reduceConcat(args ast.AppArguments, out ast.ASTRef) (result error) {
	var combined []ast.ASTRef
	defer func() { result = errors.Join(result, releaseRefs(r.manager, combined)) }()
	for i := args.Count(); i > 0; {
		i--
		arg, err := args.At(i)
		if err != nil {
			return err
		}
		bits, err := r.getBits(arg)
		if err != nil {
			return err
		}
		combined = append(combined, bits...)
	}
	return r.setBitVector(combined, out)
}

func (r *BitBlasterRewriter) reduceExtract(low, high uint32, term ast.ASTHandle, out ast.ASTRef) (result error) {
	bits, err := r.getBits(term)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, releaseRefs(r.manager, bits)) }()
	if low > high || uint64(high) >= uint64(len(bits)) {
		return ast.ErrASTDomain
	}
	selected := bits[low : high+1]
	return r.setBitVector(selected, out)
}

func (r *BitBlasterRewriter) reduceSignExtend(extra uint32, term ast.ASTHandle, out ast.ASTRef) (result error) {
	bits, err := r.getBits(term)
	if err != nil {
		return err
	}
	var extended []ast.ASTRef
	defer func() { result = errors.Join(result, releaseGroups(r.manager, bits, extended)) }()
	handles, err := r.blaster.handles(bits)
	if err != nil {
		return err
	}
	extended, err = r.blaster.MkSignExtend(handles, int(extra))
	if err != nil {
		return err
	}
	return r.setBitVector(extended, out)
}
