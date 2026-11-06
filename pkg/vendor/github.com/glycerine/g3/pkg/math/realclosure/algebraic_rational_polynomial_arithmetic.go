package realclosure

import "math/big"

func bigPolynomialCopy(a []big.Rat) []big.Rat {
	out := make([]big.Rat, len(a))
	for i := range a {
		out[i].Set(&a[i])
	}
	return out
}

func bigPolynomialNeg(a []big.Rat) []big.Rat {
	out := bigPolynomialCopy(a)
	for i := range out {
		out[i].Neg(&out[i])
	}
	return trimBigPolynomial(out)
}

func bigPolynomialMul(a, b []big.Rat) []big.Rat {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	out := make([]big.Rat, len(a)+len(b)-1)
	var product big.Rat
	for i := range a {
		for j := range b {
			product.Mul(&a[i], &b[j])
			out[i+j].Add(&out[i+j], &product)
		}
	}
	return trimBigPolynomial(out)
}

func bigPolynomialScale(a []big.Rat, scalar *big.Rat) []big.Rat {
	out := bigPolynomialCopy(a)
	for i := range out {
		out[i].Mul(&out[i], scalar)
	}
	return trimBigPolynomial(out)
}

func bigPolynomialDivRem(a, b []big.Rat) ([]big.Rat, []big.Rat, bool) {
	if len(b) == 0 {
		return nil, nil, false
	}
	remainder := bigPolynomialCopy(a)
	var quotient []big.Rat
	if len(a) >= len(b) {
		quotient = make([]big.Rat, len(a)-len(b)+1)
	}
	for len(remainder) >= len(b) {
		shift := len(remainder) - len(b)
		var ratio, product big.Rat
		ratio.Quo(&remainder[len(remainder)-1], &b[len(b)-1])
		quotient[shift].Add(&quotient[shift], &ratio)
		for i := range b {
			product.Mul(&ratio, &b[i])
			remainder[i+shift].Sub(&remainder[i+shift], &product)
		}
		remainder = trimBigPolynomial(remainder)
	}
	return trimBigPolynomial(quotient), remainder, true
}

func bigPolynomialMod(a, modulus []big.Rat) ([]big.Rat, bool) {
	_, remainder, ok := bigPolynomialDivRem(a, modulus)
	return remainder, ok
}
