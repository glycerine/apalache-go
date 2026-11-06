package api

// Source: generated api_commands.cpp and src/api/api_tactic.cpp.

import (
	"errors"
	"reflect"
)

type replayTactic struct {
	object apiObject
	owner  *replayContextOwner
	value  *APITactic
}

func (t *replayTactic) apiDestroy() error { return t.value.Close() }
func registerZ3GeneratedTacticCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{363, "Z3_mk_tactic"}, {364, "Z3_tactic_inc_ref"}, {365, "Z3_tactic_dec_ref"},
		{369, "Z3_tactic_and_then"}, {370, "Z3_tactic_or_else"}, {371, "Z3_tactic_par_or"},
		{372, "Z3_tactic_par_and_then"}, {373, "Z3_tactic_try_for"}, {374, "Z3_tactic_when"},
		{375, "Z3_tactic_cond"}, {376, "Z3_tactic_repeat"}, {379, "Z3_tactic_fail_if"}, {381, "Z3_tactic_using_params"},
		{377, "Z3_tactic_skip"}, {378, "Z3_tactic_fail"}, {380, "Z3_tactic_fail_if_not_decided"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayTacticCommand(r, id) }); err != nil {
			return err
		}
	}
	if err := registerZ3GeneratedTacticApply(r); err != nil {
		return err
	}
	if err := registerZ3GeneratedProbeCommands(r); err != nil {
		return err
	}
	return registerZ3GeneratedTacticRegistry(r)
}
func replayStoreTactic(r *Z3Replayer, owner *replayContextOwner, value *APITactic) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayTactic{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	t := &replayTactic{owner: owner, value: value}
	t.object = apiObject{context: owner.results, destructor: t, ownerBytes: n}
	t.object.self = &t.object
	id, err := owner.results.addObject(&t.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	t.object.id = id
	if err = owner.results.saveObject(&t.object); err != nil {
		return err
	}
	if r.tactics == nil {
		r.tactics = make(map[uint64]*replayTactic)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.tactics[replayID] = t
	owner.tacticIDs = append(owner.tacticIDs, replayID)
	return r.StoreResult(replayID)
}
func replayTacticAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*replayTactic, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	t := r.tactics[id]
	if t == nil || t.owner != owner || t.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return t, nil
}
func replayTacticCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 364 || id == 365 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
		t, e := replayTacticAt(r, owner, 1)
		if e != nil {
			return e
		}
		if id == 364 {
			return t.object.incRef()
		}
		return t.object.decRef()
	}
	var value *APITactic
	if id == 374 || id == 375 || id == 379 {
		probe, e := replayProbeAt(r, owner, 1)
		if e != nil {
			return e
		}
		switch id {
		case 374:
			child, e := replayTacticAt(r, owner, 2)
			if e != nil {
				return e
			}
			value, err = child.value.When(probe.value)
		case 375:
			yes, e := replayTacticAt(r, owner, 2)
			if e != nil {
				return e
			}
			no, e := replayTacticAt(r, owner, 3)
			if e != nil {
				return e
			}
			value, err = probe.value.Conditional(yes.value, no.value)
		case 379:
			value, err = probe.value.FailIf()
		}
		if err != nil {
			return err
		}
		return replayStoreTactic(r, owner, value)
	}
	if id == 371 {
		n, e := r.Uint(1)
		if e != nil {
			return e
		}
		ids, e := r.ObjectArray(2)
		if e != nil {
			return e
		}
		if uint64(n) > uint64(len(ids)) {
			return ErrZ3ReplayReference
		}
		children := make([]*APITactic, n)
		for i := range children {
			t := r.tactics[ids[i]]
			if t == nil || t.owner != owner || t.object.open() != nil {
				return ErrZ3ReplayReference
			}
			children[i] = t.value
		}
		value, err = owner.context.ParallelOr(children...)
		if err != nil {
			return err
		}
		return replayStoreTactic(r, owner, value)
	}
	if id == 369 || id == 370 || id == 372 || id == 373 || id == 376 || id == 381 {
		first, e := replayTacticAt(r, owner, 1)
		if e != nil {
			return e
		}
		switch id {
		case 369, 370, 372:
			second, e := replayTacticAt(r, owner, 2)
			if e != nil {
				return e
			}
			switch id {
			case 369:
				value, err = first.value.AndThen(second.value)
			case 370:
				value, err = first.value.OrElse(second.value)
			case 372:
				value, err = first.value.ParallelAndThen(second.value)
			}
		case 373, 376:
			n, e := r.Uint(2)
			if e != nil {
				return e
			}
			if id == 373 {
				value, err = first.value.TryFor(n)
			} else {
				value, err = first.value.Repeat(n)
			}
		case 381:
			params, e := replayParametersAt(r, owner, 2)
			if e != nil {
				return e
			}
			value, err = first.value.UsingParameters(params.value)
		}
		if err != nil {
			return err
		}
		return replayStoreTactic(r, owner, value)
	}
	switch id {
	case 363:
		var name string
		name, err = r.String(1)
		if err == nil {
			value, err = owner.context.Tactic(name)
		}
	case 377:
		value, err = owner.context.SkipTactic()
	case 378:
		value, err = owner.context.FailTactic()
	case 380:
		value, err = owner.context.FailIfUndecidedTactic()
	}
	if err != nil {
		return err
	}
	return replayStoreTactic(r, owner, value)
}
