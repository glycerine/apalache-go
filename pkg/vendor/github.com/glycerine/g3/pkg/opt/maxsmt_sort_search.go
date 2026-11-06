package opt

// Source: src/opt/sortmax.cpp operator() monotone-output search.

import (
	"math/big"

	"github.com/glycerine/g3/pkg/util/lbool"
)

func (e *sortMaxEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	status, err := e.preprocessMutexes()
	if err != nil || status != lbool.True {
		return status, false, err
	}
	encoder := newSortMaxEncoder(e)
	e.outputs, err = encoder.network(e.inputs)
	if err != nil {
		return lbool.Undef, false, err
	}
	e.stats.Inputs = uint32(len(e.inputs))
	if err = e.updateResult(result); err != nil {
		return lbool.Undef, false, err
	}
	first, err := e.countSatisfiedInputs(result.Model)
	if err != nil {
		return lbool.Undef, false, err
	}
	lower := new(big.Rat).Add(&e.objective.softOffset, new(big.Rat).SetInt64(int64(e.offset)))
	upper := new(big.Rat).Add(lower, new(big.Rat).SetInt64(int64(len(e.inputs)-first)))
	e.setBounds(result, lower, upper)
	for i := 0; i < first; i++ {
		if err = e.solver.backend.Assert(e.outputs[i]); err != nil {
			return lbool.Undef, false, err
		}
	}
	for first < len(e.outputs) {
		if err = e.solver.backend.Assert(e.outputs[first]); err != nil {
			return lbool.Undef, false, err
		}
		e.stats.Checks++
		status, err = e.solver.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, false, err
		}
		if status == lbool.Undef {
			return status, false, nil
		}
		if status == lbool.False {
			lower.Set(upper)
			e.setBounds(result, lower, upper)
			return lbool.True, true, nil
		}
		if err = e.solver.replaceModel(result); err != nil {
			return lbool.Undef, false, err
		}
		if err = e.updateResult(result); err != nil {
			return lbool.Undef, false, err
		}
		e.stats.Improvements++
		first++
		for first < len(e.outputs) {
			isTrue, evalErr := e.outputIsTrue(result.Model, e.outputs[first])
			if evalErr != nil {
				return lbool.Undef, false, evalErr
			}
			if !isTrue {
				break
			}
			if err = e.solver.backend.Assert(e.outputs[first]); err != nil {
				return lbool.Undef, false, err
			}
			first++
		}
		upper = new(big.Rat).Add(lower, new(big.Rat).SetInt64(int64(len(e.outputs)-first)))
		e.setBounds(result, lower, upper)
	}
	e.setBounds(result, lower, lower)
	return lbool.True, true, nil
}
