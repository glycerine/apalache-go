package api

// Source: generated api_commands.cpp and src/api/api_tactic.cpp probes.

import (
	"errors"
	"reflect"
)

type replayProbe struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIProbe
}

func (p *replayProbe) apiDestroy() error { return p.value.Close() }
func registerZ3GeneratedProbeCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{{366, "Z3_mk_probe"}, {367, "Z3_probe_inc_ref"}, {368, "Z3_probe_dec_ref"}, {382, "Z3_probe_const"}, {383, "Z3_probe_lt"}, {384, "Z3_probe_gt"}, {385, "Z3_probe_le"},
		{386, "Z3_probe_ge"}, {387, "Z3_probe_eq"}, {388, "Z3_probe_and"}, {389, "Z3_probe_or"}, {390, "Z3_probe_not"}, {399, "Z3_probe_apply"}}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayProbeCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreProbe(r *Z3Replayer, owner *replayContextOwner, value *APIProbe) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayProbe{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	p := &replayProbe{owner: owner, value: value}
	p.object = apiObject{context: owner.results, destructor: p, ownerBytes: n}
	p.object.self = &p.object
	id, err := owner.results.addObject(&p.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	p.object.id = id
	if err = owner.results.saveObject(&p.object); err != nil {
		return err
	}
	if r.probes == nil {
		r.probes = make(map[uint64]*replayProbe)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.probes[replayID] = p
	owner.probeIDs = append(owner.probeIDs, replayID)
	return r.StoreResult(replayID)
}
func replayProbeAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*replayProbe, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	p := r.probes[id]
	if p == nil || p.owner != owner || p.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return p, nil
}
func replayProbeCommand(r *Z3Replayer, id uint32) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 366 || id == 382 {
		var value *APIProbe
		if id == 366 {
			name, e := r.String(1)
			if e != nil {
				return e
			}
			value, err = owner.context.NamedProbe(name)
		} else {
			n, e := r.Double(1)
			if e != nil {
				return e
			}
			value, err = owner.context.ConstantProbe(n)
		}
		if err != nil {
			return err
		}
		return replayStoreProbe(r, owner, value)
	}
	if id >= 383 && id <= 390 {
		left, e := replayProbeAt(r, owner, 1)
		if e != nil {
			return e
		}
		var result *APIProbe
		if id == 390 {
			result, err = left.value.Not()
		} else {
			right, e := replayProbeAt(r, owner, 2)
			if e != nil {
				return e
			}
			switch id {
			case 383:
				result, err = left.value.Compare(APIProbeLess, right.value)
			case 384:
				result, err = left.value.Compare(APIProbeGreater, right.value)
			case 385:
				result, err = left.value.Compare(APIProbeLessEqual, right.value)
			case 386:
				result, err = left.value.Compare(APIProbeGreaterEqual, right.value)
			case 387:
				result, err = left.value.Compare(APIProbeEqual, right.value)
			case 388:
				result, err = left.value.And(right.value)
			case 389:
				result, err = left.value.Or(right.value)
			}
		}
		if err != nil {
			return err
		}
		return replayStoreProbe(r, owner, result)
	}
	if id == 367 || id == 368 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
	}
	p, err := replayProbeAt(r, owner, 1)
	if err != nil {
		return err
	}
	switch id {
	case 367:
		return p.object.incRef()
	case 368:
		return p.object.decRef()
	case 399:
		goal, e := replayGoalAt(r, owner, 2)
		if e != nil {
			return e
		}
		_, err = p.value.Apply(goal.value)
	}
	return err
}
