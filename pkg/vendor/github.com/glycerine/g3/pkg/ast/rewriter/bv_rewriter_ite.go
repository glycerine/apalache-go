package rewriter

// Source: src/ast/rewriter/bv_rewriter.cpp:2657-2704.

import "github.com/glycerine/g3/pkg/ast"

func bvConditionIsEquality(m *ast.ASTManager, arg ast.ASTHandle) (bool, error) {
	kind, e := m.GetKind(arg)
	if e != nil || kind != ast.ASTApp {
		return false, e
	}
	family, e := m.GetFamilyID(arg)
	if e != nil || family != m.BasicFamilyID() {
		return false, e
	}
	op, e := m.GetDeclKind(arg)
	return op == ast.ASTOpEq, e
}
func (p *BVRewriter) RewriteIte(condition, then, otherwise ast.ASTHandle, out ast.ASTRef) (st RewriteStatus, err error) {
	if e := p.open(); e != nil {
		return RewriteFailed, e
	}
	p.running = true
	st = RewriteFailed
	defer func() { p.running = false; p.fail(err) }()
	m := p.manager
	if err = p.traceITECore(condition, then, otherwise); err != nil {
		return st, err
	}
	assign := func(value ast.ASTHandle) error { st = Rewrite1; return m.SetRef(out, value) }
	equal, e := m.AreEqual(then, otherwise)
	if e != nil {
		return st, e
	}
	if equal {
		err = assign(otherwise)
		return
	}
	negative, inner, e := m.IsNot(condition)
	if e != nil {
		return st, e
	}
	if negative {
		h, e := m.MkIte(inner, otherwise, then)
		if e != nil {
			return st, e
		}
		err = assign(h)
		return
	}
	if !p.polynomial.config.bitVector.IteToIdentity {
		return
	}
	yes, e := bvConditionIsEquality(m, condition)
	if e != nil || !yes {
		return st, e
	}
	yes, e = m.IsBitVector(then)
	if e != nil || !yes {
		return st, e
	}
	yes, e = m.IsBitVector(otherwise)
	if e != nil || !yes {
		return st, e
	}
	lhs, e := m.GetArg(condition, 0)
	if e != nil {
		return st, e
	}
	rhs, e := m.GetArg(condition, 1)
	if e != nil {
		return st, e
	}
	yes, e = m.IsBitVector(rhs)
	if e != nil || !yes {
		return st, e
	}
	yes, e = m.IsBitVectorOperation(lhs, 0)
	if e != nil {
		return st, e
	}
	if yes {
		lhs, rhs = rhs, lhs
	}
	matched, e := m.AreEqual(lhs, then)
	if e != nil {
		return st, e
	}
	if matched {
		matched, e = m.AreEqual(rhs, otherwise)
		if e != nil {
			return st, e
		}
	}
	if !matched {
		matched, e = m.AreEqual(lhs, otherwise)
		if e != nil {
			return st, e
		}
		if matched {
			matched, e = m.AreEqual(rhs, then)
			if e != nil {
				return st, e
			}
		}
	}
	if matched {
		err = assign(otherwise)
		return
	}
	width, e := m.BitVectorSize(rhs)
	if e != nil {
		return st, e
	}
	if width != 1 {
		return
	}
	n, e := ast.NewBitVectorIteNumerals(m)
	p.ite = n
	if e != nil {
		return st, e
	}
	rhsWidth, elseWidth, thenWidth := uint32(0), uint32(0), uint32(0)
	yes, e = n.Read(0, rhs, &rhsWidth)
	if e != nil {
		return st, e
	}
	if yes {
		yes, e = n.Read(2, then, &thenWidth)
		if e != nil {
			return st, e
		}
	}
	if yes {
		yes, e = n.Read(1, otherwise, &elseWidth)
		if e != nil {
			return st, e
		}
	}
	if yes && thenWidth == 1 {
		if ast.ASTSourceDebug {
			if rhsWidth != width || elseWidth != width || thenWidth != width {
				return st, ast.ErrASTDomain
			}
			same, e := m.AreEqual(then, otherwise)
			if e != nil {
				return st, e
			}
			if same {
				return st, ast.ErrASTDomain
			}
		}
		same, e := m.AreEqual(rhs, then)
		if e != nil {
			return st, e
		}
		value := lhs
		if !same {
			value, e = m.MkBitVectorNot(lhs)
			if e != nil {
				return st, e
			}
		}
		if err = assign(value); err != nil {
			return
		}
	}
	if err = n.SourceEnd(); err != nil {
		return
	}
	if err = n.Close(); err != nil {
		return
	}
	p.ite = nil
	return
}
