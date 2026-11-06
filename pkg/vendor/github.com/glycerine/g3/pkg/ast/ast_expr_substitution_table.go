package ast

// Native typed collection keys; only queried AST identities are dereferenced.

import "github.com/glycerine/g3/pkg/arena"

func (m *ASTManager) substKey(key astNodeRef) (uint64, error) {
	if e := m.validNode(key); e != nil {
		return 0, e
	}
	kind := m.nodeAt(key.slot).kind
	if kind != ASTApp && kind != ASTVar && kind != ASTQuantifier {
		return 0, ErrASTDomain
	}
	return uint64(key.slot)<<32 | uint64(key.generation), nil
}
func (m *ASTManager) substFind(t *astExprSubstTable, key astNodeRef) (uint64, bool, error) {
	token, e := m.substKey(key)
	if e != nil {
		return 0, false, e
	}
	_, found := t.entries.Get2(token)
	return token, found, nil
}
func (m *ASTManager) substRawExpr(h ASTHandle) (astNodeRef, error) {
	if h.IsNull() {
		return astNodeRef{}, nil
	}
	if h.origin != m.origin {
		return astNodeRef{}, ErrASTHandle
	}
	return astNodeRef{h.slot, h.generation}, nil
}
func (m *ASTManager) substRawDependency(h ASTDependency) (astNodeRef, error) {
	if h.IsNull() {
		return astNodeRef{}, nil
	}
	if h.origin != m.origin {
		return astNodeRef{}, ErrASTHandle
	}
	return astNodeRef{uint32(h.record.Slot), uint32(h.record.Generation)}, nil
}
func (m *ASTManager) substDependency(r astNodeRef) ASTDependency {
	if r.slot == 0 {
		return ASTDependency{}
	}
	return ASTDependency{m.origin, substDependencyPointer(r)}
}

func substDependencyPointer(r astNodeRef) arena.SmallBytesRef {
	return arena.SmallBytesRef{Slot: arena.Slot(r.slot), Generation: arena.Generation(r.generation)}
}
