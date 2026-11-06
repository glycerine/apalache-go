package api

// Source: src/api/api_ast_map.cpp:57-112.

import "github.com/glycerine/g3/pkg/ast"

// Insert retains key and value. Replacing a key retains the incoming value
// before releasing the previous value, including self-replacement.
func (m *ASTMap) Insert(key, value ast.ASTHandle) error {
	if err := m.check(); err != nil {
		return err
	}
	return m.object.insert(key, value)
}

// Erase removes key and releases the map's key/value ownership. Erasing an
// absent key succeeds, matching Z3_ast_map_erase.
func (m *ASTMap) Erase(key ast.ASTHandle) error {
	if err := m.check(); err != nil {
		return err
	}
	return m.object.erase(key)
}

// Reset removes every entry while keeping the map open for reuse.
func (m *ASTMap) Reset() error {
	if err := m.check(); err != nil {
		return err
	}
	return m.object.reset()
}
