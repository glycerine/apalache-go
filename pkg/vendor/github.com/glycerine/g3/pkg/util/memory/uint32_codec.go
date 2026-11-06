package memory

// Explicit little-endian word views over the sole initialized-byte backing.

import (
	"github.com/glycerine/g3/pkg/arena"
)

// ReadUint32 reads one initialized word, without borrowing a backing slice.
// The complete word range is checked at this access, with no offset overflow.

func (m *Runtime) ReadUint32(ref Block, offset uint64) (uint32, error) {
	b, e := m.block(ref)
	if e != nil {
		return 0, e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return 0, e
	}
	header := uint64(m.config.SizeTBits / 8)
	extent := n - header
	if offset > extent || extent-offset < 4 {
		return 0, arena.ErrBounds
	}
	at := header + offset
	var word uint32
	for i := uint64(0); i < 4; i++ {
		byteAt := at + i
		if b.initialized[byteAt/64]&(uint64(1)<<(byteAt%64)) == 0 {
			return 0, ErrRuntimeUninitialized
		}
		word |= uint32(b.data[byteAt]) << (8 * i)
	}
	return word, nil
}

// WriteUint32 validates the whole word before publishing its four bytes.
// It neither reserves storage nor alters source allocation counters.
func (m *Runtime) WriteUint32(ref Block, offset uint64, word uint32) error {
	b, e := m.block(ref)
	if e != nil {
		return e
	}
	n, e := m.headerRead(b)
	if e != nil {
		return e
	}
	header := uint64(m.config.SizeTBits / 8)
	extent := n - header
	if offset > extent || extent-offset < 4 {
		return arena.ErrBounds
	}
	at := header + offset
	for i := uint64(0); i < 4; i++ {
		byteAt := at + i
		b.data[byteAt] = byte(word >> (8 * i))
		b.initialized[byteAt/64] |= uint64(1) << (byteAt % 64)
	}
	return nil
}

// ReadUint32 retains the allocator byte API's requested-size bound. Source
// typed-object views can instead use ObjectExtent and the actual raw location.
func (a *SmallAllocator) ReadUint32(ref ObjectRef, offset uint64) (uint32, error) {
	if e := a.open(); e != nil {
		return 0, e
	}
	loc, e := a.object(ref)
	if e != nil {
		return 0, e
	}
	if offset > loc.requested || loc.requested-offset < 4 {
		return 0, arena.ErrBounds
	}
	return a.config.Runtime.ReadUint32(loc.raw, uint64(loc.offset)+offset)
}

func (a *SmallAllocator) WriteUint32(ref ObjectRef, offset uint64, word uint32) error {
	if e := a.open(); e != nil {
		return e
	}
	loc, e := a.object(ref)
	if e != nil {
		return e
	}
	if offset > loc.requested || loc.requested-offset < 4 {
		return arena.ErrBounds
	}
	return a.config.Runtime.WriteUint32(loc.raw, uint64(loc.offset)+offset, word)
}

// ObjectExtent reports actual allocated storage: the rounded class block for a
// pooled object, or the stored runtime extent for a direct object. It does not
// widen Read/Write requested bounds or permit access past the object's lifetime.
func (a *SmallAllocator) ObjectExtent(ref ObjectRef) (uint64, error) {
	if e := a.open(); e != nil {
		return 0, e
	}
	loc, e := a.object(ref)
	if e != nil {
		return 0, e
	}
	if loc.kind == smallObjectPooled {
		return uint64(loc.extent), nil
	}
	return a.config.Runtime.Size(loc.raw)
}
