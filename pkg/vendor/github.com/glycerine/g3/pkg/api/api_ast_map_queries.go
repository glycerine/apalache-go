package api

// Source: src/api/api_ast_map.cpp:57-76,114-119.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
)

func (m *ASTMap) Contains(key ast.ASTHandle) (bool, error) {
	if err := m.check(); err != nil {
		return false, err
	}
	return m.object.contains(key)
}

// Find returns the borrowed value for key. The map keeps it live until that
// entry is replaced, erased, reset, or the map is closed.
func (m *ASTMap) Find(key ast.ASTHandle) (ast.ASTHandle, error) {
	if err := m.check(); err != nil {
		return ast.ASTHandle{}, err
	}
	value, err := m.object.find(key)
	if errors.Is(err, errAPIASTMapMissing) {
		return ast.ASTHandle{}, ErrASTMapMissing
	}
	return value, err
}

func (m *ASTMap) Size() (uint32, error) {
	if err := m.check(); err != nil {
		return 0, err
	}
	return m.object.size()
}
