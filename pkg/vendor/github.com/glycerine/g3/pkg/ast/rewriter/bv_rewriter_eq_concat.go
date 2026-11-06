package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2306-2370.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvConcatEqualityInvocation struct {
	manager            *ast.ASTManager
	singleton          [2]ast.ASTHandle
	operands           [2]ast.AppArguments
	equalities         []ast.ASTHandle
	low1, low2, i1, i2 uint32
	header             uint64
}

func (b *bvConcatEqualityInvocation) close() {
	clear(b.equalities)
	b.equalities = nil
	b.manager.Budget().Release(b.header)
	b.header = 0
}

// RewriteConcatenationEquality aligns suffixes of the original borrowed
// concat arguments. It borrows the source cached extraction procedure.
func (p *BVPolynomialRewriter) RewriteConcatenationEquality(lhs, rhs ast.ASTHandle, out ast.ASTRef, extract *MkExtractProc) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() {
		p.running = false
		if err != nil {
			p.fail(err)
		}
	}()
	m := p.manager
	lc, e := m.IsBitVectorOperation(lhs, 37)
	if e != nil {
		return st, e
	}
	rc := false
	if ast.ASTSourceDebug && !lc {
		rc, e = m.IsBitVectorOperation(rhs, 37)
		if e != nil {
			return st, e
		}
		if !rc {
			return st, ast.ErrASTDomain
		}
	}
	bytes := uint64(reflect.TypeOf(bvConcatEqualityInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	b := &bvConcatEqualityInvocation{manager: m, header: bytes}
	p.concatEquality = b
	if lc {
		b.operands[0], e = m.AppArgumentView(lhs)
	} else {
		b.singleton[0] = lhs
		b.operands[0] = ast.SliceArguments(b.singleton[0:1])
	}
	if e != nil {
		return st, e
	}
	// This source guard is separate from the initial DEBUG assertion.
	rc, e = m.IsBitVectorOperation(rhs, 37)
	if e != nil {
		return st, e
	}
	if rc {
		b.operands[1], e = m.AppArgumentView(rhs)
	} else {
		b.singleton[1] = rhs
		b.operands[1] = ast.SliceArguments(b.singleton[1:2])
	}
	if e != nil {
		return st, e
	}
	b.i1 = b.operands[0].Count()
	b.i2 = b.operands[1].Count()
	for b.i1 > 0 && b.i2 > 0 {
		a, e := b.operands[0].At(b.i1 - 1)
		if e != nil {
			return st, e
		}
		d, e := b.operands[1].At(b.i2 - 1)
		if e != nil {
			return st, e
		}
		size1, e := m.BitVectorSize(a)
		if e != nil {
			return st, e
		}
		size2, e := m.BitVectorSize(d)
		if e != nil {
			return st, e
		}
		if ast.ASTSourceDebug && (b.low1 >= size1 || b.low2 >= size2) {
			return st, ast.ErrASTDomain
		}
		rem1, rem2 := size1-b.low1, size2-b.low2
		high1, high2 := size1-1, size2-1
		if rem1 < rem2 {
			high2 = rem1 + b.low2 - 1
		} else if rem1 > rem2 {
			high1 = rem2 + b.low1 - 1
		}
		em, e := rotationManager(extract)
		if e != nil {
			return st, e
		}
		if em != m {
			return st, ast.ErrASTHandle
		}
		left, e := extract.Apply(high1, b.low1, a)
		if e != nil {
			return st, e
		}
		right, e := extract.Apply(high2, b.low2, d)
		if e != nil {
			return st, e
		}
		eq, e := m.MkEq(left, right)
		if e != nil {
			return st, e
		}
		b.equalities = append(b.equalities, eq)
		// Publish each original cursor update only after the equality push succeeds.
		if rem1 == rem2 {
			b.low1 = 0
			b.low2 = 0
			b.i1--
			b.i2--
		} else if rem1 < rem2 {
			b.low1 = 0
			b.low2 += rem1
			b.i1--
		} else {
			b.low1 += rem2
			b.low2 = 0
			b.i2--
		}
	}
	if ast.ASTSourceDebug && (b.i1 != 0 || b.i2 != 0 || len(b.equalities) < 1) {
		return st, ast.ErrASTDomain
	}
	result, e := m.MkAndArgs(b.equalities)
	if e != nil {
		return st, e
	}
	st = Rewrite3
	if e = m.SetRef(out, result); e != nil {
		return st, e
	}
	b.close()
	p.concatEquality = nil
	return
}

// Source: src/ast/rewriter/bv_rewriter.cpp:2372-2378.
func IsBitVectorConcatSplitTarget(m *ast.ASTManager, arg ast.ASTHandle, splitConcatEquality bool) (bool, error) {
	if splitConcatEquality {
		return true, nil
	}
	for _, kind := range [3]int32{37, 0, 31} {
		yes, e := m.IsBitVectorOperation(arg, kind)
		if e != nil || yes {
			return yes, e
		}
	}
	return false, nil
}

// Source: src/ast/rewriter/bv_rewriter.cpp:2433-2437.
func IsBitVectorConcatEqualityTarget(m *ast.ASTManager, lhs, rhs ast.ASTHandle, splitConcatEquality bool) (bool, error) {
	yes, e := m.IsBitVectorOperation(lhs, 37)
	if e != nil {
		return false, e
	}
	if yes {
		yes, e = IsBitVectorConcatSplitTarget(m, rhs, splitConcatEquality)
		if e != nil || yes {
			return yes, e
		}
	}
	yes, e = m.IsBitVectorOperation(rhs, 37)
	if e != nil || !yes {
		return yes, e
	}
	return IsBitVectorConcatSplitTarget(m, lhs, splitConcatEquality)
}
