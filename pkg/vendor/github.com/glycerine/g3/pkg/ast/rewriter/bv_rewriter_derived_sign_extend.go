package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1525-1553;
// src/ast/rewriter/bv_rewriter.h:89.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

type bvSignExtensionInvocation struct {
	numerals *ast.BitVectorSignExtensionNumerals
	args     []ast.ASTHandle
	header   uint64
}

func (v *bvSignExtensionInvocation) close(m *ast.ASTManager) error {
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
	}
	clear(v.args)
	v.args = nil
	if v.numerals != nil {
		if e := v.numerals.Close(); e != nil {
			return e
		}
		v.numerals = nil
	}
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}
func (p *BVRewriter) RewriteSignExtend(amount uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	if amount == 0 {
		return RewriteDone, m.SetRef(out, arg)
	}
	bytes := uint64(reflect.TypeOf(bvSignExtensionInvocation{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return RewriteFailed, e
	}
	v := &bvSignExtensionInvocation{header: bytes}
	p.signExtension = v
	var e error
	v.numerals, e = ast.NewBitVectorSignExtensionNumerals(m)
	if e != nil {
		return RewriteFailed, e
	}
	var width uint32
	numeric, e := v.numerals.Read(arg, &width)
	if e != nil {
		return RewriteFailed, e
	}
	var node ast.ASTHandle
	st = RewriteFailed
	if numeric {
		node, e = v.numerals.Evaluate(amount, width)
		st = RewriteDone
	} else if p.polynomial.config.bitVector.EliminateSignExtension {
		width, e = m.BitVectorSize(arg)
		if e != nil {
			return st, e
		}
		sign, e := p.extract.Apply(width-1, width-1, arg)
		if e != nil {
			return st, e
		}
		for i := uint32(0); i < amount; i++ {
			v.args = append(v.args, sign)
		}
		v.args = append(v.args, arg)
		node, e = m.MkBitVectorConcat(v.args)
		st = Rewrite2
	}
	if e != nil {
		return st, e
	}
	if st != RewriteFailed {
		if e = m.SetRef(out, node); e != nil {
			return st, e
		}
	}
	clear(v.args)
	v.args = nil
	if e = v.numerals.SourceEnd(); e != nil {
		return st, e
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.signExtension = nil
	return st, nil
}
