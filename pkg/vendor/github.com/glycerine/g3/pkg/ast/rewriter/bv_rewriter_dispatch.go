package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:52-208.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

// RewriteApp dispatches one bit-vector declaration after traversal has
// rewritten its arguments. The declaration is borrowed and args remains owned
// by the caller.
func (p *BVRewriter) RewriteApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if err := p.open(); err != nil {
		return RewriteFailed, err
	}
	family, err := p.manager.GetFamilyID(decl)
	if err != nil {
		return RewriteFailed, err
	}
	if family != p.bvFamily {
		return RewriteFailed, nil
	}
	kind, err := p.manager.GetDeclKind(decl)
	if err != nil {
		return RewriteFailed, err
	}
	count := args.Count()
	arg := func(index uint32) (ast.ASTHandle, error) { return args.At(index) }
	binary := func(call func(ast.ASTHandle, ast.ASTHandle, ast.ASTRef) (RewriteStatus, error)) (RewriteStatus, error) {
		if count != 2 {
			return RewriteFailed, nil
		}
		left, e := arg(0)
		if e != nil {
			return RewriteFailed, e
		}
		right, e := arg(1)
		if e != nil {
			return RewriteFailed, e
		}
		return call(left, right, out)
	}
	unary := func(call func(ast.ASTHandle, ast.ASTRef) (RewriteStatus, error)) (RewriteStatus, error) {
		if count != 1 {
			return RewriteFailed, nil
		}
		value, e := arg(0)
		if e != nil {
			return RewriteFailed, e
		}
		return call(value, out)
	}

	switch kind {
	case 1, 2: // OP_BIT1, OP_BIT0
		if count != 0 {
			return RewriteFailed, nil
		}
		value := uint32(0)
		if kind == 1 {
			value = 1
		}
		result, e := p.manager.BitVectorNumeralUnsigned(value, 1)
		if e != nil {
			return RewriteFailed, e
		}
		return RewriteDone, p.manager.SetRef(out, result)
	case 3: // OP_BNEG
		return unary(p.polynomial.RewriteUnaryMinus)
	case 4: // OP_BADD
		if count == 0 {
			return RewriteFailed, nil
		}
		return p.RewriteAddition(args, out)
	case 5: // OP_BSUB
		if count == 0 {
			return RewriteFailed, nil
		}
		return p.polynomial.RewriteSubtraction(args, out)
	case 6: // OP_BMUL
		if count == 0 {
			return RewriteFailed, nil
		}
		return p.RewriteMultiplication(args, out)
	case 7:
		return binary(p.RewriteSignedDivision)
	case 8:
		return binary(p.RewriteUnsignedDivision)
	case 9:
		return binary(p.RewriteSignedRemainder)
	case 10:
		return binary(p.RewriteUnsignedRemainder)
	case 11:
		return binary(p.RewriteSignedModulo)
	case 17:
		return binary(p.RewriteSignedDivisionInternal)
	case 18:
		return binary(p.RewriteUnsignedDivisionInternal)
	case 19:
		return binary(p.RewriteSignedRemainderInternal)
	case 20:
		return binary(p.RewriteUnsignedRemainderInternal)
	case 21:
		return binary(p.RewriteSignedModuloInternal)
	case 22:
		return binary(p.RewriteULE)
	case 23:
		return binary(p.RewriteSLE)
	case 24:
		return binary(p.RewriteUGE)
	case 25:
		return binary(p.RewriteSGE)
	case 26:
		return binary(p.RewriteULT)
	case 27:
		return binary(p.RewriteSLT)
	case 28:
		return binary(func(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
			return p.RewriteULT(b, a, out)
		})
	case 29:
		return binary(func(a, b ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
			return p.RewriteSLT(b, a, out)
		})
	case 30:
		return p.RewriteAND(args, out)
	case 31:
		return p.RewriteOR(args, out)
	case 32:
		return unary(p.RewriteNot)
	case 33:
		return p.RewriteXOR(args, out)
	case 34:
		return p.RewriteNAND(args, out)
	case 35:
		return p.RewriteNOR(args, out)
	case 36:
		return p.RewriteXNOR(args, out)
	case 37:
		return p.RewriteConcat(args, out)
	case 38, 39, 41, 48, 49: // indexed unary operations
		if count != 1 {
			return RewriteFailed, nil
		}
		parameters, e := p.declIntegerParameters(decl, 1)
		if e != nil {
			return RewriteFailed, e
		}
		value, e := arg(0)
		if e != nil {
			return RewriteFailed, e
		}
		n := uint32(parameters[0])
		switch kind {
		case 38:
			return p.RewriteSignExtend(n, value, out)
		case 39:
			return p.RewriteZeroExtend(n, value, out)
		case 41:
			return p.RewriteRepeat(n, value, out)
		case 48:
			return p.RewriteRotateLeft(n, value, out)
		default:
			return p.RewriteRotateRight(n, value, out)
		}
	case 40: // OP_EXTRACT
		if count != 1 {
			return RewriteFailed, nil
		}
		parameters, e := p.declIntegerParameters(decl, 2)
		if e != nil {
			return RewriteFailed, e
		}
		value, e := arg(0)
		if e != nil {
			return RewriteFailed, e
		}
		return p.RewriteExtract(uint32(parameters[0]), uint32(parameters[1]), value, out)
	case 42:
		return unary(p.RewriteReductionOR)
	case 43:
		return unary(p.RewriteReductionAND)
	case 44:
		return binary(p.RewriteCompare)
	case 45:
		return binary(p.RewriteShiftLeft)
	case 46:
		return binary(p.RewriteLogicalShiftRight)
	case 47:
		return binary(p.RewriteArithmeticShiftRight)
	case 50:
		return binary(p.RewriteExtendedRotateLeft)
	case 51:
		return binary(p.RewriteExtendedRotateRight)
	case 52:
		return p.RewriteMultiplyBound(args, out, BVUnsignedNoOverflow)
	case 53:
		return p.RewriteMultiplyBound(args, out, BVSignedNoOverflow)
	case 54:
		return p.RewriteMultiplyBound(args, out, BVSignedNoUnderflow)
	case 56:
		return p.RewriteMkBV(args, out)
	case 57: // OP_INT2BV obtains its width from the declaration range.
		if count != 1 {
			return RewriteFailed, nil
		}
		rangeSort, e := p.manager.GetRange(decl)
		if e != nil {
			return RewriteFailed, e
		}
		width, e := p.manager.BitVectorSize(rangeSort)
		if e != nil {
			return RewriteFailed, e
		}
		value, e := arg(0)
		if e != nil {
			return RewriteFailed, e
		}
		return p.RewriteInt2BV(width, value, out)
	case 58:
		return unary(p.RewriteBV2Int)
	default:
		return RewriteFailed, nil
	}
}

func (p *BVRewriter) declIntegerParameters(decl ast.ASTHandle, want int) (values []int32, result error) {
	parameters, err := p.manager.GetDeclParametersCopy(decl)
	if err != nil {
		return nil, err
	}
	defer func() {
		for i := range parameters {
			result = errors.Join(result, p.manager.DelParameter(&parameters[i]))
		}
	}()
	if len(parameters) != want {
		return nil, ast.ErrASTDomain
	}
	values = make([]int32, want)
	for i := range parameters {
		values[i], err = p.manager.ParameterInt(parameters[i])
		if err != nil {
			return nil, err
		}
		if values[i] < 0 {
			return nil, ast.ErrASTDomain
		}
	}
	return values, nil
}
