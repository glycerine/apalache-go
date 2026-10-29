package spacer

// Source: src/muz/spacer/spacer_context.cpp pred_transformer::is_reachable.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type ReachabilityResult struct {
	Status        lbool.LBool
	Core          []ast.ASTRef
	Model         *model.ModelCore
	UsesLevel     uint32
	Rule          *base.Rule
	IsConcrete    bool
	ReachPredUsed []bool
	NumReuseReach uint32
}

func (r *ReachabilityResult) Close(manager *ast.ASTManager) (result error) {
	if r == nil {
		return nil
	}
	if manager == nil && len(r.Core) != 0 {
		return ast.ErrASTHandle
	}
	result = errors.Join(result, releaseNormalizeRefs(manager, r.Core))
	r.Core = nil
	if r.Model != nil {
		result = errors.Join(result, r.Model.DecRef())
		r.Model = nil
	}
	if r.Rule != nil {
		result = errors.Join(result, r.Rule.Close())
		r.Rule = nil
	}
	r.ReachPredUsed = nil
	return result
}

func (p *PredicateTransformer) SetInitializeReachFacts(value bool) error {
	if err := p.open(); err != nil {
		return err
	}
	p.initializeReachFacts = value
	return nil
}

func (p *PredicateTransformer) reachAssumptions(level uint32) (_ []ast.ASTHandle, references []ast.ASTRef, resultErr error) {
	if level == 0 || p.allInitial {
		return nil, nil, nil
	}
	var result []ast.ASTHandle
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, references))
		}
	}()
	for _, rule := range p.rules {
		if rule.tag.IsNull() || len(rule.predecessors) == 0 {
			continue
		}
		for index, predecessor := range rule.predecessors {
			if len(predecessor.reachFacts) != 0 {
				latest, err := predecessor.manager.RefGet(predecessor.reachCaseVariables[len(predecessor.reachCaseVariables)-1])
				if err != nil {
					return nil, nil, err
				}
				originRef, err := p.owner.FormulaNewToOld(latest, uint32(index), false)
				if err != nil {
					return nil, nil, err
				}
				references = append(references, originRef)
				origin, err := p.manager.RefGet(originRef)
				if err != nil {
					return nil, nil, err
				}
				negated, err := p.manager.MkNot(origin)
				if err != nil {
					return nil, nil, err
				}
				negatedRef, err := p.manager.NewRef(negated)
				if err != nil {
					return nil, nil, err
				}
				references = append(references, negatedRef)
				result = append(result, negated)
			} else if p.initializeReachFacts {
				negated, err := p.manager.MkNot(rule.tag)
				if err != nil {
					return nil, nil, err
				}
				negatedRef, err := p.manager.NewRef(negated)
				if err != nil {
					return nil, nil, err
				}
				references = append(references, negatedRef)
				result = append(result, negated)
				break
			}
		}
	}
	return result, references, nil
}

// CheckReachability checks an obligation at its frame level and, on SAT,
// selects the active rule and predecessor reach facts from the returned model.
func (p *PredicateTransformer) CheckReachability(obligation *ProofObligation) (result *ReachabilityResult, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if obligation == nil || obligation.valid() != nil || obligation.transformer != p {
		return nil, ErrManagerOwner
	}
	level, err := obligation.Level()
	if err != nil {
		return nil, err
	}
	if err = p.EnsureLevel(level); err != nil {
		return nil, err
	}
	post, err := obligation.Post()
	if err != nil {
		return nil, err
	}
	soft, references, err := p.reachAssumptions(level)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, releaseNormalizeRefs(p.manager, references)) }()
	extension, err := p.manager.RefGet(p.extendLiteral)
	if err != nil {
		return nil, err
	}
	useFarkas, err := obligation.UseFarkas()
	if err != nil {
		return nil, err
	}
	weakness, err := obligation.Weakness()
	if err != nil {
		return nil, err
	}
	check, err := p.solver.CheckAssumptions([]ast.ASTHandle{post}, soft, PropCheckOptions{
		Background:      []ast.ASTHandle{extension},
		NeedCore:        true,
		NeedModel:       true,
		SubsetBasedCore: !useFarkas,
		Level:           &level,
		WeakAbstraction: p.weakAbstraction,
		Weakness:        &weakness,
	})
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, check.Close(p.manager)) }()
	result = &ReachabilityResult{Status: check.Status, UsesLevel: check.UsesLevel, Core: check.Core, Model: check.Model, IsConcrete: check.Status != lbool.True}
	check.Core = nil
	check.Model = nil
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, result.Close(p.manager))
			result = nil
		}
	}()
	if result.Status != lbool.False && result.Model != nil {
		selection, err := p.SelectRule(result.Model)
		if err != nil {
			return nil, err
		}
		result.Rule = selection.Rule
		result.IsConcrete = selection.IsConcrete
		result.ReachPredUsed = selection.ReachPredUsed
		result.NumReuseReach = selection.NumReuseReach
		selection.Rule = nil
		selection.ReachPredUsed = nil
	}
	committed = true
	return result, nil
}
