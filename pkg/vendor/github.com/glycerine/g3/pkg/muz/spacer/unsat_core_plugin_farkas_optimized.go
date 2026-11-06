package spacer

// Source: unsat_core_plugin_farkas_lemma_optimized in
// src/muz/spacer/spacer_unsat_core_plugin.cpp and .h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type farkasTerm struct {
	literal     ast.ASTHandle
	coefficient *big.Rat
}

type UnsatCoreOptimizedFarkasPlugin struct {
	learner      *UnsatCoreLearner
	farkas       *pdr.FarkasLearner
	combinations [][]farkasTerm
	closed       bool
}

func NewUnsatCoreOptimizedFarkasPlugin(learner *UnsatCoreLearner, context *uparams.ParamsContext, globals *uparams.GlobalParamsService) (*UnsatCoreOptimizedFarkasPlugin, error) {
	if learner == nil || learner.open() != nil || context == nil || globals == nil {
		return nil, ErrUnsatCoreLearnerOwner
	}
	farkas, err := pdr.NewFarkasLearner(learner.manager, context, globals)
	if err != nil {
		return nil, err
	}
	return &UnsatCoreOptimizedFarkasPlugin{learner: learner, farkas: farkas}, nil
}

func (p *UnsatCoreOptimizedFarkasPlugin) open() error {
	if p == nil || p.closed || p.learner == nil || p.farkas == nil {
		return ErrUnsatCoreLearnerOwner
	}
	return p.learner.open()
}

func (p *UnsatCoreOptimizedFarkasPlugin) ComputePartialCore(step ast.ASTHandle) error {
	if err := p.open(); err != nil {
		return err
	}
	facts, coefficients, matched, err := p.farkas.FarkasPremises(step)
	if err != nil || !matched || p.learner.IsClosed(step) {
		return err
	}
	combination := make([]farkasTerm, 0, len(facts))
	canClose := true
	for i := range facts {
		parent, err := p.learner.manager.GetProofParent(step, uint32(i))
		if err != nil {
			return err
		}
		if !p.learner.IsBOpen(parent) {
			continue
		}
		pure, err := p.learner.OnlyContainsSymbolsB(facts[i])
		if err != nil {
			return err
		}
		if pure && !p.learner.IsHMarked(parent) {
			combination = append(combination, farkasTerm{literal: facts[i], coefficient: new(big.Rat).Set(coefficients[i])})
		} else {
			canClose = false
		}
	}
	if !canClose {
		return nil
	}
	if err = p.learner.SetClosed(step, true); err != nil {
		return err
	}
	if len(combination) != 0 {
		p.combinations = append(p.combinations, combination)
	}
	return nil
}

func (p *UnsatCoreOptimizedFarkasPlugin) combine(terms []farkasTerm) (resultErr error) {
	literals := make([]ast.ASTHandle, len(terms))
	coefficients := make([]*big.Rat, len(terms))
	for i := range terms {
		literals[i] = terms[i].literal
		coefficients[i] = terms[i].coefficient
	}
	combined, err := p.farkas.CombineConstraints(literals, coefficients)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, p.learner.manager.DelRef(&combined)) }()
	complement, err := p.learner.manager.RefGet(combined)
	if err != nil {
		return err
	}
	negated, err := p.learner.manager.MkNot(complement)
	if err != nil {
		return err
	}
	return p.learner.AddLemmaToCore(negated)
}

func (p *UnsatCoreOptimizedFarkasPlugin) Finalize() error {
	if err := p.open(); err != nil {
		return err
	}
	if len(p.combinations) == 0 {
		return nil
	}
	basis := make([]ast.ASTHandle, 0)
	indices := make(map[ast.ASTHandle]uint32)
	for _, combination := range p.combinations {
		for _, term := range combination {
			if _, found := indices[term.literal]; !found {
				indices[term.literal] = uint32(len(basis))
				basis = append(basis, term.literal)
			}
		}
	}
	matrix := newSpacerMatrix(uint32(len(p.combinations)), uint32(len(basis)))
	for row, combination := range p.combinations {
		for _, term := range combination {
			if err := matrix.set(uint32(row), indices[term.literal], term.coefficient); err != nil {
				return err
			}
		}
	}
	rank, err := matrix.gaussianElimination()
	if err != nil {
		return err
	}
	for row := uint32(0); row < rank; row++ {
		terms := make([]farkasTerm, 0, len(basis))
		for column, literal := range basis {
			coefficient, err := matrix.get(row, uint32(column))
			if err != nil {
				return err
			}
			if coefficient.Sign() != 0 {
				terms = append(terms, farkasTerm{literal: literal, coefficient: coefficient})
			}
		}
		if len(terms) == 0 {
			return ast.ErrASTDomain
		}
		if err = p.combine(terms); err != nil {
			return err
		}
	}
	p.combinations = nil
	return nil
}

func (p *UnsatCoreOptimizedFarkasPlugin) Close() error {
	if p == nil {
		return ErrUnsatCoreLearnerOwner
	}
	if p.closed {
		return nil
	}
	p.closed = true
	p.combinations = nil
	err := p.farkas.Close()
	p.farkas = nil
	p.learner = nil
	return err
}
