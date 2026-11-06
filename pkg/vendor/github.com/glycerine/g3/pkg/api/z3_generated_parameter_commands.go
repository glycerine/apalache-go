package api

// Source: src/api/api_params.cpp and generated api_commands.cpp, parameter-set commands.

import (
	"errors"
	"reflect"

	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type replayParameterSet struct {
	object apiObject
	owner  *replayContextOwner
	value  *ParameterSet
}

func (p *replayParameterSet) apiDestroy() error { return p.value.Close() }

// RegisterZ3GeneratedParameterCommands installs parameter-set replay handlers.
func RegisterZ3GeneratedParameterCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{13, "Z3_mk_params", replayMkParameters}, {14, "Z3_params_inc_ref", replayIncParameters},
		{15, "Z3_params_dec_ref", replayDecParameters}, {16, "Z3_params_set_bool", replaySetParameterBool},
		{17, "Z3_params_set_uint", replaySetParameterUInt}, {18, "Z3_params_set_double", replaySetParameterDouble},
		{19, "Z3_params_set_symbol", replaySetParameterSymbol}, {20, "Z3_params_to_string", replayParametersString},
		{21, "Z3_params_validate", replayValidateParameters}, {22, "Z3_param_descrs_inc_ref", replayDescriptionsIncRef},
		{23, "Z3_param_descrs_dec_ref", replayDescriptionsDecRef}, {24, "Z3_param_descrs_get_kind", replayDescriptionsKind},
		{25, "Z3_param_descrs_size", replayDescriptionsSize}, {26, "Z3_param_descrs_get_name", replayDescriptionsName},
		{27, "Z3_param_descrs_get_documentation", replayDescriptionsDocumentation}, {28, "Z3_param_descrs_to_string", replayDescriptionsString},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return nil
}

func replayMkParameters(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if r.nextObjectID == ^uint64(0) {
		return ErrZ3ReplayState
	}
	value, err := owner.context.NewParameterSet()
	if err != nil {
		return err
	}
	n := uint64(reflect.TypeOf(replayParameterSet{}).Size())
	if err = owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	p := &replayParameterSet{owner: owner, value: value}
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
	if r.parameterSets == nil {
		r.parameterSets = make(map[uint64]*replayParameterSet)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.parameterSets[replayID] = p
	owner.parameterIDs = append(owner.parameterIDs, replayID)
	return r.StoreResult(replayID)
}

func replayParameters(r *Z3Replayer) (*replayParameterSet, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	return replayParametersAt(r, owner, 1)
}

func replayParametersAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*replayParameterSet, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	p := r.parameterSets[id]
	if p == nil || p.owner != owner || p.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return p, nil
}
func replayIncParameters(r *Z3Replayer) error {
	p, e := replayParameters(r)
	if e != nil {
		return e
	}
	return p.object.incRef()
}
func replayDecParameters(r *Z3Replayer) error {
	p, e := replayParameters(r)
	if e != nil {
		return e
	}
	return p.object.decRef()
}

func replayParameterName(r *Z3Replayer) (string, error) {
	v, err := r.value(2, replaySymbol)
	if err != nil {
		return "", err
	}
	if v.numericSymbol {
		return "", ErrZ3ReplayReference
	}
	return v.s, nil
}
func replaySetParameterBool(r *Z3Replayer) error {
	p, err := replayParameters(r)
	if err != nil {
		return err
	}
	k, err := replayParameterName(r)
	if err != nil {
		return err
	}
	v, err := r.Bool(3)
	if err != nil {
		return err
	}
	return p.value.SetBool(k, v)
}
func replaySetParameterUInt(r *Z3Replayer) error {
	p, err := replayParameters(r)
	if err != nil {
		return err
	}
	k, err := replayParameterName(r)
	if err != nil {
		return err
	}
	v, err := r.Uint(3)
	if err != nil {
		return err
	}
	return p.value.SetUInt(k, v)
}
func replaySetParameterDouble(r *Z3Replayer) error {
	p, err := replayParameters(r)
	if err != nil {
		return err
	}
	k, err := replayParameterName(r)
	if err != nil {
		return err
	}
	v, err := r.Double(3)
	if err != nil {
		return err
	}
	return p.value.SetDouble(k, v)
}
func replaySetParameterSymbol(r *Z3Replayer) error {
	p, err := replayParameters(r)
	if err != nil {
		return err
	}
	k, err := replayParameterName(r)
	if err != nil {
		return err
	}
	v, err := r.value(3, replaySymbol)
	if err != nil {
		return err
	}
	var sym usymbol.Symbol
	if v.numericSymbol {
		sym = usymbol.Numerical(uint32(v.u))
	} else if !v.nullSymbol {
		sym, err = p.value.params.Symbols().Intern(v.s)
		if err != nil {
			return err
		}
	}
	k = apiParameterName(k)
	return p.value.params.SetSymbolName(&p.value.value, &k, sym)
}
func replayParametersString(r *Z3Replayer) error {
	p, err := replayParameters(r)
	if err != nil {
		return err
	}
	_, err = p.value.String()
	return err
}
