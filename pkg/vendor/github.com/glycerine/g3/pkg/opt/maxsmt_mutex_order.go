package opt

// Source: src/opt/maxsmt.cpp maxsmt_compare_soft.

import (
	"math/big"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
)

func maxSMTSortMutex(mutex []ast.ASTHandle, weights map[ast.ASTHandle]*big.Int) {
	sort.SliceStable(mutex, func(i, j int) bool {
		return weights[mutex[i]].Cmp(weights[mutex[j]]) > 0
	})
}
