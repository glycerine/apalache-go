package numeral

// Concrete numeric-cell specialization of the reviewed slab storage algorithm.
// Slot/generation/budget semantics are preserved without generic declarations.

import (
	"math"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

type numericCellSlotState struct {
	Generation    arena.Generation
	Next          arena.Slot
	live, retired bool
}
type numericCellPage struct {
	records []NumericCell
	meta    []numericCellSlotState
	live    uint32
}

// numericCellSlab stores pointer-free records without per-record heap objects. arena.Slot zero is
// invalid. Its owner must perform domain cleanup before Recycle/Reset/Close.
// Read returns a copy; there are no mutable slice or pointer borrows.
// numericCellSlab and its Budget are single-owner and are not safe for concurrent use.
type numericCellSlab struct {
	pages                                          []numericCellPage
	config                                         arena.Config
	Free                                           arena.Slot
	slots, Live, retired                           uint32
	recordPageBytes, metaPageBytes, directoryBytes uint64
	closed                                         bool
}

func newNumericCellSlab(config arena.Config) (*numericCellSlab, error) {
	if config.PageSize == 0 {
		config.PageSize = 1024
	}
	if config.PageSize&(config.PageSize-1) != 0 {
		return nil, arena.ErrConfig
	}
	if config.SlotLimit == 0 {
		config.SlotLimit = math.MaxUint32
	}
	t := reflect.TypeOf(NumericCell{})
	if err := arena.ValidateRecordType(t); err != nil {
		return nil, err
	}
	// Prevent oversized make lengths on 32-bit targets before reserving storage.
	maxInt := uint64(^uint(0) >> 1)
	if uint64(config.PageSize) > maxInt || (t.Size() != 0 && uint64(config.PageSize) > maxInt/uint64(t.Size())) {
		return nil, arena.ErrConfig
	}
	metaSize := uint64(reflect.TypeOf(numericCellSlotState{}).Size())
	if uint64(config.PageSize) > maxInt/metaSize {
		return nil, arena.ErrConfig
	}
	if e := config.Budget.Reserve(uint64(reflect.TypeOf(numericCellSlab{}).Size())); e != nil {
		return nil, e
	}
	return &numericCellSlab{config: config, recordPageBytes: uint64(config.PageSize) * uint64(t.Size()), metaPageBytes: uint64(config.PageSize) * metaSize}, nil
}

func (s *numericCellSlab) location(slot arena.Slot) (uint32, uint32) {
	n := uint32(slot) - 1
	return n / s.config.PageSize, n & (s.config.PageSize - 1)
}
func (s *numericCellSlab) State(slot arena.Slot) *numericCellSlotState {
	if slot == 0 || uint32(slot) > s.slots {
		return nil
	}
	p, i := s.location(slot)
	return &s.pages[p].meta[i]
}

func (s *numericCellSlab) Allocate() (arena.Slot, arena.Generation, error) {
	if s.closed {
		return 0, 0, arena.ErrClosed
	}
	slot := s.Free
	if slot == 0 {
		if s.slots == s.config.SlotLimit {
			return 0, 0, arena.ErrSlotLimit
		}
		slot = arena.Slot(s.slots + 1)
	}
	p, i := s.location(slot)
	if uint64(p) >= uint64(len(s.pages)) {
		newCap := cap(s.pages)
		if len(s.pages) == newCap {
			if newCap == 0 {
				newCap = 1
			} else {
				if newCap > int(^uint(0)>>1)/2 {
					return 0, 0, arena.ErrSlotLimit
				}
				newCap *= 2
			}
		}
		entrySize := uint64(reflect.TypeOf(numericCellPage{}).Size())
		if uint64(newCap) > uint64(^uint(0)>>1)/entrySize {
			return 0, 0, arena.ErrSlotLimit
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
			pages := make([]numericCellPage, len(s.pages), newCap)
			copy(pages, s.pages)
			s.pages = pages
			s.config.Budget.Release(s.directoryBytes)
			s.directoryBytes = newDirectoryBytes
		}
		s.pages = append(s.pages, numericCellPage{records: make([]NumericCell, s.config.PageSize), meta: make([]numericCellSlotState, s.config.PageSize)})
	} else if s.pages[p].records == nil {
		if err := s.config.Budget.Reserve(s.recordPageBytes); err != nil {
			return 0, 0, err
		}
		s.pages[p].records = make([]NumericCell, s.config.PageSize)
	}
	m := &s.pages[p].meta[i]
	if s.Free != 0 {
		s.Free = m.Next
	} else {
		s.slots++
	}
	m.Next = 0
	m.Generation++
	m.live = true
	s.pages[p].live++
	s.Live++
	return slot, m.Generation, nil
}

func (s *numericCellSlab) IsLive(slot arena.Slot, generation arena.Generation) bool {
	if s.closed || generation == 0 {
		return false
	}
	m := s.State(slot)
	return m != nil && m.live && m.Generation == generation
}
func (s *numericCellSlab) Read(slot arena.Slot) (NumericCell, error) {
	var zero NumericCell
	if s.closed {
		return zero, arena.ErrClosed
	}
	m := s.State(slot)
	if m == nil || !m.live {
		return zero, arena.ErrInvalidSlot
	}
	p, i := s.location(slot)
	return s.pages[p].records[i], nil
}
func (s *numericCellSlab) Write(slot arena.Slot, value NumericCell) error {
	if s.closed {
		return arena.ErrClosed
	}
	m := s.State(slot)
	if m == nil || !m.live {
		return arena.ErrInvalidSlot
	}
	p, i := s.location(slot)
	s.pages[p].records[i] = value
	return nil
}
func (s *numericCellSlab) Recycle(slot arena.Slot) error {
	if s.closed {
		return arena.ErrClosed
	}
	m := s.State(slot)
	if m == nil || !m.live {
		return arena.ErrInvalidSlot
	}
	p, i := s.location(slot)
	var zero NumericCell
	s.pages[p].records[i] = zero
	m.live = false
	s.pages[p].live--
	s.Live--
	if m.Generation == arena.Generation(math.MaxUint32) {
		m.retired = true
		s.retired++
	} else {
		m.Next = s.Free
		s.Free = slot
	}
	return nil
}

// reserveDead allocates one real, fully charged ledger slot that can never
// become an ordinary live numeral. It represents undefined source pointer
// states after an allocation error, without changing the hot Int layout.
func (s *numericCellSlab) reserveDead() (arena.Slot, arena.Generation, error) {
	slot, generation, err := s.Allocate()
	if err != nil {
		return 0, 0, err
	}
	p, i := s.location(slot)
	state := s.State(slot)
	s.pages[p].records[i] = NumericCell{}
	state.live = false
	state.retired = true
	s.pages[p].live--
	s.Live--
	s.retired++
	return slot, generation, nil
}

// TrimFreePages drops wholly dead record pages. arena.Generation metadata is retained.
func (s *numericCellSlab) TrimFreePages() {
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
func (s *numericCellSlab) Reset() {
	if s.closed {
		return
	}
	for n := uint64(1); n <= uint64(s.slots); n++ {
		slot := arena.Slot(n)
		if s.State(slot).live {
			s.Recycle(slot)
		}
	}
	s.TrimFreePages()
}
func (s *numericCellSlab) Close() {
	if s.closed {
		return
	}
	stats := s.Stats()
	s.config.Budget.Release(stats.RecordBytes + stats.MetadataBytes + stats.DirectoryBytes + stats.OwnerBytes)
	s.pages = nil
	s.Free = 0
	s.Live = 0
	s.slots = 0
	s.retired = 0
	s.directoryBytes = 0
	s.closed = true
}
func (s *numericCellSlab) Stats() arena.Stats {
	stats := arena.Stats{Live: s.Live, Slots: s.slots, Retired: s.retired, MetadataBytes: uint64(len(s.pages)) * s.metaPageBytes, DirectoryBytes: s.directoryBytes}
	if !s.closed {
		stats.OwnerBytes = uint64(reflect.TypeOf(numericCellSlab{}).Size())
	}
	for i := range s.pages {
		if s.pages[i].records != nil {
			stats.Pages++
			stats.RecordBytes += s.recordPageBytes
		}
	}
	return stats
}
