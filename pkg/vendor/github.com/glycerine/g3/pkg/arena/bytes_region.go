package arena

// Storage translation of util/region.h/.cpp and util/page.h/.cpp.

import (
	"math"
	"reflect"
	"strconv"
	"sync/atomic"
)

// Process-local cold namespace, borrowed by all byte-region owners. It never
// wraps or reuses IDs; failed constructors may consume an unpublished ID.
var bytesRegionNextOwnerID atomic.Uint64

func nextBytesRegionOwnerID() (uint64, error) {
	for {
		previous := bytesRegionNextOwnerID.Load()
		if previous == math.MaxUint64 {
			return 0, ErrSlotLimit
		}
		if bytesRegionNextOwnerID.CompareAndSwap(previous, previous+1) {
			return previous + 1, nil
		}
	}
}

type BytesRegionConfig struct {
	// Alignment selects source pointer width4/8, default8. PageSize is source
	// payload bytes (default8192-Alignment), a positive aligned multiple. This
	// models source offsets, not a raw Go address/ABI alignment promise.
	Alignment, PageSize uint32
	Descriptors         Config
	// DirectOnly selects source Z3DEBUG region: independent direct chunks, no
	// alignment/default pages/in-payload marks, scopes save allocation-log length.
	DirectOnly bool
}
type BytesRegionRef struct {
	Slot       Slot
	Generation Generation
}
type BytesRegionMark struct {
	owner  *regionIdentity
	serial uint64
}
type bytesRegionScope struct {
	serial               uint64
	length, page, cursor uint32
}
type bytesRegionPage struct {
	data         []byte
	prev, header uint32
	defaultPage  bool
}
type BytesRegionStats struct {
	Descriptors                                                                         Stats
	Live, Scopes, ActivePages, DefaultPages, FreePages, BigPages, DirectObjects, Cursor uint32
	OwnerBytes, PayloadBytes, PageHeaderBytes, DirectoryBytes, LogBytes, ScopeBytes     uint64
	// MarkPayloadBytes describes live release scope mark bytes already contained
	// in PayloadBytes; do not add it again when computing owned storage charges.
	MarkPayloadBytes uint64
}

// BytesRegion owns scoped variable byte payloads with source page recycling.
// Caller performs all logical cleanup BEFORE pop/reset/close; no callbacks or
// slices escape this storage. Owner and its Budget are externally synchronized.
// Source page headers are allocated byte backing; scalar page metadata replaces
// tagged address links. Both representations are charged, including all cold
// owner/identity/descriptor headers and old+new metadata growth peaks.
// Zero-size allocations are live checked descriptors. Source release pointers
// can alias the same byte offset; the distinct descriptor identity is explicit.
// Prepared reusable metadata can remain charged after predictable failure; no
// live ref/mark or page-chain/cursor mutation is published on that failure.
type BytesRegion struct {
	descriptors            *descriptorSlab
	config                 BytesRegionConfig
	pages                  []bytesRegionPage
	log                    []BytesRegionRef
	scopes                 []bytesRegionScope
	ownerID                uint64
	identity               *regionIdentity
	serial, directoryBytes uint64
	current, free, cursor  uint32
	closed                 bool
}

func NewBytesRegion(c BytesRegionConfig) (*BytesRegion, error) {
	if c.Alignment == 0 {
		c.Alignment = 8
	}
	if c.Alignment != 4 && c.Alignment != 8 {
		return nil, ErrConfig
	}
	if c.PageSize == 0 {
		c.PageSize = 8192 - c.Alignment
	}
	if c.PageSize < c.Alignment || c.PageSize%c.Alignment != 0 || uint64(c.PageSize)+uint64(c.Alignment) > uint64(^uint(0)>>1) {
		return nil, ErrConfig
	}
	id, e := nextBytesRegionOwnerID()
	if e != nil {
		return nil, e
	}
	d, e := newDescriptorSlab(c.Descriptors)
	if e != nil {
		return nil, e
	}
	owner := uint64(reflect.TypeOf(BytesRegion{}).Size()) + uint64(reflect.TypeOf(regionIdentity{}).Size())
	if e = c.Descriptors.Budget.Reserve(owner); e != nil {
		d.Close()
		return nil, e
	}
	r := &BytesRegion{ownerID: id, descriptors: d, config: c, identity: &regionIdentity{1}}
	if !c.DirectOnly {
		page, e := r.prepareDefault()
		if e != nil {
			r.Close()
			return nil, e
		}
		r.useDefault(page, 0)
	}
	return r, nil
}
func (r *BytesRegion) descriptor(ref BytesRegionRef) (vectorDescriptor, error) {
	if r.closed {
		return vectorDescriptor{}, ErrClosed
	}
	if !r.descriptors.IsLive(ref.Slot, ref.Generation) {
		return vectorDescriptor{}, ErrInvalidSlot
	}
	return r.descriptors.Read(ref.Slot)
}
func (r *BytesRegion) IsLive(ref BytesRegionRef) bool {
	return !r.closed && r.descriptors.IsLive(ref.Slot, ref.Generation)
}
func (r *BytesRegion) Size(ref BytesRegionRef) (uint32, error) {
	d, e := r.descriptor(ref)
	return d.length, e
}
func (r *BytesRegion) Read(ref BytesRegionRef, i uint32) (byte, error) {
	d, e := r.descriptor(ref)
	if e != nil {
		return 0, e
	}
	if i >= d.length {
		return 0, ErrBounds
	}
	p := &r.pages[d.page]
	return p.data[uint64(p.header)+uint64(d.offset)+uint64(i)], nil
}
func (r *BytesRegion) Write(ref BytesRegionRef, i uint32, v byte) error {
	d, e := r.descriptor(ref)
	if e != nil {
		return e
	}
	if i >= d.length {
		return ErrBounds
	}
	p := &r.pages[d.page]
	p.data[uint64(p.header)+uint64(d.offset)+uint64(i)] = v
	return nil
}
func (r *BytesRegion) reserveLog() error {
	if len(r.log) < cap(r.log) {
		return nil
	}
	n := cap(r.log) * 2
	if n == 0 {
		n = 1
	}
	size := uint64(reflect.TypeOf(BytesRegionRef{}).Size())
	if n <= cap(r.log) || uint64(n) > math.MaxUint32 || uint64(n) > uint64(^uint(0)>>1)/size {
		return ErrSlotLimit
	}
	bytes := uint64(n) * size
	if e := r.config.Descriptors.Budget.Reserve(bytes); e != nil {
		return e
	}
	replacement := make([]BytesRegionRef, len(r.log), n)
	copy(replacement, r.log)
	r.config.Descriptors.Budget.Release(uint64(cap(r.log)) * size)
	r.log = replacement
	return nil
}
func (r *BytesRegion) reserveScopes() error {
	if len(r.scopes) < cap(r.scopes) {
		return nil
	}
	n := cap(r.scopes) * 2
	if n == 0 {
		n = 1
	}
	size := uint64(reflect.TypeOf(bytesRegionScope{}).Size())
	if n <= cap(r.scopes) || uint64(n) > math.MaxUint32 || uint64(n) > uint64(^uint(0)>>1)/size {
		return ErrSlotLimit
	}
	bytes := uint64(n) * size
	if e := r.config.Descriptors.Budget.Reserve(bytes); e != nil {
		return e
	}
	replacement := make([]bytesRegionScope, len(r.scopes), n)
	copy(replacement, r.scopes)
	r.config.Descriptors.Budget.Release(uint64(cap(r.scopes)) * size)
	r.scopes = replacement
	return nil
}

// newPage prepares an unlinked page. Its directory ID is a conceptual address,
// reused only after old page allocations have been invalidated or preparation
// failed before publication. Live byte descriptors never move.
func (r *BytesRegion) newPage(size uint32, defaultPage bool) (uint32, error) {
	header := uint32(0)
	if !r.config.DirectOnly {
		header = r.config.Alignment
	}
	bytes := uint64(size) + uint64(header)
	if bytes > uint64(^uint(0)>>1) {
		return 0, ErrBounds
	}
	page := len(r.pages)
	for i := range r.pages {
		if r.pages[i].data == nil {
			page = i
			break
		}
	}
	if uint64(page) >= math.MaxUint32 {
		return 0, ErrSlotLimit
	}
	var directory []bytesRegionPage
	var dirBytes uint64
	if page == len(r.pages) && len(r.pages) == cap(r.pages) {
		n := cap(r.pages) * 2
		if n == 0 {
			n = 1
		}
		size := uint64(reflect.TypeOf(bytesRegionPage{}).Size())
		if n <= cap(r.pages) || uint64(n) > math.MaxUint32 || uint64(n) > uint64(^uint(0)>>1)/size {
			return 0, ErrSlotLimit
		}
		dirBytes = uint64(n) * size
		if e := r.config.Descriptors.Budget.Reserve(dirBytes); e != nil {
			return 0, e
		}
		directory = make([]bytesRegionPage, len(r.pages), n)
		copy(directory, r.pages)
	}
	if bytes > 0 {
		if e := r.config.Descriptors.Budget.Reserve(bytes); e != nil {
			r.config.Descriptors.Budget.Release(dirBytes)
			return 0, e
		}
	}
	p := bytesRegionPage{data: make([]byte, int(bytes)), header: header, defaultPage: defaultPage}
	if directory != nil {
		r.config.Descriptors.Budget.Release(r.directoryBytes)
		r.pages = directory
		r.directoryBytes = dirBytes
	}
	if page == len(r.pages) {
		r.pages = append(r.pages, p)
	} else {
		r.pages[page] = p
	}
	return uint32(page) + 1, nil
}
func (r *BytesRegion) prepareDefault() (uint32, error) {
	if r.free != 0 {
		return r.free, nil
	}
	return r.newPage(r.config.PageSize, true)
}
func (r *BytesRegion) useDefault(id, prev uint32) {
	p := &r.pages[id-1]
	if id == r.free {
		r.free = p.prev
	}
	p.prev = prev
	r.current = id
	r.cursor = 0
}
func (r *BytesRegion) releasePage(id uint32) {
	p := &r.pages[id-1]
	r.config.Descriptors.Budget.Release(uint64(len(p.data)))
	*p = bytesRegionPage{}
}
func (r *BytesRegion) allocateLocation(size uint32) (uint32, uint32, error) {
	if r.config.DirectOnly {
		page, e := r.newPage(size, false)
		if e != nil {
			return 0, 0, e
		}
		r.pages[page-1].prev = r.current
		r.current = page
		return page - 1, 0, nil
	}
	end := uint64(r.cursor) + uint64(size)
	if end < uint64(r.config.PageSize) {
		offset := r.cursor
		r.cursor = uint32((end + uint64(r.config.Alignment) - 1) / uint64(r.config.Alignment) * uint64(r.config.Alignment))
		return r.current - 1, offset, nil
	}
	if size < r.config.PageSize {
		page, e := r.prepareDefault()
		if e != nil {
			return 0, 0, e
		}
		r.useDefault(page, r.current)
		r.cursor = (size + r.config.Alignment - 1) / r.config.Alignment * r.config.Alignment
		return page - 1, 0, nil
	}
	// Source big page is followed eagerly by a default page. Prepare BOTH before
	// linking either; if the second allocation fails, release dedicated backing.
	big, e := r.newPage(size, false)
	if e != nil {
		return 0, 0, e
	}
	page, e := r.prepareDefault()
	if e != nil {
		r.releasePage(big)
		return 0, 0, e
	}
	r.pages[big-1].prev = r.current
	r.useDefault(page, big)
	return big - 1, 0, nil
}
func (r *BytesRegion) Allocate(size uint32) (BytesRegionRef, error) {
	if r.closed {
		return BytesRegionRef{}, ErrClosed
	}
	if uint64(len(r.log)) == math.MaxUint32 {
		return BytesRegionRef{}, ErrSlotLimit
	}
	header := uint64(r.config.Alignment)
	if r.config.DirectOnly {
		header = 0
	}
	if uint64(size)+header > uint64(^uint(0)>>1) {
		return BytesRegionRef{}, ErrBounds
	}
	if e := r.reserveLog(); e != nil {
		return BytesRegionRef{}, e
	}
	slot, g, e := r.descriptors.Allocate()
	if e != nil {
		return BytesRegionRef{}, e
	}
	page, offset, e := r.allocateLocation(size)
	if e != nil {
		r.descriptors.Recycle(slot)
		return BytesRegionRef{}, e
	}
	r.descriptors.Write(slot, vectorDescriptor{page: page, offset: offset, length: size, capacity: size})
	ref := BytesRegionRef{slot, g}
	r.log = append(r.log, ref)
	return ref, nil
}
func (r *BytesRegion) PushScope() (BytesRegionMark, error) {
	if r.closed {
		return BytesRegionMark{}, ErrClosed
	}
	if r.serial == math.MaxUint64 || uint64(len(r.scopes)) == math.MaxUint32 {
		return BytesRegionMark{}, ErrSlotLimit
	}
	if e := r.reserveScopes(); e != nil {
		return BytesRegionMark{}, e
	}
	saved := bytesRegionScope{serial: r.serial + 1, length: uint32(len(r.log)), page: r.current, cursor: r.cursor}
	// Capture BEFORE the real source mark allocation, including its overflow
	// into a default/big page. Scalar metadata represents the source mark fields.
	if !r.config.DirectOnly {
		if _, _, e := r.allocateLocation(3 * r.config.Alignment); e != nil {
			return BytesRegionMark{}, e
		}
	}
	r.serial++
	r.scopes = append(r.scopes, saved)
	return BytesRegionMark{r.identity, r.serial}, nil
}
func (r *BytesRegion) recycleCurrent() {
	id := r.current
	p := &r.pages[id-1]
	prev := p.prev
	if p.defaultPage {
		p.prev = r.free
		r.free = id
	} else {
		r.releasePage(id)
	}
	r.current = prev
}
func (r *BytesRegion) pop() {
	saved := r.scopes[len(r.scopes)-1]
	for i := len(r.log); i > int(saved.length); i-- {
		r.descriptors.Recycle(r.log[i-1].Slot)
	}
	clear(r.log[saved.length:])
	r.log = r.log[:saved.length]
	for r.current != saved.page {
		r.recycleCurrent()
	}
	r.cursor = saved.cursor
	r.scopes[len(r.scopes)-1] = bytesRegionScope{}
	r.scopes = r.scopes[:len(r.scopes)-1]
}
func (r *BytesRegion) PopScope() error {
	if r.closed {
		return ErrClosed
	}
	if len(r.scopes) == 0 {
		return ErrMark
	}
	r.pop()
	return nil
}
func (r *BytesRegion) Rollback(mark BytesRegionMark) error {
	if r.closed {
		return ErrClosed
	}
	if mark.owner != r.identity || len(r.scopes) == 0 || r.scopes[len(r.scopes)-1].serial != mark.serial {
		return ErrMark
	}
	r.pop()
	return nil
}
func (r *BytesRegion) PopScopes(n uint32) error {
	if r.closed {
		return ErrClosed
	}
	if uint64(n) > uint64(len(r.scopes)) {
		return ErrMark
	}
	for i := uint32(0); i < n; i++ {
		r.pop()
	}
	return nil
}
func (r *BytesRegion) Reset() error {
	if r.closed {
		return ErrClosed
	}
	r.descriptors.Reset()
	clear(r.log)
	r.log = r.log[:0]
	clear(r.scopes)
	r.scopes = r.scopes[:0]
	for r.current != 0 {
		r.recycleCurrent()
	}
	r.cursor = 0
	// Every successful release constructor retains at least one default page.
	// Source reset therefore reuses it without ANY new storage reservation.
	if !r.config.DirectOnly {
		r.useDefault(r.free, 0)
	}
	return nil
}
func (r *BytesRegion) Close() error {
	if r.closed {
		return nil
	}
	for i := range r.pages {
		if r.pages[i].data != nil {
			r.releasePage(uint32(i) + 1)
		}
	}
	r.descriptors.Close()
	owner := uint64(reflect.TypeOf(BytesRegion{}).Size()) + uint64(reflect.TypeOf(regionIdentity{}).Size())
	r.config.Descriptors.Budget.Release(owner + r.directoryBytes + uint64(cap(r.log))*uint64(reflect.TypeOf(BytesRegionRef{}).Size()) + uint64(cap(r.scopes))*uint64(reflect.TypeOf(bytesRegionScope{}).Size()))
	r.pages = nil
	r.log = nil
	r.scopes = nil
	r.directoryBytes = 0
	r.current = 0
	r.free = 0
	r.cursor = 0
	r.closed = true
	return nil
}
func (r *BytesRegion) Stats() BytesRegionStats {
	s := BytesRegionStats{Descriptors: r.descriptors.Stats(), Live: uint32(len(r.log)), Scopes: uint32(len(r.scopes)), Cursor: r.cursor, DirectoryBytes: r.directoryBytes, LogBytes: uint64(cap(r.log)) * uint64(reflect.TypeOf(BytesRegionRef{}).Size()), ScopeBytes: uint64(cap(r.scopes)) * uint64(reflect.TypeOf(bytesRegionScope{}).Size())}
	if !r.closed {
		s.OwnerBytes = uint64(reflect.TypeOf(BytesRegion{}).Size()) + uint64(reflect.TypeOf(regionIdentity{}).Size())
	}
	if r.config.DirectOnly {
		s.DirectObjects = s.Live
	} else {
		s.MarkPayloadBytes = uint64(len(r.scopes)) * 3 * uint64(r.config.Alignment)
		for page := r.current; page != 0; page = r.pages[page-1].prev {
			s.ActivePages++
		}
	}
	for page := r.free; page != 0; page = r.pages[page-1].prev {
		s.FreePages++
	}
	for i := range r.pages {
		p := &r.pages[i]
		if p.data == nil {
			continue
		}
		s.PayloadBytes += uint64(len(p.data)) - uint64(p.header)
		s.PageHeaderBytes += uint64(p.header)
		if p.defaultPage {
			s.DefaultPages++
		} else if !r.config.DirectOnly {
			s.BigPages++
		}
	}
	return s
}

// OwnerID is a stable nonzero scalar origin for pointer-free borrowed keys.
// It remains unchanged after rollback, reset and close; it is not an AST ID.
func (r *BytesRegion) OwnerID() uint64 { return r.ownerID }

// DisplayMemStats translates region::display_mem_stats's exact text. Release
// counts the active page chain only; DirectOnly counts allocated objects.
// The returned Go string is caller-owned output, outside the storage budget.
func (r *BytesRegion) DisplayMemStats() string {
	if r.config.DirectOnly {
		return "num. objects:      " + strconv.FormatUint(uint64(len(r.log)), 10) + "\n"
	}
	var count uint64
	for page := r.current; page != 0; page = r.pages[page-1].prev {
		count++
	}
	return "num. pages:      " + strconv.FormatUint(count, 10) + "\n"
}
