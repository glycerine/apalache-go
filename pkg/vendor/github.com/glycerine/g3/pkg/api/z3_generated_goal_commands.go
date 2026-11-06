package api

// Source: generated api_commands.cpp and src/api/api_goal.cpp.

import (
	"errors"
	"reflect"
)

type replayGoal struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIGoal
}

func (g *replayGoal) apiDestroy() error { return g.value.Close() }

func registerZ3GeneratedGoalCommands(r *Z3Replayer) error {
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{348, "Z3_mk_goal", replayMkGoal}, {349, "Z3_goal_inc_ref", replayGoalIncRef}, {350, "Z3_goal_dec_ref", replayGoalDecRef},
		{351, "Z3_goal_precision", func(r *Z3Replayer) error { return replayGoalOperation(r, 351) }},
		{352, "Z3_goal_assert", func(r *Z3Replayer) error { return replayGoalOperation(r, 352) }},
		{353, "Z3_goal_inconsistent", func(r *Z3Replayer) error { return replayGoalOperation(r, 353) }},
		{354, "Z3_goal_depth", func(r *Z3Replayer) error { return replayGoalOperation(r, 354) }},
		{355, "Z3_goal_reset", func(r *Z3Replayer) error { return replayGoalOperation(r, 355) }},
		{356, "Z3_goal_size", func(r *Z3Replayer) error { return replayGoalOperation(r, 356) }},
		{357, "Z3_goal_formula", func(r *Z3Replayer) error { return replayGoalOperation(r, 357) }},
		{358, "Z3_goal_num_exprs", func(r *Z3Replayer) error { return replayGoalOperation(r, 358) }},
		{359, "Z3_goal_is_decided_sat", func(r *Z3Replayer) error { return replayGoalOperation(r, 359) }},
		{360, "Z3_goal_is_decided_unsat", func(r *Z3Replayer) error { return replayGoalOperation(r, 360) }},
		{361, "Z3_goal_translate", func(r *Z3Replayer) error { return replayGoalOperation(r, 361) }},
		{362, "Z3_goal_to_string", func(r *Z3Replayer) error { return replayGoalOperation(r, 362) }},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreGoal(r *Z3Replayer, owner *replayContextOwner, value *APIGoal) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayGoal{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	g := &replayGoal{owner: owner, value: value}
	g.object = apiObject{context: owner.results, destructor: g, ownerBytes: n}
	g.object.self = &g.object
	id, err := owner.results.addObject(&g.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	g.object.id = id
	if err = owner.results.saveObject(&g.object); err != nil {
		return err
	}
	if r.goals == nil {
		r.goals = make(map[uint64]*replayGoal)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.goals[replayID] = g
	owner.goalIDs = append(owner.goalIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetGoal(r *Z3Replayer) (*replayGoal, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	return replayGoalAt(r, owner, 1)
}
func replayGoalAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*replayGoal, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	g := r.goals[id]
	if g == nil || g.owner != owner || g.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return g, nil
}
func replayMkGoal(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	models, err := r.Bool(1)
	if err != nil {
		return err
	}
	cores, err := r.Bool(2)
	if err != nil {
		return err
	}
	proofs, err := r.Bool(3)
	if err != nil {
		return err
	}
	value, err := owner.context.NewGoal(models, cores, proofs)
	if err != nil {
		return err
	}
	return replayStoreGoal(r, owner, value)
}
func replayGoalIncRef(r *Z3Replayer) error {
	if _, err := replayContext(r, 0); err != nil {
		return err
	}
	if id, err := r.Object(1); err != nil || id == 0 {
		return err
	}
	g, err := replayGetGoal(r)
	if err != nil {
		return err
	}
	return g.object.incRef()
}
func replayGoalDecRef(r *Z3Replayer) error {
	if _, err := replayContext(r, 0); err != nil {
		return err
	}
	if id, err := r.Object(1); err != nil || id == 0 {
		return err
	}
	g, err := replayGetGoal(r)
	if err != nil {
		return err
	}
	return g.object.decRef()
}

func replayGoalOperation(r *Z3Replayer, id uint32) (err error) {
	g, err := replayGetGoal(r)
	if err != nil {
		return err
	}
	switch id {
	case 351:
		_, err = g.value.Precision()
	case 352:
		var formula *replayASTOwner
		formula, err = r.astObject(g.owner, 2)
		if err == nil {
			err = g.value.Assert(formula.handle)
		}
	case 353:
		_, err = g.value.Inconsistent()
	case 354:
		_, err = g.value.Depth()
	case 355:
		err = g.value.Reset()
	case 356:
		_, err = g.value.Size()
	case 357:
		var index uint32
		index, err = r.Uint(2)
		if err != nil {
			return err
		}
		ref, e := g.value.Formula(index)
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, g.owner.results.manager.DelRef(&ref)) }()
		var result = g.owner.results.manager
		h, e := result.RefGet(ref)
		if e != nil {
			return e
		}
		return r.storeAST(g.owner, h)
	case 358:
		_, err = g.value.NumExpressions()
	case 359:
		_, err = g.value.IsDecidedSat()
	case 360:
		_, err = g.value.IsDecidedUnsat()
	case 361:
		var target *replayContextOwner
		target, err = replayContext(r, 2)
		if err != nil {
			return err
		}
		value, e := g.value.Translate(target.context)
		if e != nil {
			return e
		}
		return replayStoreGoal(r, target, value)
	case 362:
		_, err = g.value.String()
	}
	return err
}
