package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2078-2079,2109-2137.

import "github.com/glycerine/g3/pkg/ast"

// RewriteBitVectorAdditionAfterCore performs the original disjoint-bit
// reduction after mk_add_core has returned incoming and assigned out. It does
// not implement the polynomial core. FAILED uses the original operands; DONE
// uses the operands of the resulting addition. Other statuses pass through.
// A final-assignment deletion error may accompany an already-published OR;
// the output reference records which decrement was consumed.
func RewriteBitVectorAdditionAfterCore(m *ast.ASTManager, incoming RewriteStatus, original ast.AppArguments, out ast.ASTRef) (RewriteStatus, error) {
	return rewriteBitVectorAdditionAfterCore(m, incoming, original, out, nil)
}

func rewriteBitVectorAdditionAfterCore(m *ast.ASTManager, incoming RewriteStatus, original ast.AppArguments, out ast.ASTRef, child *ast.ASTBitVectorNumeralInvocation) (RewriteStatus, error) {
	if incoming != RewriteFailed && incoming != RewriteDone {
		return incoming, nil
	}
	args := original
	if incoming == RewriteDone {
		if m == nil {
			return incoming, ast.ErrASTHandle
		}
		result, e := m.RefGet(out)
		if e != nil {
			return incoming, e
		}
		addition, e := m.IsBitVectorOperation(result, 4)
		if e != nil || !addition {
			return incoming, e
		}
		args, e = m.AppArgumentView(result)
		if e != nil {
			return incoming, e
		}
	}
	if args.Count() < 2 {
		return incoming, nil
	}
	if m == nil {
		return incoming, ast.ErrASTHandle
	}
	first, e := args.At(0)
	if e != nil {
		return incoming, e
	}
	width, e := m.BitVectorSize(first)
	if e != nil {
		return incoming, e
	}
	for bit := uint32(0); bit < width; bit++ {
		foundNonZero := false
		for j := uint32(0); j < args.Count(); j++ {
			arg, e := args.At(j)
			if e != nil {
				return incoming, e
			}
			var zero bool
			if child == nil {
				zero, e = m.BitVectorZeroBit(arg, bit)
			} else {
				zero, e = m.BitVectorZeroBitWithInvocation(arg, bit, child)
			}
			if e != nil {
				return incoming, e
			}
			if !zero {
				if foundNonZero {
					return incoming, nil
				}
				foundNonZero = true
			}
		}
	}
	result, e := m.MkBitVectorOrArguments(args)
	if e != nil {
		return incoming, e
	}
	// The factory acquires the operand references before assignment can delete
	// the old sum. Do not read its borrowed argument view after this assignment.
	return Rewrite1, m.SetRef(out, result)
}

// Source: src/ast/rewriter/bv_rewriter.cpp:2076-2079,2109-2137.
// The core's source scopes end before the disjoint-bit rule reads the original
// arguments (FAILED) or borrows the newly assigned addition's arguments (DONE).
func (p *BVPolynomialRewriter) RewriteBitVectorAddition(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
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
	st, e := p.rewriteBody(args, out, true, false)
	if e != nil {
		return st, e
	}
	st, e = rewriteBitVectorAdditionAfterCore(p.manager, st, args, out, &p.additionBits)
	if e != nil {
		return st, e
	}
	return st, p.manager.BitVectorNumeralInvocationSourceReady(&p.additionBits)
}
