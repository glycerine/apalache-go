package ast

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) allocateKind(kind ASTKind) (uint32, error) {
	if kind > ASTFuncDecl {
		return 0, ErrASTDomain
	}
	i := m.freeKind[kind]
	if i != 0 {
		switch kind {
		case ASTSort:
			r := &m.sorts[(i-1)/astPageSize][(i-1)%astPageSize]
			m.freeKind[kind] = r.nextFree
			*r = astSortRecord{}
		case ASTFuncDecl:
			r := &m.decls[(i-1)/astPageSize][(i-1)%astPageSize]
			m.freeKind[kind] = r.nextFree
			*r = astFuncDeclRecord{}
		case ASTApp:
			r := &m.apps[(i-1)/astPageSize][(i-1)%astPageSize]
			m.freeKind[kind] = r.nextFree
			*r = astAppRecord{}
		case ASTVar:
			r := &m.vars[(i-1)/astPageSize][(i-1)%astPageSize]
			m.freeKind[kind] = r.nextFree
			*r = astVarRecord{}
		case ASTQuantifier:
			r := &m.quantifiers[(i-1)/astPageSize][(i-1)%astPageSize]
			m.freeKind[kind] = r.nextFree
			*r = astQuantifierRecord{}
		}
		return i, nil
	}
	i = m.nextKind[kind]
	if i == math.MaxUint32 {
		return 0, arena.ErrBounds
	}
	var e error
	switch kind {
	case ASTSort:
		e = m.ensureSortPage(i)
	case ASTFuncDecl:
		e = m.ensureFuncDeclPage(i)
	case ASTApp:
		e = m.ensureAppPage(i)
	case ASTVar:
		e = m.ensureVarPage(i)
	case ASTQuantifier:
		e = m.ensureQuantifierPage(i)
	}
	if e != nil {
		return 0, e
	}
	m.nextKind[kind]++
	return i, nil
}
func (m *ASTManager) freeKindRecord(slot uint32) {
	n := m.nodeAt(slot)
	next := m.freeKind[n.kind]
	switch n.kind {
	case ASTSort:
		m.sortAt(slot).nextFree = next
	case ASTFuncDecl:
		m.declAt(slot).nextFree = next
	case ASTApp:
		m.appAt(slot).nextFree = next
	case ASTVar:
		m.varAt(slot).nextFree = next
	case ASTQuantifier:
		m.quantifierAt(slot).nextFree = next
	}
	m.freeKind[n.kind] = n.kindIndex
}
func (m *ASTManager) allocateNode(kind ASTKind) (astNodeRef, error) {
	// Slab generations are independent of source ID generator actions.
	var slot, generation uint32
	for m.freeNode != 0 {
		slot = m.freeNode
		n := m.nodeAt(slot)
		m.freeNode = n.nextFree
		if n.generation == math.MaxUint32 {
			slot = 0
			continue
		}
		generation = n.generation + 1
		break
	}
	if slot == 0 {
		if m.nextNode == math.MaxUint32 {
			return astNodeRef{}, arena.ErrBounds
		}
		slot = m.nextNode
		if e := m.ensureNodePage(slot); e != nil {
			return astNodeRef{}, e
		}
		m.nextNode++
		generation = 1
	}
	n := m.nodeAt(slot)
	*n = astNodeRecord{generation: generation, kind: kind, id: math.MaxUint32}
	index, e := m.allocateKind(kind)
	if e != nil {
		n.nextFree = m.freeNode
		m.freeNode = slot
		return astNodeRef{}, e
	}
	n.kindIndex = index
	n.live = true
	m.liveNodes++
	return astNodeRef{slot, generation}, nil
}
func (m *ASTManager) freeNodeStorage(r astNodeRef) error {
	if e := m.validNode(r); e != nil {
		return e
	}
	n := m.nodeAt(r.slot)
	var e error
	switch n.kind {
	case ASTFuncDecl:
		v := m.declAt(r.slot)
		e = m.freePayload(v.domain, v.arity, astRefBytes)
	case ASTApp:
		v := m.appAt(r.slot)
		e = m.freePayload(v.args, v.count, astRefBytes)
	case ASTQuantifier:
		v := m.quantifierAt(r.slot)
		if e = m.freePayload(v.sorts, v.count, astRefBytes); e == nil {
			e = m.freePayload(v.names, v.count, astSymbolBytes)
		}
		if e == nil {
			e = m.freePayload(v.patterns, v.patternCount, astRefBytes)
		}
		if e == nil {
			e = m.freePayload(v.noPatterns, v.noPatternCount, astRefBytes)
		}
	}
	if e != nil {
		return e
	}
	m.freeKindRecord(r.slot)
	n.live = false
	n.nextFree = m.freeNode
	m.freeNode = r.slot
	m.liveNodes--
	return nil
}
