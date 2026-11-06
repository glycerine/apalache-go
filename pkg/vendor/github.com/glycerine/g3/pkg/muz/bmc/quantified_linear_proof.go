package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear::get_model proof assembly.

import (
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/muz/transforms"
)

// QuantifiedLinearProof constructs the unconverted proof for the selected
// rule trace. The caller owns the returned proof reference. An optional
// normalization value supplies the query-local rule policies.
func (c *Compiler) QuantifiedLinearProof(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig, normalization ...transforms.RuleNormalizationParams) (result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager || services.Context == nil || services.Globals == nil {
		return result, ast.ErrASTHandle
	}
	steps, err := c.quantifiedLinearSteps(rules, query, width, md, services)
	if err != nil {
		return result, err
	}
	return c.quantifiedLinearProofSteps(rules, width, md, services, steps, normalization...)
}

// QuantifiedLinearTraceAndProof selects rules once for both exports.
// The caller owns the returned rules and proof.
func (c *Compiler) QuantifiedLinearTraceAndProof(rules *base.RuleSet, query ast.ASTHandle, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig, normalization ...transforms.RuleNormalizationParams) (trace []*base.Rule, result ast.ASTRef, resultErr error) {
	if c == nil || c.manager == nil || rules == nil || md == nil || md.Manager() != c.manager || services.Context == nil || services.Globals == nil {
		return nil, result, ast.ErrASTHandle
	}
	m := c.manager
	owner := &linearProofOwner{manager: m}
	var previous *base.Rule
	defer func() {
		owner.previous, owner.proof = previous, result
		result, resultErr = finishLinearProof(owner, resultErr)
		if resultErr != nil && len(trace) != 0 {
			resultErr = disposeRuleOwners(m, trace, resultErr)
			trace = nil
		}
	}()
	rm, err := rules.RuleManager()
	if err != nil {
		return nil, result, err
	}
	normalizer := transforms.NewRuleInlinerPluginWithServices(m, rm, nil, true, true, services.Context, services.Globals, 0)
	owner.normalizer = normalizer
	if len(normalization) != 0 {
		normalizer.SetNormalizationParams(normalization[0])
	}
	steps, err := c.quantifiedLinearStepsVisit(rules, query, width, md, services, func(step quantifiedLinearStep) (visitErr error) {
		bindings, cause := c.QuantifiedLinearModelBindings(step.rule, step.index, step.level, width, md, services)
		if cause != nil {
			return cause
		}
		position := ast.ASTProofPosition{First: 1, Second: 0}
		next, proof, cause := c.linearProofStepWithBindings(rm, step.rule, bindings, previous, result, normalizer, position, true, false)
		if cause != nil {
			_, _, cause = advanceQuantifiedLinearProofStep(m, nil, next, ast.ASTRef{}, proof, cause)
			return cause
		}
		oldPrevious, oldResult := previous, result
		previous, result = nil, ast.ASTRef{}
		previous, result, cause = advanceQuantifiedLinearProofStep(m, oldPrevious, next, oldResult, proof, nil)
		return cause
	})
	if err != nil {
		return nil, result, err
	}
	trace = make([]*base.Rule, len(steps))
	for i := range steps {
		trace[i] = steps[i].rule
	}
	if result == (ast.ASTRef{}) {
		return trace, result, ast.ErrASTDomain
	}
	return trace, result, nil
}

// quantifiedLinearProofSteps takes ownership of steps even on failure.
func (c *Compiler) quantifiedLinearProofSteps(rules *base.RuleSet, width uint32, md *model.ModelCore, services model.ModelEvaluatorConfig, steps []quantifiedLinearStep, normalization ...transforms.RuleNormalizationParams) (result ast.ASTRef, resultErr error) {
	m := c.manager
	owner := &linearProofOwner{manager: m, steps: steps}
	var previous *base.Rule
	defer func() {
		owner.previous, owner.proof = previous, result
		result, resultErr = finishLinearProof(owner, resultErr)
	}()
	rm, err := rules.RuleManager()
	if err != nil {
		return result, err
	}
	normalizer := transforms.NewRuleInlinerPluginWithServices(m, rm, nil, true, true, services.Context, services.Globals, 0)
	owner.normalizer = normalizer
	if len(normalization) != 0 {
		normalizer.SetNormalizationParams(normalization[0])
	}
	for _, step := range steps {
		bindings, cause := c.QuantifiedLinearModelBindings(step.rule, step.index, step.level, width, md, services)
		if cause != nil {
			return result, cause
		}
		// The corrected qlinear::get_model resolves the body at 1 against the head at 0.
		position := ast.ASTProofPosition{First: 1, Second: 0}
		// This exported direct helper can start on a rule with a quantified
		// body. Unlike the fixedpoint interface it has no synthetic query
		// bridge, so its first proof must conclude the instantiated rule:
		// otherwise the next resolution can lose the first substitution.
		next, proof, cause := c.linearProofStepWithBindings(rm, step.rule, bindings, previous, result, normalizer, position, true, true)
		if cause != nil {
			_, _, cause = advanceQuantifiedLinearProofStep(m, nil, next, ast.ASTRef{}, proof, cause)
			return result, cause
		}
		oldPrevious, oldResult := previous, result
		previous, result = nil, ast.ASTRef{}
		previous, result, cause = advanceQuantifiedLinearProofStep(m, oldPrevious, next, oldResult, proof, nil)
		if cause != nil {
			return result, cause
		}
	}
	if result == (ast.ASTRef{}) {
		return result, ast.ErrASTDomain
	}
	return result, nil
}
