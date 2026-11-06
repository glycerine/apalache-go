package api

// Source: src/api/api_bv.cpp:172-382.

import "github.com/glycerine/g3/pkg/ast"

func (c *SMTContext) bitVectorZero(sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.BitVectorValue(0, sort)
}
func (c *SMTContext) BitVectorMSB(sort ast.ASTHandle) (ast.ASTHandle, error) {
	bits, err := c.BitVectorSortSize(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if bits == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	one, err := c.BitVectorValue(1, sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	shift, err := c.BitVectorValue(int64(bits-1), sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.BitVectorSHL(one, shift)
}
func (c *SMTContext) BitVectorSignedMin(sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.BitVectorMSB(sort)
}
func (c *SMTContext) BitVectorSignedMax(sort ast.ASTHandle) (ast.ASTHandle, error) {
	v, e := c.BitVectorMSB(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return c.BitVectorNot(v)
}
func (c *SMTContext) BitVectorUnsignedMax(sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	bits, e := m.BitVectorSize(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.BitVectorNumeralAllOnes(bits)
}

func (c *SMTContext) BitVectorAddNoOverflow(a, b ast.ASTHandle, signed bool) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	if signed {
		sort, e := m.GetSort(a)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		zero, e := c.bitVectorZero(sort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		sum, e := c.BitVectorAdd(a, b)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		p1, e := c.BitVectorSLT(zero, a)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		p2, e := c.BitVectorSLT(zero, b)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		both, e := m.MkAnd(p1, p2)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		positive, e := c.BitVectorSLT(zero, sum)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		return m.MkImplies(both, positive)
	}
	bits, e := m.BitVectorSize(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	x, e := c.BitVectorZeroExtend(1, a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	y, e := c.BitVectorZeroExtend(1, b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sum, e := c.BitVectorAdd(x, y)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	carry, e := c.BitVectorExtract(bits, bits, sum)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	oneSort, e := c.BitVectorSort(1)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	zero, e := c.BitVectorValue(0, oneSort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkEq(carry, zero)
}
func (c *SMTContext) BitVectorAddNoUnderflow(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sort, e := m.GetSort(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	zero, e := c.bitVectorZero(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sum, e := c.BitVectorAdd(a, b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	n1, e := c.BitVectorSLT(a, zero)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	n2, e := c.BitVectorSLT(b, zero)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	both, e := m.MkAnd(n1, n2)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	negative, e := c.BitVectorSLT(sum, zero)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkImplies(both, negative)
}
func (c *SMTContext) BitVectorSubNoOverflow(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	neg, e := c.BitVectorNeg(b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sort, e := m.GetSort(b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	min, e := c.BitVectorSignedMin(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	isMin, e := m.MkEq(b, min)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	zero, e := c.bitVectorZero(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	aNegative, e := c.BitVectorSLT(a, zero)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	normal, e := c.BitVectorAddNoOverflow(a, neg, true)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkIte(isMin, aNegative, normal)
}
func (c *SMTContext) BitVectorSubNoUnderflow(a, b ast.ASTHandle, signed bool) (ast.ASTHandle, error) {
	if !signed {
		return c.BitVectorULE(b, a)
	}
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sort, e := m.GetSort(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	zero, e := c.bitVectorZero(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	neg, e := c.BitVectorNeg(b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	positive, e := c.BitVectorSLT(zero, b)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	safe, e := c.BitVectorAddNoUnderflow(a, neg)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkImplies(positive, safe)
}
func (c *SMTContext) BitVectorMulNoOverflow(a, b ast.ASTHandle, signed bool) (ast.ASTHandle, error) {
	// api_bv.cpp uses MK_BINARY_BODY directly. The bv_util convenience
	// methods reverse their arguments and are not the C API construction path.
	if signed {
		return c.bitVectorApp(53, a, b)
	}
	return c.bitVectorApp(52, a, b)
}
func (c *SMTContext) BitVectorMulNoUnderflow(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(54, a, b)
}
func (c *SMTContext) BitVectorNegNoOverflow(a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sort, e := m.GetSort(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	min, e := c.BitVectorSignedMin(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	eq, e := m.MkEq(a, min)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkNot(eq)
}
func (c *SMTContext) BitVectorSDivNoOverflow(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	sort, e := m.GetSort(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	min, e := c.BitVectorSignedMin(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	minusOne, e := c.BitVectorValue(-1, sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	x, e := m.MkEq(a, min)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	y, e := m.MkEq(b, minusOne)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	both, e := m.MkAnd(x, y)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkNot(both)
}
