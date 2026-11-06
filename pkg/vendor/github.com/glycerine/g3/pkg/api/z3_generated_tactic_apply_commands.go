package api

// Source: generated api_commands.cpp and src/api/api_tactic.cpp tactic application.

import (
	"errors"
	"reflect"
)

type replayTacticResult struct {
	object apiObject
	owner  *replayContextOwner
	value  *APITacticResult
}

func (r *replayTacticResult) apiDestroy() error { return r.value.Close() }
func registerZ3GeneratedTacticApply(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{400, "Z3_tactic_apply"}, {401, "Z3_tactic_apply_ex"}, {402, "Z3_apply_result_inc_ref"}, {403, "Z3_apply_result_dec_ref"},
		{404, "Z3_apply_result_to_string"}, {405, "Z3_apply_result_get_num_subgoals"}, {406, "Z3_apply_result_get_subgoal"}, {407, "Z3_apply_result_convert_model"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayTacticApplyCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreTacticResult(r *Z3Replayer, owner *replayContextOwner, value *APITacticResult) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayTacticResult{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	a := &replayTacticResult{owner: owner, value: value}
	a.object = apiObject{context: owner.results, destructor: a, ownerBytes: n}
	a.object.self = &a.object
	id, err := owner.results.addObject(&a.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	a.object.id = id
	if err = owner.results.saveObject(&a.object); err != nil {
		return err
	}
	if r.tacticResults == nil {
		r.tacticResults = make(map[uint64]*replayTacticResult)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.tacticResults[replayID] = a
	owner.tacticResultIDs = append(owner.tacticResultIDs, replayID)
	return r.StoreResult(replayID)
}
func replayTacticApplyCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 400 || id == 401 {
		tactic, e := replayTacticAt(r, owner, 1)
		if e != nil {
			return e
		}
		goal, e := replayGoalAt(r, owner, 2)
		if e != nil {
			return e
		}
		var result *APITacticResult
		if id == 400 {
			result, err = tactic.value.Apply(goal.value)
		} else {
			params, e := replayParametersAt(r, owner, 3)
			if e != nil {
				return e
			}
			result, err = tactic.value.ApplyWithParameters(goal.value, params.value)
		}
		if err != nil {
			return err
		}
		return replayStoreTacticResult(r, owner, result)
	}
	object, err := r.Object(1)
	if err != nil {
		return err
	}
	if object == 0 && (id == 402 || id == 403) {
		return nil
	}
	result := r.tacticResults[object]
	if result == nil || result.owner != owner || result.object.open() != nil {
		return ErrZ3ReplayReference
	}
	switch id {
	case 402:
		return result.object.incRef()
	case 403:
		return result.object.decRef()
	case 404:
		_, err = result.value.String()
	case 405:
		_, err = result.value.NumSubgoals()
	case 406:
		index, e := r.Uint(2)
		if e != nil {
			return e
		}
		goal, e := result.value.Subgoal(index)
		if e != nil {
			return e
		}
		return replayStoreGoal(r, owner, goal)
	case 407:
		index, e := r.Uint(2)
		if e != nil {
			return e
		}
		model, e := replayModelAt(r, owner, 3)
		if e != nil {
			return e
		}
		converted, e := result.value.ConvertModel(index, model.value)
		if e != nil {
			return e
		}
		return replayStoreModel(r, owner, converted)
	}
	return err
}
