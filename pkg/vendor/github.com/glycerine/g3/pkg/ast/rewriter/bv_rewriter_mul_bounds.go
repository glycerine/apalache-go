package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2706-2800.

import "github.com/glycerine/g3/pkg/ast"

type BVMultiplyBound uint8

const (
	BVSignedNoOverflow BVMultiplyBound = iota
	BVUnsignedNoOverflow
	BVSignedNoUnderflow
)

func (p *BVRewriter) RewriteMultiplyBound(args ast.AppArguments, out ast.ASTRef, mode BVMultiplyBound) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	if mode > BVSignedNoUnderflow {
		return RewriteFailed, ast.ErrASTDomain
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	if ast.ASTSourceDebug && args.Count() != 2 {
		return st, ast.ErrASTDomain
	}
	m := p.manager
	n, e := ast.NewBitVectorMultiplyBoundNumerals(m)
	p.mulBound = n
	if e != nil {
		return st, e
	}
	var width uint32
	first, e := args.At(0)
	if e != nil {
		return st, e
	}
	num1, e := n.Read(0, first, &width)
	if e != nil {
		return st, e
	}
	second, e := args.At(1)
	if e != nil {
		return st, e
	}
	num2, e := n.Read(1, second, &width)
	if e != nil {
		return st, e
	}
	result := false
	matched := false
	if num1 {
		matched, e = n.ZeroOrOne(0)
		if e != nil {
			return st, e
		}
	}
	if !matched && num2 {
		matched, e = n.ZeroOrOne(1)
		if e != nil {
			return st, e
		}
	}
	if matched {
		result = true
	} else if num1 && num2 {
		matched = true
		if mode == BVSignedNoUnderflow {
			if e = n.Power(2, width); e != nil {
				return st, e
			}
			if e = n.Power(3, width-1); e != nil {
				return st, e
			}
			if e = n.AdjustSigned(0); e != nil {
				return st, e
			}
			if e = n.AdjustSigned(1); e != nil {
				return st, e
			}
			if e = n.Product(4); e != nil {
				return st, e
			}
			if e = n.NegativeLimit(); e != nil {
				return st, e
			}
			if e = p.traceSignedMultiplyUnderflow(n); e != nil {
				return st, e
			}
			result, e = n.Compare(4, 5, true)
		} else {
			if e = n.Product(2); e != nil {
				return st, e
			}
			limit := width
			if mode == BVSignedNoOverflow {
				limit--
			}
			if e = n.Power(3, limit); e != nil {
				return st, e
			}
			result, e = n.Compare(2, 3, false)
		}
		if e != nil {
			return st, e
		}
	}
	if matched {
		var h ast.ASTHandle
		if result {
			h, e = m.MkTrue()
		} else {
			h, e = m.MkFalse()
		}
		if e != nil {
			return st, e
		}
		st = RewriteDone
		if err = m.SetRef(out, h); err != nil {
			return
		}
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.mulBound = nil
	return
}
