package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:2504-2701.

import "github.com/glycerine/g3/pkg/ast"

func (c *Converter) convertedArithmeticNumeral(declaration, value ast.ASTHandle, modeKind int32) (ast.ASTHandle, error) {
	mode, err := c.manager.MkFamilyApp(c.family, modeKind, nil, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numeral, found, err := c.manager.RewriteFPAToFPArithmeticNumeral(declaration, mode, value)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	encoded, found, err := c.MkNumeral(numeral)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return encoded, nil
}

func (c *Converter) convertedExponentRationalNumerals(declaration, first, second ast.ASTHandle, modeKind int32) (ast.ASTHandle, error) {
	mode, err := c.manager.MkFamilyApp(c.family, modeKind, nil, nil)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	numeral, found, err := c.manager.RewriteFPAToFPExponentRationalNumerals(declaration, mode, first, second)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	encoded, found, err := c.MkNumeral(numeral)
	if err != nil || !found {
		if err == nil {
			err = ast.ErrASTDomain
		}
		return ast.ASTHandle{}, err
	}
	return encoded, nil
}

func (c *Converter) selectRealRounding(rm ast.ASTHandle, values map[int32]ast.ASTHandle, defaultKind int32, includeNegative bool) (ast.ASTHandle, error) {
	bits, err := c.splitRoundingMode(rm)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	b := roundBuilder{c: c}
	result := values[defaultKind]
	if includeNegative {
		result = b.fpITE(b.mode(bits, 3), values[ast.FPAOpRMTowardNegative], result)
	}
	result = b.fpITE(b.mode(bits, 2), values[ast.FPAOpRMTowardPositive], result)
	result = b.fpITE(b.mode(bits, 0), values[ast.FPAOpRMNearestEven], result)
	result = b.fpITE(b.mode(bits, 1), values[ast.FPAOpRMNearestAway], result)
	if b.err != nil {
		return ast.ASTHandle{}, b.err
	}
	return result, nil
}

// ToFPReal converts an arithmetic real or integer expression to floating
// point. Symbolic values introduce the source equality side constraint.
func (c *Converter) ToFPReal(declaration, targetSort, roundingMode, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, floating, err := c.manager.FloatingPointSortShape(targetSort)
	if err != nil || !floating || ebits < 2 || sbits < 2 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	_, _, numeral, err := c.manager.ArithmeticNumeralText(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if numeral {
		values := make(map[int32]ast.ASTHandle, 5)
		for _, kind := range []int32{
			ast.FPAOpRMNearestAway, ast.FPAOpRMNearestEven,
			ast.FPAOpRMTowardPositive, ast.FPAOpRMTowardNegative,
		} {
			values[kind], err = c.convertedArithmeticNumeral(declaration, value, kind)
			if err != nil {
				return ast.ASTHandle{}, err
			}
		}
		// The source initializes the final fallback from v_tp, so RTZ follows
		// the toward-positive value in this retained implementation.
		return c.selectRealRounding(roundingMode, values, ast.FPAOpRMTowardPositive, true)
	}

	rm, err := c.splitRoundingMode(roundingMode)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	signSort, err := c.manager.BitVectorSort(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sigSort, err := c.manager.BitVectorSort(sbits + 4)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expSort, err := c.manager.BitVectorSort(ebits + 2)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sign, err := c.manager.MkFreshConst("fpa2bv_to_fp_real_sgn", signSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := c.manager.MkFreshConst("fpa2bv_to_fp_real_sig", sigSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := c.manager.MkFreshConst("fpa2bv_to_fp_real_exp", expSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	result, err := c.roundBits(targetSort, rm, sign, significand, exponent)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	toReal, err := c.manager.MkFamilyApp(c.family, ast.FPAOpToReal, nil, []ast.ASTHandle{result})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	assertion, err := c.manager.MkEq(toReal, value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.addAssertion(assertion); err != nil {
		return ast.ASTHandle{}, err
	}
	return result, nil
}

// ToFPRealInt converts the retained rounding-mode/real/integer overload.
func (c *Converter) ToFPRealInt(declaration ast.ASTHandle, roundingMode, first, second ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	values := make(map[int32]ast.ASTHandle, 5)
	var err error
	for _, kind := range []int32{
		ast.FPAOpRMNearestAway, ast.FPAOpRMNearestEven,
		ast.FPAOpRMTowardPositive, ast.FPAOpRMTowardZero,
	} {
		values[kind], err = c.convertedExponentRationalNumerals(declaration, first, second, kind)
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	// The source tests toward-positive twice, leaving toward-negative on the
	// toward-zero fallback.
	return c.selectRealRounding(roundingMode, values, ast.FPAOpRMTowardZero, false)
}
