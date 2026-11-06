package ast

// Concrete ast_ref specialization of src/util/obj_ref.h.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
)

func (m *ASTManager) reference(r ASTRef) (*astRefRecord, error) {
	if r.origin != m.origin || r.slot == 0 || r.slot >= m.nextReference {
		return nil, ErrASTHandle
	}
	v := m.referenceAt(r.slot)
	if !v.live || v.generation != r.generation {
		return nil, ErrASTHandle
	}
	return v, nil
}
func (m *ASTManager) allocateReference() (ASTRef, error) {
	for m.freeReference != 0 {
		slot := m.freeReference
		r := m.referenceAt(slot)
		m.freeReference = r.nextFree
		if r.generation == math.MaxUint32 {
			continue
		}
		gen := r.generation + 1
		*r = astRefRecord{generation: gen, live: true}
		m.liveReferences++
		return ASTRef{m.origin, slot, gen}, nil
	}
	if m.nextReference == math.MaxUint32 {
		return ASTRef{}, arena.ErrBounds
	}
	slot := m.nextReference
	if e := m.ensureRefPage(slot); e != nil {
		return ASTRef{}, e
	}
	m.nextReference++
	*m.referenceAt(slot) = astRefRecord{generation: 1, live: true}
	m.liveReferences++
	return ASTRef{m.origin, slot, 1}, nil
}
func (m *ASTManager) releaseReferenceRecord(ref ASTRef) {
	r := m.referenceAt(ref.slot)
	r.live = false
	r.node = astNodeRef{}
	r.owned = false
	r.pending = nil
	r.nextFree = m.freeReference
	m.freeReference = ref.slot
	m.liveReferences--
}
func (m *ASTManager) NewRef(h ASTHandle) (ASTRef, error) {
	if e := m.lock(); e != nil {
		return ASTRef{}, e
	}
	defer m.mu.Unlock()
	return m.newRef(h)
}

// newRef is the same checked owning operation with the manager lock held.
func (m *ASTManager) newRef(h ASTHandle) (ASTRef, error) {
	var node astNodeRef
	var e error
	if !h.IsNull() {
		node, e = m.internal(h)
		if e != nil {
			return ASTRef{}, e
		}
	}
	ref, e := m.allocateReference()
	if e != nil {
		return ref, e
	}
	if e = m.incNode(node); e != nil {
		m.releaseReferenceRecord(ref)
		return ASTRef{}, e
	}
	m.referenceAt(ref.slot).node = node
	m.referenceAt(ref.slot).owned = node.slot != 0
	return ref, nil
}
func (m *ASTManager) RefGet(ref ASTRef) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.reference(ref)
	if e != nil {
		return ASTHandle{}, e
	}
	return m.external(r.node), nil
}
func (m *ASTManager) CopyRef(source ASTRef) (ASTRef, error) {
	if e := m.lock(); e != nil {
		return ASTRef{}, e
	}
	defer m.mu.Unlock()
	src, e := m.reference(source)
	if e != nil {
		return ASTRef{}, e
	}
	node := src.node
	ref, e := m.allocateReference()
	if e != nil {
		return ref, e
	}
	if e = m.incNode(node); e != nil {
		m.releaseReferenceRecord(ref)
		return ASTRef{}, e
	}
	m.referenceAt(ref.slot).node = node
	m.referenceAt(ref.slot).owned = node.slot != 0
	return ref, nil
}
func (m *ASTManager) assignRef(dst ASTRef, node astNodeRef) error {
	r, e := m.reference(dst)
	if e != nil {
		return e
	}
	if e = m.incNode(node); e != nil {
		return e
	}
	if r.node.slot != 0 && !r.owned {
		r.pending = append(r.pending, node)
		return ErrASTHandle
	}
	consumed, e := m.decNode(r.node)
	if e != nil || !consumed {
		// Source assignment has not reached m_obj = n. Its incoming INC
		// remains owned by this reference until explicit owning deletion.
		r.pending = append(r.pending, node)
		if consumed {
			r.owned = false
		}
		if e == nil {
			return ErrASTPending
		}
		return e
	}
	r.node = node
	r.owned = node.slot != 0
	return nil
}
func (m *ASTManager) AssignRef(dst, src ASTRef) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.reference(src)
	if e != nil {
		return e
	}
	return m.assignRef(dst, r.node)
}
func (m *ASTManager) SetRef(dst ASTRef, h ASTHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	var r astNodeRef
	var e error
	if !h.IsNull() {
		r, e = m.internal(h)
		if e != nil {
			return e
		}
	}
	return m.assignRef(dst, r)
}
func (m *ASTManager) ResetRef(ref ASTRef) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.reference(ref)
	if e != nil {
		return e
	}
	if r.node.slot != 0 && !r.owned {
		return ErrASTHandle
	}
	consumed, e := m.decNode(r.node)
	if e != nil {
		if consumed {
			r.owned = false
		}
		return e
	}
	if consumed {
		r.node = astNodeRef{}
		r.owned = false
	}
	return nil
}
func (m *ASTManager) drainRefPending(r *astRefRecord) error {
	var failure error
	for len(r.pending) != 0 {
		last := len(r.pending) - 1
		consumed, e := m.decNode(r.pending[last])
		if consumed {
			r.pending = r.pending[:last]
		}
		if failure == nil {
			failure = e
		}
		if !consumed {
			if e != nil {
				return e
			}
			return ErrASTPending
		}
	}
	return failure
}
func (m *ASTManager) DelRef(ref *ASTRef) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	return m.delRef(ref)
}

// delRef is used by concrete lexical owners while the manager lock is held.
func (m *ASTManager) delRef(ref *ASTRef) error {
	if ref == nil {
		return ErrASTHandle
	}
	r, e := m.reference(*ref)
	if e != nil {
		return e
	}
	var failure error
	if r.owned || r.node.slot == 0 {
		consumed, e := m.decNode(r.node)
		if consumed {
			r.owned = false
		}
		if !consumed {
			if e != nil {
				return e
			}
			return ErrASTPending
		}
		failure = e
	}
	if e := m.drainRefPending(r); failure == nil {
		failure = e
	}
	if len(r.pending) != 0 {
		return failure
	}
	m.releaseReferenceRecord(*ref)
	*ref = ASTRef{}
	return failure
}
func (m *ASTManager) MoveRef(source ASTRef) (ASTRef, error) {
	if e := m.lock(); e != nil {
		return ASTRef{}, e
	}
	defer m.mu.Unlock()
	src, e := m.reference(source)
	if e != nil {
		return ASTRef{}, e
	}
	node := src.node
	ref, e := m.allocateReference()
	if e != nil {
		return ref, e
	}
	m.referenceAt(ref.slot).node = node
	m.referenceAt(ref.slot).owned = src.owned
	m.referenceAt(ref.slot).pending = src.pending
	m.referenceAt(source.slot).node = astNodeRef{}
	m.referenceAt(source.slot).owned = false
	m.referenceAt(source.slot).pending = nil
	return ref, nil
}
func (m *ASTManager) SwapRefs(a, b ASTRef) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	x, e := m.reference(a)
	if e != nil {
		return e
	}
	y, e := m.reference(b)
	if e != nil {
		return e
	}
	x.node, y.node = y.node, x.node
	x.owned, y.owned = y.owned, x.owned
	x.pending, y.pending = y.pending, x.pending
	return nil
}
func (m *ASTManager) StealRef(source ASTRef) (ASTHandle, error) {
	if e := m.lock(); e != nil {
		return ASTHandle{}, e
	}
	defer m.mu.Unlock()
	src, e := m.reference(source)
	if e != nil {
		return ASTHandle{}, e
	}
	node := src.node
	src.node = astNodeRef{}
	src.owned = false
	return m.external(node), nil
}
