package smtcore

// Source: src/opt/opt_solver.cpp:32-49,185-194,288-309.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type smtOptimizationSolverState struct {
	first          bool
	wasUnknown     bool
	model          *model.ModelCore
	models         []*model.ModelCore
	dumpBenchmarks bool
	parameters     uparams.ModuleParamsOwner
}

// NewOptimizationSMTSolver selects the concrete opt_solver result/model policy
// while sharing the existing kernel-backed solver implementation.
func NewOptimizationSMTSolver(m *ast.ASTManager, p *uparams.ParamsRef, logic string, config SMTSolverConfig) (*SMTSolver, error) {
	s, err := NewSMTSolver(m, p, logic, config)
	if err != nil {
		return nil, err
	}
	s.optimization = &smtOptimizationSolverState{first: true}
	owner := &s.optimization.parameters
	owner.Self, owner.Budget = owner, m.Budget()
	if err = owner.Bind(s.config.Params, s.kernel.paramGlobals); err != nil {
		return nil, errors.Join(err, s.Close())
	}
	if s.kernel.smtParams.state.CaseSplitStrategy == 1 { // CS_ACTIVITY_DELAY_NEW
		s.setOptimizationRelevancy(0)
	}
	return s, nil
}

// The C++ kernel borrows this fparams field. Keep its Go snapshots and the
// actual relevance service synchronized, without changing caller parameters.
func (s *SMTSolver) setOptimizationRelevancy(level uint32) {
	k := s.kernel
	k.smtParams.state.RelevancyLvl = level
	k.smtState.RelevancyLvl = level
	if !k.relevancy.enabled && level > 0 {
		k.repairRelevancy = true
	}
	k.relevancy.enabled = level > 0
	k.internalizer.relevancyLevel = level
}

func (s *SMTSolver) adjustOptimizationResult(status lbool.LBool) lbool.LBool {
	if status == lbool.Undef && s.kernel.lastFailure == SMTFailureQuantifiers {
		s.optimization.wasUnknown = true
		return lbool.True
	}
	return status
}

func (s *SMTSolver) OptimizationWasUnknown() (bool, error) {
	if err := s.open(); err != nil {
		return false, err
	}
	return s.optimization != nil && s.optimization.wasUnknown, nil
}

// current is an acquired reference. The returned model remains independently
// owned by the caller, including when it comes from the persistent cache.
func (s *SMTSolver) acquireOptimizationModel(current *model.ModelCore) (*model.ModelCore, error) {
	state := s.optimization
	if current == nil {
		if state.model != nil {
			if err := state.model.IncRef(); err != nil {
				return nil, err
			}
		}
		return state.model, nil
	}
	if current != state.model {
		if err := current.IncRef(); err != nil {
			return nil, errors.Join(err, current.DecRef())
		}
		if state.model != nil {
			if err := state.model.DecRef(); err != nil {
				return nil, errors.Join(err, current.DecRef(), current.DecRef())
			}
		}
		state.model = current
	}
	return current, nil
}
