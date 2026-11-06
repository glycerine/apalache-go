package opt

// Source: src/opt/maxres.cpp maxres state and init_local.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
)

type maxResEngine struct {
	solver            *ArithmeticSolver
	objective         *arithmeticObjective
	index             int
	config            maxSMTConfig
	strategy          maxResStrategy
	owner             maxSMTASTOwner
	active            []ast.ASTHandle
	weights           map[ast.ASTHandle]*big.Rat
	proxies           map[ast.ASTHandle]ast.ASTHandle
	mus               *solver.MUS
	lower             big.Rat
	upper             big.Rat
	bestSet           bool
	correction        []ast.ASTHandle
	correctionModel   *model.ModelCore
	correctionSize    int
	currentCorrection []ast.ASTHandle
	stats             maxResStatistics
}

func newMaxResEngine(s *ArithmeticSolver, objective *arithmeticObjective, index int, config maxSMTConfig) (result *maxResEngine, resultErr error) {
	e := &maxResEngine{solver: s, objective: objective, index: index, config: config, strategy: maxResStrategyFromConfig(config), weights: make(map[ast.ASTHandle]*big.Rat), proxies: make(map[ast.ASTHandle]ast.ASTHandle)}
	e.lower.Set(&objective.softOffset)
	e.owner.manager = s.manager
	mus, err := solver.NewMUS(s.backend, solver.MUSConfig{})
	if err != nil {
		return nil, err
	}
	e.mus = mus
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, e.close())
			result = nil
		}
	}()
	terms, err := e.preprocessMutexes(objective)
	if err != nil {
		return nil, err
	}
	for i := range terms {
		if err = e.addSoft(terms[i].expression, &terms[i].weight); err != nil {
			return nil, fmt.Errorf("maxres mutex term %d: %w", i, err)
		}
	}
	return e, nil
}

func (e *maxResEngine) close() error {
	if e == nil {
		return nil
	}
	e.active, e.correction, e.currentCorrection, e.weights, e.proxies = nil, nil, nil, nil, nil
	var correctionErr error
	if e.correctionModel != nil {
		correctionErr = e.correctionModel.DecRef()
		e.correctionModel = nil
	}
	var musErr error
	if e.mus != nil {
		musErr = e.mus.Close()
		e.mus = nil
	}
	return errors.Join(musErr, correctionErr, e.owner.close())
}

func (e *maxResEngine) clearCorrectionModel() error {
	if e.correctionModel == nil {
		e.correctionSize = 0
		return nil
	}
	err := e.correctionModel.DecRef()
	if err == nil {
		e.correctionModel = nil
		e.correctionSize = 0
	}
	return err
}
