package opt

// Source: src/opt/optsmt.cpp symba_opt, geometric_opt, update_lower and set_max.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/lbool"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// CheckBox computes the joint arithmetic box optimum and keeps one independent
// model snapshot per objective. The returned ObjectiveModels are owned by the
// result and follow objective registration order.
func (s *ArithmeticSolver) CheckBox() (result *ArithmeticResult, resultErr error) {
	if s == nil || s.backend == nil || len(s.objectives) == 0 {
		return nil, ast.ErrASTHandle
	}
	for _, objective := range s.objectives {
		if objective.soft {
			return nil, ast.ErrASTDomain
		}
	}
	result = &ArithmeticResult{Status: lbool.Undef, Values: make([]ObjectiveValue, len(s.objectives))}
	owner := maxSMTASTOwner{manager: s.manager}
	defer func() {
		resultErr = errors.Join(resultErr, owner.close())
		if resultErr == nil && result != nil && result.Status == lbool.True {
			var unknown bool
			unknown, resultErr = s.backend.OptimizationWasUnknown()
			if resultErr == nil && unknown {
				result.Status, result.Optimal = lbool.Undef, false
				result.ReasonUnknown, resultErr = s.backend.ReasonUnknown()
			}
		}
		if resultErr == nil && result != nil {
			resultErr = s.filterResultModel(result)
			for i, snapshot := range result.ObjectiveModels {
				if resultErr != nil || snapshot == nil {
					break
				}
				filtered := &ArithmeticResult{Model: snapshot}
				resultErr = s.filterResultModel(filtered)
				result.ObjectiveModels[i] = filtered.Model
			}
		}
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	if err := s.backend.Push(); err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, s.backend.Pop(1)) }()
	status, err := s.backend.CheckSat(nil)
	if err != nil {
		return nil, err
	}
	result.Status = status
	if status != lbool.True {
		if status == lbool.Undef {
			result.ReasonUnknown, err = s.backend.ReasonUnknown()
		}
		return result, err
	}
	if err = s.replaceModel(result); err != nil {
		return nil, err
	}
	result.InitialLabels = append([]usymbol.Symbol(nil), result.Labels...)
	lower := make([]simplex.Infinitesimal, len(s.objectives))
	lowerFormulas := make([]ast.ASTRef, len(s.objectives))
	for i := range lower {
		lower[i].Infinity.SetInt64(-1)
		truth, cause := s.manager.MkTrue()
		if cause != nil {
			return nil, cause
		}
		ref, cause := s.manager.NewRef(truth)
		if cause != nil {
			return nil, cause
		}
		owner.refs = append(owner.refs, ref)
		lowerFormulas[i] = ref
	}
	var deltaScopes uint32
	defer func() {
		if deltaScopes != 0 {
			resultErr = errors.Join(resultErr, s.backend.Pop(deltaScopes))
		}
	}()
	var delta big.Int
	delta.SetInt64(1)
	// geometric_opt initializes delta_index to zero. get_max_delta in the
	// source reports whether an integral objective advanced but does not assign
	// the index reference, so the jump bound remains attached to objective 0.
	deltaIndex := uint32(0)
	var steps, stepIncs uint32
	farkas := s.optsmtEngine == "farkas"
	if farkas {
		status, err = s.farkasBoxSearch(result, &owner, lower, lowerFormulas)
	} else if s.optsmtEngine == "symba" {
		status, err = s.symbaBoxSearch(&owner, lowerFormulas)
	}
	if err == nil && status == lbool.True && !farkas {
		status, err = s.geometricBoxSearch(result, &owner, lower, lowerFormulas, deltaIndex, &delta, &steps, &stepIncs, &deltaScopes)
	}
	if err != nil {
		return nil, err
	}
	if status == lbool.Undef {
		result.ReasonUnknown, err = s.backend.ReasonUnknown()
		result.Status = status
		return result, err
	}
	if status != lbool.False && !(farkas && status == lbool.True) {
		result.Status = status
		return result, nil
	}
	result.Status, result.Optimal = lbool.True, true
	if farkas {
		if len(result.ObjectiveModels) != len(s.objectives) {
			return nil, ast.ErrASTHandle
		}
	} else {
		result.ObjectiveModels = make([]*model.ModelCore, len(s.objectives))
		for i := range result.ObjectiveModels {
			result.ObjectiveModels[i], err = s.backend.AcquireSavedObjectiveModel(uint32(i))
			if err != nil {
				return nil, err
			}
			if result.ObjectiveModels[i] == nil {
				return nil, ast.ErrASTHandle
			}
			value, cause := s.backend.SavedObjectiveValue(uint32(i))
			if cause != nil {
				return nil, cause
			}
			result.Values[i].Arithmetic = s.orient(i, value)
		}
	}
	for i := range result.ObjectiveModels {
		if err = s.captureArithmeticModel(i, &result.Values[i], result.ObjectiveModels[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func readOptSMTEngine(m *ast.ASTManager, local *uparams.ParamsRef, config smtcore.SMTSolverConfig) (engine string, resultErr error) {
	globals := config.Globals
	if globals == nil {
		globals = config.ModelEvaluator.Globals
	}
	if globals == nil {
		globals = uparams.BorrowProcessGlobalParamsService()
	}
	if globals == nil || config.Params == nil {
		return "basic", nil
	}
	params, err := NewParams(config.Params, globals, local)
	if err != nil {
		return "", err
	}
	defer func() { resultErr = errors.Join(resultErr, params.Close()) }()
	value, err := params.OptSMTEngine()
	if err != nil {
		return "", err
	}
	return m.Symbols.String(value)
}

// symba_opt first asks whether any objective still reaches its current upper
// endpoint. optsmt::setup initializes those endpoints to +infinity, while the
// live optimizer context only updates lower endpoints before box execution;
// therefore every initial endpoint atom is false and the guarded probe is
// unsatisfiable. The source then asserts the disjunction of the initial true
// lower formulas and enters geometric_opt.
func (s *ArithmeticSolver) symbaBoxSearch(owner *maxSMTASTOwner, lowerFormulas []ast.ASTRef) (lbool.LBool, error) {
	tors := make([]ast.ASTHandle, len(s.objectives))
	for i := range tors {
		var err error
		tors[i], err = s.manager.MkFalse()
		if err != nil {
			return lbool.Undef, err
		}
	}
	upperDisjunction, err := s.manager.MkOrArray(tors)
	if err != nil {
		return lbool.Undef, err
	}
	boolean, err := s.manager.MkBoolSort()
	if err != nil {
		return lbool.Undef, err
	}
	probe, err := owner.hold(s.manager.MkFreshConst("b", boolean))
	if err != nil {
		return lbool.Undef, err
	}
	implication, err := owner.hold(s.manager.MkImplies(probe, upperDisjunction))
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.backend.Assert(implication); err != nil {
		return lbool.Undef, err
	}
	status, err := s.backend.CheckSat([]ast.ASTHandle{probe})
	if err != nil || status != lbool.False {
		return status, err
	}
	lower := make([]ast.ASTHandle, len(lowerFormulas))
	for i := range lowerFormulas {
		lower[i], err = s.manager.RefGet(lowerFormulas[i])
		if err != nil {
			return lbool.Undef, err
		}
	}
	bound, err := s.manager.MkOrArray(lower)
	if err != nil {
		return lbool.Undef, err
	}
	if err = s.backend.Assert(bound); err != nil {
		return lbool.Undef, err
	}
	return lbool.True, nil
}

// farkasBoxSearch ports optsmt::farkas_opt and update_upper. Each objective
// bound is internalized before the assumption checks begin, matching the
// source rule that all bound atoms must exist at base scope.
func (s *ArithmeticSolver) farkasBoxSearch(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef) (lbool.LBool, error) {
	result.ObjectiveModels = make([]*model.ModelCore, len(s.objectives))
	for _, objective := range s.objectives {
		supported, err := s.backend.ObjectiveSupportsConflictRecording(objective.backendIndex)
		if err != nil {
			return lbool.Undef, err
		}
		if !supported {
			return lbool.Undef, nil
		}
	}
	upper := make([]simplex.Infinitesimal, len(s.objectives))
	for i := range upper {
		upper[i].Infinity.SetInt64(1)
	}
	for {
		status, err := s.updateFarkasUpper(result, owner, lower, upper, lowerFormulas)
		if err != nil {
			return lbool.Undef, err
		}
		if status == lbool.True {
			continue
		}
		if status == lbool.Undef {
			return lbool.Undef, nil
		}
		for i := range upper {
			upper[i] = lower[i].Clone()
		}
		return lbool.True, nil
	}
}

func (s *ArithmeticSolver) updateFarkasUpper(result *ArithmeticResult, owner *maxSMTASTOwner, lower, upper []simplex.Infinitesimal, lowerFormulas []ast.ASTRef) (status lbool.LBool, resultErr error) {
	if len(lower) != len(s.objectives) || len(upper) != len(lower) || len(lowerFormulas) != len(lower) {
		return lbool.Undef, ast.ErrASTDomain
	}
	if err := s.backend.Push(); err != nil {
		return lbool.Undef, err
	}
	defer func() {
		resultErr = errors.Join(resultErr, s.backend.Pop(1))
		if resultErr != nil {
			status = lbool.Undef
		}
	}()
	mids := make([]simplex.Infinitesimal, len(lower))
	bounds := make([]ast.ASTHandle, len(lower))
	active := make([]bool, len(lower))
	for i := range lower {
		if lower[i].Cmp(&upper[i]) >= 0 {
			continue
		}
		mids[i] = upper[i].Clone()
		mids[i].Add(&lower[i])
		mids[i].Quo(big.NewRat(2, 1))
		bound, err := s.backend.MakeObjectiveLowerBound(s.objectives[i].backendIndex, s.filter, mids[i])
		if err != nil {
			return lbool.Undef, err
		}
		bounds[i], err = owner.hold(bound, nil)
		if err != nil {
			return lbool.Undef, err
		}
		active[i] = true
	}
	progress := false
	for i := range lower {
		if !active[i] || mids[i].Cmp(&lower[i]) < 0 || mids[i].Cmp(&upper[i]) > 0 {
			continue
		}
		if err := s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, bounds[i]); err != nil {
			return lbool.Undef, err
		}
		status, err := s.backend.CheckSat([]ast.ASTHandle{bounds[i]})
		if err != nil {
			err = errors.Join(err, s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}))
			return lbool.Undef, err
		}
		switch status {
		case lbool.True:
			lower[i] = mids[i].Clone()
			if err = s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}); err != nil {
				return lbool.Undef, err
			}
			if err = s.updateFarkasLower(result, owner, lower, lowerFormulas); err != nil {
				return lbool.Undef, err
			}
		case lbool.False:
			bound, finite, cause := s.backend.ObjectiveConflictMinimize(s.objectives[i].backendIndex)
			if cause != nil {
				return lbool.Undef, cause
			}
			if !finite {
				upper[i] = lower[i].Clone()
				return lbool.False, nil
			}
			if bound.Cmp(&upper[i]) < 0 {
				upper[i] = bound
			}
		case lbool.Undef:
			if err = s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}); err != nil {
				return lbool.Undef, err
			}
			return lbool.Undef, nil
		}
		if err = s.backend.EnableObjectiveConflictRecording(s.objectives[i].backendIndex, ast.ASTHandle{}); err != nil {
			return lbool.Undef, err
		}
		progress = true
	}
	if !progress {
		return lbool.False, nil
	}
	return lbool.True, nil
}

func (s *ArithmeticSolver) updateFarkasLower(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef) error {
	if err := s.replaceModel(result); err != nil {
		return err
	}
	blockers := make([]ast.ASTHandle, len(lower))
	for i := range lower {
		value, blocker, err := s.backend.MaximizeOptimizationObjective(s.objectives[i].backendIndex, s.filter)
		if err != nil {
			if blocker != (ast.ASTRef{}) {
				err = errors.Join(err, s.manager.DelRef(&blocker))
			}
			return err
		}
		candidate := value.Finite.Clone()
		if value.Unbounded {
			candidate.Infinity.SetInt64(1)
		}
		if candidate.Cmp(&lower[i]) >= 0 {
			lower[i] = candidate
			if err = owner.take(blocker); err != nil {
				return err
			}
			lowerFormulas[i] = owner.refs[len(owner.refs)-1]
			if candidate.Infinity.Sign() > 0 {
				falsity, cause := s.manager.MkFalse()
				if cause != nil {
					return cause
				}
				ref, cause := s.manager.NewRef(falsity)
				if cause != nil {
					return cause
				}
				owner.refs = append(owner.refs, ref)
				lowerFormulas[i] = ref
			}
			result.Values[i].Arithmetic = s.orient(i, value)
			modelSnapshot, cause := s.backend.AcquireSavedObjectiveModel(s.objectives[i].backendIndex)
			if cause != nil {
				return cause
			}
			if modelSnapshot == nil {
				return ast.ErrASTHandle
			}
			if old := result.ObjectiveModels[i]; old != nil {
				if cause = old.DecRef(); cause != nil {
					return errors.Join(cause, modelSnapshot.DecRef())
				}
			}
			result.ObjectiveModels[i] = modelSnapshot
		}
		blockers[i], err = s.manager.RefGet(lowerFormulas[i])
		if err != nil {
			return err
		}
	}
	disjunction, err := s.manager.MkOrArray(blockers)
	if err != nil {
		return err
	}
	return s.backend.Assert(disjunction)
}

func (s *ArithmeticSolver) geometricBoxSearch(result *ArithmeticResult, owner *maxSMTASTOwner, lower []simplex.Infinitesimal, lowerFormulas []ast.ASTRef, deltaIndex uint32, delta *big.Int, steps, stepIncs *uint32, deltaScopes *uint32) (lbool.LBool, error) {
	for {
		status, err := s.backend.CheckSat(nil)
		if err != nil {
			return lbool.Undef, err
		}
		if status != lbool.True {
			if status == lbool.False && delta.Cmp(big.NewInt(1)) > 0 {
				if *deltaScopes == 0 {
					return lbool.Undef, ast.ErrASTDomain
				}
				if err = s.backend.Pop(1); err != nil {
					return lbool.Undef, err
				}
				*deltaScopes--
				delta.SetInt64(1)
				*steps, *stepIncs = 0, 0
				continue
			}
			return status, nil
		}
		if err = s.replaceModel(result); err != nil {
			return lbool.Undef, err
		}
		previous := make([]simplex.Infinitesimal, len(lower))
		for i := range lower {
			previous[i] = lower[i].Clone()
		}
		blockers := make([]ast.ASTHandle, len(s.objectives))
		for i := range s.objectives {
			value, blocker, cause := s.backend.MaximizeOptimizationObjective(s.objectives[i].backendIndex, s.filter)
			if cause != nil {
				if blocker != (ast.ASTRef{}) {
					cause = errors.Join(cause, s.manager.DelRef(&blocker))
				}
				return lbool.Undef, cause
			}
			bound := value.Finite.Clone()
			if value.Unbounded {
				bound.Infinity.SetInt64(1)
			}
			if bound.Cmp(&lower[i]) >= 0 {
				lower[i] = bound
				if err = owner.take(blocker); err != nil {
					return lbool.Undef, err
				}
				lowerFormulas[i] = owner.refs[len(owner.refs)-1]
				if bound.Infinity.Sign() > 0 {
					falsity, cause := s.manager.MkFalse()
					if cause != nil {
						return lbool.Undef, cause
					}
					ref, cause := s.manager.NewRef(falsity)
					if cause != nil {
						return lbool.Undef, cause
					}
					owner.refs = append(owner.refs, ref)
					lowerFormulas[i] = ref
				}
			} else {
				if err = s.manager.DelRef(&blocker); err != nil {
					return lbool.Undef, err
				}
				formula, cause := s.manager.RefGet(lowerFormulas[i])
				if cause != nil {
					return lbool.Undef, cause
				}
				blockers[i] = formula
				continue
			}
			blockers[i], err = s.manager.RefGet(lowerFormulas[i])
			if err != nil {
				return lbool.Undef, err
			}
			result.Values[i].Arithmetic = s.orient(i, value)
		}
		increase := false
		for i := range lower {
			if !s.objectives[i].integer || lower[i].Infinity.Sign() != 0 {
				continue
			}
			var difference simplex.Infinitesimal
			difference.Set(&lower[i])
			difference.Sub(&previous[i])
			if difference.Sign() > 0 {
				increase = true
				break
			}
		}
		if !increase {
			delta.SetInt64(1)
		} else if *steps > *stepIncs {
			delta.Lsh(delta, 1)
			*stepIncs++
			*steps = 0
		} else {
			*steps++
		}
		if delta.Cmp(big.NewInt(1)) > 0 {
			if err = s.backend.Push(); err != nil {
				return lbool.Undef, err
			}
			*deltaScopes++
			if int(deltaIndex) >= len(lower) {
				return lbool.Undef, ast.ErrASTDomain
			}
			jump := lower[deltaIndex].Clone()
			jump.Main.Add(&jump.Main, new(big.Rat).SetInt(delta))
			bound, cause := s.backend.MakeObjectiveLowerBound(s.objectives[deltaIndex].backendIndex, s.filter, jump)
			if cause != nil {
				return lbool.Undef, cause
			}
			if err = s.backend.Assert(bound); err != nil {
				return lbool.Undef, err
			}
		}
		bound, err := s.manager.MkOrArray(blockers)
		if err != nil {
			return lbool.Undef, err
		}
		if err = s.backend.Assert(bound); err != nil {
			return lbool.Undef, err
		}
	}
}

func (o *maxSMTASTOwner) take(ref ast.ASTRef) error {
	if ref == (ast.ASTRef{}) {
		return ast.ErrASTHandle
	}
	o.refs = append(o.refs, ref)
	return nil
}
