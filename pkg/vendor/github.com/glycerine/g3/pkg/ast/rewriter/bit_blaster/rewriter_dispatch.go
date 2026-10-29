package bit_blaster

// Source: src/ast/rewriter/bit_blaster/bit_blaster_rewriter.cpp:323-527.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	parent "github.com/glycerine/g3/pkg/ast/rewriter"
)

var ErrUnsupportedOperator = parent.NewRewriterException("operator is not supported, you must simplify the goal before applying bit-blasting")

func (r *BitBlasterRewriter) reduceNumeral(decl ast.ASTHandle, out ast.ASTRef) (result error) {
	term, err := r.manager.RewriterMakeApp(decl, ast.SliceArguments(nil))
	if err != nil {
		return err
	}
	text, matched, err := r.manager.BitVectorNumeralText(term)
	if err != nil {
		return err
	}
	if !matched {
		return ast.ErrASTDomain
	}
	width, err := r.manager.BitVectorSize(term)
	if err != nil {
		return err
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return ast.ErrASTDomain
	}
	bits, err := r.blaster.NumToBits(value, int(width))
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, releaseRefs(r.manager, bits)) }()
	return r.setBitVector(bits, out)
}

func (r *BitBlasterRewriter) declIntParameter(decl ast.ASTHandle, index int) (value uint32, result error) {
	params, err := r.manager.GetDeclParametersCopy(decl)
	if err != nil {
		return 0, err
	}
	defer func() {
		for i := range params {
			result = errors.Join(result, r.manager.DelParameter(&params[i]))
		}
	}()
	if index < 0 || index >= len(params) {
		return 0, ast.ErrASTDomain
	}
	v, err := r.manager.ParameterInt(params[index])
	if err != nil || v < 0 {
		if err != nil {
			return 0, err
		}
		return 0, ast.ErrASTDomain
	}
	return uint32(v), nil
}

func dispatchArgument(args ast.AppArguments, index, arity uint32) (ast.ASTHandle, error) {
	if args.Count() != arity {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return args.At(index)
}

func dispatchPair(args ast.AppArguments) (ast.ASTHandle, ast.ASTHandle, error) {
	left, err := dispatchArgument(args, 0, 2)
	if err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, err
	}
	right, err := args.At(1)
	return left, right, err
}

func (r *BitBlasterRewriter) blastBVTerm(term ast.ASTHandle, out ast.ASTRef) (result error) {
	width, err := r.manager.BitVectorSize(term)
	if err != nil {
		return err
	}
	bits := make([]ast.ASTRef, 0, width)
	defer func() { result = errors.Join(result, releaseRefs(r.manager, bits)) }()
	for i := uint32(0); i < width; i++ {
		bit, err := r.manager.MkBitVectorBit2Bool(i, term)
		if err != nil {
			return err
		}
		ref, err := r.manager.NewRef(bit)
		if err != nil {
			return err
		}
		bits = append(bits, ref)
	}
	return r.setBitVector(bits, out)
}

func (r *BitBlasterRewriter) reduceApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (parent.RewriteStatus, error) {
	if err := r.traceApplication(decl, args, false); err != nil {
		return parent.RewriteFailed, err
	}
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return parent.RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return parent.RewriteFailed, err
	}

	if args.Count() == 0 && family == ast.NullFamilyID {
		rangeSort, err := r.manager.GetRange(decl)
		if err != nil {
			return parent.RewriteFailed, err
		}
		isBV, err := r.manager.IsBitVectorSort(rangeSort)
		if err != nil {
			return parent.RewriteFailed, err
		}
		if isBV {
			return parent.RewriteDone, r.makeConstant(decl, out)
		}
	}

	if family == r.manager.BasicFamilyID() && kind == ast.ASTOpEq {
		left, right, err := dispatchPair(args)
		if err != nil {
			return parent.RewriteFailed, err
		}
		isBV, err := r.manager.IsBitVector(left)
		if err != nil || !isBV {
			return parent.RewriteFailed, err
		}
		return parent.RewriteDone, r.reducePredicate(left, right, out, r.blaster.MkEq)
	}
	if family == r.manager.BasicFamilyID() && kind == ast.ASTOpIte {
		condition, err := dispatchArgument(args, 0, 3)
		if err != nil {
			return parent.RewriteFailed, err
		}
		thenTerm, err := args.At(1)
		if err != nil {
			return parent.RewriteFailed, err
		}
		elseTerm, err := args.At(2)
		if err != nil {
			return parent.RewriteFailed, err
		}
		isBV, err := r.manager.IsBitVector(thenTerm)
		if err != nil || !isBV {
			return parent.RewriteFailed, err
		}
		return parent.RewriteDone, r.reduceITE(condition, thenTerm, elseTerm, out)
	}

	if family == r.family {
		switch kind {
		case 0:
			if args.Count() != 0 {
				return parent.RewriteFailed, ast.ErrASTDomain
			}
			return parent.RewriteDone, r.reduceNumeral(decl, out)
		case 4:
			if !r.params.BlastAdd {
				return parent.RewriteFailed, nil
			}
			return parent.RewriteDone, r.reduceAC(args, out, r.blaster.MkAdder)
		case 6:
			if !r.params.BlastMul {
				return parent.RewriteFailed, nil
			}
			return parent.RewriteDone, r.reduceAC(args, out, r.blaster.MkMultiplier)
		case 7, 8, 9, 10, 11:
			if r.params.BlastMul {
				return parent.RewriteFailed, ErrUnsupportedOperator
			}
			return parent.RewriteFailed, nil
		case 12, 13, 14, 15, 16:
			return parent.RewriteFailed, nil
		case 17, 18, 19, 20, 21:
			if !r.params.BlastMul {
				return parent.RewriteFailed, nil
			}
			left, right, err := dispatchPair(args)
			if err != nil {
				return parent.RewriteFailed, err
			}
			operations := map[int32]bitVectorBinary{17: r.blaster.MkSignedDiv, 18: r.blaster.MkUnsignedDiv, 19: r.blaster.MkSignedRem, 20: r.blaster.MkUnsignedRem, 21: r.blaster.MkSignedMod}
			return parent.RewriteDone, r.reduceBinary(left, right, out, operations[kind])
		case 22, 23:
			left, right, err := dispatchPair(args)
			if err != nil {
				return parent.RewriteFailed, err
			}
			operation := r.blaster.MkUnsignedLE
			if kind == 23 {
				operation = r.blaster.MkSignedLE
			}
			return parent.RewriteDone, r.reducePredicate(left, right, out, operation)
		case 31:
			return parent.RewriteDone, r.reduceAC(args, out, r.blaster.MkOr)
		case 32:
			arg, err := dispatchArgument(args, 0, 1)
			if err != nil {
				return parent.RewriteFailed, err
			}
			return parent.RewriteDone, r.reduceUnary(arg, out, r.blaster.MkNot)
		case 33:
			return parent.RewriteDone, r.reduceAC(args, out, r.blaster.MkXor)
		case 37:
			return parent.RewriteDone, r.reduceConcat(args, out)
		case 38:
			arg, err := dispatchArgument(args, 0, 1)
			if err != nil {
				return parent.RewriteFailed, err
			}
			extra, err := r.declIntParameter(decl, 0)
			if err != nil {
				return parent.RewriteFailed, err
			}
			return parent.RewriteDone, r.reduceSignExtend(extra, arg, out)
		case 40:
			arg, err := dispatchArgument(args, 0, 1)
			if err != nil {
				return parent.RewriteFailed, err
			}
			high, err := r.declIntParameter(decl, 0)
			if err != nil {
				return parent.RewriteFailed, err
			}
			low, err := r.declIntParameter(decl, 1)
			if err != nil {
				return parent.RewriteFailed, err
			}
			return parent.RewriteDone, r.reduceExtract(low, high, arg, out)
		case 42, 43:
			arg, err := dispatchArgument(args, 0, 1)
			if err != nil {
				return parent.RewriteFailed, err
			}
			return parent.RewriteDone, r.reduceReduction(arg, out, kind == 43)
		case 45, 46, 47, 50, 51:
			left, right, err := dispatchPair(args)
			if err != nil {
				return parent.RewriteFailed, err
			}
			operations := map[int32]bitVectorBinary{45: r.blaster.MkShiftLeft, 46: r.blaster.MkLogicalShiftRight, 47: r.blaster.MkArithmeticShiftRight, 50: r.blaster.MkExtendedRotateLeft, 51: r.blaster.MkExtendedRotateRight}
			return parent.RewriteDone, r.reduceBinary(left, right, out, operations[kind])
		case 52, 53, 54:
			left, right, err := dispatchPair(args)
			if err != nil {
				return parent.RewriteFailed, err
			}
			operations := map[int32]bitVectorPredicate{52: r.blaster.MkUnsignedMulNoOverflow, 53: r.blaster.MkSignedMulNoOverflow, 54: r.blaster.MkSignedMulNoUnderflow}
			return parent.RewriteDone, r.reducePredicate(left, right, out, operations[kind])
		case 55, 56, 57, 58:
			return parent.RewriteFailed, nil
		default:
			if err := r.traceApplication(decl, args, true); err != nil {
				return parent.RewriteFailed, err
			}
			return parent.RewriteFailed, ErrUnsupportedOperator
		}
	}

	if r.params.BlastFull {
		rangeSort, err := r.manager.GetRange(decl)
		if err != nil {
			return parent.RewriteFailed, err
		}
		isBV, err := r.manager.IsBitVectorSort(rangeSort)
		if err != nil {
			return parent.RewriteFailed, err
		}
		if isBV {
			term, err := r.manager.RewriterMakeApp(decl, args)
			if err != nil {
				return parent.RewriteFailed, err
			}
			return parent.RewriteDone, r.blastBVTerm(term, out)
		}
	}
	return parent.RewriteFailed, nil
}
