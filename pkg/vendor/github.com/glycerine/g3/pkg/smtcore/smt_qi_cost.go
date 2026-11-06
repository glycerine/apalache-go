package smtcore

// Source: src/smt/qi_queue.cpp parser setup, value publication, cost
// evaluation, and new-generation evaluation.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	parserutil "github.com/glycerine/g3/pkg/parsers/util"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

const (
	smtQICostValueCSFactor = iota
	smtQICostValueNestedQuantifiers
	smtQICostValueScope
	smtQICostValueTotalInstances
	smtQICostValuePatternWidth
	smtQICostValueVariables
	smtQICostValueWeight
	smtQICostValueQuantifierGeneration
	smtQICostValueGeneration
	smtQICostValueDepth
	smtQICostValueSize
	smtQICostValueInstances
	smtQICostValueMaxTopGeneration
	smtQICostValueMinTopGeneration
	smtQICostValueCost
	smtQICostValueCount
)

var smtQICostVariableNames = [...]string{
	"cost", "min_top_generation", "max_top_generation", "instances", "size", "depth", "generation",
	"quant_generation", "weight", "vars", "pattern_width", "total_instances", "scope", "nested_quantifiers", "cs_factor",
}

type SMTQICostModel struct {
	manager       *ast.ASTManager
	parser        *parserutil.CostParser
	evaluator     *SMTCostEvaluator
	costFunction  ast.ASTRef
	newGeneration ast.ASTRef
	values        [smtQICostValueCount]float32
	closed        bool
}

func NewSMTQICostModel(manager *ast.ASTManager, costText, newGenerationText string, warnings *diagnostic.WarningService) (*SMTQICostModel, error) {
	if manager == nil {
		return nil, ast.ErrASTHandle
	}
	parser, err := parserutil.NewCostParser(manager)
	if err != nil {
		return nil, err
	}
	model := &SMTQICostModel{manager: manager, parser: parser}
	for _, name := range smtQICostVariableNames {
		if _, err = parser.AddCostVariable(name); err != nil {
			return model, errors.Join(err, model.Close())
		}
	}
	model.evaluator, err = NewSMTCostEvaluator(manager, warnings)
	if err != nil {
		return model, errors.Join(err, model.Close())
	}
	model.costFunction, err = model.parseWithFallback(costText, "(+ weight generation)")
	if err != nil {
		return model, errors.Join(err, model.Close())
	}
	model.newGeneration, err = model.parseWithFallback(newGenerationText, "cost")
	if err != nil {
		return model, errors.Join(err, model.Close())
	}
	return model, nil
}

func (m *SMTQICostModel) parseWithFallback(text, fallback string) (ast.ASTRef, error) {
	result, ok, err := m.parser.ParseString(text)
	if err != nil || ok {
		return result, err
	}
	result, ok, err = m.parser.ParseString(fallback)
	if err != nil {
		return ast.ASTRef{}, err
	}
	if !ok {
		return ast.ASTRef{}, ast.ErrASTDomain
	}
	return result, nil
}

func (m *SMTQICostModel) open() error {
	if m == nil || m.manager == nil || m.parser == nil || m.evaluator == nil || m.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (m *SMTQICostModel) setValues(quantifier, pattern ast.ASTHandle, stat *SMTQuantifierStat, generation, minTopGeneration, maxTopGeneration uint32, scope uint32, cost float32) error {
	if err := m.open(); err != nil {
		return err
	}
	if stat == nil {
		return ast.ErrASTDomain
	}
	kind, err := m.manager.GetKind(quantifier)
	if err != nil || kind != ast.ASTQuantifier {
		if err != nil {
			return err
		}
		return ast.ErrASTDomain
	}
	weight, err := m.manager.GetQuantifierWeight(quantifier)
	if err != nil {
		return err
	}
	variables, err := m.manager.GetNumDeclarations(quantifier)
	if err != nil {
		return err
	}
	patternWidth := uint32(1)
	if !pattern.IsNull() {
		patternWidth, err = m.manager.GetNumArgs(pattern)
		if err != nil {
			return err
		}
	}
	m.values[smtQICostValueCost] = cost
	m.values[smtQICostValueMinTopGeneration] = float32(minTopGeneration)
	m.values[smtQICostValueMaxTopGeneration] = float32(maxTopGeneration)
	m.values[smtQICostValueInstances] = float32(stat.NumInstancesCurrentBranch())
	m.values[smtQICostValueSize] = float32(stat.Size())
	m.values[smtQICostValueDepth] = float32(stat.Depth())
	m.values[smtQICostValueGeneration] = float32(generation)
	m.values[smtQICostValueQuantifierGeneration] = float32(stat.Generation())
	m.values[smtQICostValueWeight] = float32(weight)
	m.values[smtQICostValueVariables] = float32(variables)
	m.values[smtQICostValuePatternWidth] = float32(patternWidth)
	m.values[smtQICostValueTotalInstances] = float32(stat.NumInstancesCurrentSearch())
	m.values[smtQICostValueScope] = float32(scope)
	m.values[smtQICostValueNestedQuantifiers] = float32(stat.NumNestedQuantifiers())
	m.values[smtQICostValueCSFactor] = float32(stat.CaseSplitFactor())
	return nil
}

func (m *SMTQICostModel) Cost(quantifier, pattern ast.ASTHandle, stat *SMTQuantifierStat, generation, minTopGeneration, maxTopGeneration, scope uint32) (float32, error) {
	if err := m.setValues(quantifier, pattern, stat, generation, minTopGeneration, maxTopGeneration, scope, 0); err != nil {
		return 0, err
	}
	function, err := m.manager.RefGet(m.costFunction)
	if err != nil {
		return 0, err
	}
	result, err := m.evaluator.Eval(function, m.values[:])
	if err == nil {
		stat.UpdateMaxCost(result)
	}
	return result, err
}

func (m *SMTQICostModel) NewGeneration(quantifier ast.ASTHandle, stat *SMTQuantifierStat, generation, scope uint32, cost float32) (uint32, error) {
	if err := m.setValues(quantifier, ast.ASTHandle{}, stat, generation, 0, 0, scope, cost); err != nil {
		return 0, err
	}
	function, err := m.manager.RefGet(m.newGeneration)
	if err != nil {
		return 0, err
	}
	result, err := m.evaluator.Eval(function, m.values[:])
	return uint32(result), err
}

func (m *SMTQICostModel) Close() error {
	if m == nil {
		return ast.ErrASTHandle
	}
	if m.closed {
		return nil
	}
	var result error
	if m.manager != nil {
		if _, err := m.manager.RefGet(m.newGeneration); err == nil {
			result = errors.Join(result, m.manager.DelRef(&m.newGeneration))
		}
		if _, err := m.manager.RefGet(m.costFunction); err == nil {
			result = errors.Join(result, m.manager.DelRef(&m.costFunction))
		}
	}
	if m.parser != nil {
		result = errors.Join(result, m.parser.Close())
	}
	m.closed = result == nil
	return result
}
