package api

// Source: src/api/api_bv.cpp:24-170,383-400.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) bitVectorManager() (*ast.ASTManager, error) {
	if err := c.open(); err != nil {
		return nil, err
	}
	m, err := c.command.Manager()
	if err != nil {
		return nil, err
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		return nil, err
	}
	return m, nil
}
func (c *SMTContext) BitVectorSort(bits uint32) (ast.ASTHandle, error) {
	if bits == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.BitVectorSort(bits)
}
func (c *SMTContext) BitVectorSortSize(sort ast.ASTHandle) (uint32, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return 0, e
	}
	return m.BitVectorSize(sort)
}
func (c *SMTContext) BitVectorValue(value int64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	bits, e := m.BitVectorSize(sort)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.BitVectorNumeralRat(big.NewRat(value, 1), bits)
}

func (c *SMTContext) bitVectorApp(kind int32, args ...ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	family, e := m.RegisterBitVectorPlugin()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkFamilyApp(family, kind, nil, args)
}
func (c *SMTContext) bitVectorParameterized(kind int32, value uint32, arg ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.bitVectorManager()
	if err != nil {
		return result, err
	}
	family, err := m.RegisterBitVectorPlugin()
	if err != nil {
		return result, err
	}
	p, err := m.NewParameterUint(value)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, m.DelParameter(&p)) }()
	return m.MkFamilyApp(family, kind, []ast.ASTParameter{p}, []ast.ASTHandle{arg})
}

func (c *SMTContext) BitVectorNot(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(32, a)
}
func (c *SMTContext) BitVectorReductionAnd(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(43, a)
}
func (c *SMTContext) BitVectorReductionOr(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(42, a)
}
func (c *SMTContext) BitVectorAnd(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(30, a, b)
}
func (c *SMTContext) BitVectorOr(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(31, a, b)
}
func (c *SMTContext) BitVectorXor(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(33, a, b)
}
func (c *SMTContext) BitVectorNand(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(34, a, b)
}
func (c *SMTContext) BitVectorNor(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(35, a, b)
}
func (c *SMTContext) BitVectorXnor(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(36, a, b)
}
func (c *SMTContext) BitVectorAdd(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(4, a, b)
}
func (c *SMTContext) BitVectorSub(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(5, a, b)
}
func (c *SMTContext) BitVectorMul(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(6, a, b)
}
func (c *SMTContext) BitVectorNeg(a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(3, a)
}
func (c *SMTContext) BitVectorUDiv(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(8, a, b)
}
func (c *SMTContext) BitVectorSDiv(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(7, a, b)
}
func (c *SMTContext) BitVectorURem(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(10, a, b)
}
func (c *SMTContext) BitVectorSRem(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(9, a, b)
}
func (c *SMTContext) BitVectorSMod(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(11, a, b)
}
func (c *SMTContext) BitVectorULE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(22, a, b)
}
func (c *SMTContext) BitVectorSLE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(23, a, b)
}
func (c *SMTContext) BitVectorUGE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(24, a, b)
}
func (c *SMTContext) BitVectorSGE(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(25, a, b)
}
func (c *SMTContext) BitVectorULT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(26, a, b)
}
func (c *SMTContext) BitVectorSLT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(27, a, b)
}
func (c *SMTContext) BitVectorUGT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(28, a, b)
}
func (c *SMTContext) BitVectorSGT(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(29, a, b)
}
func (c *SMTContext) BitVectorConcat(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(37, a, b)
}
func (c *SMTContext) BitVectorSHL(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(45, a, b)
}
func (c *SMTContext) BitVectorLSHR(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(46, a, b)
}
func (c *SMTContext) BitVectorASHR(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(47, a, b)
}
func (c *SMTContext) BitVectorExtendedRotateLeft(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(50, a, b)
}
func (c *SMTContext) BitVectorExtendedRotateRight(a, b ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorApp(51, a, b)
}
func (c *SMTContext) BitVectorExtract(high, low uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkBitVectorExtract(high, low, a)
}
func (c *SMTContext) BitVectorSignExtend(amount uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkBitVectorSignExtend(amount, a)
}
func (c *SMTContext) BitVectorZeroExtend(amount uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkBitVectorZeroExtend(amount, a)
}
func (c *SMTContext) BitVectorRepeat(amount uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorParameterized(41, amount, a)
}
func (c *SMTContext) BitVectorRotateLeft(amount uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorParameterized(48, amount, a)
}
func (c *SMTContext) BitVectorRotateRight(amount uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	return c.bitVectorParameterized(49, amount, a)
}
func (c *SMTContext) IntToBitVector(bits uint32, a ast.ASTHandle) (ast.ASTHandle, error) {
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkIntToBitVector(bits, a)
}
func (c *SMTContext) BitVectorToInt(a ast.ASTHandle, signed ...bool) (ast.ASTHandle, error) {
	if len(signed) > 1 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	unsigned, e := m.MkBitVectorToInt(a)
	if e != nil || len(signed) == 0 || !signed[0] {
		return unsigned, e
	}
	bits, e := m.BitVectorSize(a)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	zero, e := m.BitVectorNumeralRat(big.NewRat(0, 1), bits)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	negative, e := c.BitVectorSLT(a, zero)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	bound := new(big.Int).Lsh(big.NewInt(1), uint(bits))
	limit, e := m.ArithmeticIntegerNumeralText(bound.String())
	if e != nil {
		return ast.ASTHandle{}, e
	}
	adjusted, e := m.MkArithmeticSub(unsigned, limit)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.MkIte(negative, adjusted, unsigned)
}
