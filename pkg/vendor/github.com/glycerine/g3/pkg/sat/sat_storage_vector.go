package sat

// Concrete native slices used by retained SAT consumers.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type satClauseVector struct {
	self       *satClauseVector
	data       []StoredClauseRef
	budget     *arena.Budget
	ownerBytes uint64
	closed     bool
	allocator  *ClauseAllocator
	bindings   map[uint64]*ClauseAllocator
}

func NewSATClauseVector(a *ClauseAllocator, b *arena.Budget) (*satClauseVector, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	return newsatClauseVector(a, b)
}
func newsatClauseVector(a *ClauseAllocator, b *arena.Budget) (*satClauseVector, error) {
	bytes := uint64(reflect.TypeOf(satClauseVector{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	v := &satClauseVector{budget: b, ownerBytes: bytes, allocator: a}
	v.self = v
	return v, nil
}
func (v *satClauseVector) open() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	return nil
}
func (v *satClauseVector) Size() uint32 {
	if v == nil || v.closed {
		return 0
	}
	return uint32(len(v.data))
}
func (v *satClauseVector) Capacity() uint64 {
	if v == nil || v.closed {
		return 0
	}
	return uint64(cap(v.data))
}
func (v *satClauseVector) Get(i uint32) (StoredClauseRef, error) {
	if e := v.open(); e != nil {
		return StoredClauseRef{}, e
	}
	if uint64(i) >= uint64(len(v.data)) {
		if satStorageSourceDebug {
			return StoredClauseRef{}, ErrSATAssertion
		}
		return StoredClauseRef{}, arena.ErrBounds
	}
	return v.data[i], nil
}
func (v *satClauseVector) Push(x StoredClauseRef) error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = append(v.data, x)
	return nil
}
func (v *satClauseVector) Close() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return nil
	}
	v.data = nil
	v.bindings = nil
	v.budget.Release(v.ownerBytes)
	v.ownerBytes = 0
	v.closed = true
	return nil
}
func (v *satClauseVector) Stats() satStorageVectorStats {
	return satStorageVectorStats{OwnerBytes: v.ownerBytes}
}
func (v *satClauseVector) Set(i uint32, x StoredClauseRef) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.data)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	v.data[i] = x
	return nil
}
func (v *satClauseVector) Reset() error {
	if e := v.open(); e != nil {
		return e
	}
	clear(v.data)
	v.data = v.data[:0]
	return nil
}
func (v *satClauseVector) Finalize() error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = nil
	return nil
}
func (v *satClauseVector) Contains(x StoredClauseRef) (bool, error) {
	if e := v.open(); e != nil {
		return false, e
	}
	for _, a := range v.data {
		if a == x {
			return true, nil
		}
	}
	return false, nil
}
func (v *satClauseVector) Erase(x StoredClauseRef) (bool, error) {
	if e := v.open(); e != nil {
		return false, e
	}
	for i, a := range v.data {
		if a == x {
			copy(v.data[i:], v.data[i+1:])
			v.data[len(v.data)-1] = StoredClauseRef{}
			v.data = v.data[:len(v.data)-1]
			return true, nil
		}
	}
	return false, nil
}
func (v *satClauseVector) Shrink(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(n) > uint64(len(v.data)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	clear(v.data[n:])
	v.data = v.data[:n]
	return nil
}
func (v *satClauseVector) bindRetained(a *ClauseAllocator) error {
	if a == nil || a.self != a || a.origin == 0 {
		return ErrSATOwner
	}
	if a == v.allocator {
		return nil
	}
	if v.bindings == nil {
		v.bindings = make(map[uint64]*ClauseAllocator)
	}
	v.bindings[a.origin] = a
	return nil
}
func (v *satClauseVector) BindAllocator(a *ClauseAllocator) error {
	if e := v.open(); e != nil {
		return e
	}
	if e := a.open(); e != nil {
		return e
	}
	return v.bindRetained(a)
}
func (v *satClauseVector) resolve(origin uint64) (*ClauseAllocator, error) {
	if v.allocator.origin == origin {
		return v.allocator, nil
	}
	if a := v.bindings[origin]; a != nil {
		return a, nil
	}
	return nil, ErrSATOwner
}
func (v *satClauseVector) Assign(src *satClauseVector) error {
	if e := v.open(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	if v == src {
		return nil
	}
	if e := v.bindRetained(src.allocator); e != nil {
		return e
	}
	for _, a := range src.bindings {
		if e := v.bindRetained(a); e != nil {
			return e
		}
	}
	clear(v.data)
	v.data = append(v.data[:0], src.data...)
	return nil
}
func (v *satClauseVector) Clone(b *arena.Budget) (*satClauseVector, error) {
	if e := v.open(); e != nil {
		return nil, e
	}
	x, e := newsatClauseVector(v.allocator, b)
	if e != nil {
		return nil, e
	}
	if e = x.Assign(v); e != nil {
		x.Close()
		return nil, e
	}
	return x, nil
}

type satClauseWrapperVector struct {
	self       *satClauseWrapperVector
	data       []satClauseWrapper
	budget     *arena.Budget
	ownerBytes uint64
	closed     bool
}

func NewSATClauseWrapperVector(a *ClauseAllocator, b *arena.Budget) (*satClauseWrapperVector, error) {
	if e := a.open(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(satClauseWrapperVector{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	v := &satClauseWrapperVector{budget: b, ownerBytes: bytes}
	v.self = v
	return v, nil
}
func (v *satClauseWrapperVector) open() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	return nil
}
func (v *satClauseWrapperVector) Size() uint32 {
	if v == nil || v.closed {
		return 0
	}
	return uint32(len(v.data))
}
func (v *satClauseWrapperVector) Capacity() uint64 {
	if v == nil || v.closed {
		return 0
	}
	return uint64(cap(v.data))
}
func (v *satClauseWrapperVector) Get(i uint32) (satClauseWrapper, error) {
	if e := v.open(); e != nil {
		return satClauseWrapper{}, e
	}
	if uint64(i) >= uint64(len(v.data)) {
		if satStorageSourceDebug {
			return satClauseWrapper{}, ErrSATAssertion
		}
		return satClauseWrapper{}, arena.ErrBounds
	}
	return v.data[i], nil
}
func (v *satClauseWrapperVector) Push(x satClauseWrapper) error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = append(v.data, x)
	return nil
}
func (v *satClauseWrapperVector) Close() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return nil
	}
	v.data = nil
	v.budget.Release(v.ownerBytes)
	v.ownerBytes = 0
	v.closed = true
	return nil
}
func (v *satClauseWrapperVector) Stats() satStorageVectorStats {
	return satStorageVectorStats{OwnerBytes: v.ownerBytes}
}
func (v *satClauseWrapperVector) Reset() error {
	if e := v.open(); e != nil {
		return e
	}
	clear(v.data)
	v.data = v.data[:0]
	return nil
}
func (v *satClauseWrapperVector) Finalize() error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = nil
	return nil
}
func (v *satClauseWrapperVector) Back() (satClauseWrapper, error) {
	if e := v.open(); e != nil {
		return satClauseWrapper{}, e
	}
	if len(v.data) == 0 {
		if satStorageSourceDebug {
			return satClauseWrapper{}, ErrSATAssertion
		}
		return satClauseWrapper{}, arena.ErrBounds
	}
	return v.data[len(v.data)-1], nil
}

type satActivityDomain struct {
	self       *satActivityDomain
	data       []uint32
	budget     *arena.Budget
	ownerBytes uint64
	closed     bool
}

func NewSATActivityDomain(b *arena.Budget) (*satActivityDomain, error) {
	bytes := uint64(reflect.TypeOf(satActivityDomain{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	v := &satActivityDomain{budget: b, ownerBytes: bytes}
	v.self = v
	return v, nil
}
func (v *satActivityDomain) open() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	return nil
}
func (v *satActivityDomain) Size() uint32 {
	if v == nil || v.closed {
		return 0
	}
	return uint32(len(v.data))
}
func (v *satActivityDomain) Capacity() uint64 {
	if v == nil || v.closed {
		return 0
	}
	return uint64(cap(v.data))
}
func (v *satActivityDomain) Get(i uint32) (uint32, error) {
	if e := v.open(); e != nil {
		return 0, e
	}
	if uint64(i) >= uint64(len(v.data)) {
		if satStorageSourceDebug {
			return 0, ErrSATAssertion
		}
		return 0, arena.ErrBounds
	}
	return v.data[i], nil
}
func (v *satActivityDomain) Push(x uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = append(v.data, x)
	return nil
}
func (v *satActivityDomain) Close() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return nil
	}
	v.data = nil
	v.budget.Release(v.ownerBytes)
	v.ownerBytes = 0
	v.closed = true
	return nil
}
func (v *satActivityDomain) Stats() satStorageVectorStats {
	return satStorageVectorStats{OwnerBytes: v.ownerBytes}
}
func (v *satActivityDomain) Set(i uint32, x uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(i) >= uint64(len(v.data)) {
		if satStorageSourceDebug {
			return ErrSATAssertion
		}
		return arena.ErrBounds
	}
	v.data[i] = x
	return nil
}
func (v *satActivityDomain) Len() uint32 { return v.Size() }
func (v *satActivityDomain) Resize(n uint32) error {
	if e := v.open(); e != nil {
		return e
	}
	if uint64(n) <= uint64(len(v.data)) {
		clear(v.data[n:])
		v.data = v.data[:n]
	} else {
		v.data = append(v.data, make([]uint32, int(n)-len(v.data))...)
	}
	return nil
}

type satBinClauseVector struct {
	self       *satBinClauseVector
	data       []satBinClause
	budget     *arena.Budget
	ownerBytes uint64
	closed     bool
}

func NewSATBinClauseVector(b *arena.Budget) (*satBinClauseVector, error) {
	bytes := uint64(reflect.TypeOf(satBinClauseVector{}).Size())
	if e := b.Reserve(bytes); e != nil {
		return nil, e
	}
	v := &satBinClauseVector{budget: b, ownerBytes: bytes}
	v.self = v
	return v, nil
}
func (v *satBinClauseVector) open() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	return nil
}
func (v *satBinClauseVector) Size() uint32 {
	if v == nil || v.closed {
		return 0
	}
	return uint32(len(v.data))
}
func (v *satBinClauseVector) Capacity() uint64 {
	if v == nil || v.closed {
		return 0
	}
	return uint64(cap(v.data))
}
func (v *satBinClauseVector) Get(i uint32) (satBinClause, error) {
	if e := v.open(); e != nil {
		return satBinClause{}, e
	}
	if uint64(i) >= uint64(len(v.data)) {
		if satStorageSourceDebug {
			return satBinClause{}, ErrSATAssertion
		}
		return satBinClause{}, arena.ErrBounds
	}
	return v.data[i], nil
}
func (v *satBinClauseVector) Push(x satBinClause) error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = append(v.data, x)
	return nil
}
func (v *satBinClauseVector) Close() error {
	if v == nil || v.self != v {
		return ErrSATOwner
	}
	if v.closed {
		return nil
	}
	v.data = nil
	v.budget.Release(v.ownerBytes)
	v.ownerBytes = 0
	v.closed = true
	return nil
}
func (v *satBinClauseVector) Stats() satStorageVectorStats {
	return satStorageVectorStats{OwnerBytes: v.ownerBytes}
}
func (v *satBinClauseVector) Reset() error {
	if e := v.open(); e != nil {
		return e
	}
	clear(v.data)
	v.data = v.data[:0]
	return nil
}
func (v *satBinClauseVector) Finalize() error {
	if e := v.open(); e != nil {
		return e
	}
	v.data = nil
	return nil
}
func (v *satBinClauseVector) Back() (satBinClause, error) {
	if e := v.open(); e != nil {
		return satBinClause{}, e
	}
	if len(v.data) == 0 {
		if satStorageSourceDebug {
			return satBinClause{}, ErrSATAssertion
		}
		return satBinClause{}, arena.ErrBounds
	}
	return v.data[len(v.data)-1], nil
}
func (v *satBinClauseVector) Pop() (satBinClause, error) {
	x, e := v.Back()
	if e != nil {
		return x, e
	}
	v.data[len(v.data)-1] = satBinClause{}
	v.data = v.data[:len(v.data)-1]
	return x, nil
}
func (v *satBinClauseVector) Erase(x satBinClause) (bool, error) {
	if e := v.open(); e != nil {
		return false, e
	}
	for i, a := range v.data {
		if a.Equal(x) {
			copy(v.data[i:], v.data[i+1:])
			v.data[len(v.data)-1] = satBinClause{}
			v.data = v.data[:len(v.data)-1]
			return true, nil
		}
	}
	return false, nil
}

type satStorageVectorStats struct{ OwnerBytes, BackingBytes, BindingBytes uint64 }
