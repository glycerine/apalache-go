package api

// Source: generated api_commands.cpp and src/api/api_datalog.cpp fixedpoint objects.

import (
	"errors"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/symbol"
)

type replayFixedpoint struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIFixedpoint
}

func (f *replayFixedpoint) apiDestroy() error { return f.value.Close() }
func registerZ3GeneratedFixedpointCommands(r *Z3Replayer) error {
	entries := []struct {
		id   uint32
		name string
	}{
		{511, "Z3_mk_fixedpoint"}, {512, "Z3_fixedpoint_inc_ref"}, {513, "Z3_fixedpoint_dec_ref"}, {514, "Z3_fixedpoint_add_rule"},
		{515, "Z3_fixedpoint_add_fact"}, {516, "Z3_fixedpoint_assert"}, {517, "Z3_fixedpoint_query"}, {518, "Z3_fixedpoint_query_relations"},
		{519, "Z3_fixedpoint_get_answer"}, {520, "Z3_fixedpoint_get_reason_unknown"}, {521, "Z3_fixedpoint_update_rule"}, {522, "Z3_fixedpoint_get_num_levels"}, {523, "Z3_fixedpoint_get_cover_delta"},
		{524, "Z3_fixedpoint_add_cover"}, {525, "Z3_fixedpoint_get_statistics"}, {526, "Z3_fixedpoint_register_relation"}, {527, "Z3_fixedpoint_set_predicate_representation"},
		{528, "Z3_fixedpoint_get_rules"}, {529, "Z3_fixedpoint_get_assertions"}, {536, "Z3_fixedpoint_push"}, {537, "Z3_fixedpoint_pop"},
		{534, "Z3_fixedpoint_from_string"}, {535, "Z3_fixedpoint_from_file"},
		{530, "Z3_fixedpoint_set_params"}, {531, "Z3_fixedpoint_get_help"}, {532, "Z3_fixedpoint_get_param_descrs"}, {533, "Z3_fixedpoint_to_string"},
		{651, "Z3_fixedpoint_query_from_lvl"}, {652, "Z3_fixedpoint_get_ground_sat_answer"}, {653, "Z3_fixedpoint_get_rules_along_trace"}, {654, "Z3_fixedpoint_get_rule_names_along_trace"},
		{655, "Z3_fixedpoint_add_invariant"}, {656, "Z3_fixedpoint_get_reachable"},
	}
	for _, entry := range entries {
		id := entry.id
		if err := r.RegisterCommand(id, entry.name, func(r *Z3Replayer) error { return replayFixedpointCommand(r, id) }); err != nil {
			return err
		}
	}
	return nil
}
func replayStoreFixedpoint(r *Z3Replayer, owner *replayContextOwner, value *APIFixedpoint) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayFixedpoint{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	f := &replayFixedpoint{owner: owner, value: value}
	f.object = apiObject{context: owner.results, destructor: f, ownerBytes: n}
	f.object.self = &f.object
	id, err := owner.results.addObject(&f.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	f.object.id = id
	if err = owner.results.saveObject(&f.object); err != nil {
		return err
	}
	if r.fixedpoints == nil {
		r.fixedpoints = make(map[uint64]*replayFixedpoint)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.fixedpoints[replayID] = f
	owner.fixedpointIDs = append(owner.fixedpointIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetFixedpoint(r *Z3Replayer) (*replayFixedpoint, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	id, err := r.Object(1)
	if err != nil {
		return nil, err
	}
	f := r.fixedpoints[id]
	if f == nil || f.owner != owner || f.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return f, nil
}
func replayFixedpointCommand(r *Z3Replayer, id uint32) (err error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	if id == 511 {
		value, e := owner.context.NewFixedpoint()
		if e != nil {
			return e
		}
		return replayStoreFixedpoint(r, owner, value)
	}
	if id == 512 || id == 513 {
		object, e := r.Object(1)
		if e != nil || object == 0 {
			return e
		}
	}
	f, err := replayGetFixedpoint(r)
	if err != nil {
		return err
	}
	switch id {
	case 534, 535:
		input, e := r.String(2)
		if e != nil {
			return e
		}
		var reader io.Reader = strings.NewReader(input)
		if id == 535 {
			file, x := os.Open(input)
			if x != nil {
				owner.results.setErrorCode(4)
				return r.StoreResult(0)
			}
			defer func() { err = errors.Join(err, file.Close()) }()
			reader = file
		}
		vector, e := f.value.LoadSMT2WithQueries(reader)
		if e != nil {
			owner.parserError = e.Error()
			owner.results.setErrorCode(4)
			return r.StoreResult(0)
		}
		return replayStoreVector(r, owner, vector)
	case 512:
		return f.object.incRef()
	case 513:
		return f.object.decRef()
	case 514, 521:
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		name, e := r.value(3, replaySymbol)
		if e != nil {
			return e
		}
		// Replay symbols are already values: N, # and $ distinguish null,
		// numerical and string symbols, including an interned empty string.
		label, e := replayFixedpointRuleSymbol(owner, name)
		if e != nil {
			return e
		}
		if id == 514 {
			return f.value.AddRuleWithSymbol(value.handle, label)
		}
		return f.value.UpdateRuleWithSymbol(value.handle, label)
	case 515:
		decl, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		count, e := r.Uint(3)
		if e != nil {
			return e
		}
		values, e := r.UintArray(4)
		if e != nil {
			return e
		}
		if uint64(count) > uint64(len(values)) {
			return ErrZ3ReplayReference
		}
		err = f.value.AddTableFact(decl.handle, values[:count])
	case 516:
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		err = f.value.Assert(value.handle)
	case 517:
		value, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		_, err = f.value.QueryExpression(value.handle)
	case 518:
		decls, e := replayASTArray(r, owner, 2, 3)
		if e != nil {
			return e
		}
		_, err = f.value.QueryRelations(decls)
	case 651:
		query, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		level, e := r.Uint(3)
		if e != nil {
			return e
		}
		_, err = f.value.QueryExpressionFromLevel(query.handle, level)
	case 519, 652:
		var ref ast.ASTRef
		if id == 519 {
			ref, err = f.value.LastAnswer()
		} else {
			ref, err = f.value.LastGroundSatAnswer()
		}
		if err != nil {
			return err
		}
		manager := owner.results.manager
		defer func() { err = errors.Join(err, manager.DelRef(&ref)) }()
		value, e := manager.RefGet(ref)
		if e != nil {
			return e
		}
		return r.storeAST(owner, value)
	case 520:
		_, err = f.value.ReasonUnknown()
	case 522:
		decl, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		_, err = f.value.NumLevels(decl.handle)
	case 523, 524:
		level, e := r.Int(2)
		if e != nil {
			return e
		}
		decl, e := r.astObject(owner, 3)
		if e != nil {
			return e
		}
		if id == 524 {
			property, e := r.astObject(owner, 4)
			if e != nil {
				return e
			}
			return f.value.AddCover(int(level), decl.handle, property.handle)
		}
		ref, e := f.value.CoverDelta(int(level), decl.handle)
		if e != nil {
			return e
		}
		manager := owner.results.manager
		defer func() { err = errors.Join(err, manager.DelRef(&ref)) }()
		handle, e := manager.RefGet(ref)
		if e != nil {
			return e
		}
		return r.storeAST(owner, handle)
	case 525:
		value, e := f.value.StatisticsObject()
		if e != nil {
			return e
		}
		return replayStoreStatistics(r, owner, value)
	case 526:
		var decl *replayASTOwner
		decl, err = r.astObject(owner, 2)
		if err == nil {
			err = f.value.RegisterRelation(decl.handle)
		}
	case 527:
		decl, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		count, e := r.Uint(3)
		if e != nil {
			return e
		}
		kinds, e := r.SymbolArray(4)
		if e != nil {
			return e
		}
		if uint64(count) > uint64(len(kinds)) {
			return ErrZ3ReplayReference
		}
		return f.value.SetPredicateRepresentation(decl.handle, kinds[:count]...)
	case 528, 529, 653, 654:
		var refs []ast.ASTRef
		var names []symbol.Symbol
		if id == 528 {
			refs, err = f.value.Rules()
		} else if id == 529 {
			refs, err = f.value.Assertions()
		} else {
			refs, names, err = f.value.RulesAlongTraceWithSymbols()
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
		if id == 654 {
			texts := make([]string, len(names))
			for i, name := range names {
				texts[i], err = manager.Symbols.String(name)
				if err != nil {
					return err
				}
			}
			// api_datalog_spacer.inc uses symbol::str and constructs a string
			// symbol directly, so an empty aggregate is a present empty name.
			if _, e := manager.Symbols.Intern(strings.Join(texts, ";")); e != nil {
				return e
			}
			vector, e := owner.context.NewASTVector()
			if e != nil {
				return e
			}
			if e = replayStoreVector(r, owner, vector); e != nil {
				return e
			}
			return r.StoreResult(0)
		}
		values := make([]ast.ASTHandle, len(refs))
		for i := range refs {
			values[i], err = manager.RefGet(refs[i])
			if err != nil {
				return err
			}
		}
		vector, e := owner.context.NewASTVectorValues(values...)
		if e != nil {
			return e
		}
		return replayStoreVector(r, owner, vector)
	case 536:
		err = f.value.Push()
	case 537:
		err = f.value.Pop()
	case 530:
		parameters, e := replayParametersAt(r, owner, 2)
		if e != nil {
			return e
		}
		return f.value.SetParameters(parameters.value)
	case 531:
		_, err = f.value.ParameterHelp()
	case 532:
		value, e := f.value.ParameterDescriptions()
		if e != nil {
			return e
		}
		return storeReplayDescriptions(r, owner, value)
	case 533:
		queries, e := replayASTArray(r, owner, 2, 3)
		if e != nil {
			return e
		}
		_, err = f.value.SMT2(queries...)
	case 655, 656:
		decl, e := r.astObject(owner, 2)
		if e != nil {
			return e
		}
		if id == 655 {
			property, e := r.astObject(owner, 3)
			if e != nil {
				return e
			}
			return f.value.AddInvariant(decl.handle, property.handle)
		}
		ref, e := f.value.Reachable(decl.handle)
		if e != nil {
			return e
		}
		manager := owner.results.manager
		defer func() { err = errors.Join(err, manager.DelRef(&ref)) }()
		handle, e := manager.RefGet(ref)
		if e != nil {
			return e
		}
		return r.storeAST(owner, handle)
	}
	return err
}
