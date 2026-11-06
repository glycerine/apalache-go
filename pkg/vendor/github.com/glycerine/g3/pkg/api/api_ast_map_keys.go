package api

// Source: src/api/api_ast_map.cpp:121-137.

import (
	"errors"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
)

// ASTMapKeys owns an independent AST reference for every returned key. It
// remains valid after its source map is reset or closed.
type ASTMapKeys struct {
	manager *ast.ASTManager
	refs    []ast.ASTRef
	closed  bool
}

func (m *ASTMap) Keys() (result *ASTMapKeys, resultErr error) {
	if err := m.check(); err != nil {
		return nil, err
	}
	handles, err := m.object.keys()
	if err != nil {
		return nil, err
	}
	result = &ASTMapKeys{manager: m.owner.manager, refs: make([]ast.ASTRef, 0, len(handles))}
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, result.Close())
			result = nil
		}
	}()
	for _, handle := range handles {
		ref, err := result.manager.NewRef(handle)
		if err != nil {
			resultErr = err
			return
		}
		result.refs = append(result.refs, ref)
	}
	return result, nil
}

func (k *ASTMapKeys) check() error {
	if k == nil || k.manager == nil || k.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (k *ASTMapKeys) Len() (uint32, error) {
	if err := k.check(); err != nil {
		return 0, err
	}
	return uint32(len(k.refs)), nil
}

func (k *ASTMapKeys) At(index uint32) (ast.ASTHandle, error) {
	if err := k.check(); err != nil {
		return ast.ASTHandle{}, err
	}
	if uint64(index) >= uint64(len(k.refs)) {
		return ast.ASTHandle{}, arena.ErrBounds
	}
	return k.manager.RefGet(k.refs[index])
}

func (k *ASTMapKeys) Values() ([]ast.ASTHandle, error) {
	if err := k.check(); err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, len(k.refs))
	for i := range k.refs {
		var err error
		result[i], err = k.manager.RefGet(k.refs[i])
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (k *ASTMapKeys) Close() (result error) {
	if k == nil || k.closed {
		return nil
	}
	if k.manager == nil {
		return ast.ErrASTHandle
	}
	for i := len(k.refs) - 1; i >= 0; i-- {
		result = errors.Join(result, k.manager.DelRef(&k.refs[i]))
	}
	if result == nil {
		k.refs = nil
		k.manager = nil
		k.closed = true
	}
	return result
}
