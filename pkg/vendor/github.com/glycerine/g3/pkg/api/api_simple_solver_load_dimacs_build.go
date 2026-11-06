package api

// Source: src/api/api_solver.cpp:169-180 constructs a temporary sat::solver,
// parses clauses, and recovers a goal through sat2goal before assertion.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/sat"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	basetactic "github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (s *APISimpleSolver) buildDIMACSAssertions(manager *ast.ASTManager, cnf sat.DIMACSCNF) (result []ast.ASTRef, resultErr error) {
	services, err := s.owner.command.ModelEvaluatorConfig()
	if err != nil {
		return nil, err
	}
	params, err := s.solver.Params()
	if err != nil {
		return nil, err
	}
	runtime, err := sat.NewRuntime(sat.RuntimeConfig{
		Budget: manager.Budget(), ParamsContext: services.Context,
		Process: services.Memory.ProcessOwner(), Globals: services.Globals,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, runtime.Close()) }()
	backend, err := sat.NewSATSolver(sat.SolverConfig{
		TraceTags: services.Trace, ParamsContext: services.Context, Params: *params,
		RLimit: manager.ResourceLimit(), Budget: manager.Budget(), Runtime: runtime,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, backend.Close()) }()
	var variableCount uint32
	for _, clause := range cnf.Clauses {
		literals := make([]sat.Literal, len(clause))
		for i, value := range clause {
			index := int64(value)
			negated := index < 0
			if negated {
				index = -index
			}
			for variableCount <= uint32(index) {
				if _, err = backend.MkVar(false, true); err != nil {
					return nil, err
				}
				variableCount++
			}
			literals[i] = sat.MakeLiteral(sat.BoolVar(index), negated)
		}
		if err = backend.MkClause(literals); err != nil {
			return nil, err
		}
	}
	translator, err := sattactic.NewGoalTranslator(manager, backend)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, translator.Close()) }()
	goal, err := basetactic.NewGoal(manager, false, false, false)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, goal.Close()) }()
	name := "max_memory"
	maxMemoryMB, err := services.Context.GetUIntName(params, &name, ^uint32(0))
	if err != nil {
		return nil, err
	}
	converter, err := sattactic.RecoverSATGoal(translator, goal, false, umemory.MegabytesToBytes(maxMemoryMB), services)
	if converter != nil {
		err = errors.Join(err, converter.Close())
	}
	if err != nil {
		return nil, err
	}
	formulas, err := goal.AppendFormulas(nil)
	if err != nil {
		return nil, err
	}
	for _, formula := range formulas {
		owned, ownErr := manager.NewRef(formula)
		if ownErr != nil {
			for i := range result {
				ownErr = errors.Join(ownErr, manager.DelRef(&result[i]))
			}
			return nil, ownErr
		}
		result = append(result, owned)
	}
	return result, nil
}
