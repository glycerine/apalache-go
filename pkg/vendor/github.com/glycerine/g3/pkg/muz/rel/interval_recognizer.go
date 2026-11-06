package rel

// Source: src/muz/rel/dl_interval_relation.cpp is_linear/is_le/is_lt/is_eq.

import (
	"github.com/glycerine/g3/pkg/ast"
	"math"
	"math/big"
)

type intervalConstraint struct {
	positive, negative uint32
	constant           big.Rat
	integer            bool
}

func newIntervalConstraint() intervalConstraint {
	return intervalConstraint{positive: math.MaxUint32, negative: math.MaxUint32}
}
func (c *intervalConstraint) hasVariable() bool {
	return c.positive != math.MaxUint32 || c.negative != math.MaxUint32
}

func intervalLinear(m *ast.ASTManager, family ast.FamilyID, h ast.ASTHandle, c *intervalConstraint, positive bool) (bool, error) {
	k, e := m.GetKind(h)
	if e != nil {
		return false, e
	}
	if k == ast.ASTVar {
		index, e := m.GetVarIndex(h)
		if e != nil {
			return false, e
		}
		if positive && c.positive == math.MaxUint32 {
			c.positive = index
			return true, nil
		}
		if !positive && c.negative == math.MaxUint32 {
			c.negative = index
			return true, nil
		}
		return false, nil
	}
	if k != ast.ASTApp {
		return false, nil
	}
	fid, e := m.GetFamilyID(h)
	if e != nil {
		return false, e
	}
	if fid != family {
		return false, nil
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil {
		return false, e
	}
	if op == 6 {
		for i := uint32(0); i < n; i++ {
			a, e := m.GetArg(h, i)
			if e != nil {
				return false, e
			}
			ok, e := intervalLinear(m, family, a, c, positive)
			if e != nil || !ok {
				return ok, e
			}
		}
		return true, nil
	}
	if op == 7 && n == 2 {
		a, e := m.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		b, e := m.GetArg(h, 1)
		if e != nil {
			return false, e
		}
		ok, e := intervalLinear(m, family, a, c, positive)
		if e != nil || !ok {
			return ok, e
		}
		return intervalLinear(m, family, b, c, !positive)
	}
	if op == 9 && n == 2 {
		a, e := m.GetArg(h, 0)
		if e != nil {
			return false, e
		}
		b, e := m.GetArg(h, 1)
		if e != nil {
			return false, e
		}
		text, _, numeral, e := m.ArithmeticNumeralText(a)
		if e != nil {
			return false, e
		}
		if numeral {
			value, ok := new(big.Rat).SetString(text)
			if !ok {
				return false, ast.ErrASTDomain
			}
			kind, e := m.GetKind(b)
			if e != nil {
				return false, e
			}
			// SET_VAR uses the incoming polarity in the source's -1*x branch.
			if value.Cmp(big.NewRat(-1, 1)) == 0 && kind == ast.ASTVar {
				return intervalLinear(m, family, b, c, positive)
			}
		}
	}
	text, _, numeral, e := m.ArithmeticNumeralText(h)
	if e != nil || !numeral {
		return false, e
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		return false, ast.ErrASTDomain
	}
	if positive {
		c.constant.Add(&c.constant, value)
	} else {
		c.constant.Sub(&c.constant, value)
	}
	return true, nil
}

func intervalSortKind(m *ast.ASTManager, family ast.FamilyID, h ast.ASTHandle) (int32, error) {
	sort, e := m.GetSort(h)
	if e != nil {
		return -1, e
	}
	fid, e := m.GetFamilyID(sort)
	if e != nil {
		return -1, e
	}
	if fid != family {
		return -1, nil
	}
	return m.GetDeclKind(sort)
}

// intervalComparison recognizes the non-strict, strict-real, or equality
// source filter shape. It returns 0, 1, or 2 for those cases respectively.
func intervalComparison(m *ast.ASTManager, h ast.ASTHandle) (int, intervalConstraint, bool, error) {
	family, e := m.RegisterArithmeticPlugin()
	if e != nil {
		return 0, intervalConstraint{}, false, e
	}
	c, ok, e := intervalLE(m, family, h)
	if e != nil || ok {
		return 0, c, ok, e
	}
	c, ok, e = intervalLT(m, family, h)
	if e != nil || ok {
		return 1, c, ok, e
	}
	c, ok, e = intervalEQ(m, family, h)
	return 2, c, ok, e
}

func intervalParts(m *ast.ASTManager, h ast.ASTHandle) (ast.FamilyID, int32, ast.ASTHandle, ast.ASTHandle, bool, error) {
	k, e := m.GetKind(h)
	if e != nil || k != ast.ASTApp {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	n, e := m.GetNumArgs(h)
	if e != nil || n != 2 {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	fid, e := m.GetFamilyID(h)
	if e != nil {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	op, e := m.GetDeclKind(h)
	if e != nil {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	a, e := m.GetArg(h, 0)
	if e != nil {
		return 0, 0, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	b, e := m.GetArg(h, 1)
	return fid, op, a, b, e == nil, e
}
func intervalPair(m *ast.ASTManager, family ast.FamilyID, a, b ast.ASTHandle, leftPositive bool, c *intervalConstraint) (bool, error) {
	ok, e := intervalLinear(m, family, a, c, leftPositive)
	if e != nil || !ok {
		return ok, e
	}
	ok, e = intervalLinear(m, family, b, c, !leftPositive)
	if e != nil || !ok {
		return ok, e
	}
	return c.hasVariable(), nil
}
func intervalLE(m *ast.ASTManager, family ast.FamilyID, h ast.ASTHandle) (intervalConstraint, bool, error) {
	c := newIntervalConstraint()
	fid, op, a, b, app, e := intervalParts(m, h)
	if e != nil {
		return c, false, e
	}
	if app && fid == family && op >= 2 && op <= 5 {
		kind, e := intervalSortKind(m, family, a)
		if e != nil {
			return c, false, e
		}
		c.integer = kind == 1
		if op >= 4 && !c.integer {
			return c, false, nil
		}
		leftPositive := op == 3 || op == 5
		ok, e := intervalPair(m, family, a, b, leftPositive, &c)
		if e != nil || !ok {
			return c, ok, e
		}
		if op == 4 {
			c.constant.Sub(&c.constant, big.NewRat(1, 1))
		}
		if op == 5 {
			c.constant.Add(&c.constant, big.NewRat(1, 1))
		}
		return c, true, nil
	}
	not, arg, e := m.IsNot(h)
	if e != nil || !not {
		return c, false, e
	}
	c, ok, e := intervalLE(m, family, arg)
	if e != nil {
		return c, false, e
	}
	if ok && c.integer {
		c.constant.Neg(&c.constant)
		c.constant.Sub(&c.constant, big.NewRat(1, 1))
		c.positive, c.negative = c.negative, c.positive
		return c, true, nil
	}
	c, ok, e = intervalLT(m, family, arg)
	if e != nil || !ok {
		return c, ok, e
	}
	c.integer = false
	c.constant.Neg(&c.constant)
	c.positive, c.negative = c.negative, c.positive
	return c, true, nil
}
func intervalLT(m *ast.ASTManager, family ast.FamilyID, h ast.ASTHandle) (intervalConstraint, bool, error) {
	c := newIntervalConstraint()
	fid, op, a, b, app, e := intervalParts(m, h)
	if e != nil || !app || fid != family || (op != 4 && op != 5) {
		return c, false, e
	}
	kind, e := intervalSortKind(m, family, a)
	if e != nil || kind != 0 {
		return c, false, e
	}
	ok, e := intervalPair(m, family, a, b, op == 5, &c)
	return c, ok, e
}
func intervalEQ(m *ast.ASTManager, family ast.FamilyID, h ast.ASTHandle) (intervalConstraint, bool, error) {
	c := newIntervalConstraint()
	fid, op, a, b, app, e := intervalParts(m, h)
	if e != nil || !app || fid != m.BasicFamilyID() || op != ast.ASTOpEq {
		return c, false, e
	}
	ok, e := intervalPair(m, family, a, b, false, &c)
	return c, ok, e
}
