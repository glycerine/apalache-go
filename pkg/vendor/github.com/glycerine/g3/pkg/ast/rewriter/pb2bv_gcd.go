package rewriter

// Source: src/ast/rewriter/pb2bv_rewriter.cpp gcd_reduce.

import "math/big"

func pb2BVNormalizeGCD(mode pb2BVComparison, coefficients []*big.Rat, bound *big.Rat) ([]*big.Rat, *big.Rat) {
	cs := make([]*big.Rat, len(coefficients))
	for i, c := range coefficients {
		cs[i] = new(big.Rat).Set(c)
	}
	g := new(big.Int)
	for _, c := range coefficients {
		if !c.IsInt() {
			return cs, new(big.Rat).Set(bound)
		}
		v := new(big.Int).Abs(c.Num())
		g.GCD(nil, nil, g, v)
		if g.Cmp(big.NewInt(1)) == 0 {
			return cs, new(big.Rat).Set(bound)
		}
	}
	if g.Sign() == 0 {
		return cs, new(big.Rat).Set(bound)
	}
	b := new(big.Rat).Set(bound)
	if mode == pb2BVEQ {
		if !b.IsInt() {
			return cs, b
		}
		g.GCD(nil, nil, g, new(big.Int).Abs(b.Num()))
		if g.Sign() == 0 || g.Cmp(big.NewInt(1)) == 0 {
			return cs, b
		}
		b.Quo(b, new(big.Rat).SetInt(g))
	} else {
		b.Quo(b, new(big.Rat).SetInt(g))
		q, rem := new(big.Int).QuoRem(b.Num(), b.Denom(), new(big.Int))
		if mode == pb2BVLE && rem.Sign() < 0 {
			q.Sub(q, big.NewInt(1))
		}
		if mode == pb2BVGE && rem.Sign() != 0 {
			q.Add(q, big.NewInt(1))
		}
		b.SetInt(q)
	}
	divisor := new(big.Rat).SetInt(g)
	for _, c := range cs {
		c.Quo(c, divisor)
	}
	return cs, b
}
