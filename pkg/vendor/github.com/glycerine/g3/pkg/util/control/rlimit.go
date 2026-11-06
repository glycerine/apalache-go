package control

// Source: src/util/rlimit.h; src/util/rlimit.cpp.

import (
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/messages"
)

// The source OpenMP critical section is process-wide, including recursive
// propagation. Counts/scopes/suspension are confined to their operation owner;
// cancellation and child registration may run concurrently with increments.
var utilReslimitCancelMu sync.Mutex

// This is a cold resource service, with explicitly borrowed child services.
// Children must form an acyclic graph and outlive their registration. Do not
// copy an initialized service (atomic state and owned vectors would alias).
type Reslimit struct {
	cancelCount                atomic.Uint32
	suspended                  bool
	Steps, Limit               uint64
	Limits                     []uint64
	Children                   []*Reslimit
	budget                     *arena.Budget
	HeaderCharged, ClosedField bool
}

func NewReslimit(budget *arena.Budget) (*Reslimit, error) {
	if err := budget.Reserve(uint64(reflect.TypeOf(Reslimit{}).Size())); err != nil {
		return nil, err
	}
	return &Reslimit{budget: budget, HeaderCharged: true}, nil
}

func (r *Reslimit) InitEmbedded(budget *arena.Budget) { r.budget = budget }
func (r *Reslimit) StorageBytes() uint64 {
	return uint64(cap(r.Limits))*8 + uint64(cap(r.Children))*uint64(reflect.TypeOf((*Reslimit)(nil)).Size())
}
func (r *Reslimit) Closed() bool { return r.ClosedField }

func utilLimitCapacity(old int, width uint64) (int, error) {
	next := uint64(2)
	if old != 0 {
		next = (3*uint64(old) + 1) / 2
	}
	if next <= uint64(old) || next > uint64(^uint(0)>>1)/width || next > (uint64(^uint32(0))-8)/width {
		return 0, arena.ErrBounds
	}
	return int(next), nil
}

func (r *Reslimit) CheckOpen() {
	if r.ClosedField {
		panic("g3: closed resource limit")
	}
}
func (r *Reslimit) Count() uint64 { r.CheckOpen(); return r.Steps }
func (r *Reslimit) Inc() bool     { return r.IncOffset(1) }
func (r *Reslimit) IncOffset(offset uint32) bool {
	r.CheckOpen()
	r.Steps += uint64(offset)
	return (r.cancelCount.Load() == 0 && (r.Limit == 0 || r.Steps <= r.Limit)) || r.suspended
}
func (r *Reslimit) Push(delta uint32) error {
	r.CheckOpen()
	if len(r.Limits) == cap(r.Limits) {
		n, err := utilLimitCapacity(cap(r.Limits), 8)
		if err != nil {
			return err
		}
		if err := r.budget.Reserve(uint64(n) * 8); err != nil {
			return err
		}
		v := make([]uint64, len(r.Limits), n)
		copy(v, r.Limits)
		r.budget.Release(uint64(cap(r.Limits)) * 8)
		r.Limits = v
	}
	next := r.Steps + uint64(delta)
	if next <= r.Steps {
		next = 0
	}
	r.Limits = append(r.Limits, r.Limit)
	if r.Limit == 0 || next < r.Limit {
		r.Limit = next
	}
	// Deliberately local: source push/pop do not call set_cancel.
	r.cancelCount.Store(0)
	return nil
}
func (r *Reslimit) Pop() {
	r.CheckOpen()
	if len(r.Limits) == 0 {
		panic("g3: resource limit stack underflow")
	}
	if r.Steps > r.Limit && r.Limit > 0 {
		r.Steps = r.Limit
	}
	r.Limit = r.Limits[len(r.Limits)-1]
	r.Limits = r.Limits[:len(r.Limits)-1]
	r.cancelCount.Store(0)
}
func (r *Reslimit) CancelFlag() bool    { return r.cancelCount.Load() > 0 && !r.suspended }
func (r *Reslimit) CancelCount() uint32 { return r.cancelCount.Load() }
func (r *Reslimit) CancelMessage() string {
	if r.cancelCount.Load() > 0 {
		return messages.Canceled
	}
	return messages.MaxResource
}
func (r *Reslimit) PushChild(child *Reslimit) error {
	utilReslimitCancelMu.Lock()
	defer utilReslimitCancelMu.Unlock()
	r.CheckOpen()
	if child == nil || child == r {
		panic("g3: invalid resource limit child")
	}
	width := uint64(reflect.TypeOf((*Reslimit)(nil)).Size())
	if len(r.Children) == cap(r.Children) {
		n, err := utilLimitCapacity(cap(r.Children), width)
		if err != nil {
			return err
		}
		if err := r.budget.Reserve(uint64(n) * width); err != nil {
			return err
		}
		v := make([]*Reslimit, len(r.Children), n)
		copy(v, r.Children)
		r.budget.Release(uint64(cap(r.Children)) * width)
		r.Children = v
	}
	// Source registration does not copy the parent's existing cancel flag.
	r.Children = append(r.Children, child)
	return nil
}
func (r *Reslimit) PopChild() {
	utilReslimitCancelMu.Lock()
	defer utilReslimitCancelMu.Unlock()
	r.CheckOpen()
	if len(r.Children) == 0 {
		panic("g3: resource child stack underflow")
	}
	i := len(r.Children) - 1
	r.Children[i] = nil
	r.Children = r.Children[:i]
}
func (r *Reslimit) SetCancel(f uint32) {
	r.cancelCount.Store(f)
	for _, child := range r.Children {
		child.SetCancel(f)
	}
}
func (r *Reslimit) Cancel() { r.IncCancel() }
func (r *Reslimit) IncCancel() {
	utilReslimitCancelMu.Lock()
	defer utilReslimitCancelMu.Unlock()
	r.CheckOpen()
	r.SetCancel(r.cancelCount.Load() + 1)
}
func (r *Reslimit) DecCancel() {
	utilReslimitCancelMu.Lock()
	defer utilReslimitCancelMu.Unlock()
	r.CheckOpen()
	if f := r.cancelCount.Load(); f > 0 {
		r.SetCancel(f - 1)
	}
}
func (r *Reslimit) ResetCancel() {
	utilReslimitCancelMu.Lock()
	defer utilReslimitCancelMu.Unlock()
	r.CheckOpen()
	r.SetCancel(0)
}
func (r *Reslimit) Close() {
	utilReslimitCancelMu.Lock()
	defer utilReslimitCancelMu.Unlock()
	if r.ClosedField {
		return
	}
	r.budget.Release(uint64(cap(r.Limits)) * 8)
	r.budget.Release(uint64(cap(r.Children)) * uint64(reflect.TypeOf((*Reslimit)(nil)).Size()))
	if r.HeaderCharged {
		r.budget.Release(uint64(reflect.TypeOf(Reslimit{}).Size()))
	}
	r.Limits = nil
	r.Children = nil
	r.ClosedField = true
}

// Explicit close replaces each single C++ RAII destructor; repeated close is
// harmless. Scope values must not be copied while active. Use defer scope.close.
type LimitScope struct {
	owner  *Reslimit
	active bool
}

func NewLimitScope(r *Reslimit, delta uint32) (LimitScope, error) {
	if err := r.Push(delta); err != nil {
		return LimitScope{}, err
	}
	return LimitScope{r, true}, nil
}
func (s *LimitScope) Close() {
	if s.active {
		s.owner.Pop()
		s.active = false
	}
}

type SuspendLimit struct {
	owner  *Reslimit
	active bool
}

func NewSuspendLimit(r *Reslimit) SuspendLimit {
	r.CheckOpen()
	r.suspended = true
	return SuspendLimit{r, true}
}
func (s *SuspendLimit) Close() {
	if s.active {
		s.owner.suspended = false
		s.active = false
	}
}

type utilChildLimits struct {
	owner  *Reslimit
	size   uint32
	closed bool
}

func newUtilChildLimits(r *Reslimit) utilChildLimits { return utilChildLimits{owner: r} }
func (s *utilChildLimits) PushChild(r *Reslimit) error {
	if s.closed {
		panic("g3: closed child scope")
	}
	if s.size == ^uint32(0) {
		return arena.ErrBounds
	}
	if err := s.owner.PushChild(r); err != nil {
		return err
	}
	s.size++
	return nil
}
func (s *utilChildLimits) Close() {
	if s.closed {
		return
	}
	for s.size > 0 {
		s.owner.PopChild()
		s.size--
	}
	s.closed = true
}
