package api

// Source: generated api_commands.cpp and src/api/api_interp.cpp.

import (
	"bytes"
	"errors"
	"math"
	"os"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/interp"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/lbool"
)

var replayInterpolationProfileText string
var replayInterpolationErrorText string

func replayInterpolationInputs(r *Z3Replayer, owner *replayContextOwner, countPos, constraintsPos, parentsPos, theoryCountPos, theoryPos uint32) ([]ast.ASTHandle, []int, []ast.ASTHandle, error) {
	constraints, err := replayASTArray(r, owner, countPos, constraintsPos)
	if err != nil {
		return nil, nil, nil, err
	}
	parentValues, err := r.UintArray(parentsPos)
	if err != nil {
		return nil, nil, nil, err
	}
	var parents []int
	if len(parentValues) != 0 {
		if len(parentValues) < len(constraints) {
			return nil, nil, nil, ErrZ3ReplayReference
		}
		parents = make([]int, len(constraints))
		for i := range parents {
			if parentValues[i] == math.MaxUint32 {
				parents[i] = interp.ScopeTop
			} else {
				parents[i] = int(parentValues[i])
			}
		}
	}
	theory, err := replayASTArray(r, owner, theoryCountPos, theoryPos)
	return constraints, parents, theory, err
}

func replayInterpolationServices(owner *replayContextOwner) (*InterpolationContext, error) {
	config, err := owner.context.command.SolverTranslationConfig()
	if err != nil {
		return nil, err
	}
	manager, err := interp.NewManagerWithParams(owner.results.manager, config.Params, config.Globals)
	if err != nil {
		return nil, err
	}
	factory, err := smtcore.NewSMTSolverFactory(config)
	if err != nil {
		return nil, errors.Join(err, manager.Close())
	}
	return &InterpolationContext{interpolationState: &interpolationState{manager: owner.results.manager, terms: manager, factory: factory}}, nil
}

func closeReplayInterpolationServices(c *InterpolationContext) error {
	return errors.Join(c.factory.Close(), c.terms.Close())
}

func replayCheckInterpolant(r *Z3Replayer) (resultErr error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	constraints, parents, theory, err := replayInterpolationInputs(r, owner, 1, 2, 3, 6, 7)
	if err != nil {
		return err
	}
	ids, err := r.ObjectArray(4)
	if err != nil {
		return err
	}
	if len(constraints) == 0 || len(ids) < len(constraints)-1 {
		return ErrZ3ReplayReference
	}
	interpolants := make([]ast.ASTHandle, len(constraints)-1)
	for i := range interpolants {
		value, e := r.astByID(owner, ids[i])
		if e != nil {
			return e
		}
		interpolants[i] = value.handle
	}
	if _, err = r.String(5); err != nil {
		return err
	}
	context, err := replayInterpolationServices(owner)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeReplayInterpolationServices(context)) }()
	valid, message, err := context.CheckInterpolants(constraints, parents, interpolants, theory)
	if err != nil {
		return err
	}
	if valid {
		return r.setStringNull(5)
	}
	replayInterpolationErrorText += message
	return r.SetString(5, replayInterpolationErrorText)
}

func replayWriteInterpolationProblem(r *Z3Replayer) (resultErr error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	constraints, parents, theory, err := replayInterpolationInputs(r, owner, 1, 2, 3, 5, 6)
	if err != nil {
		return err
	}
	path, err := r.String(4)
	if err != nil {
		return err
	}
	context, err := replayInterpolationServices(owner)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, closeReplayInterpolationServices(context)) }()
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	return context.WriteInterpolationProblem(file, constraints, parents, theory)
}

func replayReadInterpolationProblem(r *Z3Replayer) (resultErr error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if _, err = r.Uint(1); err != nil {
		return err
	}
	if _, err = r.ObjectArray(2); err != nil {
		return err
	}
	parentSlots, err := r.ObjectArray(3)
	if err != nil {
		return err
	}
	path, err := r.String(4)
	if err != nil {
		return err
	}
	if _, err = r.String(5); err != nil {
		return err
	}
	if _, err = r.Uint(6); err != nil {
		return err
	}
	if _, err = r.ObjectArray(7); err != nil {
		return err
	}
	context, err := replayInterpolationServices(owner)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(context.retained) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, owner.results.manager.DelRef(&context.retained[i]))
		}
		resultErr = errors.Join(resultErr, closeReplayInterpolationServices(context))
	}()
	file, err := os.Open(path)
	if err != nil {
		return r.SetString(5, "SMTLIB parse error: "+err.Error())
	}
	problem, err := context.ReadInterpolationProblem(file, len(parentSlots) != 0)
	closeErr := file.Close()
	if err != nil {
		return errors.Join(r.SetString(5, "SMTLIB parse error: "+err.Error()), closeErr)
	}
	if closeErr != nil {
		return closeErr
	}
	if err = r.SetUint64(1, uint64(len(problem.Constraints))); err != nil {
		return err
	}
	if err = r.SetUint64(6, uint64(len(problem.Theory))); err != nil {
		return err
	}
	previousResult := r.result
	defer func() { r.result = previousResult }()
	store := func(position uint32, values []ast.ASTHandle) error {
		argument, e := r.value(position, replayObjectArray)
		if e != nil {
			return e
		}
		ids := make([]uint64, len(values))
		for i, value := range values {
			if e = r.storeAST(owner, value); e != nil {
				return e
			}
			ids[i] = r.result
		}
		r.objectArrays[argument.u] = ids
		return nil
	}
	if err = store(2, problem.Constraints); err != nil {
		return err
	}
	if err = store(7, problem.Theory); err != nil {
		return err
	}
	if len(parentSlots) != 0 {
		argument, e := r.value(3, replayObjectArray)
		if e != nil {
			return e
		}
		parents := make([]uint64, len(problem.Parents))
		for i, parent := range problem.Parents {
			parents[i] = uint64(parent)
		}
		r.objectArrays[argument.u] = parents
	}
	return nil
}

func replayInterpolationProfile(r *Z3Replayer) error {
	if _, err := r.Object(0); err != nil {
		return err
	}
	var output bytes.Buffer
	if err := interp.ProcessProfiler().Print(&output); err != nil {
		return err
	}
	replayInterpolationProfileText = output.String()
	return nil
}

func replayGetInterpolant(r *Z3Replayer) (resultErr error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	proof, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	pattern, err := r.astObject(owner, 2)
	if err != nil {
		return err
	}
	if _, err = r.Object(3); err != nil {
		return err
	}
	m := owner.results.manager
	config, err := owner.context.command.SolverTranslationConfig()
	if err != nil {
		return err
	}
	manager, err := interp.NewManagerWithParams(m, config.Params, config.Globals)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.Close()) }()
	factory, err := smtcore.NewSMTSolverFactory(config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, factory.Close()) }()
	run := &interp.RunConfig{Profiler: interp.ProcessProfiler(), SolverFactory: factory, EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
		return model.NewModelEvaluator(md, config.ModelEvaluator)
	}}
	constraints, err := interp.CollectConjuncts(manager, pattern.handle)
	if err != nil {
		return err
	}
	refs, err := interp.TreeInterpolants(manager, proof.handle, constraints, pattern.handle, run)
	if err != nil {
		return err
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	vector, err := owner.context.NewASTVector()
	if err != nil {
		return err
	}
	for _, ref := range refs {
		var value ast.ASTHandle
		value, err = m.RefGet(ref)
		if err == nil {
			err = vector.Push(value)
		}
		if err != nil {
			return errors.Join(err, vector.Close())
		}
	}
	return replayStoreVector(r, owner, vector)
}

func replayComputeInterpolant(r *Z3Replayer) (resultErr error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	pattern, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	parameterID, err := r.Object(2)
	if err != nil {
		return err
	}
	timeout, rlimit, ctrlC := uint32(math.MaxUint32), uint32(0), false
	if parameterID != 0 {
		parameters, e := replayParametersAt(r, owner, 2)
		if e != nil {
			return e
		}
		timeout, err = owner.context.contextTimeout()
		if err != nil {
			return err
		}
		text, _, e := owner.context.command.GetOption("rlimit")
		if e != nil {
			return e
		}
		value, e := strconv.ParseUint(text, 10, 32)
		if e != nil {
			return e
		}
		rlimit = uint32(value)
		timeoutKey, rlimitKey, ctrlCKey := "timeout", "rlimit", "ctrl_c"
		timeout, err = parameters.value.params.GetUIntName(&parameters.value.value, &timeoutKey, timeout)
		if err == nil {
			rlimit, err = parameters.value.params.GetUIntName(&parameters.value.value, &rlimitKey, rlimit)
		}
		if err == nil {
			ctrlC, err = parameters.value.params.GetBoolName(&parameters.value.value, &ctrlCKey, false)
		}
		if err != nil {
			return err
		}
	}
	if _, err = r.Object(3); err != nil {
		return err
	}
	if _, err = r.Object(4); err != nil {
		return err
	}
	m := owner.results.manager
	proofScope, err := ast.NewScopedProof(m)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, proofScope.Close()) }()
	config, err := owner.context.command.SolverTranslationConfig()
	if err != nil {
		return err
	}
	manager, err := interp.NewManagerWithParams(m, config.Params, config.Globals)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, manager.Close()) }()
	factory, err := smtcore.NewSMTSolverFactory(config)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, factory.Close()) }()
	target, err := factory.New(m, nil, true, true, true, "")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, target.Close()) }()
	handler := newSMTContextCancelHandler(m.ResourceLimit())
	interruptScope, err := newSMTContextInterruptScope(owner.context, handler)
	if err != nil {
		handler.close()
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, interruptScope.Close())
		handler.close()
	}()
	interrupt, err := control.NewScopedCtrlC(handler, false, ctrlC)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, interrupt.Close()) }()
	timer, err := control.NewScopedTimer(timeout, handler)
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, timer.Close()) }()
	if err = m.ResourceLimit().Push(rlimit); err != nil {
		return err
	}
	defer m.ResourceLimit().Pop()
	run := &interp.RunConfig{Profiler: interp.ProcessProfiler(), SolverFactory: factory, EvaluatorFactory: func(md *model.ModelCore) (solver.ConsequenceEvaluator, error) {
		return model.NewModelEvaluator(md, config.ModelEvaluator)
	}}
	status, _, refs, md, err := interp.SolveTree(manager, target, pattern.handle, true, run)
	if md != nil {
		defer func() { resultErr = errors.Join(resultErr, md.DecRef()) }()
	}
	defer func() {
		for i := len(refs) - 1; i >= 0; i-- {
			resultErr = errors.Join(resultErr, m.DelRef(&refs[i]))
		}
	}()
	if err != nil {
		return err
	}
	if err = r.SetObject(3, 0); err != nil {
		return err
	}
	if err = r.SetObject(4, 0); err != nil {
		return err
	}
	previousResult := r.result
	defer func() { r.result = previousResult }()
	if status == lbool.False {
		vector, err := owner.context.NewASTVector()
		if err != nil {
			return err
		}
		for _, ref := range refs {
			value, e := m.RefGet(ref)
			if e == nil {
				e = vector.Push(value)
			}
			if e != nil {
				return errors.Join(e, vector.Close())
			}
		}
		if err = replayStoreVector(r, owner, vector); err != nil {
			return err
		}
		return r.SetObject(3, r.result)
	}
	if md != nil {
		wrapped, err := newAPIModel(owner.context.smtContextState, md, false)
		if err != nil {
			return err
		}
		if err = replayStoreModel(r, owner, wrapped); err != nil {
			return err
		}
		return r.SetObject(4, r.result)
	}
	return nil
}

func replayMkInterpolant(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	result, err := owner.context.Interpolant(value.handle)
	if err != nil {
		return err
	}
	return r.storeAST(owner, result)
}

func replayMkInterpolationContext(r *Z3Replayer) (err error) {
	id, err := r.Object(0)
	if err != nil {
		return err
	}
	var config *Config
	if id == 0 {
		config, err = MkConfig()
		if err != nil {
			return err
		}
		defer func() { err = errors.Join(err, config.Close()) }()
	} else {
		config = r.configs[id]
		if config == nil {
			return ErrZ3ReplayReference
		}
	}
	if err = SetParamValue(config, "proof", "true"); err != nil {
		return err
	}
	if err = SetParamValue(config, "model", "true"); err != nil {
		return err
	}
	return replayCreateContextWithConfig(r, config, false)
}
