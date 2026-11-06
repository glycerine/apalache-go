package opt

// Source: src/opt/maxsmt.cpp post-mutex soft reconstruction.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (d *maxSMTMutexDecomposition) terms() []maxSMTWeightedTerm {
	result := make([]maxSMTWeightedTerm, 0, len(d.order))
	seen := make(map[ast.ASTHandle]struct{}, len(d.order))
	for _, expression := range d.order {
		if _, duplicate := seen[expression]; duplicate {
			continue
		}
		seen[expression] = struct{}{}
		weight := d.weights[expression]
		if weight == nil || weight.Sign() <= 0 {
			continue
		}
		result = append(result, maxSMTWeightedTerm{expression: expression, weight: *new(big.Int).Set(weight)})
	}
	return result
}

func (d *maxSMTMutexDecomposition) lowerOffset() *big.Int {
	return new(big.Int).Set(&d.offset)
}
