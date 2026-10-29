package qe

// Source: src/qe/qe_vartest.h. The retained callers use only these three
// concrete selection modes; no open-ended expression-predicate adapter is needed.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

type variableTestKind uint8

const (
	variableSet variableTestKind = iota
	variableSetComplement
	variableDeclarations
)

// VariableTest owns its copied native index set and borrows its AST manager.
// The source selector examines a variable's index without adjusting for binders.
type VariableTest struct {
	self         *VariableTest
	manager      *ast.ASTManager
	indices      map[uint32]struct{}
	declarations uint32
	kind         variableTestKind
	bytes        uint64
	closed       bool
}

func NewVariableSetTest(m *ast.ASTManager, indices []uint32, indexOfBound bool) (*VariableTest, error) {
	kind := variableSetComplement
	if indexOfBound {
		kind = variableSet
	}
	return newVariableTest(m, indices, 0, kind)
}

func NewVariableDeclarationTest(m *ast.ASTManager, declarations uint32) (*VariableTest, error) {
	return newVariableTest(m, nil, declarations, variableDeclarations)
}

func newVariableTest(m *ast.ASTManager, indices []uint32, declarations uint32, kind variableTestKind) (*VariableTest, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, err := m.ProofsEnabled(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(VariableTest{}).Size()) + uint64(len(indices))*4
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	v := &VariableTest{manager: m, declarations: declarations, kind: kind, bytes: bytes}
	v.self = v
	if kind != variableDeclarations {
		v.indices = make(map[uint32]struct{}, len(indices))
		for _, index := range indices {
			v.indices[index] = struct{}{}
		}
	}
	return v, nil
}

func (v *VariableTest) check() error {
	if v == nil || v.self != v || v.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (v *VariableTest) IsVariable(expression ast.ASTHandle) (bool, error) {
	if err := v.check(); err != nil {
		return false, err
	}
	kind, err := v.manager.GetKind(expression)
	if err != nil || kind != ast.ASTVar {
		return false, err
	}
	index, err := v.manager.GetVarIndex(expression)
	if err != nil {
		return false, err
	}
	if v.kind == variableDeclarations {
		return index < v.declarations, nil
	}
	_, found := v.indices[index]
	if v.kind == variableSetComplement {
		found = !found
	}
	return found, nil
}

func (v *VariableTest) Close() error {
	if v == nil || v.self != v {
		return ast.ErrASTHandle
	}
	if v.closed {
		return nil
	}
	v.manager.Budget().Release(v.bytes)
	v.bytes = 0
	v.indices = nil
	v.closed = true
	return nil
}
