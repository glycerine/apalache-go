package model

// Source: src/model/model_core.h:31-45,70-76; src/model/model.cpp:28-40,
// 60-66,111-150; src/model/model.h:26-36,48-56;
// src/tactic/model_converter.cpp:176-181.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/insdict"
)

type modelUniverse struct {
	values          []ast.ASTHandle
	owner           *ast.ASTManager
	sortOwned       bool
	memoryTrace     *diagnostic.TraceTags
	memoryTraceLive bool
}
type Model struct {
	self            *Model
	CoreField       ModelCore
	sorts           []ast.ASTHandle
	Universes       insdict.Dict[ast.ASTHandle, *modelUniverse]
	ownerBytes      uint64
	allocationTrace *diagnostic.TraceTags
	allocationLive  bool
	Closed          bool
}

func NewModel(m *ast.ASTManager) (md *Model, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err = m.ProofsEnabled(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(Model{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	md = &Model{ownerBytes: bytes}
	md.self = md
	md.CoreField.manager = m
	md.CoreField.self = &md.CoreField
	md.CoreField.Derived = md
	md.CoreField.valueProvider = md
	md.CoreField.Constants = *insdict.NewDictFunc[ast.ASTHandle, *modelConstantEntry](modelHandleHash)
	md.CoreField.Functions = *insdict.NewDictFunc[ast.ASTHandle, *modelFunctionEntry](modelHandleHash)
	md.Universes = *insdict.NewDictFunc[ast.ASTHandle, *modelUniverse](modelHandleHash)
	return md, nil
}
func (md *Model) open() error {
	if md == nil || md.self != md {
		return ast.ErrASTHandle
	}
	if md.Closed {
		return arena.ErrClosed
	}
	return md.CoreField.open()
}
func (md *Model) checkSort(s ast.ASTHandle) error {
	if e := md.open(); e != nil {
		return e
	}
	kind, e := md.CoreField.manager.GetKind(s)
	if e != nil {
		return e
	}
	if kind != ast.ASTSort {
		return ast.ErrASTDomain
	}
	return nil
}
func (md *Model) NumUninterpretedSorts() (uint32, error) {
	if e := md.open(); e != nil {
		return 0, e
	}
	return uint32(len(md.sorts)), nil
}
func (md *Model) UninterpretedSort(i uint32) (ast.ASTHandle, error) {
	if e := md.open(); e != nil {
		return ast.ASTHandle{}, e
	}
	if uint64(i) >= uint64(len(md.sorts)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return md.sorts[i], nil
}
func (md *Model) HasUninterpretedSort(s ast.ASTHandle) (bool, error) {
	e := md.checkSort(s)
	if e != nil {
		return false, e
	}
	_, found := md.Universes.Get2(s)
	return found, nil
}
func (md *Model) Universe(s ast.ASTHandle) ([]ast.ASTHandle, error) {
	e := md.checkSort(s)
	if e != nil {
		return nil, e
	}
	u, found := md.Universes.Get2(s)
	if !found || u == nil {
		return nil, ast.ErrASTDomain
	}
	return u.values, nil
}
func (md *Model) RegisterUniverse(s ast.ASTHandle, values []ast.ASTHandle) (err error) {
	e := md.checkSort(s)
	if e != nil {
		return e
	}
	m := md.CoreField.manager
	// Reject invalid Go handles before entering the source ownership sequence.
	for _, v := range values {
		kind, e := m.GetKind(v)
		if e != nil {
			return e
		}
		if kind != ast.ASTApp && kind != ast.ASTVar && kind != ast.ASTQuantifier {
			return ast.ErrASTDomain
		}
	}
	acquired := 0
	published := false
	defer func() {
		if err != nil && !published {
			for j := 0; j < acquired; j++ {
				err = errors.Join(err, m.DecRef(values[j]))
			}
		}
	}()
	for _, v := range values {
		if e = m.IncRef(v); e != nil {
			return e
		}
		acquired++
	}
	u, found := md.Universes.Get2(s)
	if !found {
		md.sorts = append(md.sorts, s)
		if e = m.IncRef(s); e != nil {
			return e
		}
		u = &modelUniverse{sortOwned: true, owner: m, memoryTrace: m.BorrowMemoryTraceTags(), memoryTraceLive: true}
		u.traceAllocated()
		u.values = append(u.values, values...)
		md.Universes.Set(s, u)
		published = true
	} else {
		for _, old := range u.values {
			if e = m.DecRef(old); e != nil {
				return e
			}
		}
		// Literal source update appends without resetting the old physical vector.
		// Preserve that behavior; do not invent extra roots for its stale prefix.
		u.values = append(u.values, values...)
		published = true
	}
	return nil
}
func (md *Model) CopyUniverseInterpretations(source *Model) error {
	if e := md.open(); e != nil {
		return e
	}
	if e := source.open(); e != nil {
		return e
	}
	if md.CoreField.manager != source.CoreField.manager {
		return ast.ErrASTHandle
	}
	for s, u := range source.Universes.All() {
		if e := md.RegisterUniverse(s, u.values); e != nil {
			return e
		}
	}
	return nil
}
func (md *Model) Close() error {
	if md == nil || md.self != md {
		return ast.ErrASTHandle
	}
	if md.Closed {
		return nil
	}
	m := md.CoreField.manager
	for s, u := range md.Universes.All() {
		if u.sortOwned {
			consumed, e := m.DecRefConsumed(s)
			if consumed {
				u.sortOwned = false
			}
			if e != nil {
				return e
			}
			if !consumed {
				return ast.ErrASTPending
			}
		}
		for j, v := range u.values {
			if !v.IsNull() {
				consumed, e := m.DecRefConsumed(v)
				if consumed {
					u.values[j] = ast.ASTHandle{}
				}
				if e != nil {
					return e
				}
				if !consumed {
					return ast.ErrASTPending
				}
			}
		}
		u.values = nil
		u.traceDeallocated()
	}
	md.Universes = insdict.Dict[ast.ASTHandle, *modelUniverse]{}
	md.sorts = nil
	if e := md.CoreField.closeCoreMembers(); e != nil {
		return e
	}
	md.traceSourceDeallocated()
	m.Budget().Release(md.ownerBytes)
	md.ownerBytes = 0
	md.Closed = true
	return nil
}
