package smtcore

// Source: src/opt/opt_solver.cpp objective registration and arithmetic-theory
// dispatch; src/smt/theory_opt.h.

import (
	"strconv"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
)

type smtArithmeticObjectiveTheory interface {
	AddObjective(ast.ASTHandle) (SMTTheoryVar, error)
	ObjectiveValue(SMTTheoryVar) (SMTArithmeticObjectiveValue, error)
	MaximizeObjective(SMTTheoryVar) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error)
	MakeObjectiveLowerBound(*tactic.FilterModelConverter, SMTTheoryVar, simplex.Infinitesimal) (ast.ASTHandle, error)
}

func (s *SMTSolver) arithmeticObjectiveTheory() (smtArithmeticObjectiveTheory, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	family, err := s.manager.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	// opt_solver::get_optimizer installs theory_mi_arith only when the
	// context has no arithmetic plugin. An existing incompatible plugin must
	// not be silently replaced, since it can already own internalized terms.
	if s.kernel.internalizer.theories[family] == nil {
		if err = s.installOptimizerArithmetic(family); err != nil {
			return nil, err
		}
	}
	theory, ok := s.kernel.internalizer.theories[family].(smtArithmeticObjectiveTheory)
	if !ok || theory == nil {
		return nil, ast.ErrASTDomain
	}
	return theory, nil
}

// Source context::register_plugin initializes, publishes, then pushes the new
// plugin to the current scope level. Optimizer discovery can occur after the
// initial check/setup, when the ordinary cold-registration guard is too strict.
// This concrete path is only for the live theory_mi_arith fallback.
func (s *SMTSolver) installOptimizerArithmetic(family ast.FamilyID) error {
	config := s.kernel.arithState
	if owner := s.kernel.copiedArithmeticOwner; owner != nil {
		config = owner.state
	}
	globals := s.config.Globals
	if globals == nil {
		globals = s.kernel.smtParams.Globals
	}
	if globals != nil {
		modeText, modeErr := globals.SourceGetValueName("smt.arith.solver")
		if modeErr != nil {
			return modeErr
		}
		mode, modeErr := strconv.ParseUint(modeText, 10, 32)
		if modeErr != nil {
			return modeErr
		}
		config.Mode = uint32(mode)
	}
	var theory *SMTLinearArithmeticTheory
	var err error
	if config.Mode == 5 { // AS_OPTINF: src/smt/smt_setup.cpp setup_mi_arith.
		theory, err = NewSMTInfiniteArithmeticTheory(family)
	} else {
		theory, err = NewSMTMixedLinearArithmeticTheory(family)
	}
	if err != nil {
		return err
	}
	if err = theory.ConfigureArithmetic(config); err != nil {
		return err
	}
	state := s.kernel.internalizer
	if err = theory.Initialize(state); err != nil {
		return err
	}
	state.theories[family] = theory
	state.theoryOrder = append(state.theoryOrder, theory)
	for range state.scopes {
		if err = theory.PushScope(); err != nil {
			return err
		}
	}
	return nil
}

// AddObjective registers an arithmetic objective with the concrete arithmetic
// theory selected by this solver. The returned index is stable until
// ResetObjectives. The solver retains the term independently of its caller.
func (s *SMTSolver) AddObjective(term ast.ASTHandle) (uint32, error) {
	theory, err := s.arithmeticObjectiveTheory()
	if err != nil {
		return 0, err
	}
	variable, err := theory.AddObjective(term)
	if err != nil {
		return 0, err
	}
	if variable == SMTNullTheoryVar {
		return 0, ast.ErrASTDomain
	}
	ref, err := s.manager.NewRef(term)
	if err != nil {
		return 0, err
	}
	index := uint32(len(s.objectives))
	entry := smtSolverObjective{term: ref, theory: theory, variable: variable, valid: true}
	entry.saved.Finite.Infinity.SetInt64(-1)
	s.objectives = append(s.objectives, entry)
	if s.optimization != nil {
		s.optimization.models = append(s.optimization.models, nil)
	}
	return index, nil
}

func (s *SMTSolver) objective(index uint32) (*smtSolverObjective, error) {
	if err := s.open(); err != nil {
		return nil, err
	}
	if uint64(index) >= uint64(len(s.objectives)) {
		return nil, arena.ErrBounds
	}
	return &s.objectives[index], nil
}

// ObjectiveValue returns the concrete theory's current exact assignment.
func (s *SMTSolver) ObjectiveValue(index uint32) (SMTArithmeticObjectiveValue, error) {
	objective, err := s.objective(index)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	return objective.theory.ObjectiveValue(objective.variable)
}

// MaximizeObjective moves the current arithmetic assignment to a local exact
// maximum and returns the source blocker used by the outer optimization loop.
// The blocker is borrowed and must be rooted before another manager operation.
func (s *SMTSolver) MaximizeObjective(index uint32) (SMTArithmeticObjectiveValue, ast.ASTHandle, bool, error) {
	objective, err := s.objective(index)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, ast.ASTHandle{}, false, err
	}
	return objective.theory.MaximizeObjective(objective.variable)
}

// MakeObjectiveLowerBound creates the concrete theory atom that commits an
// optimized lower bound for a subsequent lexicographic objective.
func (s *SMTSolver) MakeObjectiveLowerBound(index uint32, filter *tactic.FilterModelConverter, value simplex.Infinitesimal) (ast.ASTHandle, error) {
	objective, err := s.objective(index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return objective.theory.MakeObjectiveLowerBound(filter, objective.variable, value)
}

// EnableObjectiveConflictRecording asks an infinite arithmetic objective
// theory to derive a tighter bound from conflicts involving bound.
func (s *SMTSolver) EnableObjectiveConflictRecording(index uint32, bound ast.ASTHandle) error {
	objective, err := s.objective(index)
	if err != nil {
		return err
	}
	recorder, ok := objective.theory.(interface{ EnableRecordConflict(ast.ASTHandle) error })
	if !ok {
		return ast.ErrASTDomain
	}
	return recorder.EnableRecordConflict(bound)
}

// ObjectiveSupportsConflictRecording is true only for source theory_inf_arith.
func (s *SMTSolver) ObjectiveSupportsConflictRecording(index uint32) (bool, error) {
	objective, err := s.objective(index)
	if err != nil {
		return false, err
	}
	theory, ok := objective.theory.(*SMTLinearArithmeticTheory)
	return ok && theory.infinite, nil
}

// ObjectiveConflictMinimize returns the best upper bound recorded for the
// currently watched arithmetic objective conflict.
func (s *SMTSolver) ObjectiveConflictMinimize(index uint32) (simplex.Infinitesimal, bool, error) {
	objective, err := s.objective(index)
	if err != nil {
		return simplex.Infinitesimal{}, false, err
	}
	recorder, ok := objective.theory.(interface {
		ConflictMinimize() (simplex.Infinitesimal, bool)
	})
	if !ok {
		return simplex.Infinitesimal{}, false, ast.ErrASTDomain
	}
	value, finite := recorder.ConflictMinimize()
	return value, finite, nil
}

// ResetObjectives releases every objective term retained by this solver.
func (s *SMTSolver) ResetObjectives() error {
	if err := s.open(); err != nil {
		return err
	}
	return s.clearObjectives()
}

func (s *SMTSolver) clearObjectives() error {
	for len(s.objectives) != 0 {
		last := len(s.objectives) - 1
		if s.objectives[last].term != (ast.ASTRef{}) {
			if err := s.manager.DelRef(&s.objectives[last].term); err != nil {
				return err
			}
		}
		s.objectives = s.objectives[:last]
	}
	return nil
}
