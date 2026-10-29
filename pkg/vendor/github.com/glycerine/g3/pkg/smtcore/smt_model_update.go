package smtcore

// Source: src/smt/smt_context.cpp context::update_model and mk_proto_model;
// retained caller src/opt/opt_solver.cpp maximize_objective.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/lbool"
)

// UpdateModel refreshes the model after a theory changes its assignment.
// With refinalize, exactly one native final-check round must return DONE.
// CONTINUE and GIVEUP preserve the previous published model; this operation
// does not run another search or assert that a local optimum is globally valid.
func (k *SMTKernel) UpdateModel(refinalize bool) (bool, error) {
	if err := k.open(); err != nil {
		return false, err
	}
	if !k.produceModel {
		return false, ast.ErrASTDomain
	}
	if refinalize {
		status, err := k.internalizer.FinalCheckTheories()
		if err != nil {
			return false, err
		}
		if status != SMTFinalCheckDone {
			if status == SMTFinalCheckGiveUp && len(k.internalizer.incompleteTheories) != 0 {
				k.lastFailure = SMTFailureTheory
			}
			return false, nil
		}
	}
	// A previously accepted MBQI candidate describes the old assignment.
	// Native update_model explicitly rebuilds through mk_proto_model rather
	// than reusing that earlier candidate.
	if manager := k.internalizer.quantifierManager; manager != nil && manager.acceptedModel != nil {
		if err := manager.acceptedModel.Close(); err != nil {
			return false, err
		}
		manager.acceptedModel = nil
	}
	generator, err := k.buildCandidateModelGenerator()
	if err != nil {
		return false, err
	}
	generated, err := k.finalizeGeneratedModel(generator)
	if err != nil {
		return false, err
	}
	if err = k.clearGeneratedModel(); err != nil {
		return false, errors.Join(err, generated.Core().DecRef())
	}
	k.generated = generated
	return true, nil
}

// UpdateModel synchronizes the kernel refresh with the owning solver result.
// Optimization callers invoke it only after a satisfiable, model-enabled
// check, passing the maximizer's hasShared flag as refinalize.
func (s *SMTSolver) UpdateModel(refinalize bool) (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	if s.result == nil || !s.produceModels {
		return false, ast.ErrASTDomain
	}
	status, err := s.result.Status()
	if err != nil {
		return false, err
	}
	if status != lbool.True {
		return false, ast.ErrASTDomain
	}
	updated, err := s.kernel.UpdateModel(refinalize)
	if err != nil || !updated {
		return false, err
	}
	if err = s.result.SetModel(s.kernel.GeneratedModel()); err != nil {
		return false, err
	}
	return true, nil
}
