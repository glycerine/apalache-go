package rewriter

// Source: src/ast/rewriter/fpa_rewriter.cpp:41-88,547-638.

import "github.com/glycerine/g3/pkg/ast"

// FPARewriter currently owns the retained classification dispatch. It borrows
// the AST manager and its floating-point numeral service.
type FPARewriter struct {
	self            *FPARewriter
	manager         *ast.ASTManager
	family          ast.FamilyID
	hiFPUnspecified bool
	closed          bool
}

func NewFPARewriter(m *ast.ASTManager) (*FPARewriter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	family, err := m.RegisterFloatingPointPlugin()
	if err != nil {
		return nil, err
	}
	r := &FPARewriter{manager: m, family: family}
	r.self = r
	return r, nil
}

func (r *FPARewriter) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	r.closed = true
	return nil
}

// SetHighFPUnspecified applies the source hi_fp_unspecified Boolean parameter.
func (r *FPARewriter) SetHighFPUnspecified(enabled bool) error {
	if r == nil || r.self != r || r.closed {
		return ast.ErrASTHandle
	}
	r.hiFPUnspecified = enabled
	return nil
}

func (r *FPARewriter) MkAppCore(d ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	if r == nil || r.self != r || r.closed {
		return RewriteFailed, ast.ErrASTHandle
	}
	family, err := r.manager.GetFamilyID(d)
	if err != nil {
		return RewriteFailed, err
	}
	if family != r.family {
		return RewriteFailed, ast.ErrASTDomain
	}
	kind, err := r.manager.GetDeclKind(d)
	if err != nil {
		return RewriteFailed, err
	}
	if kind >= 0 && kind <= 10 {
		if args.Count() != 0 {
			return RewriteFailed, ast.ErrASTDomain
		}
		value, err := r.manager.MkApp(d, nil)
		if err != nil {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, value)
	}
	if kind == ast.FPAOpFP {
		if args.Count() != 3 {
			return RewriteFailed, ast.ErrASTDomain
		}
		sign, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		exponent, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		significand, err := args.At(2)
		if err != nil {
			return RewriteFailed, err
		}
		answer, found, err := r.manager.RewriteFPAFPComponentsNumerals(d, sign, exponent, significand)
		if err != nil || !found {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpBVWrap {
		if args.Count() != 1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		value, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		answer, found, err := r.rewriteFPABVWrap(value)
		if err != nil || !found {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpBV2RM {
		if args.Count() != 1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		value, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		return r.fpaBV2RM(value, out)
	}
	if kind == ast.FPAOpToReal {
		if args.Count() != 1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		value, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		answer, found, err := r.manager.RewriteFPAToRealNumeral(value, r.hiFPUnspecified)
		if err != nil || !found {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpToFPUnsigned {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		rm, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		value, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		answer, found, err := r.manager.RewriteFPAToFPUnsignedNumeral(d, rm, value)
		if err != nil || !found {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpToFP {
		if args.Count() == 3 {
			first, err := args.At(0)
			if err != nil {
				return RewriteFailed, err
			}
			second, err := args.At(1)
			if err != nil {
				return RewriteFailed, err
			}
			third, err := args.At(2)
			if err != nil {
				return RewriteFailed, err
			}
			firstDomain, err := r.manager.GetDomain(d, 0)
			if err != nil {
				return RewriteFailed, err
			}
			isBV, err := r.manager.IsBitVectorSort(firstDomain)
			if err != nil {
				return RewriteFailed, err
			}
			var answer ast.ASTHandle
			var found bool
			if isBV {
				answer, found, err = r.manager.RewriteFPAToFPThreeBVNumerals(d, first, second, third)
			} else {
				answer, found, err = r.manager.RewriteFPAToFPExponentRationalNumerals(d, first, second, third)
			}
			if err != nil || !found {
				return RewriteFailed, err
			}
			return RewriteDone, r.manager.SetRef(out, answer)
		}
		if args.Count() == 1 {
			value, err := args.At(0)
			if err != nil {
				return RewriteFailed, err
			}
			answer, found, err := r.manager.RewriteFPAToFPIEEEBVNumeral(d, value)
			if err != nil || !found {
				return RewriteFailed, err
			}
			return RewriteDone, r.manager.SetRef(out, answer)
		}
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		rm, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		value, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		secondDomain, err := r.manager.GetDomain(d, 1)
		if err != nil {
			return RewriteFailed, err
		}
		isBV, err := r.manager.IsBitVectorSort(secondDomain)
		if err != nil {
			return RewriteFailed, err
		}
		var answer ast.ASTHandle
		var found bool
		if isBV {
			answer, found, err = r.manager.RewriteFPAToFPSignedBVNumeral(d, rm, value)
		} else {
			arithFamily, sortErr := r.manager.GetFamilyID(secondDomain)
			if sortErr != nil {
				return RewriteFailed, sortErr
			}
			if arithFamily != r.family {
				answer, found, err = r.manager.RewriteFPAToFPArithmeticNumeral(d, rm, value)
			} else {
				answer, found, err = r.manager.RewriteFPAToFPFormatNumeral(d, rm, value)
			}
		}
		if err != nil || !found {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpToUBV || kind == ast.FPAOpToSBV {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		rm, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		value, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		answer, found, err := r.manager.RewriteFPAToBVNumeral(d, rm, value, r.hiFPUnspecified)
		if err != nil || !found {
			return RewriteFailed, err
		}
		return RewriteDone, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpToIEEEBV {
		if args.Count() != 1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		value, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		answer, found, rewriteOne, err := r.manager.RewriteFPAToIEEEBVNumeral(value, r.hiFPUnspecified)
		if err != nil || !found {
			return RewriteFailed, err
		}
		status := RewriteDone
		if rewriteOne {
			status = Rewrite1
		}
		return status, r.manager.SetRef(out, answer)
	}
	if kind == ast.FPAOpNeg || kind == ast.FPAOpAbs {
		if args.Count() != 1 {
			return RewriteFailed, ast.ErrASTDomain
		}
		arg, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		return r.fpaUnary(arg, kind, out)
	}
	if kind == ast.FPAOpAdd || kind == ast.FPAOpSub || kind == ast.FPAOpMul || kind == ast.FPAOpDiv {
		if args.Count() != 3 {
			return RewriteFailed, ast.ErrASTDomain
		}
		rm, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		left, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		right, err := args.At(2)
		if err != nil {
			return RewriteFailed, err
		}
		if kind == ast.FPAOpSub {
			return r.fpaSub(rm, left, right, out)
		}
		return r.fpaRMBinary(rm, left, right, kind, out)
	}
	if kind == ast.FPAOpRem {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		left, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		right, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		return r.fpaRemainder(left, right, out)
	}
	if kind == ast.FPAOpMin || kind == ast.FPAOpMax {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		left, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		right, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		return r.fpaExtremum(left, right, kind, out)
	}
	if kind == ast.FPAOpSqrt || kind == ast.FPAOpRoundToIntegral {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		rm, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		value, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		return r.fpaRMUnary(rm, value, kind, out)
	}
	if kind == ast.FPAOpFMA {
		if args.Count() != 4 {
			return RewriteFailed, ast.ErrASTDomain
		}
		var operands [4]ast.ASTHandle
		for i := range operands {
			operands[i], err = args.At(uint32(i))
			if err != nil {
				return RewriteFailed, err
			}
		}
		return r.fpaFMA(operands[0], operands[1], operands[2], operands[3], out)
	}
	if kind >= ast.FPAOpEq && kind <= ast.FPAOpGe {
		if args.Count() != 2 {
			return RewriteFailed, ast.ErrASTDomain
		}
		left, err := args.At(0)
		if err != nil {
			return RewriteFailed, err
		}
		right, err := args.At(1)
		if err != nil {
			return RewriteFailed, err
		}
		switch kind {
		case ast.FPAOpEq:
			return r.MkFloatEq(left, right, out)
		case ast.FPAOpLt:
			return r.fpaLess(left, right, out)
		case ast.FPAOpGt:
			return r.fpaSwappedComparison(left, right, ast.FPAOpLt, out)
		case ast.FPAOpLe:
			return r.fpaLessEqual(left, right, out)
		case ast.FPAOpGe:
			return r.fpaSwappedComparison(left, right, ast.FPAOpLe, out)
		}
	}
	if kind < int32(ast.FPAIsNaN) || kind > int32(ast.FPAIsPositive) {
		return RewriteFailed, nil
	}
	if args.Count() != 1 {
		return RewriteFailed, ast.ErrASTDomain
	}
	arg, err := args.At(0)
	if err != nil {
		return RewriteFailed, err
	}
	found, value, err := r.manager.ClassifyFPANumeral(arg, ast.FPAClassificationKind(kind))
	if err != nil || !found {
		return RewriteFailed, err
	}
	var answer ast.ASTHandle
	if value {
		answer, err = r.manager.MkTrue()
	} else {
		answer, err = r.manager.MkFalse()
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}

// MkEqCore is SMT equality on floating-point expressions. Unlike fp.eq,
// paired NaNs compare equal and signed zeros remain distinct.
func (r *FPARewriter) MkEqCore(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if r == nil || r.self != r || r.closed {
		return RewriteFailed, ast.ErrASTHandle
	}
	found, equal, err := r.manager.EqualFPANumerals(left, right)
	if err != nil || !found {
		return RewriteFailed, err
	}
	var answer ast.ASTHandle
	if equal {
		answer, err = r.manager.MkTrue()
	} else {
		answer, err = r.manager.MkFalse()
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}

// MkFloatEq is the source fp.eq reducer, distinct from SMT equality.
func (r *FPARewriter) MkFloatEq(left, right ast.ASTHandle, out ast.ASTRef) (RewriteStatus, error) {
	if r == nil || r.self != r || r.closed {
		return RewriteFailed, ast.ErrASTHandle
	}
	found, equal, err := r.manager.EqualFPAIEEENumerals(left, right)
	if err != nil || !found {
		return RewriteFailed, err
	}
	var answer ast.ASTHandle
	if equal {
		answer, err = r.manager.MkTrue()
	} else {
		answer, err = r.manager.MkFalse()
	}
	if err != nil {
		return RewriteFailed, err
	}
	return RewriteDone, r.manager.SetRef(out, answer)
}
