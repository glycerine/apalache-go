package opt

// Source: src/opt/maxsmt.cpp maxsmt_compare_soft for rational weights.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

func sortMaxResMutex(mutex []ast.ASTHandle, weights map[ast.ASTHandle]*big.Rat) {
	sort.SliceStable(mutex, func(i, j int) bool {
		return weights[mutex[i]].Cmp(weights[mutex[j]]) > 0
	})
}
