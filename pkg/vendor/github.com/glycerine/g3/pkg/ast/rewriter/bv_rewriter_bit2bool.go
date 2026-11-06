package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2205-2277.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type bvBitBoolInvocation struct {
	manager  *ast.ASTManager
	numerals *ast.BitVectorBit2BoolNumerals
	bit1     ast.ASTRef
	args     []ast.ASTHandle
	header   uint64
}

func (b *bvBitBoolInvocation) close() error {
	clear(b.args)
	b.args = nil
	if b.bit1 != (ast.ASTRef{}) {
		if e := b.manager.DelRef(&b.bit1); e != nil {
			return e
		}
	}
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
func (b *bvBitBoolInvocation) sourceEnd() error {
	clear(b.args)
	b.args = nil
	if b.bit1 != (ast.ASTRef{}) {
		if e := b.manager.DelRef(&b.bit1); e != nil {
			return e
		}
	}
	return b.numerals.SourceEnd()
}

func (p *BVPolynomialRewriter) RewriteBit2Bool(lhs, rhs ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
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
	if width != 1 {
		return
	}
	yes, e := m.IsBitVectorOperation(lhs, 0)
	if e != nil {
		return st, e
	}
	if yes {
		lhs, rhs = rhs, lhs
	}
	bytes := uint64(reflect.TypeOf(bvBitBoolInvocation{}).Size())
	if e = m.Budget().Reserve(bytes); e != nil {
		return st, e
	}
	b := &bvBitBoolInvocation{manager: m, header: bytes}
	p.bitBool = b
	b.numerals, e = ast.NewBitVectorBit2BoolNumerals(m)
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
		p.bitBool = nil
		return st, nil
	}
	yes, e = n.Read(rhs)
	if e != nil {
		return st, e
	}
	if !yes {
		return finish()
	}
	yes, e = m.IsBitVectorOperation(lhs, 0)
	if e != nil {
		return st, e
	}
	if yes {
		result, e := m.MkBoolVal(lhs == rhs)
		if e != nil {
			return st, e
		}
		st = RewriteDone
		if e = m.SetRef(out, result); e != nil {
			return st, e
		}
		return finish()
	}
	// Variables are not applications and have no declaration identity.
	kind, e := m.GetKind(lhs)
	if e != nil {
		return st, e
	}
	if kind == ast.ASTApp {
		family, e := m.GetFamilyID(lhs)
		if e != nil {
			return st, e
		}
		op, e := m.GetDeclKind(lhs)
		if e != nil {
			return st, e
		}
		if family == m.BasicFamilyID() && op == ast.ASTOpIte {
			condition, e := m.GetArg(lhs, 0)
			if e != nil {
				return st, e
			}
			a, e := m.GetArg(lhs, 1)
			if e != nil {
				return st, e
			}
			left, e := m.MkEq(a, rhs)
			if e != nil {
				return st, e
			}
			a, e = m.GetArg(lhs, 2)
			if e != nil {
				return st, e
			}
			right, e := m.MkEq(a, rhs)
			if e != nil {
				return st, e
			}
			result, e := m.MkIte(condition, left, right)
			if e != nil {
				return st, e
			}
			st = Rewrite2
			if e = m.SetRef(out, result); e != nil {
				return st, e
			}
			return finish()
		}
	}
	yes, e = m.IsBitVectorOperation(lhs, 32)
	if e != nil {
		return st, e
	}
	if yes {
		if ast.ASTSourceDebug {
			one, e := n.IsOne()
			if e != nil {
				return st, e
			}
			if !one {
				zero, e := n.IsZero()
				if e != nil {
					return st, e
				}
				if !zero {
					return st, ast.ErrASTDomain
				}
			}
		}
		arg, e := m.GetArg(lhs, 0)
		if e != nil {
			return st, e
		}
		opposite, e := n.LiteralOne(true)
		if e != nil {
			return st, e
		}
		result, e := m.MkEq(arg, opposite)
		if e != nil {
			return st, e
		}
		st = Rewrite1
		if e = m.SetRef(out, result); e != nil {
			return st, e
		}
		return finish()
	}
	one, e := n.IsOne()
	if e != nil {
		return st, e
	}
	b.bit1, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return st, e
	}
	bit := rhs
	if !one {
		bit, e = n.LiteralOne(false)
		if e != nil {
			return st, e
		}
	}
	if e = m.SetRef(b.bit1, bit); e != nil {
		return st, e
	}
	if e = n.EndExpression(); e != nil {
		return st, e
	}
	bor, e := m.IsBitVectorOperation(lhs, 31)
	if e != nil {
		return st, e
	}
	bxor := false
	if !bor {
		bxor, e = m.IsBitVectorOperation(lhs, 33)
		if e != nil {
			return st, e
		}
	}
	if !bor && !bxor {
		return finish()
	}
	operands, e := m.AppArgumentView(lhs)
	if e != nil {
		return st, e
	}
	for i := uint32(0); i < operands.Count(); i++ {
		arg, e := operands.At(i)
		if e != nil {
			return st, e
		}
		eq, e := m.MkEq(arg, bit)
		if e != nil {
			return st, e
		}
		b.args = append(b.args, eq)
	}
	var result ast.ASTHandle
	if bor {
		result, e = m.MkOrArgs(b.args)
	} else {
		result, e = m.MkXorArgs(b.args)
	}
	if e != nil {
		return st, e
	}
	st = Rewrite2
	if e = m.SetRef(out, result); e != nil {
		return st, e
	}
	if !one {
		result, e = m.MkNot(result)
		if e != nil {
			return st, e
		}
		st = Rewrite3
		if e = m.SetRef(out, result); e != nil {
			return st, e
		}
	}
	return finish()
}
