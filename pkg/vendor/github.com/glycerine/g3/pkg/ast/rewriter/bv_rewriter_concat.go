package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:1451-1510.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// pending keeps a popped root reachable if its destructor has not consumed it.
type bvConcatInvocation struct {
	args     []ast.ASTHandle
	pending  ast.ASTHandle
	numerals *ast.BitVectorConcatNumerals
	header   uint64
}

func (v *bvConcatInvocation) endPending(m *ast.ASTManager) error {
	if v.pending.IsNull() {
		return nil
	}
	consumed, e := m.DecRefConsumed(v.pending)
	if consumed {
		v.pending = ast.ASTHandle{}
	}
	return e
}
func (v *bvConcatInvocation) popArgs(m *ast.ASTManager) error {
	if !v.pending.IsNull() {
		return ast.ErrASTPending
	}
	if len(v.args) == 0 {
		return ast.ErrASTDomain
	}
	last := len(v.args) - 1
	v.pending = v.args[last]
	v.args[last] = ast.ASTHandle{}
	v.args = v.args[:last]
	return v.endPending(m)
}
func (v *bvConcatInvocation) close(m *ast.ASTManager) error {
	if v.numerals != nil {
		if e := v.numerals.Close(); e != nil {
			return e
		}
		v.numerals = nil
	}
	if e := v.endPending(m); e != nil {
		return e
	}
	if e := shrinkRewriterRoots(m, &v.args, 0); e != nil {
		return e
	}
	v.args = nil
	m.Budget().Release(v.header)
	v.header = 0
	return nil
}

func (p *BVRewriter) RewriteConcat(args ast.AppArguments, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	bytes := uint64(reflect.TypeOf(bvConcatInvocation{}).Size())
	if e := m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	v := &bvConcatInvocation{header: bytes}
	p.concat = v
	n, e := ast.NewBitVectorConcatNumerals(m)
	v.numerals = n
	if e != nil {
		return st, e
	}
	fusedNumeral, expanded, fusedExtract := false, false, false
	var sz1, sz2 uint32
	for i := uint32(0); i < args.Count(); i++ {
		arg, e := args.At(i)
		if e != nil {
			return st, e
		}
		var prev ast.ASTHandle
		if i > 0 {
			hs := v.args
			if len(hs) == 0 {
				return st, ast.ErrASTDomain
			}
			prev = hs[len(hs)-1]
		}
		num, e := n.Read(0, arg, &sz1)
		if e != nil {
			return st, e
		}
		priorNum := false
		if num && !prev.IsNull() {
			priorNum, e = n.Read(1, prev, &sz2)
			if e != nil {
				return st, e
			}
		}
		if num && priorNum {
			if e = n.Fuse(sz1); e != nil {
				return st, e
			}
			if e = v.popArgs(m); e != nil {
				return st, e
			}
			h, e := n.Constant(1, sz1+sz2)
			if e != nil {
				return st, e
			}
			if e = appendRewriterRoot(m, &v.args, h); e != nil {
				return st, e
			}
			fusedNumeral = true
			continue
		}
		nested := false
		if p.polynomial.config.state.Flat {
			nested, e = m.IsBitVectorOperation(arg, 37)
			if e != nil {
				return st, e
			}
		}
		if nested {
			inner, e := m.AppArgumentView(arg)
			if e != nil {
				return st, e
			}
			for j := uint32(0); j < inner.Count(); j++ {
				h, e := inner.At(j)
				if e != nil {
					return st, e
				}
				if e = appendRewriterRoot(m, &v.args, h); e != nil {
					return st, e
				}
			}
			expanded = true
			continue
		}
		extract, e := m.IsBitVectorOperation(arg, 40)
		if e != nil {
			return st, e
		}
		if extract && !prev.IsNull() {
			prior, e := m.IsBitVectorOperation(prev, 40)
			if e != nil {
				return st, e
			}
			if prior {
				av, e := m.AppArgumentView(arg)
				if e != nil {
					return st, e
				}
				base, e := av.At(0)
				if e != nil {
					return st, e
				}
				pv, e := m.AppArgumentView(prev)
				if e != nil {
					return st, e
				}
				pbase, e := pv.At(0)
				if e != nil {
					return st, e
				}
				if base == pbase {
					low, e := m.BitVectorExtractLow(prev)
					if e != nil {
						return st, e
					}
					high, e := m.BitVectorExtractHigh(arg)
					if e != nil {
						return st, e
					}
					if low == high+1 {
						high, e = m.BitVectorExtractHigh(prev)
						if e != nil {
							return st, e
						}
						low, e = m.BitVectorExtractLow(arg)
						if e != nil {
							return st, e
						}
						h, e := p.extract.Apply(high, low, base)
						if e != nil {
							return st, e
						}
						if e = v.popArgs(m); e != nil {
							return st, e
						}
						if e = appendRewriterRoot(m, &v.args, h); e != nil {
							return st, e
						}
						fusedExtract = true
						continue
					}
				}
			}
		}
		if e = appendRewriterRoot(m, &v.args, arg); e != nil {
			return st, e
		}
	}
	if fusedNumeral || expanded || fusedExtract {
		hs := v.args
		if len(hs) == 0 {
			return st, ast.ErrASTDomain
		}
		h := hs[0]
		if len(hs) == 1 {
			st = RewriteDone
			if fusedExtract {
				st = Rewrite1
			}
		} else {
			h, e = m.MkBitVectorConcat(hs)
			if e != nil {
				return st, e
			}
			st = RewriteDone
			if fusedExtract {
				st = Rewrite2
			} else if expanded {
				st = Rewrite1
			}
		}
		if err = m.SetRef(out, h); err != nil {
			return
		}
	}
	// The numeric locals end before the older owning argument slice.
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = v.close(m); err != nil {
		return
	}
	p.concat = nil
	return
}
