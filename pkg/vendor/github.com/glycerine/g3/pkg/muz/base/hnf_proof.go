package base

// Source: src/muz/base/hnf.cpp, the proof-carrying disjunctive antecedent
// split in hnf::imp::mk_horn.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/qe"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// AddFormulaWithProof ingests a formula and proof through the source Horn
// normalization worklist, carrying proofs through each transformation.
func (m *RuleManager) AddFormulaWithProof(set *RuleSet, formula, proof ast.ASTHandle, name string) (resultErr error) {
	label, err := m.ruleName(name)
	if err != nil {
		return err
	}
	return m.AddFormulaWithProofAndSymbol(set, formula, proof, label)
}

// AddFormulaWithProofAndSymbol carries the input proof and exact source label
// through the Horn normalization worklist.
func (m *RuleManager) AddFormulaWithProofAndSymbol(set *RuleSet, formula, proof ast.ASTHandle, name usymbol.Symbol) error {
	return m.addFormulaWithProof(set, formula, proof, name, nil, false)
}

// AddFormulaWithProofAndUnboundFix applies the source post-ingestion rule
// rewrite after transferring the formula proof to each normalized rule.
func (m *RuleManager) AddFormulaWithProofAndUnboundFix(set *RuleSet, formula, proof ast.ASTHandle, name string, lite *qe.Lite, enabled bool) error {
	if lite == nil {
		return fmt.Errorf("muz/base: nil unbound-variable eliminator")
	}
	label, err := m.ruleName(name)
	if err != nil {
		return err
	}
	return m.AddFormulaWithProofAndSymbolAndUnboundFix(set, formula, proof, label, lite, enabled)
}

// AddFormulaWithProofAndSymbolAndUnboundFix preserves the label while carrying
// proofs through normalization and the configured unbound-variable rewrite.
func (m *RuleManager) AddFormulaWithProofAndSymbolAndUnboundFix(set *RuleSet, formula, proof ast.ASTHandle, name usymbol.Symbol, lite *qe.Lite, enabled bool) error {
	if lite == nil {
		return fmt.Errorf("muz/base: nil unbound-variable eliminator")
	}
	return m.addFormulaWithProof(set, formula, proof, name, lite, enabled)
}

func (m *RuleManager) addFormulaWithProof(set *RuleSet, formula, proof ast.ASTHandle, name usymbol.Symbol, lite *qe.Lite, fixUnbound bool) (resultErr error) {
	if proof.IsNull() {
		return m.addFormula(set, formula, name, lite, fixUnbound)
	}
	if err := m.checkRuleName(name); err != nil {
		return err
	}
	if set == nil || set.manager != m {
		return ErrRuleSetOwner
	}
	oldFact, err := m.manager.GetProofFact(proof)
	if err != nil {
		return err
	}
	if oldFact != formula {
		return fmt.Errorf("muz/base: HNF proof does not prove the supplied formula")
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
	formulaRef, err := m.manager.NewRef(formula)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&formulaRef)) }()
	proofRef, err := m.manager.NewRef(proof)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, m.manager.DelRef(&proofRef)) }()
	labels, err := rewriter.NewLabelRewriter(m.manager)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, labels.Close()) }()
	if err = labels.RemoveLabels(formulaRef, proofRef); err != nil {
		return err
	}
	formula, err = m.manager.RefGet(formulaRef)
	if err != nil {
		return err
	}
	proof, err = m.manager.RefGet(proofRef)
	if err != nil {
		return err
	}
	normalizer, err := m.prepareHornNormalizer(name)
	if err != nil {
		return err
	}
	normalized, err := normalizer.NormalizeWithProof(formula, proof)
	if normalized != nil {
		defer func() { resultErr = errors.Join(resultErr, normalized.Close()) }()
	}
	if err != nil {
		return err
	}
	if err = m.registerHornPredicates(); err != nil {
		return err
	}
	for i, ref := range normalized.Formulas {
		clause, err := m.manager.RefGet(ref)
		if err != nil {
			return err
		}
		clauseProof, err := m.manager.RefGet(normalized.Proofs[i])
		if err != nil {
			return err
		}
		rule, e := m.newRuleFromFormulaWithProofAndSymbol(clause, clauseProof, name, true)
		if e != nil {
			return e
		}
		if lite != nil {
			fixed, _, fixErr := m.FixUnboundVariables(rule, lite, fixUnbound)
			if fixErr != nil {
				return errors.Join(fixErr, rule.Close())
			}
			if fixErr = m.MkRuleRewriteProof(rule, fixed); fixErr != nil {
				return errors.Join(fixErr, fixed.Close(), rule.Close())
			}
			if fixErr = rule.Close(); fixErr != nil {
				return errors.Join(fixErr, fixed.Close())
			}
			rule = fixed
		}
		if e = set.Add(rule); e != nil {
			return errors.Join(e, rule.Close())
		}
		if e = rule.Close(); e != nil {
			return e
		}
	}
	return nil
}
