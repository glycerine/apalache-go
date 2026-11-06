package opt

// Source: src/opt/maxsmt.cpp maxsmt_solver_base::find_mutexes.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *sortMaxEngine) preprocessMutexes() (status lbool.LBool, resultErr error) {
	expressions := make([]ast.ASTHandle, len(e.terms))
	for i := range e.terms {
		expressions[i] = e.terms[i].expression
	}
	status, mutexes, err := e.solver.backend.FindMutexes(expressions)
	if err != nil || status != lbool.True {
		return status, err
	}
	decomposition := newMaxSMTMutexDecomposition(e.solver.manager, e.terms)
	defer func() { resultErr = errors.Join(resultErr, decomposition.close()) }()
	for _, mutex := range mutexes {
		if err = decomposition.transform(mutex); err != nil {
			return lbool.Undef, err
		}
		e.stats.Mutexes++
		e.stats.MutexTerms += uint32(len(mutex))
	}
	offset := decomposition.lowerOffset()
	if !offset.IsInt64() {
		return lbool.Undef, errors.New("opt: SortMax mutex offset overflow")
	}
	e.offset = int(offset.Int64())
	e.terms = decomposition.terms()
	total := 0
	for i := range e.terms {
		total += int(e.terms[i].weight.Int64())
	}
	decomposition.transferOwner(&e.owner)
	e.inputs = maxSMTExpandTerms(e.terms, total)
	return lbool.True, nil
}
