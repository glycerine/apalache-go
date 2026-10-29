package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1736-1903;
// src/ast/rewriter/bv_rewriter.h:93.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvXorInvocation struct {
	flat, args        []ast.ASTHandle
	positive          map[ast.ASTHandle]bool
	negative          map[ast.ASTHandle]bool
	numerals          *ast.BitVectorBitwiseNumerals
	pieces            []ast.ASTHandle
	notTerm, constant ast.ASTRef
	allone, zero      ast.ASTBitVectorNumeralInvocation
	header            uint64
}

func (v *bvXorInvocation) endRefs(m *ast.ASTManager) error {
	if v.constant != (ast.ASTRef{}) {
		if e := m.DelRef(&v.constant); e != nil {
			return e
		}
	}
	if v.notTerm != (ast.ASTRef{}) {
		if e := m.DelRef(&v.notTerm); e != nil {
			return e
		}
	}
	return nil
}
func (v *bvXorInvocation) close(m *ast.ASTManager) error {
	if e := m.CleanupBitVectorNumeralInvocation(&v.allone); e != nil {
		return e
	}
	if e := m.CleanupBitVectorNumeralInvocation(&v.zero); e != nil {
		return e
	}
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
	}
	if e := v.endRefs(m); e != nil {
		return e
	}
	v.args = nil
	if e := shrinkRewriterRoots(m, &v.pieces, 0); e != nil {
		return e
	}
	v.pieces = nil
	if v.numerals != nil {
		if e := v.numerals.Close(); e != nil {
			return e
		}
		v.numerals = nil
	}
	v.negative = nil
	v.positive = nil
	v.flat = nil
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}

func (p *BVRewriter) RewriteXOR(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	if args.Count() == 0 {
		return st, ast.ErrASTDomain
	}
	first, e := args.At(0)
	if e != nil {
		return st, e
	}
	if args.Count() == 1 {
		return RewriteDone, m.SetRef(out, first)
	}
	width, e := m.BitVectorSize(first)
	if e != nil {
		return st, e
	}
	bytes := uint64(reflect.TypeOf(bvXorInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	v := &bvXorInvocation{header: bytes}
	p.bvxor = v
	flattened := false
	if p.polynomial.config.state.Flat {
		for i := uint32(0); i < args.Count(); i++ {
			arg, e := args.At(i)
			if e != nil {
				return st, e
			}
			nested, e := m.IsBitVectorOperation(arg, 33)
			if e != nil {
				return st, e
			}
			if nested {
				children, e := m.AppArgumentView(arg)
				if e != nil {
					return st, e
				}
				for j := uint32(0); j < children.Count(); j++ {
					child, e := children.At(j)
					if e != nil {
						return st, e
					}
					v.flat = append(v.flat, child)
				}
			} else {
				v.flat = append(v.flat, arg)
			}
		}
		if uint32(len(v.flat)) != args.Count() {
			flattened = true
			args = ast.SliceArguments(v.flat)
		}
	}
	v.positive = make(map[ast.ASTHandle]bool)
	v.negative = make(map[ast.ASTHandle]bool)
	v.numerals, e = ast.NewBitVectorBitwiseNumerals(m)
	if e != nil {
		return st, e
	}
	result, status, e := p.bvXorBody(v, args, width, flattened)
	st = status
	if e != nil {
		return st, e
	}
	if st != RewriteFailed {
		if e = m.SetRef(out, result); e != nil {
			return st, e
		}
	}
	if e = m.BitVectorNumeralInvocationSourceReady(&v.allone); e != nil {
		return st, e
	}
	if e = m.BitVectorNumeralInvocationSourceReady(&v.zero); e != nil {
		return st, e
	}
	if e = v.numerals.End(8, 7, 6, 5, 4, 3); e != nil {
		return st, e
	}
	// In the mask branch not_t ends before forward exs DEC, then two.
	// In the rebuild branch c ends before borrowed new_args and v2/v1.
	if e = v.endRefs(m); e != nil {
		return st, e
	}
	v.args = nil
	if e = shrinkRewriterRoots(m, &v.pieces, 0); e != nil {
		return st, e
	}
	if e = v.numerals.SourceEnd(); e != nil {
		return st, e
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.bvxor = nil
	return st, nil
}

func (p *BVRewriter) bvXorBody(v *bvXorInvocation, args ast.AppArguments, width uint32, flattened bool) (ast.ASTHandle, RewriteStatus, error) {
	m, n := p.manager, v.numerals
	failed := ast.ASTHandle{}
	merged := false
	var coefficients uint32
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return failed, RewriteFailed, e
		}
		numeric, e := n.Read(arg, &width)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if numeric {
			if e = n.Combine(true); e != nil {
				return failed, RewriteFailed, e
			}
			coefficients++
			continue
		}
		negative, e := m.IsBitVectorOperation(arg, 32)
		if e != nil {
			return failed, RewriteFailed, e
		}
		atom := arg
		if negative {
			atom, e = m.GetArg(arg, 0)
			if e != nil {
				return failed, RewriteFailed, e
			}
		}
		marks, other := v.positive, v.negative
		if negative {
			marks, other = other, marks
		}
		duplicate := marks[atom]
		opposite := !duplicate && other[atom]
		if duplicate || opposite {
			if duplicate {
				delete(marks, atom)
			} else {
				delete(other, atom)
			}
			merged = true
			if opposite {
				if e = n.ToggleAllOne(width); e != nil {
					return failed, RewriteFailed, e
				}
			}
		} else {
			marks[atom] = true
		}
	}
	zero, e := n.IsZero()
	if e != nil {
		return failed, RewriteFailed, e
	}
	if !zero && coefficients == args.Count()-1 {
		var term ast.ASTHandle
		for i := uint32(0); i < args.Count(); i++ {
			term, e = args.At(i)
			if e != nil {
				return failed, RewriteFailed, e
			}
			numeric, e := m.IsBitVectorNumeral(term)
			if e != nil {
				return failed, RewriteFailed, e
			}
			if !numeric {
				break
			}
		}
		if term.IsNull() {
			return failed, RewriteFailed, ast.ErrASTDomain
		}
		result, e := p.bvXorMask(v, term, width)
		return result, Rewrite3, e
	}
	if !merged && !flattened {
		condition := coefficients == 0
		if coefficients == 1 && !zero {
			allone, e := n.IsAllOne(width)
			if e != nil {
				return failed, RewriteFailed, e
			}
			condition = !allone
		}
		if condition {
			sorted := true
			if p.polynomial.config.bitVector.SortAC {
				sorted, e = m.ASTIsSorted(args)
			}
			if e != nil || sorted {
				return failed, RewriteFailed, e
			}
		}
	}
	// Original expr_ref c is constructed even when the coefficient is zero.
	v.constant, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return failed, RewriteFailed, e
	}
	if !zero {
		h, e := n.Constant(0, width)
		if e == nil {
			e = m.SetRef(v.constant, h)
		}
		if e == nil {
			v.args = append(v.args, h)
		}
		if e != nil {
			return failed, RewriteFailed, e
		}
	}
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return failed, RewriteFailed, e
		}
		numeric, e := m.IsBitVectorNumeral(arg)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if numeric {
			continue
		}
		negative, e := m.IsBitVectorOperation(arg, 32)
		if e != nil {
			return failed, RewriteFailed, e
		}
		atom := arg
		marks := v.positive
		if negative {
			atom, e = m.GetArg(arg, 0)
			if e != nil {
				return failed, RewriteFailed, e
			}
			marks = v.negative
		}
		if marks[atom] {
			v.args = append(v.args, arg)
			delete(marks, atom)
		}
	}
	hs := v.args
	switch len(hs) {
	case 0:
		result, e := m.BitVectorNumeralUnsignedWithInvocation(0, width, &v.zero)
		return result, RewriteDone, e
	case 1:
		return hs[0], RewriteDone, nil
	case 2:
		allone, e := m.IsBitVectorAllOneWithInvocation(hs[0], &v.allone)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if allone {
			result, e := m.MkBitVectorNot(hs[1])
			return result, RewriteDone, e
		}
	}
	if p.polynomial.config.bitVector.SortAC {
		if e = bvPolynomialSortMonomials(m, p.bvFamily, true, hs); e != nil {
			return failed, RewriteFailed, e
		}
	}
	result, e := m.MkBitVectorXor(hs)
	return result, RewriteDone, e
}

func (p *BVRewriter) bvXorMask(v *bvXorInvocation, arg ast.ASTHandle, width uint32) (ast.ASTHandle, error) {
	m, n := p.manager, v.numerals
	// Unlike OR, the source XOR mask does not normalize its accumulator.
	if e := n.BeginMask(); e != nil {
		return ast.ASTHandle{}, e
	}
	var e error
	v.notTerm, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, e
	}
	not, e := m.MkBitVectorNot(arg)
	if e == nil {
		e = m.SetRef(v.notTerm, not)
	}
	if e != nil {
		return ast.ASTHandle{}, e
	}
	var low, i uint32
	for i < width {
		for i < width {
			one, e := n.LowBit(true)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if !one {
				break
			}
			i++
			if e = n.ShiftMask(); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		if i != low {
			h, e := p.extract.Apply(i-1, low, not)
			if e == nil {
				e = appendRewriterRoot(m, &v.pieces, h)
			}
			if e != nil {
				return ast.ASTHandle{}, e
			}
			low = i
		}
		for i < width {
			zero, e := n.LowBit(false)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if !zero {
				break
			}
			i++
			if e = n.ShiftMask(); e != nil {
				return ast.ASTHandle{}, e
			}
		}
		if i != low {
			h, e := p.extract.Apply(i-1, low, arg)
			if e == nil {
				e = appendRewriterRoot(m, &v.pieces, h)
			}
			if e != nil {
				return ast.ASTHandle{}, e
			}
			low = i
		}
	}
	hs := v.pieces
	for i, j := 0, len(hs)-1; i < j; i, j = i+1, j-1 {
		hs[i], hs[j] = hs[j], hs[i]
	}
	if len(hs) == 1 {
		return hs[0], nil
	}
	return m.MkBitVectorConcat(hs)
}
