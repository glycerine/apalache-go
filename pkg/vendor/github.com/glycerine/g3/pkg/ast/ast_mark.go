package ast

// Concrete default obj_mark<expr> and ast_mark; IDs deliberately outlive nodes.

import (
	"reflect"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
)

type astMarkOwner struct {
	self       *astMarkOwner
	manager    *ASTManager
	identity   uint64
	ownerBytes uint64
	closed     bool
}

var astMarkIdentity atomic.Uint64

// Reserve before constructing any escaping cold Go object.
func prepareASTMarkOwner(m *ASTManager, bytes uint64) (uint64, error) {
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return 0, e
	}
	for {
		old := astMarkIdentity.Load()
		if old == ^uint64(0) {
			m.BudgetField.Release(bytes)
			return 0, arena.ErrBounds
		}
		if astMarkIdentity.CompareAndSwap(old, old+1) {
			return old + 1, nil
		}
	}
}
func (o *astMarkOwner) bind(m *ASTManager, bytes, id uint64) {
	o.self = o
	o.manager = m
	o.identity = id
	o.ownerBytes = bytes
}
func (o *astMarkOwner) check() error {
	if o == nil || o.self != o || o.manager == nil || o.manager.Self != o.manager {
		return ErrASTHandle
	}
	if o.closed {
		return arena.ErrClosed
	}
	return nil
}
func (o *astMarkOwner) lock() (*ASTManager, error) {
	if e := o.check(); e != nil {
		return nil, e
	}
	m := o.manager
	if e := m.lock(); e != nil {
		return nil, e
	}
	return m, nil
}
func (o *astMarkOwner) finish() {
	o.manager.BudgetField.Release(o.ownerBytes)
	o.ownerBytes = 0
	o.closed = true
}

// bit_vector's default constructor, resize(false), get/set/reset/destructor
// subset. The packed words use native Go slice storage.
type astMarkBits struct {
	words []uint32
	size  uint32
}

func (b *astMarkBits) resize(n uint32) error {
	if n <= b.size {
		b.size = n
		return nil
	}
	// Deliberately retain the source uint32 (n+31) arithmetic. Physical
	// checks below bound only otherwise-invalid reads/writes after wrap.
	need := (n + 31) / 32
	if need > uint32(len(b.words)) {
		if uint64(need) > uint64(^uint(0)>>1)/4 {
			return arena.ErrBounds
		}
		b.words = append(b.words, make([]uint32, int(need)-len(b.words))...)
	}
	begin := b.size / 32
	if uint64(begin) >= uint64(len(b.words)) {
		return arena.ErrBounds
	}
	pos := b.size % 32
	b.words[begin] &= (uint32(1) << pos) - 1
	for i := begin + 1; i < need; i++ {
		b.words[i] = 0
	}
	b.size = n
	return nil
}
func (b *astMarkBits) marked(id uint32) (bool, error) {
	if id >= b.size {
		return false, nil
	}
	if uint64(id/32) >= uint64(len(b.words)) {
		return false, arena.ErrBounds
	}
	return b.words[id/32]&(uint32(1)<<(id%32)) != 0, nil
}
func (b *astMarkBits) mark(id uint32, flag bool) error {
	if id >= b.size {
		if e := b.resize(id + 1); e != nil {
			return e
		}
	}
	if ASTSourceDebug && id >= b.size {
		return ErrASTDomain
	}
	if uint64(id/32) >= uint64(len(b.words)) {
		return arena.ErrBounds
	}
	mask := uint32(1) << (id % 32)
	// The source branchless unsigned setter has this exact boolean result.
	if flag {
		b.words[id/32] |= mask
	} else {
		b.words[id/32] &= ^mask
	}
	return nil
}
func (b *astMarkBits) reset() { clear(b.words); b.size = 0 }
func (b *astMarkBits) close() {
	b.words = nil
	b.size = 0
}
func (m *ASTManager) markID(h ASTHandle, expressionsOnly bool) (uint32, bool, error) {
	r, e := m.internal(h)
	if e != nil {
		return 0, false, e
	}
	n := m.nodeAt(r.slot)
	decl := n.kind == ASTSort || n.kind == ASTFuncDecl
	if expressionsOnly && decl {
		return 0, false, ErrASTHandle
	}
	id := n.id
	if decl {
		if ASTSourceDebug && id < 1<<31 {
			return 0, true, ErrASTDomain
		}
		id -= 1 << 31
	}
	return id, decl, nil
}

type ASTExprMark struct {
	owner astMarkOwner
	marks astMarkBits
}

func NewASTExprMark(m *ASTManager) (*ASTExprMark, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTExprMark{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	v := &ASTExprMark{}
	v.owner.bind(m, bytes, id)
	return v, nil
}
func (v *ASTExprMark) IsMarked(h ASTHandle) (bool, error) {
	if v == nil {
		return false, ErrASTHandle
	}
	m, e := v.owner.lock()
	if e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	id, _, e := m.markID(h, true)
	if e != nil {
		return false, e
	}
	return v.marks.marked(id)
}
func (v *ASTExprMark) Mark(h ASTHandle, flag bool) error {
	if v == nil {
		return ErrASTHandle
	}
	m, e := v.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	id, _, e := m.markID(h, true)
	if e != nil {
		return e
	}
	return v.marks.mark(id, flag)
}
func (v *ASTExprMark) Reset() error {
	if v == nil {
		return ErrASTHandle
	}
	m, e := v.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	v.marks.reset()
	return nil
}
func (v *ASTExprMark) Close() error {
	if v == nil || v.owner.self != &v.owner {
		return ErrASTHandle
	}
	if v.owner.closed {
		return nil
	}
	m := v.owner.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	v.marks.close()
	v.owner.finish()
	return nil
}

type astMarkState struct {
	owner        astMarkOwner
	exprs, decls astMarkBits
}

func (s *astMarkState) marked(m *ASTManager, h ASTHandle) (bool, error) {
	id, decl, e := m.markID(h, false)
	if e != nil {
		return false, e
	}
	if decl {
		return s.decls.marked(id)
	}
	return s.exprs.marked(id)
}
func (s *astMarkState) mark(m *ASTManager, h ASTHandle, flag bool) error {
	id, decl, e := m.markID(h, false)
	if e != nil {
		return e
	}
	if decl {
		return s.decls.mark(id, flag)
	}
	return s.exprs.mark(id, flag)
}
func (s *astMarkState) reset() { s.decls.reset(); s.exprs.reset() }
func (s *astMarkState) close() { s.decls.close(); s.exprs.close(); s.owner.finish() }

type ASTMark struct{ state astMarkState }

func NewASTMark(m *ASTManager) (*ASTMark, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	bytes := uint64(reflect.TypeOf(ASTMark{}).Size())
	id, e := prepareASTMarkOwner(m, bytes)
	if e != nil {
		return nil, e
	}
	v := &ASTMark{}
	v.state.owner.bind(m, bytes, id)
	return v, nil
}
func (v *ASTMark) IsMarked(h ASTHandle) (bool, error) {
	if v == nil {
		return false, ErrASTHandle
	}
	m, e := v.state.owner.lock()
	if e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	return v.state.marked(m, h)
}
func (v *ASTMark) Mark(h ASTHandle, flag bool) error {
	if v == nil {
		return ErrASTHandle
	}
	m, e := v.state.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	return v.state.mark(m, h, flag)
}
func (v *ASTMark) Reset() error {
	if v == nil {
		return ErrASTHandle
	}
	m, e := v.state.owner.lock()
	if e != nil {
		return e
	}
	defer m.mu.Unlock()
	v.state.reset()
	return nil
}
func (v *ASTMark) Close() error {
	if v == nil || v.state.owner.self != &v.state.owner {
		return ErrASTHandle
	}
	if v.state.owner.closed {
		return nil
	}
	m := v.state.owner.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	v.state.close()
	return nil
}
