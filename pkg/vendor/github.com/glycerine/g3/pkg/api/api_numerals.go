package api

// Source: src/api/api_numeral.cpp:50-194,265-403. Arithmetic, bit-vector,
// finite-domain and floating-point numeral construction is implemented here.

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
)

func (c *SMTContext) Numeral(value string, sort ast.ASTHandle) (result ast.ASTHandle, err error) {
	m, err := c.Manager()
	if err != nil {
		return result, err
	}
	if yes, e := m.IsBitVectorSort(sort); e == nil && yes {
		bits, e := m.BitVectorSize(sort)
		if e != nil {
			return result, e
		}
		q := m.NumeralService()
		n, e := q.NewValue()
		if e != nil {
			return result, e
		}
		defer func() { err = errors.Join(err, q.Del(&n)) }()
		if e = q.SetString(&n, value); e != nil {
			return result, e
		}
		return m.BitVectorNumeral(&n, bits)
	} else if e != nil {
		return result, e
	}
	if _, e := m.RegisterArithmeticPlugin(); e != nil {
		return result, e
	}
	arith, e := m.ArithmeticFamilyID()
	if e != nil {
		return result, e
	}
	family, e := m.GetFamilyID(sort)
	if e != nil {
		return result, e
	}
	if family == arith {
		name, e := m.GetASTName(sort)
		if e != nil {
			return result, e
		}
		text, e := m.Symbols.String(name)
		if e != nil {
			return result, e
		}
		if text == "Int" {
			return m.ArithmeticIntegerNumeralText(value)
		}
		if text == "Real" {
			return m.ArithmeticRealNumeralText(value)
		}
		return result, ast.ErrASTDomain
	}
	if finite, e := m.IsDatalogFiniteSort(sort); e == nil && finite {
		r, ok := new(big.Rat).SetString(value)
		if !ok || !r.IsInt() || r.Sign() < 0 || !r.Num().IsUint64() {
			return result, ast.ErrASTDomain
		}
		return m.MkDatalogNumeral(r.Num().Uint64(), sort)
	} else if e != nil {
		return result, e
	}
	floatingPoint, floatingPointFamily, e := c.floatingPointManager()
	if e != nil {
		return result, e
	}
	if family == floatingPointFamily {
		for i := 0; i < len(value); i++ {
			ch := value[i]
			if !('0' <= ch && ch <= '9') && ch != '/' && ch != '-' && ch != '+' && ch != ' ' && ch != '\n' && ch != '.' && ch != 'e' && ch != 'E' && ch != 'p' && ch != 'P' {
				return result, ast.ErrASTDomain
			}
		}
		return floatingPoint.MkFPANumeralString(value, sort)
	}
	return result, ast.ErrASTDomain
}
func (c *SMTContext) IntNumeral(value int64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.Numeral(strconv.FormatInt(value, 10), sort)
}
func (c *SMTContext) UintNumeral(value uint64, sort ast.ASTHandle) (ast.ASTHandle, error) {
	return c.Numeral(strconv.FormatUint(value, 10), sort)
}
func (c *SMTContext) NumeralString(value ast.ASTHandle) (string, error) {
	m, e := c.Manager()
	if e != nil {
		return "", e
	}
	if text, _, yes, e := m.ArithmeticNumeralText(value); e != nil || yes {
		return text, e
	}
	if text, yes, e := m.BitVectorNumeralText(value); e != nil || yes {
		return text, e
	}
	if text, yes, e := m.DatalogNumeralText(value); e != nil || yes {
		return text, e
	}
	if text, yes, e := m.FPANumeralString(value); e != nil || yes {
		return text, e
	}
	return "", errAPINumeralArgument
}
func (c *SMTContext) IsNumeral(value ast.ASTHandle) (bool, error) {
	_, e := c.NumeralString(value)
	if errors.Is(e, errAPINumeralArgument) {
		return false, nil
	}
	return e == nil, e
}
func (c *SMTContext) NumeralRational(value ast.ASTHandle) (*big.Rat, bool, error) {
	m, e := c.Manager()
	if e != nil {
		return nil, false, e
	}
	var text string
	var yes bool
	if text, _, yes, e = m.ArithmeticNumeralText(value); e != nil {
		return nil, false, e
	}
	if !yes {
		if text, yes, e = m.BitVectorNumeralText(value); e != nil {
			return nil, false, e
		}
	}
	if !yes {
		if text, yes, e = m.DatalogNumeralText(value); e != nil {
			return nil, false, e
		}
	}
	if !yes {
		return nil, false, nil
	}
	r, ok := new(big.Rat).SetString(text)
	if !ok {
		return nil, false, ast.ErrASTDomain
	}
	return r, true, nil
}
func (c *SMTContext) NumeralInt64(value ast.ASTHandle) (int64, bool, error) {
	r, yes, e := c.NumeralRational(value)
	if e != nil || !yes || !r.IsInt() || !r.Num().IsInt64() {
		return 0, false, e
	}
	return r.Num().Int64(), true, nil
}
func (c *SMTContext) NumeralUint64(value ast.ASTHandle) (uint64, bool, error) {
	r, yes, e := c.NumeralRational(value)
	if e != nil || !yes || !r.IsInt() || !r.Num().IsUint64() {
		return 0, false, e
	}
	return r.Num().Uint64(), true, nil
}
func (c *SMTContext) NumeralRationalInt64(value ast.ASTHandle) (num, den int64, matched bool, err error) {
	r, yes, e := c.NumeralRational(value)
	if e != nil || !yes {
		return 0, 0, false, e
	}
	if !r.Num().IsInt64() || !r.Denom().IsInt64() {
		return 0, 0, false, nil
	}
	return r.Num().Int64(), r.Denom().Int64(), true, nil
}
func (c *SMTContext) BitVectorNumeral(bits []bool) (ast.ASTHandle, error) {
	if len(bits) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	n := new(big.Int)
	for i, bit := range bits {
		if bit {
			n.SetBit(n, i, 1)
		}
	}
	m, e := c.bitVectorManager()
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.BitVectorNumeralRat(new(big.Rat).SetInt(n), uint32(len(bits)))
}
