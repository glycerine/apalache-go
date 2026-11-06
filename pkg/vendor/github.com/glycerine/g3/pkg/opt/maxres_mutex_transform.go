package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::process_mutex, exact rational
// specialization used by MaxRes.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
)

func (e *maxResEngine) transformMutex(mutex []ast.ASTHandle, weights map[ast.ASTHandle]*big.Rat, order *[]ast.ASTHandle) error {
	active := make([]ast.ASTHandle, 0, len(mutex))
	for _, expression := range mutex {
		if weight := weights[expression]; weight != nil && weight.Sign() > 0 {
			active = append(active, expression)
		}
	}
	if len(active) <= 1 {
		return nil
	}
	sortMaxResMutex(active, weights)
	values := make([]big.Rat, len(active))
	for i, expression := range active {
		values[i].Set(weights[expression])
		delete(weights, expression)
	}
	var previous big.Rat
	for i := len(active) - 1; i >= 0; i-- {
		delta := new(big.Rat).Sub(&values[i], &previous)
		previous.Set(&values[i])
		if delta.Sign() == 0 {
			continue
		}
		prefix, err := e.owner.hold(e.solver.manager.MkOrArgs(active[:i+1]))
		if err != nil {
			return err
		}
		if weights[prefix] == nil {
			*order = append(*order, prefix)
		}
		addMaxResMutexWeight(weights, prefix, delta)
		contribution := new(big.Rat).Mul(delta, new(big.Rat).SetInt64(int64(i)))
		e.lower.Add(&e.lower, contribution)
		e.stats.MutexTerms++
	}
	e.stats.Mutexes++
	return nil
}
