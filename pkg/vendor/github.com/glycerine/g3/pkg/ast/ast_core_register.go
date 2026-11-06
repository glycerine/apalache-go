package ast

// register_node_core action order; post-publication exception unwind is an
// explicit adapter for source partially constructed/invalid continuations.

import (
	"math"
)

func (m *ASTManager) incNode(r astNodeRef) error {
	if r.slot == 0 {
		return nil
	}
	if e := m.validNode(r); e != nil {
		return e
	}
	n := m.nodeAt(r.slot)
	if ASTSourceDebug && n.rc == math.MaxUint32 {
		return ErrASTDomain
	}
	n.rc++
	return nil
}
func (m *ASTManager) infoForNode(r astNodeRef) (uint32, uint32) {
	switch m.nodeAt(r.slot).kind {
	case ASTSort:
		p := m.sortAt(r.slot).info
		return p.slot, p.generation
	case ASTFuncDecl:
		p := m.declAt(r.slot).info
		return p.slot, p.generation
	}
	return 0, 0
}
func (m *ASTManager) setInfoForNode(r astNodeRef, slot, generation uint32) {
	if m.nodeAt(r.slot).kind == ASTSort {
		m.sortAt(r.slot).info = ASTSortInfo{m.origin, slot, generation}
	} else {
		m.declAt(r.slot).info = ASTFuncDeclInfo{m.origin, slot, generation}
	}
}
func (m *ASTManager) nodeChildCount(r astNodeRef) uint32 {
	switch m.nodeAt(r.slot).kind {
	case ASTFuncDecl:
		return m.declAt(r.slot).arity + 1
	case ASTApp:
		return m.appAt(r.slot).count + 1
	case ASTVar:
		return 1
	case ASTQuantifier:
		v := m.quantifierAt(r.slot)
		return v.count + 1 + v.patternCount + v.noPatternCount
	}
	return 0
}
func (m *ASTManager) nodeChild(r astNodeRef, i uint32) (astNodeRef, error) {
	switch m.nodeAt(r.slot).kind {
	case ASTFuncDecl:
		v := m.declAt(r.slot)
		if i < v.arity {
			return m.readInternal(v.domain, i)
		}
		if i == v.arity {
			return v.rangeSort, nil
		}
	case ASTApp:
		v := m.appAt(r.slot)
		if i == 0 {
			return v.decl, nil
		}
		if i <= v.count {
			return m.readInternal(v.args, i-1)
		}
	case ASTVar:
		if i == 0 {
			return m.varAt(r.slot).sort, nil
		}
	case ASTQuantifier:
		v := m.quantifierAt(r.slot)
		if i < v.count {
			return m.readInternal(v.sorts, i)
		}
		i -= v.count
		if i == 0 {
			return v.body, nil
		}
		i--
		if i < v.patternCount {
			return m.readInternal(v.patterns, i)
		}
		i -= v.patternCount
		if i < v.noPatternCount {
			return m.readInternal(v.noPatterns, i)
		}
	}
	return astNodeRef{}, ErrASTDomain
}
func (m *ASTManager) depth(r astNodeRef) uint32 {
	switch m.nodeAt(r.slot).kind {
	case ASTApp:
		return m.appAt(r.slot).depth
	case ASTQuantifier:
		return m.quantifierAt(r.slot).depth
	}
	return 1
}
func (m *ASTManager) hasLabels(r astNodeRef) bool {
	switch m.nodeAt(r.slot).kind {
	case ASTApp:
		return m.appAt(r.slot).labels
	case ASTQuantifier:
		return m.quantifierAt(r.slot).labels
	}
	return false
}
func (m *ASTManager) updateAppFlags(r astNodeRef, arg astNodeRef) {
	a := m.appAt(r.slot)
	d := m.depth(arg)
	if d > a.depth {
		a.depth = d
	}
	switch m.nodeAt(arg.slot).kind {
	case ASTApp:
		v := m.appAt(arg.slot)
		a.quantifiers = a.quantifiers || v.quantifiers
		a.labels = a.labels || v.labels
		a.ground = a.ground && v.ground
	case ASTQuantifier:
		a.quantifiers = true
		a.ground = false
	case ASTVar:
		a.ground = false
	}
}
func (m *ASTManager) registerNode(r astNodeRef) (astNodeRef, error) {
	h, e := m.nodeHash(r)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	m.nodeAt(r.slot).hash = h
	contains, e := m.registrationBefore(r)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	result, unique, e := m.tableInsert(r)
	if e != nil {
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	if e = m.registrationAfter(r, result, contains, unique); e != nil {
		if unique {
			m.tableEraseIdentity(r)
		}
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	if !unique {
		m.freeNodeStorage(r)
		return result, nil
	}
	// Preflight physical reclamation capacity, after source interner insertion
	// and before assigning the ID/publishing retained children. Growth prefixes
	// remain, and failure removes only this candidate's membership/storage.
	generator := &m.ExprIDs
	if m.nodeAt(r.slot).kind == ASTSort || m.nodeAt(r.slot).kind == ASTFuncDecl {
		generator = &m.declIDs
	}
	if e = generator.Reserve(m.liveNodes); e != nil {
		m.tableEraseIdentity(r)
		m.freeNodeStorage(r)
		return astNodeRef{}, e
	}
	m.nodeAt(r.slot).id = generator.Mk()
	m.traceRegisteredNode(m.nodeAt(r.slot).id)
	infoSlot, infoGen := m.infoForNode(r)
	ownedInfo := false
	attached := uint32(0)
	children := uint32(0)
	fail := func(err error) (astNodeRef, error) {
		m.tableEraseIdentity(r)
		if ownedInfo {
			info := m.infoAt(infoSlot)
			for i := uint32(0); i < attached; i++ {
				p, x := m.infoParameter(info, i)
				if x == nil {
					v := m.parameterAt(p.slot)
					if v.Kind == ASTParameterAST {
						m.decNode(v.ast)
					}
				}
			}
			m.dropInfoStorage(infoSlot, infoGen)
		}
		for i := uint32(0); i < children; i++ {
			child, x := m.nodeChild(r, i)
			if x == nil {
				m.decNode(child)
			}
		}
		// Source assigned ID consumption remains; only the failed candidate's
		// physical resources/acquired ownership are unwound.
		m.freeNodeStorage(r)
		return astNodeRef{}, err
	}
	if infoSlot != 0 {
		infoSlot, infoGen, e = m.copyInfo(infoSlot, infoGen)
		if e != nil {
			return fail(e)
		}
		ownedInfo = true
		m.setInfoForNode(r, infoSlot, infoGen)
		info := m.infoAt(infoSlot)
		for attached < info.count {
			p, x := m.infoParameter(info, attached)
			if x != nil {
				return fail(x)
			}
			v := m.parameterAt(p.slot)
			if v.Kind == ASTParameterAST {
				if x = m.incNode(v.ast); x != nil {
					return fail(x)
				}
			}
			attached++
		}
		info.attached = true
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		a := m.appAt(r.slot)
		a.depth = 1
		a.ground = true
		if a.count != 0 {
			a.depth = 0
			fid, k, x := m.declIdentity(a.decl)
			if x != nil {
				return fail(x)
			}
			a.labels = fid == 1 && k == 0
		}
	}
	count := m.nodeChildCount(r)
	for children < count {
		child, x := m.nodeChild(r, children)
		if x != nil {
			return fail(x)
		}
		if x = m.incNode(child); x != nil {
			return fail(x)
		}
		if frame := m.traceAcquisitionExecuting; frame != nil && frame.node == r {
			frame.references[frame.acquired] = child
			frame.acquired++
		}
		if m.nodeAt(r.slot).kind == ASTApp && children > 0 {
			m.updateAppFlags(r, child)
		}
		children++
	}
	if m.nodeAt(r.slot).kind == ASTApp {
		a := m.appAt(r.slot)
		if a.count != 0 {
			a.depth++
			if a.depth > 65535 {
				a.depth = 65535
			}
		}
	}
	return r, nil
}
