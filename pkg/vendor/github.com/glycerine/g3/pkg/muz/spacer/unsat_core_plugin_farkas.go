package spacer

// Source: unsat_core_plugin_farkas_lemma in
// src/muz/spacer/spacer_unsat_core_plugin.cpp and .h.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/pdr"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type UnsatCoreFarkasPlugin struct {
	learner          *UnsatCoreLearner
	farkas           *pdr.FarkasLearner
	useConstantFromA bool
	closed           bool
}

func NewUnsatCoreFarkasPlugin(learner *UnsatCoreLearner, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, splitLiterals, useConstantFromA bool) (*UnsatCoreFarkasPlugin, error) {
	if learner == nil || learner.open() != nil || context == nil || globals == nil {
		return nil, ErrUnsatCoreLearnerOwner
	}
	farkas, err := pdr.NewFarkasLearner(learner.manager, context, globals)
	if err != nil {
		return nil, err
	}
	if err = farkas.SetSplitLiterals(splitLiterals); err != nil {
		return nil, errors.Join(err, farkas.Close())
	}
	return &UnsatCoreFarkasPlugin{learner: learner, farkas: farkas, useConstantFromA: useConstantFromA}, nil
}

func (p *UnsatCoreFarkasPlugin) open() error {
	if p == nil || p.closed || p.learner == nil || p.farkas == nil {
		return ErrUnsatCoreLearnerOwner
	}
	return p.learner.open()
}

func (p *UnsatCoreFarkasPlugin) ComputePartialCore(step ast.ASTHandle) (resultErr error) {
	if err := p.open(); err != nil {
		return err
	}
	matched, err := p.farkas.IsFarkasLemma(step)
	if err != nil || !matched || p.learner.IsClosed(step) {
		return err
	}
	count, err := p.learner.manager.GetProofNumParents(step)
	if err != nil {
		return err
	}
	bOpen := make([]bool, count)
	openCount := 0
	for i := uint32(0); i < count; i++ {
		parent, err := p.learner.manager.GetProofParent(step, i)
		if err != nil {
			return err
		}
		bOpen[i] = p.learner.IsBOpen(parent)
		if bOpen[i] {
			openCount++
		}
	}
	if openCount == 0 {
		return nil
	}
	pure, err := p.learner.IsBPure(step)
	if err != nil || !pure {
		return err
	}
	var combined ast.ASTRef
	produced := false
	if p.useConstantFromA {
		combined, produced, err = p.farkas.CombineFarkasLemma(step, bOpen)
		if err != nil || !produced {
			return err
		}
	} else {
		facts, coefficients, found, err := p.farkas.FarkasPremises(step)
		if err != nil || !found {
			return err
		}
		selectedFacts := make([]ast.ASTHandle, 0, openCount)
		selectedCoefficients := make([]*big.Rat, 0, openCount)
		for i := range facts {
			if bOpen[i] {
				selectedFacts = append(selectedFacts, facts[i])
				selectedCoefficients = append(selectedCoefficients, coefficients[i])
			}
		}
		combined, err = p.farkas.CombineConstraints(selectedFacts, selectedCoefficients)
		if err != nil {
			return err
		}
		weighted, err := p.learner.manager.RefGet(combined)
		if err != nil {
			return errors.Join(err, p.learner.manager.DelRef(&combined))
		}
		negated, err := p.learner.manager.MkNot(weighted)
		if err != nil {
			return errors.Join(err, p.learner.manager.DelRef(&combined))
		}
		negatedRef, err := p.learner.manager.NewRef(negated)
		if err != nil {
			return errors.Join(err, p.learner.manager.DelRef(&combined))
		}
		if err = p.learner.manager.DelRef(&combined); err != nil {
			return errors.Join(err, p.learner.manager.DelRef(&negatedRef))
		}
		combined = negatedRef
		produced = true
	}
	if !produced {
		return nil
	}
	defer func() { resultErr = errors.Join(resultErr, p.learner.manager.DelRef(&combined)) }()
	handle, err := p.learner.manager.RefGet(combined)
	if err != nil {
		return err
	}
	if err = p.learner.SetClosed(step, true); err != nil {
		return err
	}
	return p.learner.AddLemmaToCore(handle)
}

func (p *UnsatCoreFarkasPlugin) Finalize() error { return p.open() }

func (p *UnsatCoreFarkasPlugin) Close() error {
	if p == nil {
		return ErrUnsatCoreLearnerOwner
	}
	if p.closed {
		return nil
	}
	p.closed = true
	err := p.farkas.Close()
	p.farkas = nil
	p.learner = nil
	return err
}
