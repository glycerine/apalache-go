package arena

import (
	"math"
	"reflect"
)

type wordPage struct {
	data                      []uint32
	next                      []uint32
	class, blocks, head, used uint32
}

// Words pools pointer-free payloads in geometric capacity classes. PageSize
// is an element count (default 16384). Descriptors.Budget accounts all backing
// arrays and metadata. Constructors account cold owner headers; caller scratch/cleanup stays external.
// No backing slice escapes. Ownership is single-threaded and owner-local.
type Words struct {
	descriptors                  *descriptorSlab
	pages                        []wordPage
	config                       VectorConfig
	elementBytes, directoryBytes uint64
	closed                       bool
}

func NewWords(config VectorConfig) (*Words, error) {
	if !pointerFree(reflect.TypeOf(uint32(0))) {
		return nil, ErrPointerful
	}
	if config.PageSize == 0 {
		config.PageSize = 16384
	}
	size := uint64(reflect.TypeOf(uint32(0)).Size())
	maxInt := uint64(^uint(0) >> 1)
	if config.PageSize&(config.PageSize-1) != 0 || uint64(config.PageSize) > maxInt/4 || (size != 0 && uint64(config.PageSize) > maxInt/size) {
		return nil, ErrConfig
	}
	d, e := newDescriptorSlab(config.Descriptors)
	if e != nil {
		return nil, e
	}
	if e := config.Descriptors.Budget.Reserve(uint64(reflect.TypeOf(Words{}).Size())); e != nil {
		d.Close()
		return nil, e
	}
	return &Words{descriptors: d, config: config, elementBytes: size}, nil
}
func (v *Words) descriptor(ref VectorRef) (vectorDescriptor, error) {
	if v.closed {
		return vectorDescriptor{}, ErrClosed
	}
	if ref == (VectorRef{}) {
		return vectorDescriptor{}, nil
	}
	if !v.descriptors.IsLive(ref.Slot, ref.Generation) {
		return vectorDescriptor{}, ErrInvalidSlot
	}
	return v.descriptors.Read(ref.Slot)
}
func (v *Words) IsLive(ref VectorRef) bool {
	return !v.closed && ref != (VectorRef{}) && v.descriptors.IsLive(ref.Slot, ref.Generation)
}
func (v *Words) Len(ref VectorRef) (uint32, error) {
	d, e := v.descriptor(ref)
	return d.length, e
}
func (v *Words) Cap(ref VectorRef) (uint32, error) {
	d, e := v.descriptor(ref)
	return d.capacity, e
}
func (v *Words) Read(ref VectorRef, index uint32) (uint32, error) {
	var zero uint32
	d, e := v.descriptor(ref)
	if e != nil {
		return zero, e
	}
	if index >= d.length {
		return zero, ErrBounds
	}
	return v.pages[d.page].data[d.offset+index], nil
}
func (v *Words) Write(ref VectorRef, index uint32, value uint32) error {
	d, e := v.descriptor(ref)
	if e != nil {
		return e
	}
	if index >= d.length {
		return ErrBounds
	}
	v.pages[d.page].data[d.offset+index] = value
	return nil
}

func (v *Words) initializePage(p *wordPage) {
	count := p.class * p.blocks
	p.data = make([]uint32, count)
	p.next = make([]uint32, p.blocks)
	for i := uint32(0); i+1 < p.blocks; i++ {
		p.next[i] = i + 2
	}
	p.head = 1
}

// takeBlock allocates metadata and payload before publishing a free-list change.
func (v *Words) takeBlock(class uint32) (uint32, uint32, error) {
	maxInt := uint64(^uint(0) >> 1)
	if uint64(class) > maxInt || (v.elementBytes != 0 && uint64(class) > maxInt/v.elementBytes) {
		return 0, 0, ErrBounds
	}
	for i := range v.pages {
		p := &v.pages[i]
		if p.class != class || p.used == p.blocks {
			continue
		}
		if p.data == nil {
			amount := uint64(p.class)*uint64(p.blocks)*v.elementBytes + uint64(p.blocks)*4
			if e := v.config.Descriptors.Budget.Reserve(amount); e != nil {
				return 0, 0, e
			}
			v.initializePage(p)
		}
		block := p.head - 1
		p.head = p.next[block]
		p.used++
		return uint32(i), block * class, nil
	}
	blocks := uint32(1)
	if class < v.config.PageSize {
		blocks = v.config.PageSize / class
	}
	if uint64(len(v.pages)) >= math.MaxUint32 {
		return 0, 0, ErrSlotLimit
	}
	newCap := cap(v.pages)
	newDir := uint64(0)
	if len(v.pages) == newCap {
		if newCap == 0 {
			newCap = 1
		} else {
			if newCap > int(^uint(0)>>1)/2 {
				return 0, 0, ErrBounds
			}
			newCap *= 2
		}
		entrySize := uint64(reflect.TypeOf(wordPage{}).Size())
		if uint64(newCap) > maxInt/entrySize {
			return 0, 0, ErrBounds
		}
		newDir = uint64(newCap) * entrySize
	}
	amount := uint64(class)*uint64(blocks)*v.elementBytes + uint64(blocks)*4 + newDir
	if e := v.config.Descriptors.Budget.Reserve(amount); e != nil {
		return 0, 0, e
	}
	if newDir != 0 {
		pages := make([]wordPage, len(v.pages), newCap)
		copy(pages, v.pages)
		v.pages = pages
		v.config.Descriptors.Budget.Release(v.directoryBytes)
		v.directoryBytes = newDir
	}
	p := wordPage{class: class, blocks: blocks}
	v.initializePage(&p)
	p.head = p.next[0]
	p.used = 1
	index := uint32(len(v.pages))
	v.pages = append(v.pages, p)
	return index, 0, nil
}
func (v *Words) releaseBlock(d vectorDescriptor) {
	p := &v.pages[d.page]
	clear(p.data[d.offset : d.offset+d.capacity])
	block := d.offset / d.capacity
	p.next[block] = p.head
	p.head = block + 1
	p.used--
}

func (v *Words) Allocate(length, capacity uint32) (VectorRef, error) {
	if v.closed {
		return VectorRef{}, ErrClosed
	}
	if length > capacity {
		return VectorRef{}, ErrBounds
	}
	if capacity == 0 {
		return VectorRef{}, nil
	}
	class, e := vectorClass(capacity)
	if e != nil {
		return VectorRef{}, e
	}
	slot, gen, e := v.descriptors.Allocate()
	if e != nil {
		return VectorRef{}, e
	}
	page, offset, e := v.takeBlock(class)
	if e != nil {
		v.descriptors.Recycle(slot)
		return VectorRef{}, e
	}
	v.descriptors.Write(slot, vectorDescriptor{page: page, offset: offset, length: length, capacity: class})
	return VectorRef{slot, gen}, nil
}

// Reserve preserves nonempty descriptor identity; empty growth returns a new
// ref. Growth failures preserve logical contents, length, capacity and identity.
func (v *Words) Reserve(ref VectorRef, capacity uint32) (VectorRef, error) {
	d, e := v.descriptor(ref)
	if e != nil {
		return ref, e
	}
	if capacity <= d.capacity {
		return ref, nil
	}
	if ref == (VectorRef{}) {
		return v.Allocate(0, capacity)
	}
	class, e := vectorClass(capacity)
	if e != nil {
		return ref, e
	}
	page, offset, e := v.takeBlock(class)
	if e != nil {
		return ref, e
	}
	copy(v.pages[page].data[offset:offset+d.length], v.pages[d.page].data[d.offset:d.offset+d.length])
	v.releaseBlock(d)
	d.page = page
	d.offset = offset
	d.capacity = class
	v.descriptors.Write(ref.Slot, d)
	return ref, nil
}
func (v *Words) Resize(ref VectorRef, length uint32) (VectorRef, error) {
	d, e := v.descriptor(ref)
	if e != nil {
		return ref, e
	}
	if length > d.capacity {
		ref, e = v.Reserve(ref, length)
		if e != nil {
			return ref, e
		}
		d, _ = v.descriptor(ref)
	}
	if ref == (VectorRef{}) {
		return ref, nil
	}
	if length < d.length {
		clear(v.pages[d.page].data[d.offset+length : d.offset+d.length])
	}
	d.length = length
	v.descriptors.Write(ref.Slot, d)
	return ref, nil
}
func (v *Words) Append(ref VectorRef, value uint32) (VectorRef, error) {
	d, e := v.descriptor(ref)
	if e != nil {
		return ref, e
	}
	if d.length == math.MaxUint32 {
		return ref, ErrBounds
	}
	ref, e = v.Resize(ref, d.length+1)
	if e != nil {
		return ref, e
	}
	e = v.Write(ref, d.length, value)
	return ref, e
}
func (v *Words) Free(ref VectorRef) error {
	d, e := v.descriptor(ref)
	if e != nil {
		return e
	}
	if ref == (VectorRef{}) {
		return nil
	}
	v.releaseBlock(d)
	return v.descriptors.Recycle(ref.Slot)
}
func (v *Words) TrimFreePages() {
	if v.closed {
		return
	}
	for i := range v.pages {
		p := &v.pages[i]
		if p.used == 0 && p.data != nil {
			v.config.Descriptors.Budget.Release(uint64(len(p.data))*v.elementBytes + uint64(len(p.next))*4)
			p.data = nil
			p.next = nil
			p.head = 0
		}
	}
	v.descriptors.TrimFreePages()
}
func (v *Words) Reset() {
	if v.closed {
		return
	}
	for i := range v.pages {
		p := &v.pages[i]
		if p.data != nil {
			v.config.Descriptors.Budget.Release(uint64(len(p.data))*v.elementBytes + uint64(len(p.next))*4)
			p.data = nil
			p.next = nil
		}
		p.used = 0
		p.head = 0
	}
	v.descriptors.Reset()
}
func (v *Words) Close() {
	if v.closed {
		return
	}
	v.Reset()
	v.descriptors.Close()
	v.config.Descriptors.Budget.Release(v.directoryBytes + uint64(reflect.TypeOf(Words{}).Size()))
	v.pages = nil
	v.directoryBytes = 0
	v.closed = true
}
func (v *Words) Stats() VectorStats {
	s := VectorStats{Descriptors: v.descriptors.Stats(), DirectoryBytes: v.directoryBytes}
	if !v.closed {
		s.OwnerBytes = uint64(reflect.TypeOf(Words{}).Size())
	}
	s.Live = s.Descriptors.Live
	for i := range v.pages {
		p := &v.pages[i]
		s.PayloadBytes += uint64(len(p.data)) * v.elementBytes
		s.BlockMetadataBytes += uint64(len(p.next)) * 4
	}
	return s
}

// SwapBudgets exchanges the accounting destinations of two complete Words owner
// graphs without moving storage or changing identities. Each destination reserves
// incoming owned bytes while its outgoing graph is still charged. A failure
// refunds prepared Used charges and leaves owners unchanged; successful Reserve
// attempts still consume FailAfter prefixes under the ordinary Budget policy.
// Both Words owners and all affected budgets require external synchronization.
func (v *Words) SwapBudgets(other *Words) error {
	if v != nil && v.closed || other != nil && other.closed {
		return ErrClosed
	}
	if v == nil || other == nil {
		return ErrConfig
	}
	a, b := v.config.Descriptors.Budget, other.config.Descriptors.Budget
	if a == b {
		return nil
	}
	own, e := v.ownedBudgetBytes()
	if e != nil {
		return e
	}
	opposite, e := other.ownedBudgetBytes()
	if e != nil {
		return e
	}
	if e := b.Reserve(own); e != nil {
		return e
	}
	if e := a.Reserve(opposite); e != nil {
		b.Release(own)
		return e
	}
	a.Release(own)
	b.Release(opposite)
	v.config.Descriptors.Budget = b
	v.descriptors.config.Budget = b
	other.config.Descriptors.Budget = a
	other.descriptors.config.Budget = a
	return nil
}
func (v *Words) ownedBudgetBytes() (uint64, error) {
	s := v.Stats()
	total := uint64(0)
	for _, bytes := range [...]uint64{s.OwnerBytes, s.PayloadBytes, s.BlockMetadataBytes, s.DirectoryBytes, s.Descriptors.OwnerBytes, s.Descriptors.RecordBytes, s.Descriptors.MetadataBytes, s.Descriptors.DirectoryBytes} {
		if bytes > ^uint64(0)-total {
			return 0, ErrBounds
		}
		total += bytes
	}
	return total, nil
}
