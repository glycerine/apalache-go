package opt

// Source: src/opt/wmax.cpp weighted-theory MaxSAT search.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type wmaxEngine struct {
	solver       *ArithmeticSolver
	objective    *arithmeticObjective
	index        int
	owner        maxSMTASTOwner
	prepared     arithmeticObjective
	theory       *smtcore.SMTWeightedMaxTheory
	lower, upper big.Rat
	checks       uint32
	mutexes      uint32
	mutexTerms   uint32
	closed       bool
}

func newWMaxEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (result *wmaxEngine, resultErr error) {
	if s == nil || objective == nil {
		return nil, errMaxResState
	}
	prepared := &maxResEngine{
		solver: s, objective: objective, index: index,
		weights: make(map[ast.ASTHandle]*big.Rat), proxies: make(map[ast.ASTHandle]ast.ASTHandle),
	}
	prepared.owner.manager = s.manager
	prepared.lower.Set(&objective.softOffset)
	terms, err := prepared.preprocessMutexes(objective)
	if err != nil {
		return nil, errors.Join(err, prepared.owner.close())
	}
	e := &wmaxEngine{solver: s, objective: objective, index: index, owner: prepared.owner}
	prepared.owner = maxSMTASTOwner{}
	e.lower.Set(&prepared.lower)
	e.mutexes, e.mutexTerms = prepared.stats.Mutexes, prepared.stats.MutexTerms
	e.owner.manager = s.manager
	e.prepared.soft = true
	e.prepared.softOffset.SetInt64(0)
	for _, term := range terms {
		ref, cause := s.manager.NewRef(term.expression)
		if cause != nil {
			return nil, errors.Join(cause, e.close())
		}
		e.owner.refs = append(e.owner.refs, ref)
		e.prepared.softTerms = append(e.prepared.softTerms, ref)
		weight := new(big.Rat).Set(&term.weight)
		e.prepared.softWeights = append(e.prepared.softWeights, weight)
	}
	e.theory, err = s.backend.EnsureWeightedMaxTheory(s.filter)
	if err != nil {
		return nil, errors.Join(err, e.close())
	}
	for i := range e.prepared.softTerms {
		formula, cause := s.manager.RefGet(e.prepared.softTerms[i])
		if cause != nil {
			return nil, errors.Join(cause, e.close())
		}
		slack, assertion, cause := e.theory.AddWeighted(formula, e.prepared.softWeights[i])
		if cause == nil {
			slack, cause = e.owner.hold(slack, nil)
		}
		if cause == nil {
			assertion, cause = e.owner.hold(assertion, nil)
		}
		if cause == nil {
			cause = s.backend.Assert(assertion)
		}
		if cause != nil {
			return nil, errors.Join(cause, e.close())
		}
	}
	return e, nil
}

func (e *wmaxEngine) close() error {
	if e == nil || e.closed {
		return nil
	}
	e.closed = true
	e.prepared.softTerms, e.prepared.softWeights = nil, nil
	err := e.owner.close()
	if e.theory != nil {
		err = errors.Join(err, e.theory.ResetLocal())
		e.theory = nil
	}
	return err
}

func (e *wmaxEngine) scorePrepared(mdModel *model.ModelCore) (satisfied, penalty big.Rat, resultErr error) {
	expressions := make([]ast.ASTHandle, len(e.prepared.softTerms))
	for i := range e.prepared.softTerms {
		value, err := e.solver.manager.RefGet(e.prepared.softTerms[i])
		if err != nil {
			return satisfied, penalty, err
		}
		expressions[i] = value
	}
	assignment, err := wmaxModelAssignment(e.solver, mdModel, expressions)
	if err != nil {
		return satisfied, penalty, err
	}
	for i, yes := range assignment {
		if yes {
			satisfied.Add(&satisfied, e.prepared.softWeights[i])
		} else {
			penalty.Add(&penalty, e.prepared.softWeights[i])
		}
	}
	return satisfied, penalty, nil
}

func (e *wmaxEngine) publishAssignment(result *ArithmeticResult) error {
	expressions := make([]ast.ASTHandle, len(e.objective.softTerms))
	for i := range e.objective.softTerms {
		value, err := e.solver.manager.RefGet(e.objective.softTerms[i])
		if err != nil {
			return err
		}
		expressions[i] = value
	}
	assignment, err := wmaxModelAssignment(e.solver, result.Model, expressions)
	if err != nil {
		return err
	}
	var cost, satisfied big.Rat
	cost.Set(&e.objective.softOffset)
	for i, yes := range assignment {
		if yes {
			satisfied.Add(&satisfied, e.objective.softWeights[i])
		} else {
			cost.Add(&cost, e.objective.softWeights[i])
		}
	}
	setSoftObjectiveValue(result, e.index, cost, satisfied, assignment)
	value := &result.Values[e.index]
	value.SoftLower.Set(&e.lower)
	value.SoftUpper.Set(&e.upper)
	value.SoftBoundsValid = true
	return nil
}

func (e *wmaxEngine) optimize(result *ArithmeticResult) (status lbool.LBool, optimal bool, resultErr error) {
	if e == nil || e.solver == nil || e.theory == nil || result == nil || result.Model == nil {
		return lbool.Undef, false, errMaxResState
	}
	_, penalty, err := e.scorePrepared(result.Model)
	if err != nil {
		return lbool.Undef, false, err
	}
	e.upper.Add(&e.lower, &penalty)
	if err = e.theory.InitMinCost(&penalty); err != nil {
		return lbool.Undef, false, err
	}
	status = lbool.True
	for e.lower.Cmp(&e.upper) < 0 {
		status, err = e.solver.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, false, err
		}
		e.checks++
		if status != lbool.True {
			break
		}
		isOptimal, cause := e.theory.IsOptimal()
		if cause != nil {
			return lbool.Undef, false, cause
		}
		if isOptimal {
			cost, cause := e.theory.Cost()
			if cause != nil {
				return lbool.Undef, false, cause
			}
			e.upper.Add(&e.lower, cost)
			if cause = e.solver.replaceModel(result); cause != nil {
				return lbool.Undef, false, cause
			}
		}
		block, cause := e.theory.MakeBlock()
		if cause == nil {
			block, cause = e.owner.hold(block, nil)
		}
		if cause == nil {
			cause = e.solver.backend.Assert(block)
		}
		if cause != nil {
			return lbool.Undef, false, cause
		}
		var limit big.Rat
		limit.Sub(&e.upper, &e.lower)
		if err = e.theory.InitMinCost(&limit); err != nil {
			return lbool.Undef, false, err
		}
	}
	if status == lbool.False {
		e.lower.Set(&e.upper)
		status = lbool.True
	}
	if status == lbool.Undef && e.lower.Cmp(&e.upper) == 0 {
		status = lbool.True
	}
	if err = e.publishAssignment(result); err != nil {
		return lbool.Undef, false, err
	}
	return status, status == lbool.True && e.lower.Cmp(&e.upper) == 0, nil
}

func (e *wmaxEngine) statistics() MaxSMTStatistics {
	return MaxSMTStatistics{Engine: "wmax", Checks: e.checks, Mutexes: e.mutexes, MutexTerms: e.mutexTerms}
}
