package opt

// Source: src/opt/maxres.cpp get_cores: extract disjoint cores from one
// unsatisfiable round before applying MaxRes transformations.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func removeMaxResCoreMembers(values, core []ast.ASTHandle) []ast.ASTHandle {
	members := make(map[ast.ASTHandle]struct{}, len(core))
	for _, expression := range core {
		members[expression] = struct{}{}
	}
	answer := values[:0]
	for _, expression := range values {
		if _, found := members[expression]; !found {
			answer = append(answer, expression)
		}
	}
	return answer
}

func (e *maxResEngine) collectCoreBatch(first []ast.ASTHandle, result *ArithmeticResult) ([][]ast.ASTHandle, lbool.LBool, error) {
	core, err := e.filterCore(first)
	if err != nil {
		return nil, lbool.Undef, err
	}
	if len(core) == 0 {
		e.lower.Set(&e.upper)
		return nil, lbool.True, nil
	}
	if err = e.updateMUSModel(result); err != nil {
		return nil, lbool.Undef, err
	}
	var status lbool.LBool
	core, status, err = e.minimizeCore(core)
	if err != nil {
		return nil, lbool.Undef, err
	}
	if status != lbool.True {
		return nil, status, nil
	}
	cores := [][]ast.ASTHandle{core}
	remaining := removeMaxResCoreMembers(append([]ast.ASTHandle(nil), e.active...), core)
	for len(remaining) != 0 && !e.coreBatchLimitReached(len(cores), len(core)) {
		e.stats.Checks++
		status, err = e.solver.backend.CheckSat(remaining)
		if err != nil {
			return nil, lbool.Undef, err
		}
		switch status {
		case lbool.True:
			if err = e.updateCandidate(result); err != nil {
				return nil, lbool.Undef, err
			}
			return cores, lbool.True, nil
		case lbool.False:
			raw, err := e.solver.backend.UnsatCore()
			if err != nil {
				return nil, lbool.Undef, err
			}
			if err = e.updateMUSModel(result); err != nil {
				return nil, lbool.Undef, err
			}
			core, err = e.filterCore(raw)
			if err != nil {
				return nil, lbool.Undef, err
			}
			if len(core) == 0 {
				e.lower.Set(&e.upper)
				return nil, lbool.True, nil
			}
			core, status, err = e.minimizeCore(core)
			if err != nil {
				return nil, lbool.Undef, err
			}
			if status != lbool.True {
				return nil, status, nil
			}
			cores = append(cores, core)
			remaining = removeMaxResCoreMembers(remaining, core)
		case lbool.Undef:
			return nil, lbool.Undef, nil
		default:
			return nil, lbool.Undef, errMaxResState
		}
	}
	if len(remaining) == 0 {
		return cores, lbool.True, nil
	}
	return nil, lbool.False, nil
}
