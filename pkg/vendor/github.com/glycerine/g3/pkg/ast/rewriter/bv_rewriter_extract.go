package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:668-790;
// src/ast/rewriter/bv_rewriter.h:85.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvExtractInvocation struct {
	numerals   *ast.BitVectorExtractNumerals
	args       []ast.ASTHandle
	propagated ast.ASTRef
	ite        [3]ast.ASTHandle
	header     uint64
}

func (v *bvExtractInvocation) endRef(m *ast.ASTManager) error {
	if v.propagated != (ast.ASTRef{}) {
		return m.DelRef(&v.propagated)
	}
	return nil
}
func (v *bvExtractInvocation) close(m *ast.ASTManager) error {
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
	}
	if e := v.endRef(m); e != nil {
		return e
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

func (p *BVRewriter) RewriteExtract(high, low uint32, arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	width, e := m.BitVectorSize(arg)
	if e != nil {
		return st, e
	}
	if width == 0 {
		return st, ast.ErrASTDomain
	}
	if low == 0 && high == width-1 {
		return RewriteDone, m.SetRef(out, arg)
	}
	bytes := uint64(reflect.TypeOf(bvExtractInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	v := &bvExtractInvocation{header: bytes}
	p.extraction = v
	v.numerals, e = ast.NewBitVectorExtractNumerals(m)
	if e != nil {
		return st, e
	}
	result, status, e := p.bvExtractBody(v, high, low, arg, width)
	st = status
	if e != nil {
		return st, e
	}
	if st != RewriteFailed {
		if e = m.SetRef(out, result); e != nil {
			return st, e
		}
	}
	if e = v.numerals.End(2, 1); e != nil {
		return st, e
	}
	if e = v.endRef(m); e != nil {
		return st, e
	}
	clear(v.args)
	v.args = nil
	if e = v.numerals.SourceEnd(); e != nil {
		return st, e
	}
	if e = v.close(m); e != nil {
		return st, e
	}
	p.extraction = nil
	return st, nil
}

func (p *BVRewriter) bvExtractBody(v *bvExtractInvocation, high, low uint32, arg ast.ASTHandle, width uint32) (ast.ASTHandle, RewriteStatus, error) {
	m := p.manager
	failed := ast.ASTHandle{}
	numeric, e := v.numerals.Read(arg, &width)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if numeric {
		node, e := v.numerals.Evaluate(high, low)
		return node, RewriteDone, e
	}
	nested, e := m.IsBitVectorOperation(arg, 40)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if nested {
		offset, e := m.BitVectorExtractLow(arg)
		if e != nil {
			return failed, RewriteFailed, e
		}
		child, e := m.GetArg(arg, 0)
		if e != nil {
			return failed, RewriteFailed, e
		}
		node, e := p.extract.Apply(high+offset, low+offset, child)
		return node, RewriteDone, e
	}
	concat, e := m.IsBitVectorOperation(arg, 37)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if concat {
		return p.bvExtractConcat(v, high, low, arg, width)
	}
	var distribute bool
	for _, k := range []int32{32, 31, 33} {
		distribute, e = m.IsBitVectorOperation(arg, k)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if distribute {
			break
		}
	}
	if !distribute && low == 0 {
		distribute, e = m.IsBitVectorOperation(arg, 4)
		if e == nil && !distribute {
			distribute, e = m.IsBitVectorOperation(arg, 6)
		}
		if e != nil {
			return failed, RewriteFailed, e
		}
	}
	if distribute {
		args, e := m.AppArgumentView(arg)
		if e != nil {
			return failed, RewriteFailed, e
		}
		for i := uint32(0); i < args.Count(); i++ {
			child, e := args.At(i)
			if e == nil {
				child, e = p.extract.Apply(high, low, child)
			}
			if e == nil {
				v.args = append(v.args, child)
			}
			if e != nil {
				return failed, RewriteFailed, e
			}
		}
		kind, e := m.GetDeclKind(arg)
		if e != nil {
			return failed, RewriteFailed, e
		}
		node, e := m.MkFamilyApp(p.bvFamily, kind, nil, v.args)
		return node, Rewrite2, e
	}
	if p.polynomial.config.bitVector.ExtractPropagation && high >= low {
		v.propagated, e = m.NewRef(ast.ASTHandle{})
		if e != nil {
			return failed, RewriteFailed, e
		}
		removed, e := p.propagateExtract(high, arg, v.propagated)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if removed != 0 {
			term, e := m.RefGet(v.propagated)
			if e != nil {
				return failed, RewriteFailed, e
			}
			node, e := p.extract.Apply(high, low, term)
			if e != nil {
				return failed, RewriteFailed, e
			}
			return node, Rewrite2, p.traceExtractPropagation(arg, node, high, low, removed)
		}
		// A failed predicate still ends ep_res before the following ITE guard.
		if e = v.endRef(m); e != nil {
			return failed, RewriteFailed, e
		}
	}
	kind, e := m.GetKind(arg)
	if e != nil {
		return failed, RewriteFailed, e
	}
	if kind == ast.ASTApp {
		family, e := m.GetFamilyID(arg)
		if e != nil {
			return failed, RewriteFailed, e
		}
		if family == m.BasicFamilyID() {
			kind, e := m.GetDeclKind(arg)
			if e != nil {
				return failed, RewriteFailed, e
			}
			if kind == ast.ASTOpIte {
				v.ite[0], e = m.GetArg(arg, 0)
				if e != nil {
					return failed, RewriteFailed, e
				}
				for i := uint32(1); i < 3; i++ {
					v.ite[i], e = m.GetArg(arg, i)
					if e == nil {
						v.ite[i], e = p.extract.Apply(high, low, v.ite[i])
					}
					if e != nil {
						return failed, RewriteFailed, e
					}
				}
				node, e := m.MkIte(v.ite[0], v.ite[1], v.ite[2])
				return node, Rewrite2, e
			}
		}
	}
	return failed, RewriteFailed, nil
}

func (p *BVRewriter) bvExtractConcat(v *bvExtractInvocation, high, low uint32, arg ast.ASTHandle, width uint32) (ast.ASTHandle, RewriteStatus, error) {
	m := p.manager
	failed := ast.ASTHandle{}
	args, e := m.AppArgumentView(arg)
	if e != nil {
		return failed, RewriteFailed, e
	}
	index := width
	for i := uint32(0); i < args.Count(); i++ {
		current, e := args.At(i)
		if e != nil {
			return failed, RewriteFailed, e
		}
		bits, e := m.BitVectorSize(current)
		if e != nil {
			return failed, RewriteFailed, e
		}
		index -= bits
		if index > high {
			continue
		}
		if index <= low {
			if low == index && high-index == bits-1 {
				return current, RewriteDone, nil
			}
			node, e := p.extract.Apply(high-index, low-index, current)
			return node, Rewrite1, e
		}
		usedExtract := high-index != bits-1
		if usedExtract {
			current, e = p.extract.Apply(high-index, 0, current)
		}
		if e == nil {
			v.args = append(v.args, current)
		}
		if e != nil {
			return failed, RewriteFailed, e
		}
		for j := i + 1; j < args.Count(); j++ {
			current, e = args.At(j)
			if e != nil {
				return failed, RewriteFailed, e
			}
			bits, e = m.BitVectorSize(current)
			if e != nil {
				return failed, RewriteFailed, e
			}
			index -= bits
			if index > low {
				v.args = append(v.args, current)
				continue
			}
			st := Rewrite2
			if index == low {
				if !usedExtract {
					st = RewriteDone
				}
			} else {
				current, e = p.extract.Apply(bits-1, low-index, current)
			}
			if e == nil {
				v.args = append(v.args, current)
			}
			if e != nil {
				return failed, RewriteFailed, e
			}
			node, e := m.MkBitVectorConcat(v.args)
			return node, st, e
		}
		return failed, RewriteFailed, ast.ErrASTDomain
	}
	return failed, RewriteFailed, ast.ErrASTDomain
}
