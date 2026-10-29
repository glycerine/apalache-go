package arena

import (
	"math"
	"reflect"
)

// Region64 appends typed records and supports nested marked rollback. It stores
// scalar handle logs and marks; its Slab retains generation history when reused.
// Regions are owner-local and not safe for concurrent use. No borrowed slices
// or pointers to record storage escape. Individual record recycling is absent.
type Region64 struct {
	storage      *Slab64
	log          []RegionRef
	marks        []regionMarkState
	identity     *regionIdentity
	serial       uint64
	budget       *Budget
	busy, closed bool
}

func NewRegion64(config Config) (*Region64, error) {
	s, e := NewSlab64(config)
	if e != nil {
		return nil, e
	}
	if e := config.Budget.Reserve(uint64(reflect.TypeOf(Region64{}).Size()) + uint64(reflect.TypeOf(regionIdentity{}).Size())); e != nil {
		s.Close()
		return nil, e
	}
	return &Region64{storage: s, budget: config.Budget, identity: &regionIdentity{1}}, nil
}
func (r *Region64) mutationError() error {
	if r.closed {
		return ErrClosed
	}
	if r.busy {
		return ErrReentrant
	}
	return nil
}

func (r *Region64) Allocate(value uint64) (RegionRef, error) {
	if e := r.mutationError(); e != nil {
		return RegionRef{}, e
	}
	if uint64(len(r.log)) == math.MaxUint32 {
		return RegionRef{}, ErrSlotLimit
	}
	log, e := reserveRegionRefs(r.log, r.budget)
	if e != nil {
		return RegionRef{}, e
	}
	r.log = log
	slot, gen, e := r.storage.Allocate()
	if e != nil {
		return RegionRef{}, e
	}
	r.storage.Write(slot, value)
	ref := RegionRef{slot, gen}
	r.log = append(r.log, ref)
	return ref, nil
}
func (r *Region64) IsLive(ref RegionRef) bool {
	return !r.closed && r.storage.IsLive(ref.Slot, ref.Generation)
}
func (r *Region64) Read(ref RegionRef) (uint64, error) {
	var zero uint64
	if r.closed {
		return zero, ErrClosed
	}
	if !r.IsLive(ref) {
		return zero, ErrInvalidSlot
	}
	return r.storage.Read(ref.Slot)
}
func (r *Region64) Write(ref RegionRef, value uint64) error {
	if e := r.mutationError(); e != nil {
		return e
	}
	if !r.IsLive(ref) {
		return ErrInvalidSlot
	}
	return r.storage.Write(ref.Slot, value)
}

func (r *Region64) Mark() (RegionMark, error) {
	if e := r.mutationError(); e != nil {
		return RegionMark{}, e
	}
	if r.serial == math.MaxUint64 || uint64(len(r.marks)) == math.MaxUint32 {
		return RegionMark{}, ErrSlotLimit
	}
	marks, e := reserveRegionMarks(r.marks, r.budget)
	if e != nil {
		return RegionMark{}, e
	}
	r.serial++
	r.marks = append(marks, regionMarkState{r.serial, uint32(len(r.log))})
	return RegionMark{r.identity, r.serial}, nil
}
func (r *Region64) Rollback(mark RegionMark, cleanup Cleanup64) error {
	if e := r.mutationError(); e != nil {
		return e
	}
	if mark.owner != r.identity || len(r.marks) == 0 || r.marks[len(r.marks)-1].serial != mark.serial {
		return ErrMark
	}
	return r.rollbackTo(r.marks[len(r.marks)-1].length, len(r.marks)-1, cleanup)
}

// PopScopes explicitly permits popping multiple nested scopes. Ordinary
// Rollback only accepts the top mark, guarding nested scratch owners.
func (r *Region64) PopScopes(count uint32, cleanup Cleanup64) error {
	if e := r.mutationError(); e != nil {
		return e
	}
	if uint64(count) > uint64(len(r.marks)) {
		return ErrMark
	}
	if count == 0 {
		return nil
	}
	keep := len(r.marks) - int(count)
	return r.rollbackTo(r.marks[keep].length, keep, cleanup)
}
func (r *Region64) rollbackTo(length uint32, scopes int, cleanup Cleanup64) error {
	r.busy = true
	defer func() { r.busy = false }()
	// No storage mutation before cleanup completes. A callback panic preserves
	// storage and mark state, but external callback effects cannot be undone;
	// retrying cleanup after panic is not guaranteed safe.
	if cleanup != nil {
		for i := len(r.log); i > int(length); i-- {
			ref := r.log[i-1]
			value, _ := r.storage.Read(ref.Slot)
			cleanup(ref, value)
		}
	}
	for i := len(r.log); i > int(length); i-- {
		r.storage.Recycle(r.log[i-1].Slot)
	}
	clear(r.log[int(length):])
	r.log = r.log[:int(length)]
	clear(r.marks[scopes:])
	r.marks = r.marks[:scopes]
	return nil
}

// Reset additionally undoes/cleans records before the first mark (level zero),
// matching trail_stack::reset's explicit base-level undo requirement.
func (r *Region64) Reset(cleanup Cleanup64) error {
	if e := r.mutationError(); e != nil {
		return e
	}
	return r.rollbackTo(0, 0, cleanup)
}
func (r *Region64) TrimFreePages() error {
	if e := r.mutationError(); e != nil {
		return e
	}
	r.storage.TrimFreePages()
	return nil
}
func (r *Region64) Close(cleanup Cleanup64) error {
	if r.closed {
		return nil
	}
	if r.busy {
		return ErrReentrant
	}
	if e := r.Reset(cleanup); e != nil {
		return e
	}
	r.storage.Close()
	r.budget.Release(uint64(reflect.TypeOf(Region64{}).Size()) + uint64(reflect.TypeOf(regionIdentity{}).Size()) + uint64(cap(r.log))*uint64(reflect.TypeOf(RegionRef{}).Size()) + uint64(cap(r.marks))*uint64(reflect.TypeOf(regionMarkState{}).Size()))
	r.log = nil
	r.marks = nil
	r.closed = true
	return nil
}
func (r *Region64) Stats() RegionStats {
	owner := uint64(0)
	if !r.closed {
		owner = uint64(reflect.TypeOf(Region64{}).Size()) + uint64(reflect.TypeOf(regionIdentity{}).Size())
	}
	return RegionStats{OwnerBytes: owner, Storage: r.storage.Stats(), Live: uint32(len(r.log)), Scopes: uint32(len(r.marks)), LogBytes: uint64(cap(r.log)) * uint64(reflect.TypeOf(RegionRef{}).Size()), MarkBytes: uint64(cap(r.marks)) * uint64(reflect.TypeOf(regionMarkState{}).Size())}
}
