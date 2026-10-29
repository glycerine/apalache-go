package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1905-1969; src/ast/rewriter/bv_rewriter.h:91.

import (
	"github.com/glycerine/g3/pkg/ast"
	"reflect"
)

type bvNotInvocation struct {
	numerals    *ast.BitVectorNotNumerals
	allone, one ast.ASTBitVectorNumeralInvocation
	args        []ast.ASTHandle
	ns, nt      ast.ASTRef
	triple      [3]ast.ASTHandle
	header      uint64
}

func (v *bvNotInvocation) endRefs(m *ast.ASTManager) error {
	if v.nt != (ast.ASTRef{}) {
		if e := m.DelRef(&v.nt); e != nil {
			return e
		}
	}
	if v.ns != (ast.ASTRef{}) {
		if e := m.DelRef(&v.ns); e != nil {
			return e
		}
	}
	return nil
}
func (v *bvNotInvocation) close(m *ast.ASTManager) error {
	if e := m.CleanupBitVectorNumeralInvocation(&v.one); e != nil {
		return e
	}
	if e := m.CleanupBitVectorNumeralInvocation(&v.allone); e != nil {
		return e
	}
	if v.numerals != nil {
		if e := v.numerals.CloseExpression(); e != nil {
			return e
		}
	}
	if e := v.endRefs(m); e != nil {
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

func (p *BVRewriter) RewriteNot(arg ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	// Original double-negation guard precedes even default val construction.
	matched, e := m.IsBitVectorOperation(arg, 32)
	if e != nil {
		return st, e
	}
	if matched {
		child, e := m.GetArg(arg, 0)
		if e != nil {
			return st, e
		}
		return RewriteDone, m.SetRef(out, child)
	}
	bytes := uint64(reflect.TypeOf(bvNotInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	v := &bvNotInvocation{header: bytes}
	p.bvnot = v
	n, e := ast.NewBitVectorNotNumerals(m)
	v.numerals = n
	if e != nil {
		return st, e
	}
	var width uint32
	matched, e = n.Read(arg, &width)
	if e != nil {
		return st, e
	}
	var result ast.ASTHandle
	if matched {
		if e = n.Complement(width); e != nil {
			return st, e
		}
		result, e = n.Constant(0, width)
		if e != nil {
			return st, e
		}
		st = RewriteDone
	} else {
		concat, e := m.IsBitVectorOperation(arg, 37)
		if e != nil {
			return st, e
		}
		if concat {
			count, e := m.GetNumArgs(arg)
			if e != nil {
				return st, e
			}
			for i := uint32(0); i < count; i++ {
				child, e := m.GetArg(arg, i)
				if e != nil {
					return st, e
				}
				child, e = m.MkBitVectorNot(child)
				if e != nil {
					return st, e
				}
				v.args = append(v.args, child)
			}
			result, e = m.MkBitVectorConcat(v.args)
			if e != nil {
				return st, e
			}
			st = Rewrite2
		} else {
			cfg, e := p.polynomial.config.BitVectorState()
			if e != nil {
				return st, e
			}
			if cfg.NotToArithmetic {
				width, e = m.BitVectorSize(arg)
				if e != nil {
					return st, e
				}
				if e = n.MinusOne(width, false); e != nil {
					return st, e
				}
				minus, e := n.Constant(4, width)
				if e != nil {
					return st, e
				}
				result, e = m.MkBitVectorSub(minus, arg)
				if e != nil {
					return st, e
				}
				st = Rewrite1
			} else if cfg.NotSimplification {
				result, st, e = p.bvNotSimplify(v, arg)
				if e != nil {
					return st, e
				}
			}
		}
	}
	if st != RewriteFailed {
		if err = m.SetRef(out, result); err != nil {
			return
		}
	}
	// The minus_one local, concat buffer, and nt/ns are younger than val.
	if err = n.End(4, 3, 2, 1); err != nil {
		return
	}
	clear(v.args)
	v.args = nil
	if err = v.endRefs(m); err != nil {
		return
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = v.close(m); err != nil {
		return
	}
	p.bvnot = nil
	return
}

func (p *BVRewriter) bvNotSimplify(v *bvNotInvocation, arg ast.ASTHandle) (ast.ASTHandle, RewriteStatus, error) {
	m := p.manager
	binary := func(kind int32) (bool, ast.ASTHandle, ast.ASTHandle, error) {
		yes, e := m.IsBitVectorOperation(arg, kind)
		if e != nil || !yes {
			return false, ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		count, e := m.GetNumArgs(arg)
		if e != nil || count != 2 {
			return false, ast.ASTHandle{}, ast.ASTHandle{}, e
		}
		s, e := m.GetArg(arg, 0)
		if e != nil {
			return false, s, ast.ASTHandle{}, e
		}
		t, e := m.GetArg(arg, 1)
		return e == nil, s, t, e
	}
	mul, s, t, e := binary(6)
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	if mul {
		width, e := m.BitVectorSize(s)
		if e != nil {
			return ast.ASTHandle{}, RewriteFailed, e
		}
		left, e := m.IsBitVectorAllOneWithInvocation(s, &v.allone)
		if e != nil {
			return ast.ASTHandle{}, RewriteFailed, e
		}
		other := t
		if !left {
			left, e = m.IsBitVectorAllOneWithInvocation(t, &v.allone)
			if e != nil {
				return ast.ASTHandle{}, RewriteFailed, e
			}
			other = s
		}
		if left {
			if e = v.numerals.MinusOne(width, true); e != nil {
				return ast.ASTHandle{}, RewriteFailed, e
			}
			minus, e := v.numerals.Constant(4, width)
			if e != nil {
				return ast.ASTHandle{}, RewriteFailed, e
			}
			result, e := m.MkBitVectorAdd(minus, other)
			return result, Rewrite1, e
		}
	}
	add, s, t, e := binary(4)
	if e != nil || !add {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	v.ns, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	v.nt, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	yes, e := p.isNegatable(t, v.nt)
	if e != nil || !yes {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	yes, e = p.isNegatable(s, v.ns)
	if e != nil || !yes {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	width, e := m.BitVectorSize(s)
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	v.triple[0], e = m.BitVectorNumeralOneWithInvocation(width, &v.one)
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	v.triple[1], e = m.RefGet(v.ns)
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	v.triple[2], e = m.RefGet(v.nt)
	if e != nil {
		return ast.ASTHandle{}, RewriteFailed, e
	}
	result, e := m.MkFamilyApp(p.bvFamily, 4, nil, v.triple[:])
	return result, Rewrite1, e
}
