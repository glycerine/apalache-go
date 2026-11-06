package api

// Source: generated api_commands.cpp and src/api/api_opt.cpp.

import (
	"errors"
	"fmt"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type replayOptimize struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIOptimize
}

func (o *replayOptimize) apiDestroy() error { return o.value.Close() }

func registerZ3GeneratedOptimizeCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{538, "Z3_mk_optimize"}, {539, "Z3_optimize_inc_ref"}, {540, "Z3_optimize_dec_ref"},
		{541, "Z3_optimize_assert"}, {542, "Z3_optimize_assert_soft"}, {543, "Z3_optimize_maximize"}, {544, "Z3_optimize_minimize"},
		{545, "Z3_optimize_push"}, {546, "Z3_optimize_pop"}, {547, "Z3_optimize_check"},
		{548, "Z3_optimize_get_reason_unknown"}, {549, "Z3_optimize_get_model"},
		{550, "Z3_optimize_set_params"}, {551, "Z3_optimize_get_param_descrs"},
		{552, "Z3_optimize_get_lower"}, {553, "Z3_optimize_get_upper"},
		{554, "Z3_optimize_get_lower_as_vector"}, {555, "Z3_optimize_get_upper_as_vector"},
		{556, "Z3_optimize_to_string"}, {557, "Z3_optimize_from_string"},
		{558, "Z3_optimize_from_file"}, {559, "Z3_optimize_get_help"},
		{560, "Z3_optimize_get_statistics"}, {561, "Z3_optimize_get_assertions"}, {562, "Z3_optimize_get_objectives"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayOptimizeCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}

func replayStoreOptimize(r *Z3Replayer, owner *replayContextOwner, value *APIOptimize) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayOptimize{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	o := &replayOptimize{owner: owner, value: value}
	o.object = apiObject{context: owner.results, destructor: o, ownerBytes: n}
	o.object.self = &o.object
	id, err := owner.results.addObject(&o.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	o.object.id = id
	if err = owner.results.saveObject(&o.object); err != nil {
		return err
	}
	if r.optimizers == nil {
		r.optimizers = make(map[uint64]*replayOptimize)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.optimizers[replayID] = o
	owner.optimizerIDs = append(owner.optimizerIDs, replayID)
	return r.StoreResult(replayID)
}

func replayGetOptimize(r *Z3Replayer) (*replayOptimize, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	o := r.optimizers[id]
	if o == nil || o.owner != owner || o.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return o, nil
}

func replayOptimizeCommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 538 {
		value, e := owner.context.NewOptimize()
		if e != nil {
			return e
		}
		return replayStoreOptimize(r, owner, value)
	}
	if id == 539 || id == 540 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
	}
	o, err := replayGetOptimize(r)
	if err != nil {
		return err
	}
	switch id {
	case 539:
		return o.object.incRef()
	case 540:
		return o.object.decRef()
	case 541, 543, 544:
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		if id == 541 {
			return o.value.Assert(value.handle)
		}
		if id == 543 {
			_, err = o.value.Maximize(value.handle)
		} else {
			_, err = o.value.Minimize(value.handle)
		}
	case 542:
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		weightText, e := r.String(3)
		if e != nil {
			return e
		}
		name, e := r.value(4, replaySymbol)
		if e != nil {
			return e
		}
		if name.numericSymbol || (!name.nullSymbol && name.s == "") {
			return fmt.Errorf("optimizer replay: numeric and empty-string soft group names are not supported")
		}
		service := owner.results.manager.NumeralService()
		weight, e := service.NewValue()
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, service.CleanupValue(&weight)) }()
		if e = service.SetString(&weight, weightText); e != nil {
			return e
		}
		text, e := service.String(&weight)
		if e != nil {
			return e
		}
		rational, ok := new(big.Rat).SetString(text)
		if !ok {
			return ast.ErrASTDomain
		}
		_, err = o.value.AssertSoft(value.handle, rational, name.s)
	case 545:
		err = o.value.Push()
	case 546:
		err = o.value.Pop()
	case 547:
		_, err = o.value.Check()
	case 548:
		_, err = o.value.ReasonUnknown()
	case 549:
		value, e := o.value.Model()
		if e != nil {
			return e
		}
		return replayStoreModel(r, owner, value)
	case 550:
		parameters, e := replayParametersAt(r, owner, 2)
		if e != nil {
			return e
		}
		return o.value.SetParameters(parameters.value)
	case 551:
		value, e := o.value.ParameterDescriptions()
		if e != nil {
			return e
		}
		return storeReplayDescriptions(r, owner, value)
	case 552, 553:
		index, e := r.Uint(2)
		if e != nil {
			return e
		}
		var ref ast.ASTRef
		if id == 552 {
			ref, err = o.value.Lower(index)
		} else {
			ref, err = o.value.Upper(index)
		}
		if err != nil {
			return err
		}
		manager := owner.results.manager
		defer func() { err = errors.Join(err, manager.DelRef(&ref)) }()
		handle, e := manager.RefGet(ref)
		if e != nil {
			return e
		}
		return r.storeAST(owner, handle)
	case 556:
		_, err = o.value.SMT2()
	case 557, 558:
		input, e := r.String(2)
		if e != nil {
			return e
		}
		if id == 557 {
			err = o.value.ParseString(input)
		} else {
			err = o.value.ParseFile(input)
		}
	case 559:
		_, err = o.value.ParameterHelp()
	case 560:
		value, e := o.value.StatisticsObject()
		if e != nil {
			return e
		}
		return replayStoreStatistics(r, owner, value)
	case 554, 555, 561, 562:
		var refs []ast.ASTRef
		if id == 554 || id == 555 {
			index, e := r.Uint(2)
			if e != nil {
				return e
			}
			if id == 554 {
				refs, err = o.value.LowerVector(index)
			} else {
				refs, err = o.value.UpperVector(index)
			}
		} else if id == 561 {
			refs, err = o.value.AssertionRefs()
		} else {
			refs, err = o.value.ObjectiveExpressions()
		}
		if err != nil {
			return err
		}
		manager := owner.results.manager
		defer func() {
			for i := range refs {
				err = errors.Join(err, manager.DelRef(&refs[i]))
			}
		}()
		handles := make([]ast.ASTHandle, len(refs))
		for i := range refs {
			handles[i], err = manager.RefGet(refs[i])
			if err != nil {
				return err
			}
		}
		vector, e := owner.context.NewASTVectorValues(handles...)
		if e != nil {
			return e
		}
		return replayStoreVector(r, owner, vector)
	}
	return err
}
