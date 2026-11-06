package base

// Source: src/muz/base/dl_rule.cpp rule_manager::mk_rule_rewrite_proof and
// rule::get_proof/set_proof.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
)

// MkRuleAssertedProof attaches an asserted proof of the rule's formula if the
// rule does not already carry a proof. Proof mode is restored on return.
func (m *RuleManager) MkRuleAssertedProof(rule *Rule) (resultErr error) {
	if err := m.open(); err != nil {
		return err
	}
	if rule == nil {
		return ErrRuleOwner
	}
	manager, err := rule.ASTManager()
	if err != nil {
		return err
	}
	if manager != m.manager {
		return fmt.Errorf("muz/base: asserted-proof rule belongs to another AST manager")
	}
	_, found, err := rule.Proof()
	if err != nil || found {
		return err
	}
	scope, err := ast.NewScopedProof(m.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, scope.Close()) }()
	formula, err := rule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formula)) }()
	fact, err := m.manager.RefGet(formula)
	if err != nil {
		return err
	}
	// Keep the fresh proof owned while SetProof allocates the rule's slot.
	owner, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&owner)) }()
	proof, err := m.manager.MkProofAsserted(fact)
	if err = errors.Join(err, m.manager.SetRef(owner, proof)); err != nil {
		return err
	}
	return rule.SetProof(proof)
}

// MkRuleRewriteProof transfers a proof of oldRule's formula to newRule by
// rewriting the proved fact to the new rule formula and applying modus ponens.
func (m *RuleManager) MkRuleRewriteProof(oldRule, newRule *Rule) (resultErr error) {
	if err := m.open(); err != nil {
		return err
	}
	if oldRule == nil || newRule == nil {
		return ErrRuleOwner
	}
	oldManager, err := oldRule.ASTManager()
	if err != nil {
		return err
	}
	newManager, err := newRule.ASTManager()
	if err != nil {
		return err
	}
	if oldManager != m.manager || newManager != m.manager {
		return fmt.Errorf("muz/base: rewrite-proof rules belong to another AST manager")
	}
	same, err := oldRule.Same(newRule)
	if err != nil || same {
		return err
	}
	oldProof, hasOldProof, err := oldRule.Proof()
	if err != nil || !hasOldProof {
		return err
	}
	_, hasNewProof, err := newRule.Proof()
	if err != nil || hasNewProof {
		return err
	}
	mode, err := m.manager.ProofMode()
	if err != nil {
		return err
	}
	if mode == ast.ASTProofDisabled {
		if err = m.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, m.manager.ToggleProofMode(mode)) }()
	}
	formula, err := newRule.ToFormula()
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formula)) }()
	newFact, err := m.manager.RefGet(formula)
	if err != nil {
		return err
	}
	oldFact, err := m.manager.GetProofFact(oldProof)
	if err != nil {
		return err
	}
	// Retain the rewrite across modus ponens, then retain the resulting proof
	// until the rule acquires its own reference.
	owner, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&owner)) }()
	rewriteProof, err := m.manager.MkProofRewrite(oldFact, newFact)
	if err = errors.Join(err, m.manager.SetRef(owner, rewriteProof)); err != nil {
		return err
	}
	proof, err := m.manager.MkProofModusPonens(oldProof, rewriteProof)
	if err = errors.Join(err, m.manager.SetRef(owner, proof)); err != nil {
		return err
	}
	return newRule.SetProof(proof)
}
