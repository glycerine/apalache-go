package opt

// Source: src/opt/sortmax.cpp sortmax state and construction.

import "github.com/glycerine/g3/pkg/ast"

type sortMaxEngine struct {
	solver    *ArithmeticSolver
	objective *arithmeticObjective
	index     int
	owner     maxSMTASTOwner
	terms     []maxSMTWeightedTerm
	inputs    []ast.ASTHandle
	outputs   []ast.ASTHandle
	offset    int
	stats     sortMaxStatistics
}

func newSortMaxEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int) (*sortMaxEngine, bool, error) {
	terms, total, usable, err := maxSMTIntegralTerms(s.manager, objective)
	if err != nil || !usable {
		return nil, usable, err
	}
	engine := &sortMaxEngine{solver: s, objective: objective, index: index, terms: terms}
	engine.owner.manager = s.manager
	engine.inputs = maxSMTExpandTerms(terms, total)
	return engine, true, nil
}

func (e *sortMaxEngine) close() error {
	if e == nil {
		return nil
	}
	e.inputs, e.outputs, e.terms = nil, nil, nil
	return e.owner.close()
}
