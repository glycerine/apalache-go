package fpa

// Source: src/ast/fpa/fpa2bv_converter.cpp:214-228,2273-2300,3072-3122,3329-3349.

import "github.com/glycerine/g3/pkg/ast"

// MkNumeral converts an AST floating-point numeral into the converter's
// three-field representation.
func (c *Converter) MkNumeral(value ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	bits, found, _, err := c.manager.RewriteFPAToIEEEBVNumeral(value, true)
	if err != nil || !found {
		return ast.ASTHandle{}, found, err
	}
	sort, err := c.manager.GetSort(value)
	if err != nil {
		return ast.ASTHandle{}, true, err
	}
	result, err := c.FromIEEEBV(sort, bits)
	return result, true, err
}

// MkVar expands one floating-point bound variable into the three consecutive
// bit-vector variables used by the source quantifier conversion.
func (c *Converter) MkVar(base uint32, sort ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, matched, err := c.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	signSort, err := c.manager.BitVectorSort(1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sigSort, err := c.manager.BitVectorSort(sbits - 1)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	expSort, err := c.manager.BitVectorSort(ebits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sign, err := c.manager.MkVar(base, signSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := c.manager.MkVar(base+1, sigSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := c.manager.MkVar(base+2, expSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(sign, exponent, significand)
}

// FromIEEEBV splits a packed IEEE bit-vector according to the requested
// floating-point sort.
func (c *Converter) FromIEEEBV(sort, bits ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	ebits, sbits, matched, err := c.manager.FloatingPointSortShape(sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if !matched {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	width, err := c.manager.BitVectorSize(bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if width != ebits+sbits {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	sign, err := c.manager.MkBitVectorExtract(width-1, width-1, bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	exponent, err := c.manager.MkBitVectorExtract(width-2, sbits-1, bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	significand, err := c.manager.MkBitVectorExtract(sbits-2, 0, bits)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.MkFP(sign, exponent, significand)
}

// ToIEEEBV packs an encoded value. With the source default high-unspecified
// setting, every NaN maps to the canonical NaN bit pattern.
func (c *Converter) toIEEEBV(declaration, value ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	joined, err := c.JoinFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nan, err := c.isNaN(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	var unspecified ast.ASTHandle
	if c.highUnspecified {
		canonical, e := c.MkSpecial(sort, NaN)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		unspecified, err = c.JoinFP(canonical)
	} else {
		if declaration.IsNull() {
			return ast.ASTHandle{}, ast.ErrASTDomain
		}
		nanWrapped, e := c.nanWrap(value)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		domainSort, e := c.manager.GetSort(nanWrapped)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		rangeSort, e := c.manager.GetRange(declaration)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		converted, e := c.convertedUFWithDomain(declaration, []ast.ASTHandle{domainSort}, rangeSort)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		unspecified, err = c.manager.MkApp(converted, []ast.ASTHandle{nanWrapped})
		if err == nil {
			ebits, sbits, _, e := c.manager.FloatingPointSortShape(sort)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			exponent := ast.ASTHandle{}
			exponent, err = c.manager.MkBitVectorExtract(ebits+sbits-2, sbits-1, unspecified)
			if err == nil {
				var allOnes ast.ASTHandle
				allOnes, err = c.manager.BitVectorNumeralAllOnes(ebits)
				if err == nil {
					var assertion ast.ASTHandle
					assertion, err = c.boolEq(exponent, allOnes)
					if err == nil {
						err = c.addAssertion(assertion)
					}
				}
			}
			if err == nil {
				var significand, zero, equal, nonzero ast.ASTHandle
				significand, err = c.manager.MkBitVectorExtract(sbits-2, 0, unspecified)
				if err == nil {
					zero, err = c.zero(sbits - 1)
				}
				if err == nil {
					equal, err = c.boolEq(significand, zero)
				}
				if err == nil {
					nonzero, err = c.boolNot(equal)
				}
				if err == nil {
					err = c.addAssertion(nonzero)
				}
			}
		}
	}
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(nan, unspecified, joined)
}

func (c *Converter) nanWrap(value ast.ASTHandle) (ast.ASTHandle, error) {
	joined, err := c.JoinFP(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	nan, err := c.isNaN(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	sort, err := c.manager.GetSort(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	canonical, err := c.MkSpecial(sort, NaN)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	canonicalBits, err := c.JoinFP(canonical)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return c.boolITE(nan, canonicalBits, joined)
}

func (c *Converter) ToIEEEBV(value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toIEEEBV(ast.ASTHandle{}, value)
}

func (c *Converter) ToIEEEBVFor(declaration, value ast.ASTHandle) (ast.ASTHandle, error) {
	return c.toIEEEBV(declaration, value)
}
