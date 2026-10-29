package arena

import (
	"math"
	"reflect"
)

type slabPage32 struct {
	records []uint32
	meta    []slotState
	live    uint32
}

// Slab32 stores pointer-free records without per-record heap objects. Slot zero is
// invalid. Its owner must perform domain cleanup before Recycle/Reset/Close.
// Read returns a copy; there are no mutable slice or pointer borrows.
// Slab32 and its Budget are single-owner and are not safe for concurrent use.
type Slab32 struct {
	pages                                          []slabPage32
	config                                         Config
	free                                           Slot
	slots, live, retired                           uint32
	recordPageBytes, metaPageBytes, directoryBytes uint64
	closed                                         bool
}

func NewSlab32(config Config) (*Slab32, error) {
	if config.PageSize == 0 {
		config.PageSize = 1024
	}
	if config.PageSize&(config.PageSize-1) != 0 {
		return nil, ErrConfig
	}
	if config.SlotLimit == 0 {
		config.SlotLimit = math.MaxUint32
	}
	t := reflect.TypeOf(uint32(0))
	if !pointerFree(t) {
		return nil, ErrPointerful
	}
	// Prevent oversized make lengths on 32-bit targets before reserving storage.
	maxInt := uint64(^uint(0) >> 1)
	if uint64(config.PageSize) > maxInt || (t.Size() != 0 && uint64(config.PageSize) > maxInt/uint64(t.Size())) {
		return nil, ErrConfig
	}
	metaSize := uint64(reflect.TypeOf(slotState{}).Size())
	if uint64(config.PageSize) > maxInt/metaSize {
		return nil, ErrConfig
	}
	if e := config.Budget.Reserve(uint64(reflect.TypeOf(Slab32{}).Size())); e != nil {
		return nil, e
	}
	return &Slab32{config: config, recordPageBytes: uint64(config.PageSize) * uint64(t.Size()), metaPageBytes: uint64(config.PageSize) * metaSize}, nil
}

func (s *Slab32) location(slot Slot) (uint32, uint32) {
	n := uint32(slot) - 1
	return n / s.config.PageSize, n & (s.config.PageSize - 1)
}
func (s *Slab32) state(slot Slot) *slotState {
	if slot == 0 || uint32(slot) > s.slots {
		return nil
	}
	p, i := s.location(slot)
	return &s.pages[p].meta[i]
}

func (s *Slab32) Allocate() (Slot, Generation, error) {
	if s.closed {
		return 0, 0, ErrClosed
	}
	slot := s.free
	if slot == 0 {
		if s.slots == s.config.SlotLimit {
			return 0, 0, ErrSlotLimit
		}
		slot = Slot(s.slots + 1)
	}
	p, i := s.location(slot)
	if uint64(p) >= uint64(len(s.pages)) {
		newCap := cap(s.pages)
		if len(s.pages) == newCap {
			if newCap == 0 {
				newCap = 1
			} else {
				if newCap > int(^uint(0)>>1)/2 {
					return 0, 0, ErrSlotLimit
				}
				newCap *= 2
			}
		}
		entrySize := uint64(reflect.TypeOf(slabPage32{}).Size())
		if uint64(newCap) > uint64(^uint(0)>>1)/entrySize {
			return 0, 0, ErrSlotLimit
		}
		newDirectoryBytes := uint64(0)
		if newCap != cap(s.pages) {
			newDirectoryBytes = uint64(newCap) * entrySize
		}
		// Charge peak old+new directory storage before copying. The old
		// directory becomes releasable only when the new one is published.
		amount := s.recordPageBytes + s.metaPageBytes + newDirectoryBytes
		if err := s.config.Budget.Reserve(amount); err != nil {
			return 0, 0, err
		}
		if newCap != cap(s.pages) {
			pages := make([]slabPage32, len(s.pages), newCap)
			copy(pages, s.pages)
			s.pages = pages
			s.config.Budget.Release(s.directoryBytes)
			s.directoryBytes = newDirectoryBytes
		}
		s.pages = append(s.pages, slabPage32{records: make([]uint32, s.config.PageSize), meta: make([]slotState, s.config.PageSize)})
	} else if s.pages[p].records == nil {
		if err := s.config.Budget.Reserve(s.recordPageBytes); err != nil {
			return 0, 0, err
		}
		s.pages[p].records = make([]uint32, s.config.PageSize)
	}
	m := &s.pages[p].meta[i]
	if s.free != 0 {
		s.free = m.next
	} else {
		s.slots++
	}
	m.next = 0
	m.generation++
	m.live = true
	s.pages[p].live++
	s.live++
	return slot, m.generation, nil
}

func (s *Slab32) IsLive(slot Slot, generation Generation) bool {
	if s.closed || generation == 0 {
		return false
	}
	m := s.state(slot)
	return m != nil && m.live && m.generation == generation
}
func (s *Slab32) Read(slot Slot) (uint32, error) {
	var zero uint32
	if s.closed {
		return zero, ErrClosed
	}
	m := s.state(slot)
	if m == nil || !m.live {
		return zero, ErrInvalidSlot
	}
	p, i := s.location(slot)
	return s.pages[p].records[i], nil
}
func (s *Slab32) Write(slot Slot, value uint32) error {
	if s.closed {
		return ErrClosed
	}
	m := s.state(slot)
	if m == nil || !m.live {
		return ErrInvalidSlot
	}
	p, i := s.location(slot)
	s.pages[p].records[i] = value
	return nil
}
func (s *Slab32) Recycle(slot Slot) error {
	if s.closed {
		return ErrClosed
	}
	m := s.state(slot)
	if m == nil || !m.live {
		return ErrInvalidSlot
	}
	p, i := s.location(slot)
	var zero uint32
	s.pages[p].records[i] = zero
	m.live = false
	s.pages[p].live--
	s.live--
	if m.generation == Generation(math.MaxUint32) {
		m.retired = true
		s.retired++
	} else {
		m.next = s.free
		s.free = slot
	}
	return nil
}

// TrimFreePages drops wholly dead record pages. Generation metadata is retained.
func (s *Slab32) TrimFreePages() {
	if s.closed {
		return
	}
	for i := range s.pages {
		p := &s.pages[i]
		if p.live == 0 && p.records != nil {
			p.records = nil
			s.config.Budget.Release(s.recordPageBytes)
		}
	}
}

// Reset invalidates all live tokens, retaining the generation ledger. Cleanup
// of owned domain values must have been performed by the caller beforehand.
func (s *Slab32) Reset() {
	if s.closed {
		return
	}
	for n := uint64(1); n <= uint64(s.slots); n++ {
		slot := Slot(n)
		if s.state(slot).live {
			s.Recycle(slot)
		}
	}
	s.TrimFreePages()
}
func (s *Slab32) Close() {
	if s.closed {
		return
	}
	stats := s.Stats()
	s.config.Budget.Release(stats.RecordBytes + stats.MetadataBytes + stats.DirectoryBytes + stats.OwnerBytes)
	s.pages = nil
	s.free = 0
	s.live = 0
	s.slots = 0
	s.retired = 0
	s.directoryBytes = 0
	s.closed = true
}
func (s *Slab32) Stats() Stats {
	stats := Stats{Live: s.live, Slots: s.slots, Retired: s.retired, MetadataBytes: uint64(len(s.pages)) * s.metaPageBytes, DirectoryBytes: s.directoryBytes}
	if !s.closed {
		stats.OwnerBytes = uint64(reflect.TypeOf(Slab32{}).Size())
	}
	for i := range s.pages {
		if s.pages[i].records != nil {
			stats.Pages++
			stats.RecordBytes += s.recordPageBytes
		}
	}
	return stats
}
