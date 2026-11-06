package api

// Source: src/api/api_solver.cpp and generated api_commands.cpp, solver replay.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
)

type replaySolver struct {
	object        apiObject
	owner         *replayContextOwner
	value         *APISimpleSolver
	kind          uint8
	logic         string
	params        *ParameterSet
	retiredParams []*ParameterSet
	tactic        *APITactic
}

func (s *replaySolver) apiDestroy() error {
	if err := s.value.Close(); err != nil {
		return err
	}
	if s.params != nil {
		if err := s.params.Close(); err != nil {
			return err
		}
		s.params = nil
	}
	for len(s.retiredParams) != 0 {
		i := len(s.retiredParams) - 1
		if err := s.retiredParams[i].Close(); err != nil {
			return err
		}
		s.retiredParams = s.retiredParams[:i]
	}
	if s.tactic != nil {
		if err := s.tactic.Close(); err != nil {
			return err
		}
		s.tactic = nil
	}
	return nil
}

func RegisterZ3GeneratedSolverCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{408, "Z3_mk_solver", replayMkSolver}, {409, "Z3_mk_simple_solver", replayMkSimpleSolver}, {410, "Z3_mk_solver_for_logic", replayMkSolverForLogic},
		{411, "Z3_mk_solver_from_tactic", replayMkSolverFromTactic}, {412, "Z3_solver_translate", replaySolverTranslate}, {413, "Z3_solver_get_help", replaySolverHelp},
		{414, "Z3_solver_get_param_descrs", replaySolverDescriptions}, {415, "Z3_solver_set_params", replaySolverSetParams}, {416, "Z3_solver_inc_ref", replaySolverIncRef}, {417, "Z3_solver_dec_ref", replaySolverDecRef},
		{418, "Z3_solver_push", replaySolverPush}, {419, "Z3_solver_pop", replaySolverPop}, {420, "Z3_solver_reset", replaySolverReset}, {421, "Z3_solver_get_num_scopes", replaySolverScopes},
		{422, "Z3_solver_assert", replaySolverAssert}, {423, "Z3_solver_assert_and_track", replaySolverAssertTracked},
		{424, "Z3_solver_get_assertions", func(r *Z3Replayer) error { return replaySolverVector(r, false) }},
		{432, "Z3_solver_get_proof", replaySolverProof},
		{433, "Z3_solver_get_unsat_core", func(r *Z3Replayer) error { return replaySolverVector(r, true) }},
		{429, "Z3_get_implied_equalities", replaySolverImpliedEqualities},
		{430, "Z3_solver_get_consequences", replaySolverConsequences},
		{425, "Z3_solver_from_file", replaySolverFromFile}, {426, "Z3_solver_from_string", replaySolverFromString}, {427, "Z3_solver_check", replaySolverCheck},
		{428, "Z3_solver_check_assumptions", replaySolverCheckAssumptions}, {434, "Z3_solver_get_reason_unknown", replaySolverReasonUnknown},
		{436, "Z3_solver_to_string", replaySolverString},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return registerZ3GeneratedStatistics(r)
}
func replayMkSolver(r *Z3Replayer) error           { return replayCreateSolver(r, 0) }
func replayMkSimpleSolver(r *Z3Replayer) error     { return replayCreateSolver(r, 1) }
func replayMkSolverForLogic(r *Z3Replayer) error   { return replayCreateSolver(r, 2) }
func replayMkSolverFromTactic(r *Z3Replayer) error { return replayCreateSolver(r, 3) }
func replayCreateSolver(r *Z3Replayer, kind uint8) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	var value *APISimpleSolver
	var logic string
	var tacticValue *APITactic
	switch kind {
	case 0:
		value, err = owner.context.NewSolver()
	case 1:
		value, err = owner.context.NewSimpleSolver()
	case 2:
		name, e := r.value(1, replaySymbol)
		if e != nil {
			return e
		}
		if name.numericSymbol || name.nullSymbol {
			return ErrZ3ReplayReference
		}
		logic = name.s
		value, err = owner.context.NewSolverForLogic(logic)
	case 3:
		tactic, e := replayTacticAt(r, owner, 1)
		if e != nil {
			return e
		}
		tacticValue, err = tactic.value.Retain()
		if err != nil {
			return err
		}
		value, err = owner.context.NewSolverFromTactic(tacticValue)
		if err != nil {
			return errors.Join(err, tacticValue.Close())
		}
	default:
		return ErrZ3ReplayState
	}
	if err != nil {
		return err
	}
	return replayStoreSolver(r, owner, value, kind, logic, tacticValue, nil)
}

func replayStoreSolver(r *Z3Replayer, owner *replayContextOwner, value *APISimpleSolver, kind uint8, logic string, tacticValue *APITactic, parameters *ParameterSet) error {
	closeInputs := func(cause error) error {
		cause = errors.Join(cause, value.Close())
		if tacticValue != nil {
			cause = errors.Join(cause, tacticValue.Close())
		}
		if parameters != nil {
			cause = errors.Join(cause, parameters.Close())
		}
		return cause
	}
	if r.nextObjectID == ^uint64(0) {
		return closeInputs(ErrZ3ReplayState)
	}
	n := uint64(reflect.TypeOf(replaySolver{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return closeInputs(err)
	}
	s := &replaySolver{owner: owner, value: value, kind: kind, logic: logic, tactic: tacticValue, params: parameters}
	s.object = apiObject{context: owner.results, destructor: s, ownerBytes: n}
	s.object.self = &s.object
	id, err := owner.results.addObject(&s.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return closeInputs(err)
	}
	s.object.id = id
	if err = owner.results.saveObject(&s.object); err != nil {
		return err
	}
	if r.solvers == nil {
		r.solvers = make(map[uint64]*replaySolver)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.solvers[replayID] = s
	owner.solverIDs = append(owner.solverIDs, replayID)
	return r.StoreResult(replayID)
}
func replaySolverTranslate(r *Z3Replayer) error {
	source, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	target, err := replayContext(r, 2)
	if err != nil {
		return err
	}
	translated, err := source.value.Translate(target.context)
	if err != nil {
		return err
	}
	var parameters *ParameterSet
	if source.params != nil {
		globals, e := target.context.command.GlobalParameters()
		if e != nil {
			return errors.Join(e, translated.Close())
		}
		value, e := source.params.params.TranslateTo(&source.params.value, globals.Context)
		if e != nil {
			return errors.Join(e, translated.Close())
		}
		parameters = &ParameterSet{owner: target.context.smtContextState, params: globals.Context, value: value}
	}
	kind, logic := source.kind, source.logic
	var tacticValue *APITactic
	if source.tactic != nil {
		if target == source.owner {
			tacticValue, err = source.tactic.Retain()
		} else {
			tactical, ok := translated.solver.(*solver.TacticSolver)
			if !ok {
				err = ErrZ3ReplayState
			} else {
				var raw tactic.Tactic
				raw, err = tactical.TranslationTactic()
				if err == nil {
					tacticValue, err = newAPITactic(target.context.smtContextState, raw)
				}
			}
		}
		if err != nil {
			if parameters != nil {
				err = errors.Join(err, parameters.Close())
			}
			return errors.Join(err, translated.Close())
		}
	}
	return replayStoreSolver(r, target, translated, kind, logic, tacticValue, parameters)
}
func replayGetSolver(r *Z3Replayer) (*replaySolver, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	s := r.solvers[id]
	if s == nil || s.owner != owner || s.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return s, nil
}
func replaySolverSetParams(r *Z3Replayer) error {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	id, err := r.Object(2)
	if err != nil {
		return err
	}
	p := r.parameterSets[id]
	if p == nil || p.owner != s.owner || p.object.open() != nil {
		return ErrZ3ReplayReference
	}
	return s.setParameters(p.value)
}
func replaySolverIncRef(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	return s.object.incRef()
}
func replaySolverDecRef(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	return s.object.decRef()
}
func replaySolverPush(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	return s.value.Push()
}
func replaySolverPop(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	n, e := r.Uint(2)
	if e != nil {
		return e
	}
	return s.value.Pop(n)
}
func replaySolverScopes(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.ScopeLevel()
	return e
}
func replaySolverAssert(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	a, e := r.astObject(s.owner, 2)
	if e != nil {
		return e
	}
	return s.value.Assert(a.handle)
}
func replaySolverAssertTracked(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	a, e := r.astObject(s.owner, 2)
	if e != nil {
		return e
	}
	b, e := r.astObject(s.owner, 3)
	if e != nil {
		return e
	}
	return s.value.AssertTracked(a.handle, b.handle)
}
func replaySolverFromString(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	text, e := r.String(2)
	if e != nil {
		return e
	}
	return s.value.LoadSMT2String(text)
}
func replaySolverCheck(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.Check()
	return e
}
func replaySolverCheckAssumptions(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	args, e := replayASTArray(r, s.owner, 2, 3)
	if e != nil {
		return e
	}
	_, e = s.value.Check(args...)
	return e
}
func replaySolverReasonUnknown(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.ReasonUnknown()
	return e
}
func replaySolverString(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	_, e = s.value.String()
	return e
}

func replaySolverVector(r *Z3Replayer, core bool) (err error) {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	var refs []ast.ASTRef
	if core {
		refs, err = s.value.UnsatCore()
	} else {
		refs, err = s.value.Assertions()
	}
	if err != nil {
		return err
	}
	manager := s.owner.results.manager
	defer func() {
		for i := range refs {
			err = errors.Join(err, manager.DelRef(&refs[i]))
		}
	}()
	values := make([]ast.ASTHandle, len(refs))
	for i := range refs {
		values[i], err = manager.RefGet(refs[i])
		if err != nil {
			return err
		}
	}
	vector, err := s.owner.context.NewASTVectorValues(values...)
	if err != nil {
		return err
	}
	return replayStoreVector(r, s.owner, vector)
}

func replaySolverConsequences(r *Z3Replayer) (err error) {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	assumptions, err := replayVectorAt(r, s.owner, 2)
	if err != nil {
		return err
	}
	variables, err := replayVectorAt(r, s.owner, 3)
	if err != nil {
		return err
	}
	output, err := replayVectorAt(r, s.owner, 4)
	if err != nil {
		return err
	}
	av, err := assumptions.value.Values()
	if err != nil {
		return err
	}
	vv, err := variables.value.Values()
	if err != nil {
		return err
	}
	_, refs, err := s.value.Consequences(av, vv)
	if err != nil {
		return err
	}
	manager := s.owner.results.manager
	defer func() {
		for i := range refs {
			err = errors.Join(err, manager.DelRef(&refs[i]))
		}
	}()
	for i := range refs {
		value, e := manager.RefGet(refs[i])
		if e != nil {
			return e
		}
		if e = output.value.Push(value); e != nil {
			return e
		}
	}
	return nil
}

func replaySolverImpliedEqualities(r *Z3Replayer) error {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	terms, err := replayASTArray(r, s.owner, 2, 3)
	if err != nil {
		return err
	}
	output, err := r.UintArray(4)
	if err != nil {
		return err
	}
	if len(output) < len(terms) {
		return ErrZ3ReplayReference
	}
	_, classes, err := s.value.ImpliedEqualities(terms)
	if err != nil {
		return err
	}
	copy(output, classes)
	return nil
}

func replaySolverFromFile(r *Z3Replayer) error {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	path, err := r.String(2)
	if err != nil {
		return err
	}
	return s.value.LoadFile(path)
}
func replaySolverProof(r *Z3Replayer) (err error) {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	ref, err := s.value.Proof()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.owner.results.manager.DelRef(&ref)) }()
	value, err := s.owner.results.manager.RefGet(ref)
	if err != nil {
		return err
	}
	return r.storeAST(s.owner, value)
}

func replaySolverHelp(r *Z3Replayer) error {
	s, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	_, err = s.value.ParameterHelp()
	return err
}
