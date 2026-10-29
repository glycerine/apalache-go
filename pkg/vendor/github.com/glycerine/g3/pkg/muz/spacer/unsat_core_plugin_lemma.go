package spacer

// Source: unsat_core_plugin_lemma in
// src/muz/spacer/spacer_unsat_core_plugin.cpp and .h.

import (
	"github.com/glycerine/g3/pkg/ast"
)

type UnsatCoreLemmaPlugin struct {
	learner *UnsatCoreLearner
	closed  bool
}

func NewUnsatCoreLemmaPlugin(learner *UnsatCoreLearner) (*UnsatCoreLemmaPlugin, error) {
	if learner == nil || learner.open() != nil {
		return nil, ErrUnsatCoreLearnerOwner
	}
	return &UnsatCoreLemmaPlugin{learner: learner}, nil
}

func (p *UnsatCoreLemmaPlugin) open() error {
	if p == nil || p.closed || p.learner == nil {
		return ErrUnsatCoreLearnerOwner
	}
	return p.learner.open()
}

func (p *UnsatCoreLemmaPlugin) addLowestSplitToCore(step ast.ASTHandle) error {
	stack := []ast.ASTHandle{step}
	for len(stack) != 0 {
		last := len(stack) - 1
		current := stack[last]
		stack = stack[:last]
		if p.learner.IsClosed(current) {
			continue
		}
		if err := p.learner.SetClosed(current, true); err != nil {
			return err
		}
		fact, err := p.learner.manager.GetProofFact(current)
		if err != nil {
			return err
		}
		pure, err := p.learner.IsBPure(current)
		if err != nil {
			return err
		}
		asserted, err := p.learner.manager.IsProofKind(current, ast.ASTPrAsserted)
		if err != nil {
			return err
		}
		literal, err := p.learner.manager.IsLiteral(fact)
		if err != nil {
			return err
		}
		if pure && (asserted || literal) {
			if err = p.learner.AddLemmaToCore(fact); err != nil {
				return err
			}
			continue
		}
		count, err := p.learner.manager.GetProofNumParents(current)
		if err != nil {
			return err
		}
		for i := uint32(0); i < count; i++ {
			parent, err := p.learner.manager.GetProofParent(current, i)
			if err != nil {
				return err
			}
			if p.learner.IsBOpen(parent) {
				stack = append(stack, parent)
			}
		}
	}
	return nil
}

func (p *UnsatCoreLemmaPlugin) ComputePartialCore(step ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	count, err := p.learner.manager.GetProofNumParents(step)
	if err != nil {
		return err
	}
	for i := uint32(0); i < count; i++ {
		parent, err := p.learner.manager.GetProofParent(step, i)
		if err != nil {
			return err
		}
		if p.learner.IsBOpen(parent) {
			if err = p.addLowestSplitToCore(parent); err != nil {
				return err
			}
		}
	}
	return p.learner.SetClosed(step, true)
}

func (p *UnsatCoreLemmaPlugin) Finalize() error { return p.open() }

func (p *UnsatCoreLemmaPlugin) Close() error {
	if p == nil {
		return ErrUnsatCoreLearnerOwner
	}
	if p.closed {
		return nil
	}
	p.learner = nil
	p.closed = true
	return nil
}
