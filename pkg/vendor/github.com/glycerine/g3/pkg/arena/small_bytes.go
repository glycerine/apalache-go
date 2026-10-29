package arena

import (
	"errors"
	"reflect"
)

var ErrSizeMismatch = errors.New("arena: deallocation size differs from allocation")

type SmallBytesRef struct {
	Slot       Slot
	Generation Generation
}
type SmallBytesConfig struct {
	// Alignment is the source target pointer width (4 or8), default8. It is a
	// size-class policy, not a Go address alignment guarantee. ChunkSize defaults
	// to8192-2*Alignment; smaller values exercise source boundaries in tests.
	Alignment, ChunkSize uint32
	// DirectOnly selects the source Z3DEBUG && !WINDOWS diagnostic bypass:
	// every nonzero payload is separately allocated and immediately freed.
	DirectOnly  bool
	Descriptors Config
}
type smallByteBlock struct {
	next uint64
	free bool
}
type smallByteChunk struct {
	data                []byte
	blocks              []smallByteBlock
	class, cursor, free uint32
}
type SmallBytesStats struct {
	// OwnerBytes includes this header and the private descriptor owner header.
	OwnerBytes                                                                            uint64
	Live, Chunks, FreeObjects, DirectObjects                                              uint32
	RequestedBytes, WastedBytes, PayloadBytes, MetadataBytes, DirectoryBytes, DirectBytes uint64
	// SourceAllocationBytes matches source m_alloc_size, which remains zero in
	// the diagnostic bypass. RequestedBytes measures owned live bytes in both modes.
	SourceAllocationBytes uint64
}

// SmallBytes ports release-build small_object_allocator pooled/direct storage.
// DirectOnly selects the diagnostic direct-allocation bypass. Each
// owner and Budget is single-threaded. Caller destroys logical objects before
// Deallocate/Reset/Close; this storage invokes no domain callbacks.
// Reset/Close also reclaim tracked live direct blocks: an explicit owner cleanup
// extension to source, which only reclaims pooled chunks and leaves direct
// allocation cleanup to its callers. Callers must first release logical resources.
// No borrowed slice escapes. Pooled data after Deallocate is unspecified.
// Chunk indices replace raw addresses for deterministic consolidation order;
// released conceptual addresses are reusable, without moving live contents.
// Budget charges all backing arrays and both cold owner headers.
type SmallBytes struct {
	descriptors               *descriptorSlab
	chunks                    []smallByteChunk
	config                    SmallBytesConfig
	latest                    [64]uint32
	heads                     [64]uint64
	freeCounts                [64]uint32
	requested, directoryBytes uint64
	closed                    bool
}

func NewSmallBytes(c SmallBytesConfig) (*SmallBytes, error) {
	if c.Alignment == 0 {
		c.Alignment = 8
	}
	if c.Alignment != 4 && c.Alignment != 8 {
		return nil, ErrConfig
	}
	if c.ChunkSize == 0 {
		c.ChunkSize = 8192 - 2*c.Alignment
	}
	maxInt := uint64(^uint(0) >> 1)
	blockBytes := uint64(reflect.TypeOf(smallByteBlock{}).Size())
	if c.ChunkSize < c.Alignment || uint64(c.ChunkSize) > maxInt || uint64(c.ChunkSize/c.Alignment) > maxInt/blockBytes {
		return nil, ErrConfig
	}
	if e := ValidateRecordType(reflect.TypeOf(smallByteBlock{})); e != nil {
		return nil, e
	}
	d, e := newDescriptorSlab(c.Descriptors)
	if e != nil {
		return nil, e
	}
	if e := c.Descriptors.Budget.Reserve(uint64(reflect.TypeOf(SmallBytes{}).Size())); e != nil {
		d.Close()
		return nil, e
	}
	return &SmallBytes{descriptors: d, config: c}, nil
}
func (a *SmallBytes) descriptor(r SmallBytesRef) (vectorDescriptor, error) {
	if a.closed {
		return vectorDescriptor{}, ErrClosed
	}
	if r == (SmallBytesRef{}) {
		return vectorDescriptor{}, nil
	}
	if !a.descriptors.IsLive(r.Slot, r.Generation) {
		return vectorDescriptor{}, ErrInvalidSlot
	}
	return a.descriptors.Read(r.Slot)
}
func (a *SmallBytes) IsLive(r SmallBytesRef) bool {
	return !a.closed && r != (SmallBytesRef{}) && a.descriptors.IsLive(r.Slot, r.Generation)
}
func (a *SmallBytes) Size(r SmallBytesRef) (uint32, error) {
	d, e := a.descriptor(r)
	return d.length, e
}
func (a *SmallBytes) Read(r SmallBytesRef, i uint32) (byte, error) {
	d, e := a.descriptor(r)
	if e != nil {
		return 0, e
	}
	if i >= d.length {
		return 0, ErrBounds
	}
	return a.chunks[d.page].data[d.offset+i], nil
}
func (a *SmallBytes) Write(r SmallBytesRef, i uint32, v byte) error {
	d, e := a.descriptor(r)
	if e != nil {
		return e
	}
	if i >= d.length {
		return ErrBounds
	}
	a.chunks[d.page].data[d.offset+i] = v
	return nil
}
func smallByteLocation(page, block uint32) uint64 { return uint64(page+1)<<32 | uint64(block+1) }
func smallByteDecode(id uint64) (uint32, uint32)  { return uint32(id>>32) - 1, uint32(id) - 1 }
func (a *SmallBytes) newChunk(class, directSize uint32) (uint32, error) {
	size := class * a.config.Alignment
	if class != 0 && size > a.config.ChunkSize {
		return 0, ErrConfig
	}
	page := uint32(len(a.chunks))
	for i := range a.chunks {
		if a.chunks[i].data == nil {
			page = uint32(i)
			break
		}
	}
	var newDir []smallByteChunk
	var dirBytes uint64
	if int(page) == len(a.chunks) && len(a.chunks) == cap(a.chunks) {
		n := cap(a.chunks) * 2
		if n == 0 {
			n = 1
		}
		entry := uint64(reflect.TypeOf(smallByteChunk{}).Size())
		if n <= cap(a.chunks) || uint64(n) > uint64(^uint32(0)) || uint64(n) > uint64(^uint(0)>>1)/entry {
			return 0, ErrSlotLimit
		}
		dirBytes = uint64(n) * entry
		if e := a.config.Descriptors.Budget.Reserve(dirBytes); e != nil {
			return 0, e
		}
		newDir = make([]smallByteChunk, len(a.chunks), n)
		copy(newDir, a.chunks)
	}
	payload := a.config.ChunkSize
	count := uint32(0)
	if class == 0 {
		payload = directSize
	} else {
		count = payload / size
	}
	if uint64(payload) > uint64(^uint(0)>>1) {
		a.config.Descriptors.Budget.Release(dirBytes)
		return 0, ErrConfig
	}
	bytes := uint64(payload) + uint64(count)*uint64(reflect.TypeOf(smallByteBlock{}).Size())
	if e := a.config.Descriptors.Budget.Reserve(bytes); e != nil {
		a.config.Descriptors.Budget.Release(dirBytes)
		return 0, e
	}
	c := smallByteChunk{data: make([]byte, payload), blocks: make([]smallByteBlock, count), class: class}
	if newDir != nil {
		a.config.Descriptors.Budget.Release(a.directoryBytes)
		a.directoryBytes = dirBytes
		a.chunks = newDir
	}
	if int(page) == len(a.chunks) {
		a.chunks = append(a.chunks, c)
	} else {
		a.chunks[page] = c
	}
	if class != 0 {
		a.latest[class] = page + 1
	}
	return page, nil
}
func (a *SmallBytes) Allocate(size uint32) (SmallBytesRef, error) {
	if a.closed {
		return SmallBytesRef{}, ErrClosed
	}
	if size == 0 {
		return SmallBytesRef{}, nil
	}
	if a.config.DirectOnly || size >= 256-a.config.Alignment {
		slot, g, e := a.descriptors.Allocate()
		if e != nil {
			return SmallBytesRef{}, e
		}
		page, e := a.newChunk(0, size)
		if e != nil {
			a.descriptors.Recycle(slot)
			return SmallBytesRef{}, e
		}
		a.descriptors.Write(slot, vectorDescriptor{page: page, length: size, capacity: size})
		a.requested += uint64(size)
		return SmallBytesRef{slot, g}, nil
	}
	class := (size + a.config.Alignment - 1) / a.config.Alignment
	capacity := class * a.config.Alignment
	if capacity > a.config.ChunkSize {
		return SmallBytesRef{}, ErrConfig
	}
	slot, g, e := a.descriptors.Allocate()
	if e != nil {
		return SmallBytesRef{}, e
	}
	var page, offset uint32
	if head := a.heads[class]; head != 0 {
		var block uint32
		page, block = smallByteDecode(head)
		c := &a.chunks[page]
		m := &c.blocks[block]
		a.heads[class] = m.next
		m.next = 0
		m.free = false
		c.free--
		a.freeCounts[class]--
		offset = block * capacity
	} else {
		if id := a.latest[class]; id != 0 {
			page = id - 1
		} else {
			page = uint32(len(a.chunks))
		}
		if a.latest[class] == 0 || uint64(a.chunks[page].cursor)+uint64(capacity) >= uint64(a.config.ChunkSize) {
			page, e = a.newChunk(class, 0)
			if e != nil {
				a.descriptors.Recycle(slot)
				return SmallBytesRef{}, e
			}
		}
		c := &a.chunks[page]
		offset = c.cursor
		c.cursor += capacity
	}
	a.descriptors.Write(slot, vectorDescriptor{page: page, offset: offset, length: size, capacity: capacity})
	a.requested += uint64(size)
	return SmallBytesRef{slot, g}, nil
}
func (a *SmallBytes) Deallocate(size uint32, r SmallBytesRef) error {
	if a.closed {
		return ErrClosed
	}
	if size == 0 {
		return nil
	}
	d, e := a.descriptor(r)
	if e != nil {
		return e
	}
	if r == (SmallBytesRef{}) {
		return ErrInvalidSlot
	}
	if size != d.length {
		return ErrSizeMismatch
	}
	if a.chunks[d.page].class == 0 {
		a.releaseChunk(int(d.page))
		a.requested -= uint64(size)
		return a.descriptors.Recycle(r.Slot)
	}
	class := d.capacity / a.config.Alignment
	c := &a.chunks[d.page]
	block := d.offset / d.capacity
	m := &c.blocks[block]
	m.free = true
	m.next = a.heads[class]
	a.heads[class] = smallByteLocation(d.page, block)
	c.free++
	a.freeCounts[class]++
	a.requested -= uint64(size)
	return a.descriptors.Recycle(r.Slot)
}
func (a *SmallBytes) releaseChunk(i int) {
	c := &a.chunks[i]
	a.config.Descriptors.Budget.Release(uint64(len(c.data)) + uint64(len(c.blocks))*uint64(reflect.TypeOf(smallByteBlock{}).Size()))
	*c = smallByteChunk{}
}

// Consolidate translates the source floor-count criterion exactly, including
// retained wholly dead chunks when strict endpoint allocation cannot fill that
// count. It reclaims only chunks with no live allocation; it never relocates.
// Source address-sort is conceptual chunk/block-index sort. Rebuilding links
// needs no allocated scratch and cannot fail after validation.
func (a *SmallBytes) Consolidate() error {
	if a.closed {
		return ErrClosed
	}
	for class := uint32(1); class < 256/a.config.Alignment; class++ {
		if a.freeCounts[class] == 0 {
			continue
		}
		expected := a.config.ChunkSize / (class * a.config.Alignment)
		if expected == 0 || a.freeCounts[class] < expected {
			continue
		}
		a.heads[class] = 0
		a.latest[class] = 0
		for i := range a.chunks {
			c := &a.chunks[i]
			if c.data == nil || c.class != class {
				continue
			}
			if c.free == expected {
				a.freeCounts[class] -= c.free
				a.releaseChunk(i)
				continue
			}
			a.latest[class] = uint32(i) + 1
			for block := uint32(0); block < c.cursor/(class*a.config.Alignment); block++ {
				m := &c.blocks[block]
				if m.free {
					m.next = a.heads[class]
					a.heads[class] = smallByteLocation(uint32(i), block)
				}
			}
		}
	}
	return nil
}
func (a *SmallBytes) Reset() {
	if a.closed {
		return
	}
	for i := range a.chunks {
		if a.chunks[i].data != nil {
			a.releaseChunk(i)
		}
	}
	a.chunks = a.chunks[:0]
	a.latest = [64]uint32{}
	a.heads = [64]uint64{}
	a.freeCounts = [64]uint32{}
	a.requested = 0
	a.descriptors.Reset()
}
func (a *SmallBytes) Close() {
	if a.closed {
		return
	}
	a.Reset()
	a.descriptors.Close()
	a.config.Descriptors.Budget.Release(a.directoryBytes + uint64(reflect.TypeOf(SmallBytes{}).Size()))
	a.directoryBytes = 0
	a.chunks = nil
	a.closed = true
}
func (a *SmallBytes) Stats() SmallBytesStats {
	s := SmallBytesStats{RequestedBytes: a.requested, DirectoryBytes: a.directoryBytes}
	if !a.config.DirectOnly {
		s.SourceAllocationBytes = a.requested
	}
	d := a.descriptors.Stats()
	s.OwnerBytes = d.OwnerBytes
	if !a.closed {
		s.OwnerBytes += uint64(reflect.TypeOf(SmallBytes{}).Size())
	}
	s.Live = d.Live
	s.MetadataBytes = d.RecordBytes + d.MetadataBytes
	s.DirectoryBytes += d.DirectoryBytes
	for i := range a.chunks {
		c := &a.chunks[i]
		if c.data == nil {
			continue
		}
		if c.class == 0 {
			s.DirectObjects++
			s.DirectBytes += uint64(len(c.data))
		} else {
			s.Chunks++
		}
		s.FreeObjects += c.free
		s.WastedBytes += uint64(c.free) * uint64(c.class*a.config.Alignment)
		s.PayloadBytes += uint64(len(c.data))
		s.MetadataBytes += uint64(len(c.blocks)) * uint64(reflect.TypeOf(smallByteBlock{}).Size())
	}
	return s
}
