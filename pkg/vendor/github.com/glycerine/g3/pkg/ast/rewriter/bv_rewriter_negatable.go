package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1155-1170;
// src/ast/rewriter/bv_rewriter.h:90.

import "github.com/glycerine/g3/pkg/ast"

func (p *BVRewriter) IsNegatable(arg ast.ASTHandle, out ast.ASTRef) (matched bool, err error) {
	if e := p.open(); e != nil {
		return false, e
	}
	p.running = true
	defer func() { p.running = false; p.fail(err) }()
	return p.isNegatable(arg, out)
}
func (p *BVRewriter) isNegatable(arg ast.ASTHandle, out ast.ASTRef) (bool, error) {
	m := p.manager
	n, e := ast.NewBitVectorNegatableNumerals(m)
	p.negatable = n
	if e != nil {
		return false, e
	}
	var width uint32
	numeral, e := n.Read(arg, &width)
	if e != nil {
		return false, e
	}
	var result ast.ASTHandle
	matched := numeral
	if numeral {
		if e = n.Complement(width); e != nil {
			return false, e
		}
		result, e = n.Constant(0, width)
		if e != nil {
			return false, e
		}
	} else {
		matched, e = m.IsBitVectorOperation(arg, 32)
		if e != nil {
			return false, e
		}
		if matched {
			if ast.ASTSourceDebug {
				count, x := m.GetNumArgs(arg)
				if x != nil {
					return false, x
				}
				if count != 1 {
					return false, ast.ErrASTDomain
				}
			}
			result, e = m.GetArg(arg, 0)
			if e != nil {
				return false, e
			}
		}
	}
	if matched {
		if e = m.SetRef(out, result); e != nil {
			return false, e
		}
	}
	if e = n.SourceEnd(); e != nil {
		return false, e
	}
	if e = n.Close(); e != nil {
		return false, e
	}
	p.negatable = nil
	return matched, nil
}
