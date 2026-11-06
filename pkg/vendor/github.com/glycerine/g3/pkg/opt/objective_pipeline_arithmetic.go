package opt

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (p *ObjectivePipeline) arithmeticSolver() (solver *ArithmeticSolver, resultErr error) {
	solver, err := NewArithmeticSolver(p.manager, p.params, p.logic, p.config)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, solver.Close())
			solver = nil
		}
	}()
	for _, ref := range p.hard {
		formula, e := p.manager.RefGet(ref)
		if e != nil {
			return nil, e
		}
		if e = solver.Assert(formula); e != nil {
			return nil, e
		}
	}
	for i := range p.objectives {
		objective := &p.objectives[i]
		if objective.term != (ast.ASTRef{}) {
			term, e := p.manager.RefGet(objective.term)
			if e != nil {
				return nil, e
			}
			if e = solver.AddObjective(term, objective.sourceKind == FormulaMaximize); e != nil {
				return nil, e
			}
			continue
		}
		terms, e := p.refHandles(objective.soft)
		if e != nil {
			return nil, e
		}
		if e = solver.AddSoftObjective(terms, pipelineWeightPointers(objective.weights)); e != nil {
			return nil, e
		}
	}
	return solver, nil
}

func (p *ObjectivePipeline) checkArithmetic() (result *ObjectivePipelineResult, resultErr error) {
	solver, err := p.arithmeticSolver()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, solver.Close()) }()
	backend, err := solver.Check()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, backend.Close()) }()
	result = &ObjectivePipelineResult{Status: backend.Status, Optimal: backend.Optimal, Model: backend.Model, Values: make([]ObjectivePipelineValue, len(backend.Values))}
	backend.Model = nil
	result.Statistics = ObjectivePipelineStatistics{Arithmetic: true, MaxSMT: append([]MaxSMTStatistics(nil), backend.MaxSMT...), Objectives: uint32(len(p.objectives)), Hard: uint32(len(p.hard))}
	for i := range backend.Values {
		source, value := &p.objectives[i], &backend.Values[i]
		target := ObjectivePipelineValue{SourceKind: source.sourceKind, Soft: value.Soft, Arithmetic: value.Arithmetic, ModelValue: value.ArithmeticModel, ModelValid: value.ArithmeticModelValid, Interval: value.ArithmeticInterval, OpenBounds: value.ArithmeticOpenBounds}
		if value.Soft {
			target.Exact = source.restore(&value.SoftCost)
		}
		result.Values[i] = target
	}
	return result, nil
}
