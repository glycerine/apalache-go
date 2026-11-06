package arith

// Source: src/tactic/arith/bv2int_rewriter.cpp:100-295,338-472.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

func (c *bv2IntCall) comparison(s, t ast.ASTHandle, eq bool) (ast.ASTHandle, bool) {
	s1, a := c.unsigned(s)
	if a {
		t1, b := c.unsigned(t)
		if b {
			s1, t1 = c.align(s1, t1, false)
			if eq {
				return c.basic(ast.ASTOpEq, s1, t1), true
			}
			return c.bi(22, s1, t1), true
		}
	}
	s1, s2, a := c.difference(s)
	if a {
		t1, t2, b := c.difference(t)
		if b {
			s1 = c.addBV(s1, t2, false)
			if eq {
				t1 = c.addBV(s2, t1, false)
			} else {
				t1 = c.addBV(t1, s2, false)
			}
			s1, t1 = c.align(s1, t1, false)
			if eq {
				return c.basic(ast.ASTOpEq, s1, t1), true
			}
			return c.bi(22, s1, t1), true
		}
	}
	s1, a = c.signed(s)
	if a {
		t1, b := c.signed(t)
		if b {
			s1, t1 = c.align(s1, t1, true)
			if eq {
				return c.basic(ast.ASTOpEq, s1, t1), true
			}
			return c.bi(23, s1, t1), true
		}
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) ite(cond, s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	s1, a := c.unsigned(s)
	if a {
		t1, b := c.unsigned(t)
		if b {
			s1, t1 = c.align(s1, t1, false)
			return c.toInt(c.basic(ast.ASTOpIte, cond, s1, t1)), true
		}
	}
	s1, a = c.signed(s)
	if a {
		t1, b := c.signed(t)
		if b {
			s1, t1 = c.align(s1, t1, true)
			return c.signedInt(c.basic(ast.ASTOpIte, cond, s1, t1)), true
		}
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) add(s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	s1, a := c.unsigned(s)
	if a {
		t1, b := c.unsigned(t)
		if b {
			return c.toInt(c.addBV(s1, t1, false)), true
		}
	}
	s1, s2, a := c.difference(s)
	if a {
		t1, t2, b := c.difference(t)
		if b {
			t1 = c.toInt(c.addBV(s1, t1, false))
			t2 = c.toInt(c.addBV(s2, t2, false))
			return c.ai(7, t1, t2), true
		}
	}
	s1, a = c.signed(s)
	if a {
		t1, b := c.signed(t)
		if b {
			return c.signedInt(c.addBV(s1, t1, true)), true
		}
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) sub(s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	s1, s2, a := c.difference(s)
	if a {
		t1, t2, b := c.difference(t)
		if b {
			s1 = c.toInt(c.addBV(s1, t2, false))
			s2 = c.toInt(c.addBV(s2, t1, false))
			return c.ai(7, s1, s2), true
		}
	}
	s1, a = c.signed(s)
	if a {
		t1, b := c.signed(t)
		if b {
			s1, t1 = c.align(s1, t1, true)
			s1, t1 = c.extend(1, s1, true), c.extend(1, t1, true)
			return c.signedInt(c.bi(5, s1, t1)), true
		}
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) mul(s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	s1, a := c.shl1(s)
	var t1 ast.ASTHandle
	var b bool
	if a {
		t1, b = c.unsigned(t)
	}
	if !a || !b {
		s1, a = c.shl1(t)
		if a {
			t1, b = c.unsigned(s)
		}
	}
	if a && b {
		n, m := c.width(s1), c.width(t1)
		s1 = c.extend(m, s1, false)
		t1 = c.extend(n, t1, false)
		return c.toInt(c.bi(45, t1, s1)), true
	}
	s1, a = c.unsigned(s)
	if a {
		t1, b = c.unsigned(t)
		if b {
			return c.toInt(c.mulBV(s1, t1, false)), true
		}
	}
	s1, a = c.unsigned(s)
	var t2 ast.ASTHandle
	if a {
		t1, t2, b = c.difference(t)
	}
	if !a || !b {
		s1, a = c.unsigned(t)
		if a {
			t1, t2, b = c.difference(s)
		}
	}
	if a && b {
		t1 = c.toInt(c.mulBV(s1, t1, false))
		t2 = c.toInt(c.mulBV(s1, t2, false))
		return c.ai(7, t1, t2), true
	}
	s1, a = c.signed(s)
	if a {
		t1, b = c.signed(t)
		if b {
			return c.signedInt(c.mulBV(s1, t1, true)), true
		}
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) mod(s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	s1, a := c.unsigned(s)
	if a {
		t1, b := c.unsigned(t)
		if b {
			s1, t1 = c.align(s1, t1, false)
			c.traceLine = 236
			return c.toInt(c.bi(10, s1, t1)), true
		}
	}
	s1, _, a = c.difference(s)
	if a {
		t1, b := c.unsigned(t)
		if b {
			s1, t1 = c.align(s1, t1, false)
			// The retained source uses s1 here, despite the explanatory comment's s2.
			u1 := c.bi(10, s1, t1)
			u1 = c.bi(5, t1, u1)
			u1 = c.addBV(s1, u1, false)
			u1, t1 = c.align(u1, t1, false)
			c.traceLine = 251
			return c.toInt(c.bi(10, u1, t1)), true
		}
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) neg(s ast.ASTHandle) (ast.ASTHandle, bool) {
	if s1, s2, ok := c.difference(s); ok {
		return c.ai(7, c.toInt(s2), c.toInt(s1)), true
	}
	if s1, ok := c.signed(s); ok {
		return c.signedInt(c.bi(3, s1)), true
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) reduce(f ast.FamilyID, k int32, a []ast.ASTHandle) (h ast.ASTHandle, status rewriter.RewriteStatus) {
	status = rewriter.RewriteFailed
	if f == c.r.arith {
		line := int32(0)
		arity := 2
		switch k {
		case 2:
			line = 104
		case 3:
			line = 105
		case 4:
			line = 106
		case 5:
			line = 107
		case 11:
			line = 112
		case 13:
			line = 113
		case 12:
			line = 114
		case 8:
			line = 115
			arity = 1
		}
		if line != 0 {
			condition := "num_args == 2"
			if arity == 1 {
				condition = "num_args == 1"
			}
			c.err = bv2IntSourceAssert(line, condition, len(a) == arity)
			if c.err != nil {
				return
			}
		}
		if k == 6 || k == 9 || k == 7 {
			line = 288
			if k == 9 {
				line = 363
			}
			if k == 7 {
				line = 448
			}
			c.err = bv2IntSourceAssert(line, "num_args > 0", len(a) > 0)
			if c.err != nil {
				return
			}
		}
	} else if f == c.r.manager.BasicFamilyID() && (k == ast.ASTOpEq || k == ast.ASTOpIte) {
		line := int32(124)
		arity := 2
		condition := "num_args == 2"
		if k == ast.ASTOpIte {
			line = 125
			arity = 3
			condition = "num_args == 3"
		}
		c.err = bv2IntSourceAssert(line, condition, len(a) == arity)
		if c.err != nil {
			return
		}
	}

	var ok bool
	if f == c.r.arith {
		switch k {
		case 2, 3, 4, 5:
			if len(a) != 2 {
				c.err = ast.ErrASTDomain
				return
			}
			switch k {
			case 2:
				h, ok = c.comparison(a[0], a[1], false)
			case 3:
				h, ok = c.comparison(a[1], a[0], false)
			case 4:
				return c.basic(ast.ASTOpNot, c.ai(2, a[1], a[0])), rewriter.Rewrite2
			case 5:
				return c.basic(ast.ASTOpNot, c.ai(2, a[0], a[1])), rewriter.Rewrite2
			}
		case 6, 7, 9:
			if len(a) == 0 {
				c.err = ast.ErrASTDomain
				return
			}
			h = a[0]
			ok = true
			for i := 1; i < len(a) && ok; i++ {
				var next ast.ASTHandle
				switch k {
				case 6:
					next, ok = c.add(h, a[i])
				case 7:
					next, ok = c.sub(h, a[i])
				case 9:
					next, ok = c.mul(h, a[i])
				}
				if ok {
					h = next
				}
			}
		case 8:
			if len(a) != 1 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.neg(a[0])
		case 13:
			if len(a) != 2 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.mod(a[0], a[1])
			// NUM, real/integer division, remainder and coercions remain BR_FAILED.
		}
	} else if f == c.r.manager.BasicFamilyID() {
		switch k {
		case ast.ASTOpEq:
			if len(a) != 2 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.comparison(a[0], a[1], true)
		case ast.ASTOpIte:
			if len(a) != 3 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.ite(a[0], a[1], a[2])
		case ast.ASTOpDistinct:
			if len(a) < 2 {
				return
			}
			sort, err := c.r.manager.GetSort(a[0])
			if err != nil {
				c.err = err
				return
			}
			fam, err := c.r.manager.GetFamilyID(sort)
			if err != nil {
				c.err = err
				return
			}
			kind, err := c.r.manager.GetDeclKind(sort)
			if err != nil {
				c.err = err
				return
			}
			if fam != c.r.arith || kind != 1 {
				return
			}
			eqs := make([]ast.ASTHandle, 0, len(a)*(len(a)-1)/2)
			for i := range a {
				for j := i + 1; j < len(a); j++ {
					h, ok = c.comparison(a[i], a[j], true)
					if !ok {
						return
					}
					eqs = append(eqs, h)
				}
			}
			or := eqs[0]
			if len(eqs) > 1 {
				or = c.basic(ast.ASTOpOr, eqs...)
			}
			return c.basic(ast.ASTOpNot, or), rewriter.RewriteDone
		}
	}
	if ok {
		status = rewriter.RewriteDone
	}
	return
}
