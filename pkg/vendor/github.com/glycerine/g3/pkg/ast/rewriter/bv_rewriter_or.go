package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1567-1734;
// src/ast/rewriter/bv_rewriter.h:92.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvOrInvocation struct {
	flat, args, pieces []ast.ASTHandle
	positive           map[ast.ASTHandle]bool
	negative           map[ast.ASTHandle]bool
	numerals           *ast.BitVectorBitwiseNumerals
	zeroBit, zero      ast.ASTBitVectorNumeralInvocation
	header             uint64
}

func (v *bvOrInvocation) close(m *ast.ASTManager) error {
	if e := m.CleanupBitVectorNumeralInvocation(&v.zero); e != nil {
		return e
	}
	if e := m.CleanupBitVectorNumeralInvocation(&v.zeroBit); e != nil {
		return e
	}
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
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
	v.args = nil
	v.flat = nil
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}

func (p *BVRewriter) RewriteOR(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	if args.Count() == 0 {
		// Source asserts nonempty; reject the release out-of-bounds domain too.
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
	bytes := uint64(reflect.TypeOf(bvOrInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	v := &bvOrInvocation{header: bytes}
	p.bvor = v
	flattened := false
	if p.polynomial.config.state.Flat {
		for i := uint32(0); i < args.Count(); i++ {
			arg, e := args.At(i)
			if e != nil {
				return st, e
			}
			nested, e := m.IsBitVectorOperation(arg, 31)
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
	// Membership is local to this invocation; numerical v1/v2 retain their scopes.
	v.positive = make(map[ast.ASTHandle]bool)
	v.negative = make(map[ast.ASTHandle]bool)
	v.numerals, e = ast.NewBitVectorBitwiseNumerals(m)
	if e != nil {
		return st, e
	}
	result, status, e := p.bvOrBody(v, args, width, flattened)
	st = status
	if e != nil {
		return st, e
	}
	if st != RewriteFailed {
		if e = m.SetRef(out, result); e != nil {
			return st, e
		}
	}
	if e = m.BitVectorNumeralInvocationSourceReady(&v.zeroBit); e != nil {
		return st, e
	}
	if e = m.BitVectorNumeralInvocationSourceReady(&v.zero); e != nil {
		return st, e
	}
	// Assignment full-expression temporaries end before branch buffers/two.
	if e = v.numerals.End(8, 7, 6, 5, 4, 3); e != nil {
		return st, e
	}
	v.pieces = nil
	if e = v.numerals.SourceEnd(); e != nil {
		return st, e
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.bvor = nil
	return st, nil
}

func (p *BVRewriter) bvOrBody(v *bvOrInvocation, args ast.AppArguments, width uint32, flattened bool) (ast.ASTHandle, RewriteStatus, error) {
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
			coefficients++
			if e = n.Combine(false); e != nil {
				return failed, RewriteFailed, e
			}
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
		var duplicate, opposite bool
		if negative {
			opposite = v.positive[atom]
			duplicate = !opposite && v.negative[atom]
		} else {
			duplicate = v.positive[atom]
			opposite = !duplicate && v.negative[atom]
		}
		if opposite {
			if e = n.AllOne(width); e != nil {
				return failed, RewriteFailed, e
			}
			result, e := n.Constant(5, width)
			return result, RewriteDone, e
		}
		if duplicate {
			merged = true
			continue
		}
		if negative {
			v.negative[atom] = true
		} else {
			v.positive[atom] = true
		}
		v.args = append(v.args, arg)
	}
	allone, e := n.IsAllOne(width)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if allone {
		result, e := n.Constant(0, width)
		return result, RewriteDone, e
	}
	if len(v.args) == 2 && coefficients == 0 {
		left, right := v.args[0], v.args[1]
		concat, e := m.IsBitVectorOperation(left, 37)
		if e == nil && concat {
			concat, e = m.IsBitVectorOperation(right, 37)
		}
		if e != nil {
			return failed, RewriteFailed, e
		}
		if concat {
			result, matched, e := p.bvOrDisjointConcat(v, left, right, width)
			if e != nil || matched {
				return result, Rewrite2, e
			}
		}
	}
	zero, e := n.IsZero()
	if e != nil {
		return failed, RewriteFailed, e
	}
	if !zero && len(v.args) == 1 {
		result, e := p.bvOrMask(v, v.args[0], width)
		return result, Rewrite2, e
	}
	if !flattened && !merged && (coefficients == 0 || coefficients == 1 && !zero) {
		sorted := true
		if p.polynomial.config.bitVector.SortAC {
			sorted, e = m.ASTIsSorted(args)
		}
		if e != nil || sorted {
			return failed, RewriteFailed, e
		}
	}
	if !zero {
		h, e := n.Constant(0, width)
		if e == nil {
			v.args = append(v.args, h)
		}
		if e != nil {
			return failed, RewriteFailed, e
		}
	}
	hs := v.args
	switch len(hs) {
	case 0:
		result, e := m.BitVectorNumeralUnsignedWithInvocation(0, width, &v.zero)
		return result, RewriteDone, e
	case 1:
		return hs[0], RewriteDone, nil
	default:
		if p.polynomial.config.bitVector.SortAC {
			// Sort with the shared Go comparator.
			if e = bvPolynomialSortMonomials(m, p.bvFamily, true, hs); e != nil {
				return failed, RewriteFailed, e
			}
		}
		result, e := m.MkBitVectorOr(hs)
		return result, RewriteDone, e
	}
}

func (p *BVRewriter) bvOrDisjointConcat(v *bvOrInvocation, left, right ast.ASTHandle, width uint32) (ast.ASTHandle, bool, error) {
	m := p.manager
	zeroBit := func(h ast.ASTHandle, bit uint32) (bool, error) {
		return m.BitVectorZeroBitWithInvocation(h, bit, &v.zeroBit)
	}
	for i := uint32(0); i < width; i++ {
		zero, e := zeroBit(left, i)
		if e != nil {
			return ast.ASTHandle{}, false, e
		}
		if !zero {
			zero, e = zeroBit(right, i)
			if e != nil || !zero {
				return ast.ASTHandle{}, false, e
			}
		}
	}
	// Source unsigned-to-int conversion and decrement are explicitly 32-bit.
	j := int32(width) - 1
	for j >= 0 {
		high := j
		for j >= 0 {
			zero, e := zeroBit(left, uint32(j))
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if !zero {
				break
			}
			j--
		}
		if j != high {
			h, e := p.extract.Apply(uint32(high), uint32(j+1), right)
			if e == nil {
				v.pieces = append(v.pieces, h)
			}
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
		}
		high = j
		for j >= 0 {
			zero, e := zeroBit(right, uint32(j))
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
			if !zero {
				break
			}
			j--
		}
		if j != high {
			h, e := p.extract.Apply(uint32(high), uint32(j+1), left)
			if e == nil {
				v.pieces = append(v.pieces, h)
			}
			if e != nil {
				return ast.ASTHandle{}, false, e
			}
		}
	}
	result, e := m.MkBitVectorConcat(v.pieces)
	return result, true, e
}

func (p *BVRewriter) bvOrMask(v *bvOrInvocation, arg ast.ASTHandle, width uint32) (ast.ASTHandle, error) {
	n := v.numerals
	if e := n.Normalize(width); e != nil {
		return ast.ASTHandle{}, e
	}
	maskValue := ""
	if p.bvTraceTags().ShouldTrace("mask_bug") {
		value, e := n.MaskValueString()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		maskValue = value
	}
	if e := n.BeginMask(); e != nil {
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
			bits := i - low
			if e := n.AllOne(bits); e != nil {
				return ast.ASTHandle{}, e
			}
			h, e := n.Constant(5, bits)
			if e == nil {
				v.pieces = append(v.pieces, h)
			}
			if e != nil {
				return ast.ASTHandle{}, e
			}
			if e = n.End(5, 4, 3); e != nil {
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
				v.pieces = append(v.pieces, h)
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
	result, e := p.manager.MkBitVectorConcat(hs)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if e = p.traceORMask(maskValue, width, arg, result); e != nil {
		return ast.ASTHandle{}, e
	}
	return result, nil
}
