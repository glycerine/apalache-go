package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2279-2304.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvBlastEqInvocation struct {
	manager  *ast.ASTManager
	numerals *ast.BitVectorBlastEqualityNumerals
	args     []ast.ASTHandle
	header   uint64
}

func (b *bvBlastEqInvocation) close() error {
	clear(b.args)
	b.args = nil
	if b.numerals != nil {
		if e := b.numerals.Close(); e != nil {
			return e
		}
		b.numerals = nil
	}
	b.manager.Budget().Release(b.header)
	b.header = 0
	return nil
}
func (b *bvBlastEqInvocation) sourceEnd() error {
	clear(b.args)
	b.args = nil
	return b.numerals.SourceEnd()
}

func (p *BVPolynomialRewriter) RewriteBlastEqualityValue(lhs, rhs ast.ASTHandle, out ast.ASTRef, extract *MkExtractProc) (st RewriteStatus, err error) {
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
	width, e := m.BitVectorSize(lhs)
	if e != nil {
		return st, e
	}
	if width == 1 {
		return
	}
	if err = p.traceBlastEqualityValue(width, lhs); err != nil {
		return st, err
	}
	yes, e := m.IsBitVectorOperation(lhs, 0)
	if e != nil {
		return st, e
	}
	if yes {
		lhs, rhs = rhs, lhs
	}
	bytes := uint64(reflect.TypeOf(bvBlastEqInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	b := &bvBlastEqInvocation{manager: m, header: bytes}
	p.blastEquality = b
	b.numerals, e = ast.NewBitVectorBlastEqualityNumerals(m)
	if e != nil {
		return st, e
	}
	n := b.numerals
	finish := func() (RewriteStatus, error) {
		if e := b.sourceEnd(); e != nil {
			return st, e
		}
		if e := b.close(); e != nil {
			return st, e
		}
		p.blastEquality = nil
		return st, nil
	}
	yes, e = n.Read(rhs, &width)
	if e != nil {
		return st, e
	}
	if !yes {
		return finish()
	}
	yes, e = m.IsBitVectorOperation(lhs, 31)
	if e != nil {
		return st, e
	}
	if !yes {
		yes, e = m.IsBitVectorOperation(lhs, 33)
		if e != nil {
			return st, e
		}
	}
	if !yes {
		yes, e = m.IsBitVectorOperation(lhs, 32)
		if e != nil {
			return st, e
		}
	}
	if !yes {
		return finish()
	}
	if e = n.BeginTwo(); e != nil {
		return st, e
	}
	for i := uint32(0); i < width; i++ {
		zero, e := n.RemainderIsZero()
		if e != nil {
			return st, e
		}
		em, e := rotationManager(extract)
		if e != nil {
			return st, e
		}
		if em != m {
			return st, ast.ErrASTHandle
		}
		bit, e := extract.Apply(i, i, lhs)
		if e != nil {
			return st, e
		}
		value, e := n.Bit(zero)
		if e != nil {
			return st, e
		}
		eq, e := m.MkEq(bit, value)
		if e != nil {
			return st, e
		}
		b.args = append(b.args, eq)
		if e = n.Divide(); e != nil {
			return st, e
		}
	}
	result, e := m.MkAndArgs(b.args)
	if e != nil {
		return st, e
	}
	st = Rewrite3
	if e = m.SetRef(out, result); e != nil {
		return st, e
	}
	return finish()
}
