package api

// Source: src/api/api_model.cpp and generated api_commands.cpp, model replay.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type replayModel struct {
	object apiObject
	owner  *replayContextOwner
	value  *APIModel
}

func (m *replayModel) apiDestroy() error { return m.value.Close() }

func RegisterZ3GeneratedModelCommands(r *Z3Replayer) error {
	if r == nil || r.closed {
		return ErrZ3ReplayState
	}
	entries := []struct {
		id      uint32
		name    string
		command ReplayCommand
	}{
		{295, "Z3_mk_model", replayMkModel}, {296, "Z3_model_inc_ref", replayModelIncRef}, {297, "Z3_model_dec_ref", replayModelDecRef},
		{298, "Z3_model_eval", replayModelEval}, {299, "Z3_model_get_const_interp", replayModelConstInterp}, {300, "Z3_model_has_interp", replayModelHasInterp},
		{302, "Z3_model_get_num_consts", replayModelNumConsts}, {303, "Z3_model_get_const_decl", replayModelConstDecl},
		{304, "Z3_model_get_num_funcs", replayModelNumFuncs}, {305, "Z3_model_get_func_decl", replayModelFuncDecl},
		{306, "Z3_model_get_num_sorts", replayModelNumSorts}, {307, "Z3_model_get_sort", replayModelSort},
		{308, "Z3_model_get_sort_universe", replayModelUniverse},
		{309, "Z3_model_translate", replayModelTranslate}, {310, "Z3_is_as_array", replayIsAsArray},
		{311, "Z3_get_as_array_func_decl", replayAsArrayFunction}, {313, "Z3_add_const_interp", replayModelAddConst},
		{333, "Z3_model_to_string", replayModelString}, {431, "Z3_solver_get_model", replaySolverGetModel},
	}
	for _, entry := range entries {
		if err := r.RegisterCommand(entry.id, entry.name, entry.command); err != nil {
			return err
		}
	}
	return registerZ3GeneratedFunctionInterpretation(r)
}
func replayStoreModel(r *Z3Replayer, owner *replayContextOwner, value *APIModel) error {
	if r.nextObjectID == ^uint64(0) {
		return errors.Join(ErrZ3ReplayState, value.Close())
	}
	n := uint64(reflect.TypeOf(replayModel{}).Size())
	if err := owner.results.manager.Budget().Reserve(n); err != nil {
		return errors.Join(err, value.Close())
	}
	m := &replayModel{owner: owner, value: value}
	m.object = apiObject{context: owner.results, destructor: m, ownerBytes: n}
	m.object.self = &m.object
	id, err := owner.results.addObject(&m.object)
	if err != nil {
		owner.results.manager.Budget().Release(n)
		return errors.Join(err, value.Close())
	}
	m.object.id = id
	if err = owner.results.saveObject(&m.object); err != nil {
		return err
	}
	if r.models == nil {
		r.models = make(map[uint64]*replayModel)
	}
	r.nextObjectID++
	replayID := r.nextObjectID
	r.models[replayID] = m
	owner.modelIDs = append(owner.modelIDs, replayID)
	return r.StoreResult(replayID)
}
func replayGetModel(r *Z3Replayer) (*replayModel, error) {
	owner, err := replayContext(r, 0)
	if err != nil {
		return nil, err
	}
	return replayModelAt(r, owner, 1)
}
func replayModelAt(r *Z3Replayer, owner *replayContextOwner, position uint32) (*replayModel, error) {
	id, err := r.Object(position)
	if err != nil {
		return nil, err
	}
	m := r.models[id]
	if m == nil || m.owner != owner || m.object.open() != nil {
		return nil, ErrZ3ReplayReference
	}
	return m, nil
}
func replayMkModel(r *Z3Replayer) error {
	owner, e := replayContext(r, 0)
	if e != nil {
		return e
	}
	value, e := owner.context.NewAPIModel()
	if e != nil {
		return e
	}
	return replayStoreModel(r, owner, value)
}
func replaySolverGetModel(r *Z3Replayer) error {
	s, e := replayGetSolver(r)
	if e != nil {
		return e
	}
	value, e := s.value.Model()
	if e != nil {
		return e
	}
	return replayStoreModel(r, s.owner, value)
}
func replayModelIncRef(r *Z3Replayer) error {
	if _, e := replayContext(r, 0); e != nil {
		return e
	}
	if id, e := r.Object(1); e != nil || id == 0 {
		return e
	}
	m, e := replayGetModel(r)
	if e != nil {
		return e
	}
	return m.object.incRef()
}
func replayModelDecRef(r *Z3Replayer) error {
	if _, e := replayContext(r, 0); e != nil {
		return e
	}
	if id, e := r.Object(1); e != nil || id == 0 {
		return e
	}
	m, e := replayGetModel(r)
	if e != nil {
		return e
	}
	return m.object.decRef()
}
func replayModelEval(r *Z3Replayer) (err error) {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	a, err := r.astObject(m.owner, 2)
	if err != nil {
		return err
	}
	completion, err := r.Bool(3)
	if err != nil {
		return err
	}
	if _, err = r.Object(4); err != nil {
		return err
	}
	ref, err := m.value.Evaluate(a.handle, completion)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.owner.results.manager.DelRef(&ref)) }()
	h, err := m.owner.results.manager.RefGet(ref)
	if err != nil {
		return err
	}
	old := r.result
	if err = r.storeAST(m.owner, h); err != nil {
		return err
	}
	id := r.result
	r.result = old
	return r.SetObject(4, id)
}
func replayModelConstInterp(r *Z3Replayer) error {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	decl, err := r.astObject(m.owner, 2)
	if err != nil {
		return err
	}
	h, err := m.value.ConstantInterpretation(decl.handle)
	if err != nil {
		return err
	}
	if h.IsNull() {
		return r.StoreResult(0)
	}
	return r.storeAST(m.owner, h)
}
func replayModelHasInterp(r *Z3Replayer) error {
	m, e := replayGetModel(r)
	if e != nil {
		return e
	}
	d, e := r.astObject(m.owner, 2)
	if e != nil {
		return e
	}
	_, e = m.value.HasInterpretation(d.handle)
	return e
}
func replayModelNumConsts(r *Z3Replayer) error {
	m, e := replayGetModel(r)
	if e != nil {
		return e
	}
	_, e = m.value.NumConstants()
	return e
}
func replayModelNumFuncs(r *Z3Replayer) error {
	m, e := replayGetModel(r)
	if e != nil {
		return e
	}
	_, e = m.value.NumFunctions()
	return e
}
func replayModelDecl(r *Z3Replayer, function bool) error {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	index, err := r.Uint(2)
	if err != nil {
		return err
	}
	var h = m.value.ConstantDeclaration
	if function {
		h = m.value.FunctionDeclaration
	}
	decl, err := h(index)
	if err != nil {
		return err
	}
	return r.storeBorrowedAST(m.owner, decl)
}
func replayModelConstDecl(r *Z3Replayer) error { return replayModelDecl(r, false) }
func replayModelFuncDecl(r *Z3Replayer) error  { return replayModelDecl(r, true) }
func replayModelString(r *Z3Replayer) error {
	m, e := replayGetModel(r)
	if e != nil {
		return e
	}
	_, e = m.value.String()
	return e
}

func replayModelNumSorts(r *Z3Replayer) error {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	_, err = m.value.NumSorts()
	return err
}
func replayModelSort(r *Z3Replayer) error {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	index, err := r.Uint(2)
	if err != nil {
		return err
	}
	sort, err := m.value.Sort(index)
	if err != nil {
		return err
	}
	return r.storeBorrowedAST(m.owner, sort)
}
func replayModelTranslate(r *Z3Replayer) error {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	target, err := replayContext(r, 2)
	if err != nil {
		return err
	}
	translated, err := m.value.Translate(target.context)
	if err != nil {
		return err
	}
	return replayStoreModel(r, target, translated)
}
func replayIsAsArray(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	_, err = owner.context.IsAsArray(value.handle)
	return err
}
func replayAsArrayFunction(r *Z3Replayer) error {
	owner, err := replayContext(r, 0)
	if err != nil {
		return err
	}
	value, err := r.astObject(owner, 1)
	if err != nil {
		return err
	}
	declaration, err := owner.context.AsArrayFunction(value.handle)
	if err != nil {
		return err
	}
	return r.storeBorrowedAST(owner, declaration)
}
func replayModelAddConst(r *Z3Replayer) error {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	declaration, err := r.astObject(m.owner, 2)
	if err != nil {
		return err
	}
	value, err := r.astObject(m.owner, 3)
	if err != nil {
		return err
	}
	return m.value.RegisterConstant(declaration.handle, value.handle)
}

func replayModelUniverse(r *Z3Replayer) (err error) {
	m, err := replayGetModel(r)
	if err != nil {
		return err
	}
	sort, err := r.astObject(m.owner, 2)
	if err != nil {
		return err
	}
	universe, err := m.value.Universe(sort.handle)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, universe.Close()) }()
	values := make([]ast.ASTHandle, universe.Size())
	for i := range values {
		values[i], err = universe.At(uint32(i))
		if err != nil {
			return err
		}
	}
	vector, err := m.owner.context.NewASTVectorValues(values...)
	if err != nil {
		return err
	}
	return replayStoreVector(r, m.owner, vector)
}
