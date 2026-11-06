package opt

// Source: src/opt/maxsmt.cpp obj_map<expr, rational> updates.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func addMaxResMutexWeight(weights map[ast.ASTHandle]*big.Rat, expression ast.ASTHandle, weight *big.Rat) {
	if old := weights[expression]; old != nil {
		old.Add(old, weight)
		return
	}
	weights[expression] = new(big.Rat).Set(weight)
}
