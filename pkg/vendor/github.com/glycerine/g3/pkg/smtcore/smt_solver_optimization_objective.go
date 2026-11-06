package smtcore

// Source: src/opt/opt_solver.cpp:226-286,342-363.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
)

func cloneOptimizationValue(v SMTArithmeticObjectiveValue) SMTArithmeticObjectiveValue {
	v.Finite = v.Finite.Clone()
	return v
}

func equalOptimizationValues(a, b SMTArithmeticObjectiveValue) bool {
	left, right := a.Finite.Clone(), b.Finite.Clone()
	if a.Unbounded {
		left.Infinity.SetInt64(1)
	}
	if b.Unbounded {
		right.Infinity.SetInt64(1)
	}
	return left.Infinity.Cmp(&right.Infinity) == 0 && left.Main.Cmp(&right.Main) == 0 && left.Epsilon.Cmp(&right.Epsilon) == 0
}

func (s *SMTSolver) saveOptimizationModel(index uint32) error {
	md, err := s.AcquireModel()
	if err != nil {
		return err
	}
	old := s.optimization.models[index]
	if old != nil {
		if err = old.DecRef(); err != nil {
			if md != nil {
				err = errors.Join(err, md.DecRef())
			}
			return err
		}
	}
	s.optimization.models[index] = md
	return nil
}

func (s *SMTSolver) optimizationObjectiveGE(index uint32, filter *tactic.FilterModelConverter, value SMTArithmeticObjectiveValue) (ast.ASTHandle, error) {
	if value.Unbounded || value.Finite.Infinity.Sign() > 0 {
		return s.manager.MkFalse()
	}
	if value.Finite.Infinity.Sign() < 0 {
		return s.manager.MkTrue()
	}
	objective, err := s.objective(index)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if value.Finite.Epsilon.Sign() != 0 {
		switch objective.theory.(type) {
		case *SMTRealDiffTheory, *SMTDenseDiffTheory:
			// opt_solver::mk_ge supports these concrete difference families
			// only at rational thresholds. Its fallback deliberately imposes
			// no constraint; do not call their lower-bound builders here.
			enabled, err := diagnostic.ProcessVerboseEnabled(0)
			if err != nil {
				return ast.ASTHandle{}, err
			}
			if enabled {
				stream, err := diagnostic.BorrowProcessVerboseStream()
				if err != nil {
					return ast.ASTHandle{}, err
				}
				_ = stream.AppendString("WARNING: unhandled theory " + fmt.Sprintf("%T", objective.theory) + "\n")
			}
			return s.manager.MkTrue()
		}
	}
	return s.MakeObjectiveLowerBound(index, filter, value.Finite)
}

func (s *SMTSolver) decrementOptimizationValue(index uint32, filter *tactic.FilterModelConverter, value *SMTArithmeticObjectiveValue) (status lbool.LBool, resultErr error) {
	if err := s.PushCore(); err != nil {
		return lbool.Undef, err
	}
	pushed := true
	defer func() {
		if pushed {
			resultErr = errors.Join(resultErr, s.PopCore(1))
		}
	}()
	bound, err := s.optimizationObjectiveGE(index, filter, *value)
	if err != nil {
		return lbool.Undef, err
	}
	root, err := s.manager.NewRef(bound)
	if err != nil {
		return lbool.Undef, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.manager.DelRef(&root)) }()
	if err = s.Assert(bound); err != nil {
		return lbool.Undef, err
	}
	// Source bypasses opt_solver::check_sat_core here: no dump, trace or
	// first-check transition, but quantifier uncertainty is still adjusted.
	status, err = s.kernel.CheckAssumptions(nil)
	if err != nil {
		return lbool.Undef, err
	}
	status = s.adjustOptimizationResult(status)
	if status == lbool.True {
		if s.kernel.GeneratedModel() == nil && s.produceModels {
			if err = s.kernel.buildGeneratedModel(); err != nil {
				return lbool.Undef, err
			}
		}
		if err = s.saveOptimizationModel(index); err != nil {
			return lbool.Undef, err
		}
	}
	if err = s.PopCore(1); err != nil {
		return lbool.Undef, err
	}
	pushed = false
	if status != lbool.True {
		term, err := s.manager.RefGet(s.objectives[index].term)
		if err != nil {
			return status, err
		}
		sort, err := s.manager.GetSort(term)
		if err != nil {
			return status, err
		}
		kind, err := s.manager.GetDeclKind(sort)
		if err != nil {
			return status, err
		}
		if kind == 0 {
			value.Finite.Epsilon.Sub(&value.Finite.Epsilon, big.NewRat(1, 1))
		} else {
			value.Finite.Main.Sub(&value.Finite.Main, big.NewRat(1, 1))
		}
		s.objectives[index].valid = false
	}
	return status, nil
}

// MaximizeOptimizationObjective implements the opt_solver wrapper around the
// raw theory maximizer. Its returned blocker reference belongs to the caller.
func (s *SMTSolver) MaximizeOptimizationObjective(index uint32, filter *tactic.FilterModelConverter) (value SMTArithmeticObjectiveValue, blocker ast.ASTRef, resultErr error) {
	objective, err := s.objective(index)
	if err != nil {
		return value, blocker, err
	}
	if s.optimization == nil || filter == nil || !s.produceModels {
		return value, blocker, ast.ErrASTDomain
	}
	raw, condition, shared, err := objective.theory.MaximizeObjective(objective.variable)
	if err != nil {
		return value, blocker, err
	}
	root, err := s.manager.NewRef(condition)
	if err != nil {
		return value, blocker, err
	}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, s.manager.DelRef(&root))
		}
	}()
	value = cloneOptimizationValue(raw)
	md, err := s.AcquireModel()
	if err != nil {
		return value, blocker, err
	}
	if md != nil {
		if err = md.DecRef(); err != nil {
			return value, blocker, err
		}
	}
	objective.valid = true
	if s.optimization.models[index] == nil {
		if err = s.saveOptimizationModel(index); err != nil {
			return value, blocker, err
		}
	}
	updated, err := s.UpdateModel(shared)
	if err != nil {
		return value, blocker, err
	}
	repair := !updated
	if updated && shared {
		current, err := s.ObjectiveValue(index)
		if err != nil {
			return value, blocker, err
		}
		repair = !equalOptimizationValues(value, current)
	}
	if repair {
		if _, err = s.decrementOptimizationValue(index, filter, &value); err != nil {
			return value, blocker, err
		}
	} else if err = s.saveOptimizationModel(index); err != nil {
		return value, blocker, err
	}
	s.objectives[index].saved = cloneOptimizationValue(value)
	return value, root, nil
}

func (s *SMTSolver) OptimizationObjectiveModelValid(index uint32) (bool, error) {
	objective, err := s.objective(index)
	if err != nil {
		return false, err
	}
	return objective.valid, nil
}

func (s *SMTSolver) SavedObjectiveValue(index uint32) (SMTArithmeticObjectiveValue, error) {
	objective, err := s.objective(index)
	if err != nil {
		return SMTArithmeticObjectiveValue{}, err
	}
	return cloneOptimizationValue(objective.saved), nil
}

// AcquireSavedObjectiveModel returns an independently owned reference to the
// model snapshot saved when this objective was last maximized. Box optimization
// keeps one best model per objective while continuing the joint search.
func (s *SMTSolver) AcquireSavedObjectiveModel(index uint32) (*model.ModelCore, error) {
	if _, err := s.objective(index); err != nil {
		return nil, err
	}
	if s.optimization == nil || uint64(index) >= uint64(len(s.optimization.models)) {
		return nil, ast.ErrASTDomain
	}
	md := s.optimization.models[index]
	if md == nil {
		return nil, nil
	}
	if err := md.IncRef(); err != nil {
		return nil, err
	}
	return md, nil
}
