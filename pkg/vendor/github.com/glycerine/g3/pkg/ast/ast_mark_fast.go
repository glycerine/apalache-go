package ast

// Concrete ast_fast_mark<1/2> and ast_ref_fast_mark<1/2> specializations.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// Persistent inline storage is a named array in the charged cold owner. No
// slice points into it; only heap overflow storage uses a slice.
type astFastMarkState struct {
	owner     astMarkOwner
	inline    [16]astNodeRef
	heap      []astNodeRef
	size      uint32
	heapBytes uint64
	plane     uint8
	retain    bool
}

func (s *astFastMarkState) at(i uint32) astNodeRef {
	if s.heap == nil {
		return s.inline[i]
	}
	return s.heap[i]
}
func (s *astFastMarkState) append(m *ASTManager, r astNodeRef) error {
	cap := uint32(16)
	if s.heap != nil {
		cap = uint32(len(s.heap))
	}
	if s.size == cap {
		n := uint64(cap) * 2
		bytes := n * uint64(reflect.TypeOf(astNodeRef{}).Size())
		if n > uint64(^uint32(0)) || bytes > uint64(^uint(0)>>1) {
			return arena.ErrBounds
		}
		if e := m.BudgetField.Reserve(bytes); e != nil {
			return e
		}
		next := make([]astNodeRef, int(n))
		if s.heap == nil {
			copy(next, s.inline[:s.size])
		} else {
			copy(next, s.heap[:s.size])
		}
		m.BudgetField.Release(s.heapBytes)
		s.heapBytes = bytes
		s.heap = next
	}
	if s.heap == nil {
		s.inline[s.size] = r
	} else {
		s.heap[s.size] = r
	}
	s.size++
	return nil
}
func (s *astFastMarkState) IsMarked(h ASTHandle) (bool, error) {
	m, e := s.owner.lock()
	if e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return s.nodeMarkedLocked(m, h)
}
func (s *astFastMarkState) nodeMarkedLocked(m *ASTManager, h ASTHandle) (bool, error) {
	if e := s.owner.check(); e != nil {
		return false, e
	}
	if s.owner.manager != m {
		return false, ErrASTHandle
	}
	r, e := m.internal(h)
	if e != nil {
		return false, e
	}
	return m.nodeMark(r, s.plane, s.owner.identity, 0, false)
}
func (s *astFastMarkState) resetMark(m *ASTManager, r astNodeRef) error {
	_, e := m.nodeMark(r, s.plane, s.owner.identity, 2, false)
	return e
}
func (s *astFastMarkState) ResetMark(h ASTHandle) error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	return s.resetMark(m, r)
}
func (s *astFastMarkState) Mark(h ASTHandle, flag bool) error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	return s.markLocked(m, h, flag)
}
func (s *astFastMarkState) markLocked(m *ASTManager, h ASTHandle, flag bool) error {
	if e := s.owner.check(); e != nil {
		return e
	}
	if s.owner.manager != m {
		return ErrASTHandle
	}
	r, e := m.internal(h)
	if e != nil {
		return e
	}
	if !flag {
		return s.resetMark(m, r)
	}
	yes, e := m.nodeMark(r, s.plane, s.owner.identity, 0, false)
	if e != nil || yes {
		return e
	}
	if _, e = m.nodeMark(r, s.plane, s.owner.identity, 1, true); e != nil {
		return e
	}
	if s.retain {
		if e = m.incNode(r); e != nil {
			return e
		}
	}
	return s.append(m, r)
}
func (s *astFastMarkState) reset(m *ASTManager) error {
	// Both source classes clear every mark before resetting the list. The
	// ref-buffer then decrements ALL values ascending, not interleaved.
	for i := uint32(0); i < s.size; i++ {
		if e := s.resetMark(m, s.at(i)); e != nil {
			return e
		}
	}
	var failure error
	if s.retain {
		for i := uint32(0); i < s.size; i++ {
			consumed, e := m.decNode(s.at(i))
			if !consumed {
				return e
			}
			if failure == nil {
				failure = e
			}
		}
	}
	s.size = 0
	return failure
}
func (s *astFastMarkState) Reset() error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	return s.reset(m)
}
func (s *astFastMarkState) Close() error {
	if s == nil || s.owner.self != &s.owner {
		return ErrASTHandle
	}
	if s.owner.closed {
		return nil
	}
	m := s.owner.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	return s.closeLocked(m)
}
func (s *astFastMarkState) closeLocked(m *ASTManager) error {
	if s == nil || s.owner.self != &s.owner || s.owner.manager != m {
		return ErrASTHandle
	}
	if s.owner.closed {
		return nil
	}
	var e error
	if !m.Closed {
		e = s.reset(m)
		if e != nil && s.size != 0 {
			return e
		}
	}
	// After forced manager teardown, raw-pointer callbacks are undefined;
	// refund only this external owner's physical graph without touching ASTs.
	m.BudgetField.Release(s.heapBytes)
	s.heap = nil
	s.heapBytes = 0
	s.size = 0
	s.owner.finish()
	return e
}
func (s *astFastMarkState) level() uint32 {
	if s == nil || s.owner.self != &s.owner || s.owner.closed {
		return 0
	}
	return s.size
}
func (s *astFastMarkState) setLevel(n uint32) error {
	m, e := s.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	if ASTSourceDebug && n > s.size {
		return ErrASTDomain
	}
	// Source Release has no increasing-level action: its while simply skips.
	for n < s.size {
		if e = s.resetMark(m, s.at(s.size-1)); e != nil {
			return e
		}
		s.size--
	}
	return nil
}

// ASTFastMark1 specializes the source mark plane 1, borrowing AST nodes.
type ASTFastMark1 struct{ astFastMarkState }

func NewASTFastMark1(m *ASTManager) (*ASTFastMark1, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	return newASTFastMark1Locked(m)
}

// Caller already holds m.mu; the stable cold owner is fully charged.
func newASTFastMark1Locked(m *ASTManager) (*ASTFastMark1, error) {
	if m == nil || m.Self != m {
		return nil, ErrASTHandle
	}
	if m.Closed {
		return nil, arena.ErrClosed
	}
	bytes := uint64(reflect.TypeOf(ASTFastMark1{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	v := &ASTFastMark1{}
	v.plane = 1
	v.retain = false
	v.owner.bind(m, bytes, id)
	return v, nil
}
func (v *ASTFastMark1) GetLevel() uint32 {
	if v == nil {
		return 0
	}
	return v.level()
}
func (v *ASTFastMark1) SetLevel(n uint32) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.setLevel(n)
}

// ASTFastMark2 specializes the source mark plane 2, borrowing AST nodes.
type ASTFastMark2 struct{ astFastMarkState }

func NewASTFastMark2(m *ASTManager) (*ASTFastMark2, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTFastMark2{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	v := &ASTFastMark2{}
	v.plane = 2
	v.retain = false
	v.owner.bind(m, bytes, id)
	return v, nil
}
func (v *ASTFastMark2) GetLevel() uint32 {
	if v == nil {
		return 0
	}
	return v.level()
}
func (v *ASTFastMark2) SetLevel(n uint32) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.setLevel(n)
}

// ASTRefFastMark1 specializes the source mark plane 1, retaining AST nodes.
type ASTRefFastMark1 struct{ astFastMarkState }

func NewASTRefFastMark1(m *ASTManager) (*ASTRefFastMark1, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTRefFastMark1{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	v := &ASTRefFastMark1{}
	v.plane = 1
	v.retain = true
	v.owner.bind(m, bytes, id)
	return v, nil
}

// ASTRefFastMark2 specializes the source mark plane 2, retaining AST nodes.
type ASTRefFastMark2 struct{ astFastMarkState }

func NewASTRefFastMark2(m *ASTManager) (*ASTRefFastMark2, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTRefFastMark2{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	v := &ASTRefFastMark2{}
	v.plane = 2
	v.retain = true
	v.owner.bind(m, bytes, id)
	return v, nil
}

func (v *ASTFastMark1) IsMarked(h ASTHandle) (bool, error) {
	if v == nil {
		return false, ErrASTHandle
	}
	return v.astFastMarkState.IsMarked(h)
}
func (v *ASTFastMark1) Mark(h ASTHandle, flag bool) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Mark(h, flag)
}
func (v *ASTFastMark1) ResetMark(h ASTHandle) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.ResetMark(h)
}
func (v *ASTFastMark1) Reset() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Reset()
}
func (v *ASTFastMark1) Close() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Close()
}

func (v *ASTFastMark2) IsMarked(h ASTHandle) (bool, error) {
	if v == nil {
		return false, ErrASTHandle
	}
	return v.astFastMarkState.IsMarked(h)
}
func (v *ASTFastMark2) Mark(h ASTHandle, flag bool) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Mark(h, flag)
}
func (v *ASTFastMark2) ResetMark(h ASTHandle) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.ResetMark(h)
}
func (v *ASTFastMark2) Reset() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Reset()
}
func (v *ASTFastMark2) Close() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Close()
}

func (v *ASTRefFastMark1) IsMarked(h ASTHandle) (bool, error) {
	if v == nil {
		return false, ErrASTHandle
	}
	return v.astFastMarkState.IsMarked(h)
}
func (v *ASTRefFastMark1) Mark(h ASTHandle, flag bool) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Mark(h, flag)
}
func (v *ASTRefFastMark1) ResetMark(h ASTHandle) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.ResetMark(h)
}
func (v *ASTRefFastMark1) Reset() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Reset()
}
func (v *ASTRefFastMark1) Close() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Close()
}

func (v *ASTRefFastMark2) IsMarked(h ASTHandle) (bool, error) {
	if v == nil {
		return false, ErrASTHandle
	}
	return v.astFastMarkState.IsMarked(h)
}
func (v *ASTRefFastMark2) Mark(h ASTHandle, flag bool) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Mark(h, flag)
}
func (v *ASTRefFastMark2) ResetMark(h ASTHandle) error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.ResetMark(h)
}
func (v *ASTRefFastMark2) Reset() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Reset()
}
func (v *ASTRefFastMark2) Close() error {
	if v == nil {
		return ErrASTHandle
	}
	return v.astFastMarkState.Close()
}
