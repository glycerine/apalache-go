package opt

// Source: src/opt/pb_sls.cpp set_model and opt_sls_solver.h pbsls_opt.

import (
	"errors"
	"fmt"

	"github.com/glycerine/g3/pkg/model"
)

func (e *maxResEngine) newSLSCandidateSearch(result *ArithmeticResult) (*maxResSLSCandidateSearch, error) {
	var err error
	core := result.Model
	if core == nil {
		return nil, errMaxResState
	}
	seed := core.DerivedModel()
	ownedSeed := false
	if seed == nil {
		seed, err = model.NewModel(e.solver.manager)
		if err == nil {
			err = seed.Core().CopyConstantInterpretations(core)
		}
		if err == nil {
			err = seed.Core().CopyFunctionInterpretations(core)
		}
		ownedSeed = true
	}
	config := e.solver.config.ModelEvaluator
	var search *PBSLS
	if err == nil {
		search, err = NewPBSLS(e.solver.manager, PBSLSConfig{
			Context: config.Context, Globals: config.Globals, Model: config,
		})
	}
	if err == nil {
		err = search.SetModel(seed)
	}
	if ownedSeed && seed != nil {
		err = errors.Join(err, seed.Close())
	}
	if err != nil {
		if search != nil {
			err = errors.Join(err, search.Close())
		}
		return nil, fmt.Errorf("maxres SLS seed: %w", err)
	}
	return &maxResSLSCandidateSearch{engine: e, search: search}, nil
}
