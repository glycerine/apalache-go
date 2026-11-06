package base

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

// NewRuleFromFormulaWithProof constructs an already-Horn rule and transfers a
// proof of the input formula to the rule formula, matching mk_horn_rule's
// rewrite and modus-ponens path. General formulas requiring HNF definitions
// remain the responsibility of AddFormula.
func (m *RuleManager) NewRuleFromFormulaWithProof(formula, proof ast.ASTHandle, name string) (result *Rule, resultErr error) {
	label, err := m.ruleName(name)
	if err != nil {
		return nil, err
	}
	return m.NewRuleFromFormulaWithProofAndSymbol(formula, proof, label)
}

// NewRuleFromFormulaWithProofAndSymbol preserves the exact source label while
// transferring the input proof to the normalized rule formula.
func (m *RuleManager) NewRuleFromFormulaWithProofAndSymbol(formula, proof ast.ASTHandle, name symbol.Symbol) (result *Rule, resultErr error) {
	return m.newRuleFromFormulaWithProofAndSymbol(formula, proof, name, false)
}

func (m *RuleManager) newRuleFromFormulaWithProofAndSymbol(formula, proof ast.ASTHandle, name symbol.Symbol, fromHNF bool) (result *Rule, resultErr error) {
	if proof.IsNull() {
		return m.newRuleFromFormulaWithSymbol(formula, name, fromHNF)
	}
	rule, err := m.newRuleFromFormulaWithSymbol(formula, name, fromHNF)
	if err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, rule.Close())
			result = nil
		}
	}()
	oldFact, err := m.manager.GetProofFact(proof)
	if err != nil {
		return nil, err
	}
	newFormula, err := rule.ToFormula()
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&newFormula)) }()
	newFact, err := m.manager.RefGet(newFormula)
	if err != nil {
		return nil, err
	}
	if oldFact == newFact {
		if err = rule.SetProof(proof); err != nil {
			return nil, err
		}
		result = rule
		return rule, nil
	}
	mode, err := m.manager.ProofMode()
	if err != nil {
		return nil, err
	}
	if mode == ast.ASTProofDisabled {
		if err = m.manager.ToggleProofMode(ast.ASTProofEnabled); err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, m.manager.ToggleProofMode(mode)) }()
	}
	// Keep both intermediate proofs owned across the next fallible
	// constructor, including SetProof's allocation of the rule's reference.
	proofOwner, err := m.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&proofOwner)) }()
	var rewrite ast.ASTHandle
	kind, err := m.manager.GetKind(newFact)
	if err != nil {
		return nil, err
	}
	if kind == ast.ASTQuantifier {
		// Source mk_horn_rule proves the quantified rule equivalent to its
		// open Horn clause with DER, then reverses that equivalence.
		rewrite, err = m.manager.MkProofDER(newFact, oldFact)
		if !rewrite.IsNull() {
			err = errors.Join(err, m.manager.SetRef(proofOwner, rewrite))
		}
		if err != nil {
			return nil, err
		}
		rewrite, err = m.manager.MkProofSymmetry(rewrite)
	} else {
		rewrite, err = m.manager.MkProofRewrite(oldFact, newFact)
	}
	err = errors.Join(err, m.manager.SetRef(proofOwner, rewrite))
	if err != nil {
		return nil, err
	}
	translated, err := m.manager.MkProofModusPonens(proof, rewrite)
	err = errors.Join(err, m.manager.SetRef(proofOwner, translated))
	if err != nil {
		return nil, err
	}
	if err = rule.SetProof(translated); err != nil {
		return nil, err
	}
	result = rule
	return rule, nil
}
