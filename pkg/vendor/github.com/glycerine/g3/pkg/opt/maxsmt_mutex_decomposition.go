package opt

// Source: src/opt/maxsmt.cpp find_mutexes/process_mutex retained shared state.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

type maxSMTMutexDecomposition struct {
	owner   maxSMTASTOwner
	weights map[ast.ASTHandle]*big.Int
	order   []ast.ASTHandle
	offset  big.Int
}

func newMaxSMTMutexDecomposition(manager *ast.ASTManager, terms []maxSMTWeightedTerm) *maxSMTMutexDecomposition {
	d := &maxSMTMutexDecomposition{weights: make(map[ast.ASTHandle]*big.Int, len(terms)), order: make([]ast.ASTHandle, 0, len(terms))}
	d.owner.manager = manager
	for i := range terms {
		if prior := d.weights[terms[i].expression]; prior != nil {
			prior.Add(prior, &terms[i].weight)
			continue
		}
		d.weights[terms[i].expression] = new(big.Int).Set(&terms[i].weight)
		d.order = append(d.order, terms[i].expression)
	}
	return d
}
