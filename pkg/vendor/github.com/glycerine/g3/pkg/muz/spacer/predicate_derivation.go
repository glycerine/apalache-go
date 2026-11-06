package spacer

// Source: src/muz/spacer/spacer_context.cpp context::create_children.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/stopwatch"
)

// CreateDerivationChild projects the selected rule/model path, builds ordered
// predecessor summaries, and attaches the derivation to its first child.
func (p *PredicateTransformer) CreateDerivationChild(obligation *ProofObligation, rule *base.Rule, md *model.ModelCore, reachPredUsed []bool, reverse bool) (_ *ProofObligation, resultErr error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if obligation == nil || obligation.valid() != nil || obligation.transformer != p || rule == nil || md == nil || md.Manager() != p.manager {
		return nil, ErrManagerOwner
	}
	if p.contextTimers != nil {
		watch, err := stopwatch.NewScopedWatch(&p.contextTimers.children, false)
		if err != nil {
			return nil, err
		}
		defer func() { resultErr = errors.Join(resultErr, watch.Close()) }()
	}
	predecessors, err := p.RulePredecessors(rule)
	if err != nil {
		return nil, err
	}
	if len(predecessors) == 0 || len(reachPredUsed) != len(predecessors) {
		return nil, ast.ErrASTDomain
	}
	transition, err := p.RuleTransition(rule)
	if err != nil {
		return nil, err
	}
	post, err := obligation.Post()
	if err != nil {
		return nil, err
	}
	path, err := p.manager.MkAnd(transition, post)
	if err != nil {
		return nil, err
	}
	derivation, err := NewDerivation(obligation, rule, path, nil)
	if err != nil {
		return nil, err
	}
	committed := false
	defer func() {
		if !committed {
			resultErr = errors.Join(resultErr, derivation.Close())
		}
	}()
	implicantRef, err := derivation.modelImplicant(md, path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, p.manager.DelRef(&implicantRef)) }()
	implicant, err := p.manager.RefGet(implicantRef)
	if err != nil {
		return nil, err
	}
	var variables []ast.ASTHandle
	signature, err := p.Signature()
	if err != nil {
		return nil, err
	}
	for _, declaration := range signature {
		current, err := p.owner.OldToNew(declaration, 0)
		if err != nil {
			return nil, err
		}
		value, err := p.manager.MkApp(current, nil)
		if err != nil {
			return nil, err
		}
		variables = append(variables, value)
	}
	auxiliaries, err := p.RuleAuxiliaries(rule)
	if err != nil {
		return nil, err
	}
	variables = append(variables, auxiliaries...)
	skolems, err := obligation.Skolems()
	if err != nil {
		return nil, err
	}
	variables = append(variables, skolems...)
	projected, remaining, err := derivation.project(p.groundCTI, true, md, variables, implicant)
	if err != nil {
		return nil, err
	}
	if err = p.manager.DelRef(&derivation.transition); err != nil {
		return nil, errors.Join(err, p.manager.DelRef(&projected), releaseNormalizeRefs(p.manager, remaining))
	}
	derivation.transition = projected
	derivation.appendExistentials(remaining)
	level, err := obligation.Level()
	if err != nil {
		return nil, err
	}
	for position := range predecessors {
		index := position
		if reverse {
			index = len(predecessors) - position - 1
		}
		summaryRef, summaryAuxiliaries, err := predecessors[index].OriginSummary(md, previousSpacerLevel(level), uint32(index), reachPredUsed[index])
		if err != nil {
			return nil, err
		}
		summary, err := p.manager.RefGet(summaryRef)
		if err == nil {
			err = derivation.AddPremise(predecessors[index], uint32(index), summary, reachPredUsed[index], summaryAuxiliaries)
		}
		err = errors.Join(err, p.manager.DelRef(&summaryRef))
		if err != nil {
			return nil, err
		}
	}
	child, err := derivation.CreateFirstChild(md)
	if err != nil {
		return nil, err
	}
	if child == nil {
		return nil, nil
	}
	if err = child.SetDerivation(derivation); err != nil {
		return nil, err
	}
	committed = true // child now owns the derivation, including on later errors.
	if !p.useDerivations {
		if err = child.SetDerivation(nil); err != nil {
			return nil, err
		}
	}
	if err = p.checkWeakDerivation(child, md, transition, post); err != nil {
		return nil, err
	}
	if p.contextStats != nil {
		p.contextStats.queries++
	}
	return child, nil
}

func (p *PredicateTransformer) checkWeakDerivation(child *ProofObligation, md *model.ModelCore, transition, post ast.ASTHandle) (resultErr error) {
	// Source create_children drops the optimization when a weak model does
	// not satisfy the original T && phi, even if its selected implicant does.
	if !p.weakAbstraction {
		return nil
	}
	config := p.owner.contexts[0].config.ModelEvaluator
	evaluator, err := newSpacerModelEvaluator(md, config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, evaluator.Close()) }()
	for _, expression := range []ast.ASTHandle{transition, post} {
		isTrue, _, err := evaluateSpacerModelBoolean(p.manager, evaluator, config, expression)
		if err != nil {
			return err
		}
		if !isTrue {
			return child.SetDerivation(nil)
		}
	}
	return nil
}
