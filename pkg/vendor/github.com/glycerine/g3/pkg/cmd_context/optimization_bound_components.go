package cmd_context

import "math/big"

// Components returns independently copied infinity, rational and
// infinitesimal coefficients in the source vector order.
func (b OptimizationBound) Components() [3]big.Rat {
	var result [3]big.Rat
	result[0].Set(&b.Value.Infinity)
	result[1].Set(&b.Value.Main)
	result[2].Set(&b.Value.Epsilon)
	return result
}
