package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2380-2431.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvIsolationInvocation struct {
	manager *ast.ASTManager
	terms   [2]ast.ASTRef
	allOne  ast.ASTBitVectorNumeralInvocation
	header  uint64
}

func (b *bvIsolationInvocation) endRefs() error {
	for i := 1; i >= 0; i-- {
		if b.terms[i] != (ast.ASTRef{}) {
			if e := b.manager.DelRef(&b.terms[i]); e != nil {
				return e
			}
		}
	}
	return nil
}
func (b *bvIsolationInvocation) close() error {
	if e := b.manager.CleanupBitVectorNumeralInvocation(&b.allOne); e != nil {
		return e
	}
	if e := b.endRefs(); e != nil {
		return e
	}
	b.manager.Budget().Release(b.header)
	b.header = 0
	return nil
}
func bvMinusOneTimes(m *ast.ASTManager, arg ast.ASTHandle, call *ast.ASTBitVectorNumeralInvocation) (bool, error) {
	yes, e := m.IsBitVectorOperation(arg, 6)
	if e != nil || !yes {
		return false, e
	}
	count, e := m.GetNumArgs(arg)
	if e != nil || count != 2 {
		return false, e
	}
	first, e := m.GetArg(arg, 0)
	if e != nil {
		return false, e
	}
	if _, e = m.GetArg(arg, 1); e != nil {
		return false, e
	}
	return m.IsBitVectorAllOneWithInvocation(first, call)
}
func (b *bvIsolationInvocation) sumEquality(first, tail, constant ast.ASTHandle, out ast.ASTRef) error {
	if ast.ASTSourceDebug {
		yes, e := b.manager.IsBitVectorOperation(constant, 0)
		if e != nil {
			return e
		}
		if !yes {
			return ast.ErrASTDomain
		}
	}
	minus, e := bvMinusOneTimes(b.manager, first, &b.allOne)
	if e != nil {
		return e
	}
	target, removed := first, tail
	if minus {
		target, removed = tail, first
	}
	sub, e := b.manager.MkBitVectorSub(constant, removed)
	if e != nil {
		return e
	}
	result, e := b.manager.MkEq(target, sub)
	if e != nil {
		return e
	}
	return b.manager.SetRef(out, result)
}
func bvNumeralAndAdd(m *ast.ASTManager, lhs, rhs ast.ASTHandle) (bool, error) {
	yes, e := m.IsBitVectorOperation(lhs, 0)
	if e != nil || !yes {
		return false, e
	}
	return m.IsBitVectorOperation(rhs, 4)
}

// IsolateBitVectorTerm keeps the original first term and raw remaining sum.
// It constructs both owning term references before assigning either one.
func (p *BVPolynomialRewriter) IsolateBitVectorTerm(lhs, rhs ast.ASTHandle, out ast.ASTRef) (matched bool, err error) {
	if e := p.open(); e != nil {
		return false, e
	}
	p.running = true
	defer func() {
		p.running = false
		if err != nil {
			p.fail(err)
		}
	}()
	m := p.manager
	yes, e := bvNumeralAndAdd(m, lhs, rhs)
	if e != nil {
		return false, e
	}
	if !yes {
		lhs, rhs = rhs, lhs
	}
	yes, e = bvNumeralAndAdd(m, lhs, rhs)
	if e != nil || !yes {
		return false, e
	}
	count, e := m.GetNumArgs(rhs)
	if e != nil {
		return false, e
	}
	bytes := uint64(reflect.TypeOf(bvIsolationInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return false, e
	}
	b := &bvIsolationInvocation{manager: m, header: bytes}
	p.isolation = b
	for i := range b.terms {
		b.terms[i], e = m.NewRef(ast.ASTHandle{})
		if e != nil {
			return false, e
		}
	}
	first, e := m.GetArg(rhs, 0)
	if e != nil {
		return false, e
	}
	if e = m.SetRef(b.terms[0], first); e != nil {
		return false, e
	}
	var tail ast.ASTHandle
	if count > 2 {
		args, e := m.AppArgumentView(rhs)
		if e != nil {
			return false, e
		}
		rest, e := args.Range(1, count-1)
		if e != nil {
			return false, e
		}
		tail, e = m.MkBitVectorPolynomialAdd(ast.ASTHandle{}, rest)
		if e != nil {
			return false, e
		}
	} else {
		if ast.ASTSourceDebug && count != 2 {
			return false, ast.ErrASTDomain
		}
		tail, e = m.GetArg(rhs, 1)
		if e != nil {
			return false, e
		}
	}
	if e = m.SetRef(b.terms[1], tail); e != nil {
		return false, e
	}
	if e = b.sumEquality(first, tail, lhs, out); e != nil {
		return false, e
	}
	matched = true
	if e = m.BitVectorNumeralInvocationSourceReady(&b.allOne); e != nil {
		return matched, e
	}
	if e = b.endRefs(); e != nil {
		return matched, e
	}
	if e = b.close(); e != nil {
		return matched, e
	}
	p.isolation = nil
	return
}

// Source: src/ast/rewriter/bv_rewriter.cpp:2416-2431.
func IsBitVectorAddMulConstant(m *ast.ASTManager, arg ast.ASTHandle) (bool, error) {
	yes, e := m.IsBitVectorOperation(arg, 4)
	if e != nil || !yes {
		return false, e
	}
	args, e := m.AppArgumentView(arg)
	if e != nil {
		return false, e
	}
	for i := uint32(0); i < args.Count(); i++ {
		term, e := args.At(i)
		if e != nil {
			return false, e
		}
		numeral, e := m.IsBitVectorOperation(term, 0)
		if e != nil {
			return false, e
		}
		if numeral {
			continue
		}
		product, e := m.IsBitVectorOperation(term, 6)
		if e != nil || !product {
			return false, e
		}
		count, e := m.GetNumArgs(term)
		if e != nil || count != 2 {
			return false, e
		}
		coefficient, e := m.GetArg(term, 0)
		if e != nil {
			return false, e
		}
		if _, e = m.GetArg(term, 1); e != nil {
			return false, e
		}
		numeral, e = m.IsBitVectorOperation(coefficient, 0)
		if e != nil || !numeral {
			return false, e
		}
	}
	return true, nil
}
