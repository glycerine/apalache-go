package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1146-1153,1172-1184;
// src/ast/rewriter/bv_rewriter.h:110-111.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) IsMinusOne(arg ast.ASTHandle) (matched bool, err error) {
	if e := p.open(); e != nil {
		return false, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.isMinusOneCore(arg)
}
func (p *BVRewriter) isMinusOneCore(arg ast.ASTHandle) (bool, error) {
	n, e := ast.NewBitVectorMinusOneNumerals(p.manager)
	p.minusOne = n
	if e != nil {
		return false, e
	}
	var width uint32
	yes, e := n.Read(arg, &width)
	if e != nil {
		return false, e
	}
	if yes {
		yes, e = n.Compare(width)
		if e != nil {
			return false, e
		}
	}
	if e = n.SourceEnd(); e != nil {
		return false, e
	}
	if e = n.Close(); e != nil {
		return false, e
	}
	p.minusOne = nil
	return yes, nil
}

// IsXMinusOne writes the borrowed other operand only after a match. The first
// all-ones operand wins if both match, as in the original ordered guards.
func (p *BVRewriter) IsXMinusOne(arg ast.ASTHandle, x *ast.ASTHandle) (matched bool, err error) {
	if e := p.open(); e != nil {
		return false, e
	}
	if x == nil {
		return false, ast.ErrASTHandle
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.isXMinusOne(arg, x)
}
func (p *BVRewriter) isXMinusOne(arg ast.ASTHandle, x *ast.ASTHandle) (bool, error) {
	m := p.manager
	yes, e := m.IsBitVectorOperation(arg, 4)
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
	yes, e = p.isMinusOneCore(first)
	if e != nil {
		return false, e
	}
	if yes {
		other, e := m.GetArg(arg, 1)
		if e != nil {
			return false, e
		}
		*x = other
		return true, nil
	}
	second, e := m.GetArg(arg, 1)
	if e != nil {
		return false, e
	}
	yes, e = p.isMinusOneCore(second)
	if e != nil {
		return false, e
	}
	if yes {
		other, e := m.GetArg(arg, 0)
		if e != nil {
			return false, e
		}
		*x = other
		return true, nil
	}
	return false, nil
}
