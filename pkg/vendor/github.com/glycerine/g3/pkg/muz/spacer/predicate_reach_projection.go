package spacer

// Source: src/muz/spacer/spacer_context.cpp context::mk_reach_fact.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

func (p *PredicateTransformer) makeConfiguredReachFact(obligation *ProofObligation, rule *base.Rule, md *model.ModelCore) (*ReachFact, error) {
	return p.MakeReachFact(obligation, rule, md, p.eliminateAuxiliaries, p.reachDNF)
}

// MakeReachFact constructs a concrete reach fact from a rule path selected by
// md. The returned fact owns retained references to every child justification.
func (p *PredicateTransformer) MakeReachFact(obligation *ProofObligation, rule *base.Rule, md *model.ModelCore, eliminateAuxiliaries, reachDNF bool) (result *ReachFact, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	defer func() {
		if resultErr != nil && result != nil {
			if closeErr := result.Close(); closeErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("spacer: reach fact result cleanup: %w", closeErr))
			}
			result = nil
		}
	}()
	if obligation == nil || obligation.valid() != nil || obligation.transformer != p || rule == nil || md == nil || md.Manager() != p.manager {
		return nil, ErrManagerOwner
	}
	transition, err := p.RuleTransition(rule)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact transition: %w", err)
	}
	path := []ast.ASTHandle{transition}
	predecessors, err := p.RulePredecessors(rule)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact predecessors: %w", err)
	}
	justifications := make([]*ReachFact, 0, len(predecessors))
	var renamedReferences []ast.ASTRef
	defer func() {
		for _, justification := range justifications {
			if closeErr := justification.Close(); closeErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("spacer: reach fact justification cleanup: %w", closeErr))
			}
		}
		if closeErr := releaseNormalizeRefs(p.manager, renamedReferences); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("spacer: reach fact renamed formula cleanup: %w", closeErr))
		}
	}()
	var variables []ast.ASTHandle
	for index, predecessor := range predecessors {
		fact, found, err := predecessor.UsedOriginReachFact(md, uint32(index))
		if err != nil || !found {
			if err == nil {
				err = ast.ErrASTDomain
			}
			return nil, fmt.Errorf("spacer: reach fact predecessor %d origin: %w", index, err)
		}
		justifications = append(justifications, fact)
		formula, err := fact.Fact()
		if err != nil {
			return nil, fmt.Errorf("spacer: reach fact predecessor %d formula: %w", index, err)
		}
		originRef, err := p.owner.FormulaNewToOld(formula, uint32(index), false)
		if err != nil {
			return nil, fmt.Errorf("spacer: reach fact predecessor %d renaming: %w", index, err)
		}
		origin, err := p.manager.RefGet(originRef)
		if err != nil {
			_ = p.manager.DelRef(&originRef)
			return nil, fmt.Errorf("spacer: reach fact predecessor %d renamed formula: %w", index, err)
		}
		renamedReferences = append(renamedReferences, originRef)
		path = append(path, origin)
		signature, err := predecessor.Signature()
		if err != nil {
			return nil, fmt.Errorf("spacer: reach fact predecessor %d signature: %w", index, err)
		}
		for _, declaration := range signature {
			originDeclaration, err := p.owner.OldToOld(declaration, 0, uint32(index))
			if err != nil {
				return nil, fmt.Errorf("spacer: reach fact predecessor %d signature renaming: %w", index, err)
			}
			value, err := p.manager.MkApp(originDeclaration, nil)
			if err != nil {
				return nil, fmt.Errorf("spacer: reach fact predecessor %d signature value: %w", index, err)
			}
			variables = append(variables, value)
		}
		auxiliaries, err := fact.Auxiliaries()
		if err != nil {
			return nil, fmt.Errorf("spacer: reach fact predecessor %d auxiliaries: %w", index, err)
		}
		for _, auxiliary := range auxiliaries {
			declaration, err := p.manager.GetDecl(auxiliary)
			if err != nil {
				return nil, fmt.Errorf("spacer: reach fact predecessor %d auxiliary declaration: %w", index, err)
			}
			originDeclaration, err := p.owner.NewToOld(declaration, uint32(index))
			if err != nil {
				return nil, fmt.Errorf("spacer: reach fact predecessor %d auxiliary renaming: %w", index, err)
			}
			value, err := p.manager.MkApp(originDeclaration, nil)
			if err != nil {
				return nil, fmt.Errorf("spacer: reach fact predecessor %d auxiliary value: %w", index, err)
			}
			variables = append(variables, value)
		}
	}
	ruleAuxiliaries, err := p.RuleAuxiliaries(rule)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact auxiliaries: %w", err)
	}
	if eliminateAuxiliaries {
		variables = append(variables, ruleAuxiliaries...)
	}
	formula, err := p.manager.MkAndArray(path)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact path: %w", err)
	}
	temporary, err := NewDerivation(obligation, rule, formula, nil)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact derivation: %w", err)
	}
	defer func() {
		if closeErr := temporary.Close(); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("spacer: reach fact derivation cleanup: %w", closeErr))
		}
	}()
	var implicantRef ast.ASTRef
	if reachDNF {
		implicantRef, err = temporary.modelImplicant(md, formula)
		if err != nil {
			return nil, fmt.Errorf("spacer: reach fact implicant: %w", err)
		}
		formula, err = p.manager.RefGet(implicantRef)
		if err != nil {
			return nil, fmt.Errorf("spacer: reach fact implicant result: %w", err)
		}
		defer func() {
			if closeErr := p.manager.DelRef(&implicantRef); closeErr != nil {
				resultErr = errors.Join(resultErr, fmt.Errorf("spacer: reach fact implicant cleanup: %w", closeErr))
			}
		}()
	}
	projectedRef, remaining, err := temporary.project(false, md, variables, formula)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact projection: %w", err)
	}
	defer func() {
		if closeErr := errors.Join(p.manager.DelRef(&projectedRef), releaseNormalizeRefs(p.manager, remaining)); closeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("spacer: reach fact projection cleanup: %w", closeErr))
		}
	}()
	if len(remaining) != 0 {
		return nil, fmt.Errorf("spacer: reach fact projection retained %d variables: %w", len(remaining), ast.ErrASTDomain)
	}
	projected, err := p.manager.RefGet(projectedRef)
	if err != nil {
		return nil, fmt.Errorf("spacer: reach fact projected formula: %w", err)
	}
	retainedAuxiliaries := ruleAuxiliaries
	if eliminateAuxiliaries {
		retainedAuxiliaries = nil
	}
	result, err = NewReachFact(p.manager, rule, projected, retainedAuxiliaries, false)
	if err != nil {
		return nil, fmt.Errorf("spacer: retain reach fact: %w", err)
	}
	for _, justification := range justifications {
		if err = result.AddJustification(justification); err != nil {
			return nil, fmt.Errorf("spacer: retain reach fact justification: %w", errors.Join(err, result.Close()))
		}
	}
	return result, nil
}
