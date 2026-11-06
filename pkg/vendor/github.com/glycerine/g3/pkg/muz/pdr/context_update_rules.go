package pdr

// Source: context::update_rules and context::reset in
// src/muz/pdr/pdr_context.cpp.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// CanUpdateRules reports whether set can replace the current program without
// rebuilding the source PDR solver configuration. Planning is read-only and
// the temporary rule references are released before return.
func (c *Context) CanUpdateRules(set *base.RuleSet) (compatible bool, resultErr error) {
	if err := c.open(); err != nil {
		return false, err
	}
	if !c.initialized || set == nil {
		return false, ast.ErrASTDomain
	}
	plan, err := c.planRules(set)
	if err != nil {
		return false, err
	}
	defer func() { resultErr = errors.Join(resultErr, plan.close()) }()
	requiredProof := plan.nonBoolean || c.useConvexClosure || c.useConvexInterior
	return requiredProof == c.proofActive && c.arithmeticPlanCompatible(plan.arithmetic), nil
}

// UpdateRules rebuilds the executable rule transformers and inherits all
// frame properties belonging to declarations retained by the new rule set.
func (c *Context) UpdateRules(set *base.RuleSet) (resultErr error) {
	if err := c.open(); err != nil {
		return err
	}
	if !c.initialized || set == nil {
		return ast.ErrASTDomain
	}
	plan, err := c.planRules(set)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, plan.close()) }()
	requiredProof := plan.nonBoolean || c.useConvexClosure || c.useConvexInterior
	if requiredProof != c.proofActive || !c.arithmeticPlanCompatible(plan.arithmetic) {
		return ast.ErrASTDomain
	}
	restoreProofMode, err := c.enterPDRProofMode()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, restoreProofMode()) }()
	staged, err := c.buildRuleTransformers(plan)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, closeTransformers(plan.order, staged))
		}
	}()
	for _, declaration := range plan.order {
		if prior := c.transformers[declaration]; prior != nil {
			if err = staged[declaration].InheritProperties(prior); err != nil {
				return err
			}
		}
	}
	c.unknownResult = false
	if err = c.search.Reset(); err != nil {
		return err
	}
	if err = closeTransformers(c.order, c.transformers); err != nil {
		return err
	}
	c.transformers = staged
	c.order = append(c.order[:0], plan.order...)
	c.query = nil
	c.queryDecl = ast.ASTHandle{}
	c.lastResult = lbool.Undef
	c.expanded, c.inductive = 0, 0
	c.farkasActive = c.useFarkas && plan.nonBoolean
	committed = true
	return nil
}
