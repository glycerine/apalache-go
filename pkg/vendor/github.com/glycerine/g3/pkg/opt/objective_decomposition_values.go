package opt

import (
	"errors"
	"math/big"
)

var ErrInvalidOptimizationValue = errors.New("g3: invalid optimization value")

// RestoreValue maps a MaxSAT penalty back to the source objective value.
func (o *DecomposedObjective) RestoreValue(penalty *big.Rat) (*big.Rat, error) {
	if o == nil || penalty == nil {
		return nil, ErrInvalidOptimizationValue
	}
	result := new(big.Rat).Set(penalty)
	if o.Negate {
		result.Neg(result)
	}
	result.Add(result, &o.Offset)
	return result, nil
}
