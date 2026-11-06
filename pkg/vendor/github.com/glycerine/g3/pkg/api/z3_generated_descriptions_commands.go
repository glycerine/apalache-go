package api

// Source: src/api/api_params.cpp, api_solver.cpp and generated api_commands.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type replayParameterDescriptions struct {
	object apiObject
	owner  *replayContextOwner
	value  *ParameterDescriptions
}

func (d *replayParameterDescriptions) apiDestroy() error { return d.value.Close() }
func replaySolverDescriptions(r *Z3Replayer) error {
	solver, err := replayGetSolver(r)
	if err != nil {
		return err
	}
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	value, err := solver.value.ParameterDescriptions()
	if err != nil {
		return err
	}
	return storeReplayDescriptions(r, solver.owner, value)
}

func storeReplayDescriptions(r *Z3Replayer, owner *replayContextOwner, value *ParameterDescriptions) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayParameterDescriptions{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	d := &replayParameterDescriptions{owner: owner, value: value}
	d.object = apiObject{context: owner.results, destructor: d, ownerBytes: n}
	d.object.self = &d.object
	id, err := owner.results.addObject(&d.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	d.object.id = id
	if err = owner.results.saveObject(&d.object); err != nil {
		return err
	}
	if r.descriptions == nil {
		r.descriptions = make(map[uint64]*replayParameterDescriptions)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.descriptions[replayID] = d
	owner.descriptionIDs = append(owner.descriptionIDs, replayID)
	return r.StoreResult(replayID)
}
func replayDescriptions(r *Z3Replayer, position uint32, owner *replayContextOwner) (*replayParameterDescriptions, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	d := r.descriptions[id]
	if d == nil || d.owner != owner || d.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return d, nil
}
func replayDescriptionArgument(r *Z3Replayer) (*replayParameterDescriptions, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	return replayDescriptions(r, 1, owner)
}
func replayValidateParameters(r *Z3Replayer) error {
	p, err := replayParameters(r)
	if err != nil {
		return err
	}
	d, err := replayDescriptions(r, 2, p.owner)
	if err != nil {
		return err
	}
	return p.value.Validate(d.value)
}
func replayDescriptionsIncRef(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	return d.object.incRef()
}
func replayDescriptionsDecRef(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	return d.object.decRef()
}
func replayDescriptionsKind(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	sym, e := replayTypedSymbol(r, d.owner, 2)
	if e != nil {
		return e
	}
	_, e = d.value.value.Kind(sym)
	return e
}
func replayDescriptionsSize(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	_, e = d.value.Size()
	return e
}
func replayDescriptionsName(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	i, e := r.Uint(2)
	if e != nil {
		return e
	}
	_, e = d.value.value.ParamName(i)
	return e
}
func replayDescriptionsDocumentation(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	sym, e := replayTypedSymbol(r, d.owner, 2)
	if e != nil {
		return e
	}
	_, found, e := d.value.value.Description(sym)
	if e != nil {
		return e
	}
	if !found {
		return arena.ErrBounds
	}
	return nil
}
func replayDescriptionsString(r *Z3Replayer) error {
	d, e := replayDescriptionArgument(r)
	if e != nil {
		return e
	}
	_, e = d.value.String()
	return e
}
