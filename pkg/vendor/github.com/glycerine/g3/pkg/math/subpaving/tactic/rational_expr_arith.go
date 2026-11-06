package tactic

// Source: src/math/subpaving/tactic/expr2subpaving.cpp:150-286,
// concrete config_mpq arithmetic term construction.

import (
	"errors"
	"math"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/subpaving"
	"github.com/glycerine/g3/pkg/util/numeral"
)

func (c *RationalExprConverter) unsignedExponent(h ast.ASTHandle) (uint32, bool, error) {
	value, _, matched, err := c.ast.ArithmeticNumeralText(h)
	if err != nil || !matched {
		return 0, false, err
	}
	r, ok := new(big.Rat).SetString(value)
	if !ok || !r.IsInt() || r.Sign() < 0 || r.Num().BitLen() > 32 {
		return 0, false, nil
	}
	return uint32(r.Num().Uint64()), true, nil
}

func (c *RationalExprConverter) asPower(h ast.ASTHandle) (ast.ASTHandle, uint32, error) {
	kind, err := c.ast.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return h, 1, err
	}
	family, err := c.ast.GetFamilyID(h)
	if err != nil || family != c.family {
		return h, 1, err
	}
	op, err := c.ast.GetDeclKind(h)
	if err != nil || op != 18 {
		return h, 1, err
	}
	exponent, err := c.ast.GetArg(h, 1)
	if err != nil {
		return h, 1, err
	}
	k, yes, err := c.unsignedExponent(exponent)
	if err != nil || !yes {
		return h, 1, err
	}
	base, err := c.ast.GetArg(h, 0)
	return base, k, err
}

func (c *RationalExprConverter) processMul(t ast.ASTHandle, n, d *numeral.Int) (variable uint32, err error) {
	z := c.qm.Z
	count, err := c.ast.GetNumArgs(t)
	if err != nil {
		return 0, err
	}
	if count <= 1 {
		return 0, ErrExprConverterNonSimplified
	}
	first, err := c.ast.GetArg(t, 0)
	if err != nil {
		return 0, err
	}
	coefficientText, _, numeric, err := c.ast.ArithmeticNumeralText(first)
	if err != nil {
		return 0, err
	}
	m := t
	hasCoefficient := false
	_, parsed := new(big.Rat).SetString(coefficientText)
	if numeric && parsed {
		if count != 2 {
			return 0, ErrExprConverterNonSimplified
		}
		if err = copyExprCoefficient(c.qm, coefficientText, n, d); err != nil {
			return 0, err
		}
		m, err = c.ast.GetArg(t, 1)
		if err != nil {
			return 0, err
		}
		hasCoefficient = true
	}
	if !hasCoefficient {
		if err = z.SetInt32(n, 1); err != nil {
			return 0, err
		}
		if err = z.SetInt32(d, 1); err != nil {
			return 0, err
		}
	}
	args := []ast.ASTHandle{m}
	kind, err := c.ast.GetKind(m)
	if err != nil {
		return 0, err
	}
	if kind == ast.ASTApp {
		family, e := c.ast.GetFamilyID(m)
		if e != nil {
			return 0, e
		}
		op, e := c.ast.GetDeclKind(m)
		if e != nil {
			return 0, e
		}
		if family == c.family && op == 9 {
			length, e := c.ast.GetNumArgs(m)
			if e != nil {
				return 0, e
			}
			args = make([]ast.ASTHandle, length)
			for i := range args {
				args[i], e = c.ast.GetArg(m, uint32(i))
				if e != nil {
					return 0, e
				}
			}
		}
	}
	var argN, argD numeral.Int
	defer func() { err = errors.Join(err, z.Del(&argD), z.Del(&argN)) }()
	powers := make([]subpaving.RationalPower, 0, len(args))
	for _, arg := range args {
		base, degree, e := c.asPower(arg)
		if e != nil {
			return 0, e
		}
		x, e := c.InternalizeTerm(base, &argN, &argD)
		if e != nil {
			return 0, e
		}
		if e = z.Power(&argN, degree, &argN); e != nil {
			return 0, e
		}
		if e = z.Power(&argD, degree, &argD); e != nil {
			return 0, e
		}
		if e = z.Mul(n, &argN, n); e != nil {
			return 0, e
		}
		if e = z.Mul(d, &argD, d); e != nil {
			return 0, e
		}
		if x != math.MaxUint32 {
			powers = append(powers, subpaving.RationalPower{Variable: x, Degree: degree})
		}
	}
	if len(powers) == 0 {
		return math.MaxUint32, c.cacheResult(t, math.MaxUint32, n, d)
	}
	if len(powers) == 1 && powers[0].Degree == 1 {
		return powers[0].Variable, c.cacheResult(t, powers[0].Variable, n, d)
	}
	context, err := c.engine.Context()
	if err != nil {
		return 0, err
	}
	variable, err = context.DefineMonomial(powers)
	if err != nil {
		return 0, err
	}
	return variable, c.cacheResult(t, variable, n, d)
}

func (c *RationalExprConverter) processPower(t ast.ASTHandle, n, d *numeral.Int, integer bool) (variable uint32, err error) {
	z := c.qm.Z
	exponent, err := c.ast.GetArg(t, 1)
	if err != nil {
		return 0, err
	}
	k, yes, err := c.unsignedExponent(exponent)
	if err != nil {
		return 0, err
	}
	if !yes {
		return c.mkVarFor(t, integer, n, d)
	}
	base, err := c.ast.GetArg(t, 0)
	if err != nil {
		return 0, err
	}
	x, err := c.InternalizeTerm(base, n, d)
	if err != nil {
		return 0, err
	}
	if x != math.MaxUint32 {
		context, err := c.engine.Context()
		if err != nil {
			return 0, err
		}
		x, err = context.DefineMonomial([]subpaving.RationalPower{{Variable: x, Degree: k}})
		if err != nil {
			return 0, err
		}
	}
	if err = z.Power(n, k, n); err != nil {
		return 0, err
	}
	if err = z.Power(d, k, d); err != nil {
		return 0, err
	}
	return x, c.cacheResult(t, x, n, d)
}

func (c *RationalExprConverter) processAdd(t ast.ASTHandle, n, d *numeral.Int) (variable uint32, err error) {
	qm, z := c.qm, c.qm.Z
	constant, argConstant := numeral.NewRational(), numeral.NewRational()
	var argN, argD, factor, sumConstant numeral.Int
	var numerators, denominators []numeral.Int
	defer func() {
		for i := range denominators {
			err = errors.Join(err, z.Del(&denominators[i]))
		}
		for i := range numerators {
			err = errors.Join(err, z.Del(&numerators[i]))
		}
		err = errors.Join(err, z.Del(&sumConstant), z.Del(&factor), z.Del(&argD), z.Del(&argN), qm.Del(&argConstant), qm.Del(&constant))
	}()
	count, err := c.ast.GetNumArgs(t)
	if err != nil {
		return 0, err
	}
	vars := make([]uint32, 0, count)
	for i := uint32(0); i < count; i++ {
		arg, e := c.ast.GetArg(t, i)
		if e != nil {
			return 0, e
		}
		x, e := c.InternalizeTerm(arg, &argN, &argD)
		if e != nil {
			return 0, e
		}
		if x == math.MaxUint32 {
			if e = qm.SetParts(&argConstant, &argN, &argD); e != nil {
				return 0, e
			}
			if e = qm.Add(&constant, &argConstant, &constant); e != nil {
				return 0, e
			}
		} else {
			vars = append(vars, x)
			numerators = append(numerators, numeral.Int{})
			denominators = append(denominators, numeral.Int{})
			last := len(vars) - 1
			if e = z.Copy(&numerators[last], &argN); e != nil {
				return 0, e
			}
			if e = z.Copy(&denominators[last], &argD); e != nil {
				return 0, e
			}
		}
	}
	if err = qm.Denominator(&constant, d); err != nil {
		return 0, err
	}
	for i := range vars {
		if err = z.LCM(d, &denominators[i], d); err != nil {
			return 0, err
		}
	}
	if err = z.Div(d, &constant.Den, &factor); err != nil {
		return 0, err
	}
	if err = z.Mul(&constant.Num, &factor, &sumConstant); err != nil {
		return 0, err
	}
	for i := range vars {
		if err = z.Div(d, &denominators[i], &factor); err != nil {
			return 0, err
		}
		if err = z.Mul(&numerators[i], &factor, &numerators[i]); err != nil {
			return 0, err
		}
	}
	if len(vars) == 0 {
		if err = z.Copy(n, &sumConstant); err != nil {
			return 0, err
		}
		return math.MaxUint32, c.cacheResult(t, math.MaxUint32, n, d)
	}
	coefficientPointers := make([]*numeral.Int, len(numerators))
	for i := range vars {
		coefficientPointers[i] = &numerators[i]
	}
	shell, err := c.engine.Shell()
	if err != nil {
		return 0, err
	}
	variable, err = shell.MkSum(&sumConstant, coefficientPointers, vars)
	if err != nil {
		return 0, err
	}
	if err = z.SetInt32(n, 1); err != nil {
		return 0, err
	}
	return variable, c.cacheResult(t, variable, n, d)
}
