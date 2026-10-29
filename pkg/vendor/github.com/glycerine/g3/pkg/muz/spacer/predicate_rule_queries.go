package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer rule maps,
// find_predecessors, remove_predecessors, and add_premises.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
)

type RuleSelection struct {
	Rule          *base.Rule
	IsConcrete    bool
	ReachPredUsed []bool
	NumReuseReach uint32
}

func (s *RuleSelection) Close() error {
	if s == nil || s.Rule == nil {
		return nil
	}
	err := s.Rule.Close()
	s.Rule = nil
	s.ReachPredUsed = nil
	return err
}

func (p *PredicateTransformer) normalizedRule(rule *base.Rule) (*spacerNormalizedRule, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if rule == nil {
		return nil, base.ErrRuleOwner
	}
	for _, candidate := range p.rules {
		same, err := candidate.rule.Same(rule)
		if err != nil {
			return nil, err
		}
		if same {
			return candidate, nil
		}
	}
	return nil, ast.ErrASTDomain
}

// RuleTag returns the retained rule-selection literal used in the transition.
func (p *PredicateTransformer) RuleTag(rule *base.Rule) (ast.ASTHandle, error) {
	normalized, err := p.normalizedRule(rule)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if normalized.tag.IsNull() {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return normalized.tag, nil
}

// RuleTransition returns the normalized, unguarded transition for rule.
func (p *PredicateTransformer) RuleTransition(rule *base.Rule) (ast.ASTHandle, error) {
	normalized, err := p.normalizedRule(rule)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return normalized.transition, nil
}

// RuleAuxiliaries returns the current-state constants introduced for variables
// that occur only in interpreted rule tails.
func (p *PredicateTransformer) RuleAuxiliaries(rule *base.Rule) ([]ast.ASTHandle, error) {
	normalized, err := p.normalizedRule(rule)
	if err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), normalized.auxiliaries...), nil
}

// RulePredecessors returns predicate transformers in uninterpreted-tail order.
func (p *PredicateTransformer) RulePredecessors(rule *base.Rule) ([]*PredicateTransformer, error) {
	normalized, err := p.normalizedRule(rule)
	if err != nil {
		return nil, err
	}
	return append([]*PredicateTransformer(nil), normalized.predecessors...), nil
}

// TaggedRule returns an owning rule reference for a selection literal.
func (p *PredicateTransformer) TaggedRule(tag ast.ASTHandle) (*base.Rule, bool, error) {
	if err := p.open(); err != nil {
		return nil, false, err
	}
	for _, normalized := range p.rules {
		if normalized.tag == tag {
			rule, err := normalized.rule.Retain()
			return rule, err == nil, err
		}
	}
	return nil, false, nil
}

// SelectRule finds a true rule tag in the model, preferring a rule whose every
// predecessor uses its latest reach fact. A nil rule means the model selected
// the extension literal instead of a rule.
func (p *PredicateTransformer) SelectRule(md *model.ModelCore) (_ RuleSelection, resultErr error) {
	if err := p.open(); err != nil {
		return RuleSelection{}, err
	}
	if md == nil {
		return RuleSelection{}, ast.ErrASTHandle
	}
	config := p.owner.contexts[0].config.ModelEvaluator
	evaluator, err := model.NewModelEvaluator(md, config)
	if err != nil {
		return RuleSelection{}, err
	}
	selection := RuleSelection{}
	defer func() {
		resultErr = errors.Join(resultErr, evaluator.Close())
		if resultErr != nil {
			resultErr = errors.Join(resultErr, selection.Close())
		}
	}()
	for _, normalized := range p.rules {
		if normalized.tag.IsNull() {
			continue
		}
		valueRef, err := evaluator.EvaluateValue(normalized.tag)
		if err != nil {
			return RuleSelection{}, err
		}
		value, err := p.manager.RefGet(valueRef)
		isTrue := err == nil && p.manager.IsTrue(value)
		deleteErr := p.manager.DelRef(&valueRef)
		if err != nil || deleteErr != nil {
			return RuleSelection{}, errors.Join(err, deleteErr)
		}
		if !isTrue {
			continue
		}
		if selection.Rule != nil {
			if err = selection.Rule.Close(); err != nil {
				return RuleSelection{}, err
			}
		}
		selection.Rule, err = normalized.rule.Retain()
		if err != nil {
			return RuleSelection{}, err
		}
		selection.IsConcrete = true
		selection.NumReuseReach = 0
		selection.ReachPredUsed = selection.ReachPredUsed[:0]
		for index, predecessor := range normalized.predecessors {
			used := false
			if len(predecessor.reachFacts) == 0 {
				selection.IsConcrete = false
			} else {
				latest, err := predecessor.manager.RefGet(predecessor.reachCaseVariables[len(predecessor.reachCaseVariables)-1])
				if err != nil {
					_ = selection.Close()
					return RuleSelection{}, err
				}
				originRef, err := p.owner.FormulaNewToOld(latest, uint32(index), false)
				if err != nil {
					_ = selection.Close()
					return RuleSelection{}, err
				}
				origin, err := p.manager.RefGet(originRef)
				if err == nil {
					var evaluated ast.ASTRef
					evaluated, err = evaluator.EvaluateValue(origin)
					if err == nil {
						var result ast.ASTHandle
						result, err = p.manager.RefGet(evaluated)
						used = err == nil && p.manager.IsFalse(result)
						err = errors.Join(err, p.manager.DelRef(&evaluated))
					}
				}
				err = errors.Join(err, p.manager.DelRef(&originRef))
				if err != nil {
					_ = selection.Close()
					return RuleSelection{}, err
				}
				selection.IsConcrete = selection.IsConcrete && used
			}
			selection.ReachPredUsed = append(selection.ReachPredUsed, used)
			if used {
				selection.NumReuseReach++
			}
		}
		if selection.IsConcrete {
			return selection, nil
		}
	}
	if selection.Rule == nil {
		selection.IsConcrete = true
	}
	return selection, nil
}

// RemovePredecessors removes positive or negative rule-selection literals.
func (p *PredicateTransformer) RemovePredecessors(literals []ast.ASTHandle) ([]ast.ASTHandle, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, 0, len(literals))
	for _, literal := range literals {
		original := literal
		negated, child, err := p.manager.IsNot(literal)
		if err != nil {
			return nil, err
		}
		if negated {
			literal = child
		}
		isTag := false
		for _, normalized := range p.rules {
			if normalized.tag == literal {
				isTag = true
				break
			}
		}
		if !isTag {
			result = append(result, original)
		}
	}
	return result, nil
}

func (p *PredicateTransformer) appendRulePremises(level uint32, normalized *spacerNormalizedRule, result *[]ast.ASTRef) error {
	for index, predecessor := range normalized.predecessors {
		formula, err := predecessor.Formulas(level, false)
		if err != nil {
			return err
		}
		if p.manager.IsTrue(formula) {
			continue
		}
		converted, err := p.owner.FormulaNewToOld(formula, uint32(index), true)
		if err != nil {
			return err
		}
		*result = append(*result, converted)
	}
	return nil
}

// RulePremises returns owned predecessor-frame formulas in old-state order.
func (p *PredicateTransformer) RulePremises(level uint32, rule *base.Rule) (_ []ast.ASTRef, resultErr error) {
	normalized, err := p.normalizedRule(rule)
	if err != nil {
		return nil, err
	}
	var result []ast.ASTRef
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, result))
		}
	}()
	if err = p.appendRulePremises(level, normalized, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Premises returns owned background, state, and predecessor-frame formulas.
func (p *PredicateTransformer) Premises(level uint32) (_ []ast.ASTRef, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if !p.initialized {
		return nil, ast.ErrASTDomain
	}
	var result []ast.ASTRef
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, result))
		}
	}()
	background, err := p.owner.Background()
	if err != nil {
		return nil, err
	}
	backgroundRef, err := p.manager.NewRef(background)
	if err != nil {
		return nil, err
	}
	result = append(result, backgroundRef)
	state := p.transition
	if level == 0 {
		state = p.initial
	}
	stateRef, err := p.manager.CopyRef(state)
	if err != nil {
		return nil, err
	}
	result = append(result, stateRef)
	for _, normalized := range p.rules {
		if err = p.appendRulePremises(level, normalized, &result); err != nil {
			return nil, err
		}
	}
	return result, nil
}
