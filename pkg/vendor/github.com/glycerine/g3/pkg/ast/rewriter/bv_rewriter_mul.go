package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2170-2205.

import "github.com/glycerine/g3/pkg/ast"

// RewriteBitVectorMultiplication consumes the source mul2concat setting and
// borrows the existing extraction procedure. It reads neither when bypassed.
func (p *BVPolynomialRewriter) RewriteBitVectorMultiplication(args ast.AppArguments, out ast.ASTRef, mul2concat bool, extract *MkExtractProc) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() {
		p.running = false
		if err != nil {
			p.fail(err)
		}
	}()
	st, err = p.rewriteBody(args, out, false, false)
	if err != nil {
		return
	}
	if st != RewriteFailed && st != RewriteDone {
		return
	}
	operands := args
	if st == RewriteFailed {
		if args.Count() != 2 {
			return
		}
	} else {
		result, e := p.manager.RefGet(out)
		if e != nil {
			return st, e
		}
		yes, e := p.manager.IsBitVectorOperation(result, 6)
		if e != nil {
			return st, e
		}
		if !yes {
			return
		}
		operands, e = p.manager.AppArgumentView(result)
		if e != nil {
			return st, e
		}
		if operands.Count() != 2 {
			return
		}
	}
	x, e := operands.At(0)
	if e != nil {
		return st, e
	}
	y, e := operands.At(1)
	if e != nil {
		return st, e
	}
	if !mul2concat {
		return
	}
	p.multiplicationNumerals, err = ast.NewBitVectorMultiplicationNumerals(p.manager)
	if err != nil {
		return
	}
	n := p.multiplicationNumerals
	yes, width, shift, e := n.ReadPower(x)
	if e != nil {
		return st, e
	}
	if yes {
		if ast.ASTSourceDebug && shift < 1 {
			return st, ast.ErrASTDomain
		}
		m, e := rotationManager(extract)
		if e != nil {
			return st, e
		}
		if m != p.manager {
			return st, ast.ErrASTHandle
		}
		// Source unsigned arithmetic and left-to-right initializer order.
		p.multiplicationConcat[0], e = extract.Apply(width-shift-1, 0, y)
		if e != nil {
			return st, e
		}
		p.multiplicationConcat[1], e = n.Zero(shift)
		if e != nil {
			return st, e
		}
		result, e := p.manager.MkBitVectorConcat(p.multiplicationConcat[:])
		if e != nil {
			return st, e
		}
		st = Rewrite2
		if e = p.manager.SetRef(out, result); e != nil {
			return st, e
		}
	}
	if e = n.SourceEnd(); e != nil {
		return st, e
	}
	if e = n.Close(); e != nil {
		return st, e
	}
	p.multiplicationNumerals = nil
	return
}
