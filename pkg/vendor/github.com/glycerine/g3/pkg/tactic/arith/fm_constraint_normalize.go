package arith

// Source: src/tactic/arith/fm_tactic.cpp:549-591.
import "math/big"

func fmAbsInt(v *big.Int) *big.Int { return new(big.Int).Abs(v) }
func fmLCM(a, b *big.Int) *big.Int {
	if a.Sign() == 0 || b.Sign() == 0 {
		return new(big.Int)
	}
	g := new(big.Int).GCD(nil, nil, a, b)
	return new(big.Int).Abs(new(big.Int).Quo(new(big.Int).Mul(a, b), g))
}
func (c *FMConstraint) MakeInteger() {
	l := new(big.Int).Set(c.Constant.Denom())
	for i := range c.Coefficients {
		l = fmLCM(l, c.Coefficients[i].Denom())
	}
	if l.Cmp(big.NewInt(1)) == 0 {
		return
	}
	c.Constant.Mul(&c.Constant, new(big.Rat).SetInt(l))
	for i := range c.Coefficients {
		c.Coefficients[i].Mul(&c.Coefficients[i], new(big.Rat).SetInt(l))
	}
}
func (c *FMConstraint) NormalizeCoefficients() {
	if len(c.Variables) == 0 {
		return
	}
	if !c.Constant.IsInt() {
		return
	}
	g := fmAbsInt(c.Constant.Num())
	for i := range c.Coefficients {
		if !c.Coefficients[i].IsInt() {
			return
		}
		g.GCD(nil, nil, g, fmAbsInt(c.Coefficients[i].Num()))
		if g.Cmp(big.NewInt(1)) == 0 {
			return
		}
	}
	if g.Sign() == 0 || g.Cmp(big.NewInt(1)) == 0 {
		return
	}
	q := new(big.Rat).SetInt(g)
	c.Constant.Quo(&c.Constant, q)
	for i := range c.Coefficients {
		c.Coefficients[i].Quo(&c.Coefficients[i], q)
	}
}
