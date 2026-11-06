package params

// Source: src/util/params.h; src/util/params.cpp.

import (
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	udiag "github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

var ErrParamsOwner = errors.New("g3: parameters owner or lifetime mismatch")
var ErrParamsPrecondition = errors.New("g3: parameters source precondition")
var paramsNextOwner uint64

// ParamsRef owns a reference, never shallow-copy it. NewValue creates the
// source null backing; Assign is the reference-counted copy operation.
type ParamsRef struct {
	Owner            uint64
	Slot, Generation uint32
}
type ParamsConfig struct {
	Budget      *arena.Budget
	PageEntries uint32
	Text        arena.VectorConfig
}
type paramsEntriesRef struct{ page, block, generation, capacity uint32 }
type paramsBlock struct {
	Refs, Generation, Size, capacity uint32
	entries                          paramsEntriesRef
	Live                             bool
	resetCursor                      uint32
	resetStage                       uint8
	resetKind                        uint8
	resetActive                      bool
}

// paramsOwnedString gives returned views an explicit invalidation point without
// indexing an arena payload. Copies share immutable bytes, with separate lives.
type paramsOwnedString struct {
	value string
	live  bool
}

func (s *paramsOwnedString) read() (string, error) {
	if s == nil || !s.live {
		return "", arena.ErrInvalidSlot
	}
	return s.value, nil
}
func (s *paramsOwnedString) release() {
	if s != nil {
		s.live = false
		s.value = ""
	}
}

type paramsText struct {
	owned   *paramsOwnedString
	present bool
	// Borrowed strings retain a canonical symbol instead of owning text.
	borrowed bool
}
type paramEntry struct {
	key        usymbol.Symbol
	Kind       ParamKind
	bits       uint64
	ratTraceID uint64
	text       paramsText
	symbol     usymbol.Symbol
	Rat        unumeral.OwnedRational
}
type paramsEntryState struct {
	generation uint32
	used       bool
}
type paramsEntryPage struct {
	entries []paramEntry
	states  []paramsEntryState
	class   uint32
}
type ParamsStats struct {
	LiveBlocks, References, Entries                                                                                                   uint32
	HeaderBytes, BlockBytes, ReceiptBytes, SourceFrameBytes, SourceSetBytes, ReplacementBytes, PageBytes, DirectoryBytes, FormatBytes uint64
}

// This cold owner borrows both services. They must outlive it; budgets shared
// across owners require external confinement. Entry storage is concrete and typed.
type ParamsContext struct {
	mu                sync.Mutex
	Self              *ParamsContext
	SymbolsField      *usymbol.Manager
	NumeralsField     *unumeral.RationalService
	BudgetField       *arena.Budget
	memoryTrace       *udiag.TraceTags
	memoryProcess     *umemory.ProcessAccounting
	Owner             uint64
	Blocks            []paramsBlock
	decrementReceipts []paramsDecrementReceipt
	SourceFrames      []paramsSourceFrame
	SetFrames         []paramsSetFrame
	replacementFrames []paramsReplacementFrame
	sourceSerial      uint64
	pages             []paramsEntryPage
	workspace         []byte
	formatWriter      *sourceTextWriter
	pageEntries       uint32
	headerBytes       uint64
	Closed            bool
}

func pdGrowth(n int) (int, error) {
	if n == 0 {
		return 2, nil
	}
	if uint64(n) > (uint64(^uint(0)>>1)-1)/3 {
		return 0, arena.ErrBounds
	}
	v := (3*n + 1) / 2
	if uint64(v) > uint64(^uint32(0)) || v <= n {
		return 0, arena.ErrBounds
	}
	return v, nil
}
func paramsBytes(n int, t reflect.Type) uint64 { return uint64(n) * uint64(t.Size()) }
func paramsBlockBytes(n int) uint64            { return paramsBytes(n, reflect.TypeOf(paramsBlock{})) }
func paramsPageBytes(p paramsEntryPage) uint64 {
	return paramsBytes(cap(p.entries), reflect.TypeOf(paramEntry{})) + paramsBytes(cap(p.states), reflect.TypeOf(paramsEntryState{}))
}
func paramsDirectoryBytes(n int) uint64 { return paramsBytes(n, reflect.TypeOf(paramsEntryPage{})) }
func NewParamsContext(s *usymbol.Manager, q *unumeral.RationalService, c ParamsConfig) (*ParamsContext, error) {
	if s == nil || q == nil || c.Text.Descriptors.Budget != nil && c.Text.Descriptors.Budget != c.Budget {
		return nil, arena.ErrConfig
	}
	if _, e := s.Hash(usymbol.Symbol{}); e != nil {
		return nil, e
	}
	r, e := q.NewValue()
	if e != nil {
		return nil, e
	}
	if e = q.Del(&r); e != nil {
		return nil, e
	}
	var id uint64
	for {
		old := atomic.LoadUint64(&paramsNextOwner)
		if old == math.MaxUint64 {
			return nil, arena.ErrBounds
		}
		if atomic.CompareAndSwapUint64(&paramsNextOwner, old, old+1) {
			id = old + 1
			break
		}
	}
	n := c.PageEntries
	if n == 0 {
		n = 64
	}
	if n > 1<<30 {
		return nil, arena.ErrBounds
	}
	h := uint64(reflect.TypeOf(ParamsContext{}).Size())
	if e = c.Budget.Reserve(h); e != nil {
		return nil, e
	}
	p := &ParamsContext{SymbolsField: s, NumeralsField: q, BudgetField: c.Budget, Owner: id, pageEntries: n, headerBytes: h}
	p.Self = p
	return p, nil
}
func (c *ParamsContext) open() error {
	if c == nil || c.Self != c || c.Closed {
		return ErrParamsOwner
	}
	if _, e := c.SymbolsField.Hash(usymbol.Symbol{}); e != nil {
		return e
	}
	return nil
}
func (c *ParamsContext) check(v *ParamsRef) error {
	if e := c.open(); e != nil {
		return e
	}
	if v == nil || v.Owner != c.Owner {
		return ErrParamsOwner
	}
	if v.Slot == 0 {
		return nil
	}
	if int(v.Slot) > len(c.Blocks) {
		return ErrParamsOwner
	}
	b := &c.Blocks[v.Slot-1]
	if !b.Live || b.Generation != v.Generation {
		return ErrParamsOwner
	}
	return nil
}
func (c *ParamsContext) NewValue() (ParamsRef, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.open(); e != nil {
		return ParamsRef{}, e
	}
	return ParamsRef{Owner: c.Owner}, nil
}
func (c *ParamsContext) Entry(b *paramsBlock, i uint32) *paramEntry {
	r := b.entries
	p := &c.pages[r.page-1]
	return &p.entries[r.block*p.class+i]
}
func (c *ParamsContext) allocateEntries(n uint32) (paramsEntriesRef, error) {
	class := uint32(1)
	for class < n {
		if class >= 1<<30 {
			return paramsEntriesRef{}, arena.ErrBounds
		}
		class *= 2
	}
	for pi := range c.pages {
		p := &c.pages[pi]
		if p.class != class {
			continue
		}
		for bi := range p.states {
			s := &p.states[bi]
			if !s.used && s.generation != math.MaxUint32 {
				s.generation++
				s.used = true
				return paramsEntriesRef{uint32(pi + 1), uint32(bi), s.generation, class}, nil
			}
		}
	}
	count := c.pageEntries / class
	if count == 0 {
		count = 1
	}
	total := uint64(count) * uint64(class)
	if total > uint64(^uint(0)>>1)/uint64(reflect.TypeOf(paramEntry{}).Size()) {
		return paramsEntriesRef{}, arena.ErrBounds
	}
	p := paramsEntryPage{class: class}
	bytes := total*uint64(reflect.TypeOf(paramEntry{}).Size()) + uint64(count)*uint64(reflect.TypeOf(paramsEntryState{}).Size())
	if e := c.BudgetField.Reserve(bytes); e != nil {
		return paramsEntriesRef{}, e
	}
	p.entries = make([]paramEntry, int(total))
	p.states = make([]paramsEntryState, count)
	if len(c.pages) == cap(c.pages) {
		n, e := pdGrowth(cap(c.pages))
		if e != nil {
			c.BudgetField.Release(bytes)
			return paramsEntriesRef{}, e
		}
		if e = c.BudgetField.Reserve(paramsDirectoryBytes(n)); e != nil {
			c.BudgetField.Release(bytes)
			return paramsEntriesRef{}, e
		}
		next := make([]paramsEntryPage, len(c.pages), n)
		copy(next, c.pages)
		c.BudgetField.Release(paramsDirectoryBytes(cap(c.pages)))
		c.pages = next
	}
	p.states[0] = paramsEntryState{1, true}
	c.pages = append(c.pages, p)
	return paramsEntriesRef{uint32(len(c.pages)), 0, 1, class}, nil
}
func (c *ParamsContext) freeEntries(r paramsEntriesRef) {
	if r.page != 0 {
		c.pages[r.page-1].states[r.block].used = false
	}
}
func (c *ParamsContext) newBlock() (uint32, error) {
	for i := range c.Blocks {
		b := &c.Blocks[i]
		if !b.Live && !c.SourceFrames[i].Active && !c.SetFrames[i].Active && !c.replacementFrames[i].active && b.Generation != math.MaxUint32 {
			g := b.Generation + 1
			*b = paramsBlock{Refs: 1, Generation: g, Live: true}
			c.decrementReceipts[i] = paramsDecrementReceipt{}
			c.SourceFrames[i] = paramsSourceFrame{}
			c.SetFrames[i] = paramsSetFrame{}
			c.replacementFrames[i] = paramsReplacementFrame{}
			return uint32(i + 1), nil
		}
	}
	if len(c.Blocks) == cap(c.Blocks) {
		n, e := pdGrowth(cap(c.Blocks))
		if e != nil {
			return 0, e
		}
		if e = c.BudgetField.Reserve(paramsBlockBytes(n) + paramsReceiptBytes(n) + paramsSourceFrameBytes(n) + paramsSetFrameBytes(n) + paramsReplacementFrameBytes(n)); e != nil {
			return 0, e
		}
		next := make([]paramsBlock, len(c.Blocks), n)
		copy(next, c.Blocks)
		receipts := make([]paramsDecrementReceipt, len(c.decrementReceipts), n)
		copy(receipts, c.decrementReceipts)
		frames := make([]paramsSourceFrame, len(c.SourceFrames), n)
		copy(frames, c.SourceFrames)
		sets := make([]paramsSetFrame, len(c.SetFrames), n)
		copy(sets, c.SetFrames)
		replacements := make([]paramsReplacementFrame, len(c.replacementFrames), n)
		copy(replacements, c.replacementFrames)
		c.BudgetField.Release(paramsBlockBytes(cap(c.Blocks)) + paramsReceiptBytes(cap(c.decrementReceipts)) + paramsSourceFrameBytes(cap(c.SourceFrames)) + paramsSetFrameBytes(cap(c.SetFrames)) + paramsReplacementFrameBytes(cap(c.replacementFrames)))
		c.Blocks = next
		c.decrementReceipts = receipts
		c.SourceFrames = frames
		c.SetFrames = sets
		c.replacementFrames = replacements
	}
	c.decrementReceipts = append(c.decrementReceipts, paramsDecrementReceipt{})
	c.SourceFrames = append(c.SourceFrames, paramsSourceFrame{})
	c.SetFrames = append(c.SetFrames, paramsSetFrame{})
	c.replacementFrames = append(c.replacementFrames, paramsReplacementFrame{})
	c.Blocks = append(c.Blocks, paramsBlock{Refs: 1, Generation: 1, Live: true})
	return uint32(len(c.Blocks)), nil
}
func (c *ParamsContext) delEntry(x *paramEntry) error {
	if x.Kind == ParamNumeral {
		if e := c.NumeralsField.Del(&x.Rat); e != nil {
			return e
		}
		c.traceRationalEntryDeallocated(x.ratTraceID)
	}
	if x.text.present && !x.text.borrowed {
		x.text.owned.release()
	}
	*x = paramEntry{}
	return nil
}
func (c *ParamsContext) resetBlock(slot uint32, out *ParamsResetReceipt) error {
	b := &c.Blocks[slot-1]
	serial, e := c.nextSourceInvocation()
	if e != nil {
		return e
	}
	b.resetActive = true
	b.resetKind = paramsResetLive
	b.resetCursor = 0
	b.resetStage = 0
	c.decrementReceipts[slot-1] = paramsDecrementReceipt{generation: b.Generation, invocation: serial}
	if out != nil {
		*out = ParamsResetReceipt{owner: c.Owner, slot: slot, generation: b.Generation, invocation: serial}
	}
	return c.continueReset(slot)
}
func (c *ParamsContext) dec(slot uint32) error {
	return c.decTracked(slot, nil)
}
func (c *ParamsContext) decTracked(slot uint32, out *ParamsDecrementReceipt) error {
	if slot == 0 {
		return nil
	}
	b := &c.Blocks[slot-1]
	if udiag.DebugBuild {
		d := udiag.BorrowProcessDebugContext()
		enabled, e := udiag.SourceAssertionsEnabled(d)
		if e != nil {
			return e
		}
		if enabled && b.Refs == 0 {
			if e := udiag.DebugAssertionFailure(d, "src/util/params.cpp", 338, "m_ref_count > 0"); e != nil {
				return e
			}
		}
	}
	// A shared scalar DEC remains defined during an interrupted live reset.
	// The last-reference destructor would replay already-consumed source leaves;
	// reject that invalid continuation rather than adopt its reset receipt.
	if (b.resetActive && (b.resetKind == paramsResetLive || b.resetKind == paramsResetKey) || c.replacementFrames[slot-1].active || c.SetFrames[slot-1].Active && c.SetFrames[slot-1].Published) && b.Refs == 1 {
		return ErrParamsPrecondition
	}
	b.Refs--
	if b.Refs == 0 {
		b.resetActive = true
		b.resetKind = paramsResetDestroy
		serial, e := c.nextSourceInvocation()
		c.decrementReceipts[slot-1] = paramsDecrementReceipt{generation: b.Generation, invocation: serial}
		if e != nil {
			c.decrementReceipts[slot-1].cause = e
			return e
		}
		if out != nil {
			*out = ParamsDecrementReceipt{owner: c.Owner, slot: slot, generation: b.Generation, InvocationField: serial}
		}
		if e := c.continueDecrement(slot); e != nil {
			return e
		}
		b.Live = false
	}
	return nil
}
func (c *ParamsContext) assign(dst, src *ParamsRef) error {
	if e := c.mutable(dst); e != nil {
		return e
	}
	if e := c.mutable(src); e != nil {
		return e
	}
	serial, e := c.nextSourceInvocation()
	if e != nil {
		return e
	}
	if src.Slot != 0 {
		b := &c.Blocks[src.Slot-1]
		if b.Refs == math.MaxUint32 {
			return arena.ErrBounds
		}
		b.Refs++
		if dst.Slot != 0 {
			c.SourceFrames[dst.Slot-1] = paramsSourceFrame{Active: true, rootGeneration: dst.Generation, Invocation: serial, kind: paramsFrameAssign, HeldSlot: src.Slot, HeldGeneration: src.Generation}
		}
	}
	old := dst.Slot
	if e := c.dec(old); e != nil {
		if old != 0 && c.SourceFrames[old-1].Active {
			c.SourceFrames[old-1].cause = e
		}
		return e
	}
	*dst = *src
	if old != 0 {
		c.SourceFrames[old-1].Active = false
	}
	return nil
}
func (c *ParamsContext) Assign(dst, src *ParamsRef) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.assign(dst, src)
}
func (c *ParamsContext) Del(v *ParamsRef) error {
	return c.DelTracked(v, nil)
}

// DelTracked stamps a receipt only at the actual zero-reference destruction entry.
func (c *ParamsContext) DelTracked(v *ParamsRef, receipt *ParamsDecrementReceipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	e := c.decTracked(v.Slot, receipt)
	if e == nil {
		*v = ParamsRef{}
	}
	return e
}
func (c *ParamsContext) init(v *ParamsRef) error {
	return c.initWithNumeralReceipt(v, nil)
}
func (c *ParamsContext) sourceAssertInitUnique(v *ParamsRef) error {
	if !udiag.DebugBuild {
		return nil
	}
	d := udiag.BorrowProcessDebugContext()
	enabled, e := udiag.SourceAssertionsEnabled(d)
	if e != nil || !enabled {
		return e
	}
	if c.Blocks[v.Slot-1].Refs == 1 {
		return nil
	}
	return udiag.DebugAssertionFailure(d, "src/util/params.cpp", 611, "m_params->m_ref_count == 1")
}
func (c *ParamsContext) initWithNumeralReceipt(v *ParamsRef, receipt *unumeral.RationalCopyAllocationReceipt) error {
	if e := c.mutable(v); e != nil {
		return e
	}
	if v.Slot != 0 && c.Blocks[v.Slot-1].Refs == 1 {
		return c.sourceAssertInitUnique(v)
	}
	serial, e := c.nextSourceInvocation()
	if e != nil {
		return e
	}
	old := v.Slot
	oldGeneration := v.Generation
	slot, e := c.newBlock()
	if e != nil {
		return e
	}
	if old == 0 {
		c.traceBlockAllocated(slot, 600)
	} else {
		c.traceBlockAllocated(slot, 605)
	}
	v.Slot = slot
	v.Generation = c.Blocks[slot-1].Generation
	if old != 0 {
		c.SourceFrames[slot-1] = paramsSourceFrame{Active: true, rootGeneration: v.Generation, Invocation: serial, kind: paramsFrameInit, HeldSlot: old, HeldGeneration: oldGeneration}
		if e = c.copyCoreWithNumeralReceipt(slot, old, receipt); e != nil {
			c.SourceFrames[slot-1].cause = e
			return e
		}
		e = c.dec(old)
		if e != nil {
			c.SourceFrames[slot-1].cause = e
			c.SourceFrames[slot-1].decrementStarted = true
		} else {
			c.SourceFrames[slot-1].Active = false
		}
	}
	if e != nil {
		return e
	}
	return c.sourceAssertInitUnique(v)
}
func (c *ParamsContext) grow(b *paramsBlock) error {
	n, e := pdGrowth(int(b.capacity))
	if e != nil {
		return e
	}
	r, e := c.allocateEntries(uint32(n))
	if e != nil {
		return e
	}
	old := b.entries
	tmp := paramsBlock{entries: r}
	for i := uint32(0); i < b.Size; i++ {
		*c.Entry(&tmp, i) = *c.Entry(b, i)
	}
	b.entries = r
	b.capacity = uint32(n)
	if old.page != 0 {
		prior := paramsBlock{entries: old}
		for i := uint32(0); i < b.Size; i++ {
			*c.Entry(&prior, i) = paramEntry{}
		}
		c.freeEntries(old)
	}
	return nil
}
func (c *ParamsContext) copyCore(dst, src uint32) error {
	return c.copyCoreWithNumeralReceipt(dst, src, nil)
}
func (c *ParamsContext) copyCoreWithNumeralReceipt(dst, src uint32, receipt *unumeral.RationalCopyAllocationReceipt) error {
	if src == 0 {
		return nil
	}
	n := c.Blocks[src-1].Size
	for i := uint32(0); i < n; i++ {
		x := c.Entry(&c.Blocks[src-1], i)
		// The source switch leaves an unrecognized entry untouched after its
		// UNREACHABLE action; a Continue action advances to the next entry.
		switch x.Kind {
		case ParamBool, ParamUInt, ParamDouble, ParamNumeral, ParamSymbol, ParamString:
		default:
			if e := udiag.BorrowProcessDebugContext().UnreachableSource("src/util/params.cpp", 592); e != nil {
				return e
			}
			continue
		}
		if e := c.setCoreWithNumeralReceipt(dst, x.key, x.Kind, x.bits, x.symbol, nil, &x.Rat, &x.text, receipt); e != nil {
			return e
		}
	}
	return nil
}
func (c *ParamsContext) Copy(dst, src *ParamsRef) error {
	return c.CopyWithNumeralReceipt(dst, src, nil)
}

// CopyWithNumeralReceipt gives the caller exact cleanup authority if an
// integer allocation while replacing a matched rational stops after physical
// acquisition. New-entry rational copies use their separate setter frame.
// The partially copied destination remains owned by dst in either case.
func (c *ParamsContext) CopyWithNumeralReceipt(dst, src *ParamsRef, receipt *unumeral.RationalCopyAllocationReceipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if receipt != nil && receipt.Active() {
		return ErrParamsPrecondition
	}
	if e := c.mutable(dst); e != nil {
		return e
	}
	if e := c.mutable(src); e != nil {
		return e
	}
	if dst.Slot == 0 {
		return c.assign(dst, src)
	}
	if e := c.initWithNumeralReceipt(dst, receipt); e != nil {
		return e
	}
	return c.copyCoreWithNumeralReceipt(dst.Slot, src.Slot, receipt)
}
func (c *ParamsContext) Append(dst, src *ParamsRef) error { return c.Copy(dst, src) }

// AppendWithNumeralReceipt exposes the same exact child cleanup authority as
// CopyWithNumeralReceipt. Source params_ref::append calls copy directly.
func (c *ParamsContext) AppendWithNumeralReceipt(dst, src *ParamsRef, receipt *unumeral.RationalCopyAllocationReceipt) error {
	return c.CopyWithNumeralReceipt(dst, src, receipt)
}
func (c *ParamsContext) keyCheck(k usymbol.Symbol) error { _, e := c.SymbolsField.Hash(k); return e }
func (c *ParamsContext) find(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, kind ParamKind, matchAnyKind bool) (*paramEntry, error) {
	if e := c.check(v); e != nil {
		return nil, e
	}
	// params_ref checks its null backing before delegating to params. A null
	// reference has no entry traversal or key hash/equality operation.
	if v.Slot == 0 {
		return nil, nil
	}
	b := &c.Blocks[v.Slot-1]
	// params::GET_VALUE and CONTAINS return before comparing a key when the
	// allocated backing exists but its entry vector is empty.
	if b.Size == 0 {
		return nil, nil
	}
	if !byName {
		if e := c.keyCheck(k); e != nil {
			return nil, e
		}
	}
	for i := uint32(0); i < b.Size; i++ {
		x := c.Entry(b, i)
		match := x.key == k
		if byName {
			var e error
			match, e = c.SymbolsField.EqualString(x.key, name)
			if e != nil {
				return nil, e
			}
		}
		if match && (matchAnyKind || x.Kind == kind || (kind == ParamNumeral && x.Kind == ParamUInt)) {
			return x, nil
		}
	}
	return nil, nil
}
func (c *ParamsContext) Contains(v *ParamsRef, k usymbol.Symbol) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, e := c.find(v, k, nil, false, 0, true)
	return x != nil, e
}
func (c *ParamsContext) ContainsName(v *ParamsRef, k *string) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, e := c.find(v, usymbol.Symbol{}, k, true, 0, true)
	return x != nil, e
}
func (c *ParamsContext) Empty(v *ParamsRef) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return false, e
	}
	return v.Slot == 0 || c.Blocks[v.Slot-1].Size == 0, nil
}
func (c *ParamsContext) Reset(v *ParamsRef) error { return c.ResetTracked(v, nil) }
func (c *ParamsContext) ResetTracked(v *ParamsRef, receipt *ParamsResetReceipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.mutable(v); e != nil {
		return e
	}
	if v.Slot == 0 {
		return nil
	}
	return c.resetBlock(v.Slot, receipt)
}
func (c *ParamsContext) resetKey(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, receipt *ParamsResetKeyReceipt) error {
	if e := c.check(v); e != nil {
		return e
	}
	// A zero-reference destructor has ended the source object's lifetime;
	// unlike a live reset, its missing-key scan is not a defined continuation.
	if v.Slot != 0 && c.Blocks[v.Slot-1].resetActive && c.Blocks[v.Slot-1].resetKind == paramsResetDestroy {
		return ErrParamsPrecondition
	}
	x, e := c.find(v, k, name, byName, 0, true)
	if e != nil || x == nil {
		return e
	}
	if e := c.mutable(v); e != nil {
		return e
	}
	b := &c.Blocks[v.Slot-1]
	for i := uint32(0); i < b.Size; i++ {
		if c.Entry(b, i) != x {
			continue
		}
		serial, e := c.nextSourceInvocation()
		if e != nil {
			return e
		}
		b.resetActive = true
		b.resetKind = paramsResetKey
		b.resetCursor = i
		b.resetStage = 0
		c.decrementReceipts[v.Slot-1] = paramsDecrementReceipt{generation: b.Generation, invocation: serial}
		if receipt != nil {
			*receipt = ParamsResetKeyReceipt{owner: c.Owner, slot: v.Slot, generation: v.Generation, invocation: serial, index: i}
		}
		return c.continueResetKey(v.Slot)
	}
	return nil
}
func (c *ParamsContext) ResetKey(v *ParamsRef, k usymbol.Symbol) error {
	return c.ResetKeyTracked(v, k, nil)
}
func (c *ParamsContext) ResetKeyTracked(v *ParamsRef, k usymbol.Symbol, receipt *ParamsResetKeyReceipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resetKey(v, k, nil, false, receipt)
}
func (c *ParamsContext) ResetKeyName(v *ParamsRef, k *string) error {
	return c.ResetKeyNameTracked(v, k, nil)
}
func (c *ParamsContext) ResetKeyNameTracked(v *ParamsRef, k *string, receipt *ParamsResetKeyReceipt) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.resetKey(v, usymbol.Symbol{}, k, true, receipt)
}
func (c *ParamsContext) storeText(s *string, source *paramsText) (paramsText, error) {
	if source != nil {
		if !source.present {
			return paramsText{}, nil
		}
		value, e := source.owned.read()
		if e != nil {
			return paramsText{}, e
		}
		return paramsText{owned: &paramsOwnedString{value: value, live: true}, present: true}, nil
	}
	if s == nil {
		return paramsText{}, nil
	}
	value := (*s)[:utext.CStringLength(*s)]
	return paramsText{owned: &paramsOwnedString{value: value, live: true}, present: true}, nil
}
func (c *ParamsContext) setCore(slot uint32, k usymbol.Symbol, kind ParamKind, bits uint64, sym usymbol.Symbol, str *string, rat *unumeral.OwnedRational, text *paramsText) error {
	return c.setCoreWithNumeralReceipt(slot, k, kind, bits, sym, str, rat, text, nil)
}
func (c *ParamsContext) setCoreWithNumeralReceipt(slot uint32, k usymbol.Symbol, kind ParamKind, bits uint64, sym usymbol.Symbol, str *string, rat *unumeral.OwnedRational, text *paramsText, receipt *unumeral.RationalCopyAllocationReceipt) error {
	b := &c.Blocks[slot-1]
	var x *paramEntry
	var matched uint32
	for i := uint32(0); i < b.Size; i++ {
		p := c.Entry(b, i)
		if p.key == k {
			x = p
			matched = i
			break
		}
	}
	if kind == ParamNumeral && x != nil {
		if x.Kind != ParamNumeral {
			return c.setMatchedNewNumeral(slot, matched, k, rat)
		}
		if receipt != nil {
			return c.NumeralsField.CopyWithAllocationReceipt(&x.Rat, rat, receipt)
		}
		return c.NumeralsField.Copy(&x.Rat, rat)
	}
	if kind == ParamNumeral && x == nil {
		return c.setNewNumeral(slot, k, rat)
	}
	var replacementSerial uint64
	if x != nil && x.Kind == ParamNumeral {
		var e error
		replacementSerial, e = c.nextSourceInvocation()
		if e != nil {
			return e
		} // No copied-text acquisition has happened yet.
	}
	// Prepare the native string lifetime before deleting a replaced value.
	next := paramEntry{key: k, Kind: kind, bits: bits, symbol: sym}
	if kind == ParamString {
		if text != nil && text.borrowed {
			next.text = *text
		} else {
			t, e := c.storeText(str, text)
			if e != nil {
				return e
			}
			next.text = t
		}
	}
	if kind == ParamNumeral {
		r, e := c.NumeralsField.NewValue()
		if e != nil {
			return e
		}
		next.Rat = r
		if e = c.NumeralsField.Copy(&next.Rat, rat); e != nil {
			c.NumeralsField.Del(&next.Rat)
			return e
		}
	}
	if x != nil {
		if x.Kind == ParamNumeral {
			return c.beginReplacement(slot, matched, next, replacementSerial)
		}
		if e := c.delEntry(x); e != nil {
			c.delEntry(&next)
			return e
		}
		if kind == ParamSymbol && sym.IsNumerical() {
			// SET_SYM reaches bare_str after del_value and the kind store.
			// A stopped assertion leaves the matched entry partly published.
			x.key, x.Kind = k, ParamSymbol
			spelling, _, e := c.SymbolsField.BareString(sym)
			if e != nil {
				return e
			}
			x.symbol, e = c.SymbolsField.Intern(spelling)
			return e
		}
		*x = next
		return nil
	}
	if b.Size == b.capacity {
		if e := c.grow(b); e != nil {
			c.delEntry(&next)
			return e
		}
	}
	*c.Entry(b, b.Size) = next
	b.Size++
	return nil
}
func (c *ParamsContext) set(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, kind ParamKind, bits uint64, sym usymbol.Symbol, str *string, rat *unumeral.OwnedRational) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	// Provenance guards precede source mutation; key construction follows init.
	if !byName {
		if e := c.keyCheck(k); e != nil {
			return e
		}
	}
	if kind == ParamSymbol {
		if e := c.keyCheck(sym); e != nil {
			return e
		}
	}
	if kind == ParamNumeral {
		if _, e := c.NumeralsField.IsRational(rat); e != nil {
			return e
		}
	}
	if e := c.init(v); e != nil {
		return e
	}
	if byName {
		var e error
		k, e = c.SymbolsField.InternNullable(name)
		if e != nil {
			return e
		}
	}
	matched := false
	if kind == ParamSymbol && sym.IsNumerical() {
		// Matched SET_SYM reaches bare_str only after deleting the old value
		// and publishing CPK_SYMBOL. Numerals retain an owning continuation.
		b := &c.Blocks[v.Slot-1]
		for i := uint32(0); i < b.Size; i++ {
			x := c.Entry(b, i)
			if x.key == k {
				matched = true
				break
			}
		}
	}
	if kind == ParamSymbol && sym.IsNumerical() && !matched {
		// params::set_sym stores bare_str(), which is an empty C string for
		// a numerical symbol after the source assertion is erased/continued.
		spelling, _, e := c.SymbolsField.BareString(sym)
		if e != nil {
			return e
		}
		sym, e = c.SymbolsField.Intern(spelling)
		if e != nil {
			return e
		}
	}
	return c.setCore(v.Slot, k, kind, bits, sym, str, rat, nil)
}
func (c *ParamsContext) SetBool(v *ParamsRef, k usymbol.Symbol, b bool) error {
	var n uint64
	if b {
		n = 1
	}
	return c.set(v, k, nil, false, ParamBool, n, usymbol.Symbol{}, nil, nil)
}
func (c *ParamsContext) SetBoolName(v *ParamsRef, k *string, b bool) error {
	var n uint64
	if b {
		n = 1
	}
	return c.set(v, usymbol.Symbol{}, k, true, ParamBool, n, usymbol.Symbol{}, nil, nil)
}
func (c *ParamsContext) SetUInt(v *ParamsRef, k usymbol.Symbol, n uint32) error {
	return c.set(v, k, nil, false, ParamUInt, uint64(n), usymbol.Symbol{}, nil, nil)
}
func (c *ParamsContext) SetUIntName(v *ParamsRef, k *string, n uint32) error {
	return c.set(v, usymbol.Symbol{}, k, true, ParamUInt, uint64(n), usymbol.Symbol{}, nil, nil)
}
func (c *ParamsContext) SetDouble(v *ParamsRef, k usymbol.Symbol, n float64) error {
	return c.set(v, k, nil, false, ParamDouble, math.Float64bits(n), usymbol.Symbol{}, nil, nil)
}
func (c *ParamsContext) SetDoubleName(v *ParamsRef, k *string, n float64) error {
	return c.set(v, usymbol.Symbol{}, k, true, ParamDouble, math.Float64bits(n), usymbol.Symbol{}, nil, nil)
}
func (c *ParamsContext) SetString(v *ParamsRef, k usymbol.Symbol, s *string) error {
	return c.set(v, k, nil, false, ParamString, 0, usymbol.Symbol{}, s, nil)
}

// SetStringSymbol retains the source interned bare-str pointer as a canonical
// scalar symbol. The borrowed SymbolManager must outlive every retained Ref.
// It never allocates or owns a second spelling payload.
func (c *ParamsContext) SetStringSymbol(v *ParamsRef, k, spelling usymbol.Symbol) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	if e := c.keyCheck(k); e != nil {
		return e
	}
	m := c.SymbolsField
	e := m.Validate(spelling)
	if e == nil && (spelling.IsNumerical() || spelling.Value == 0) {
		e = ErrParamsPrecondition
	}
	if e != nil {
		return e
	}
	if e = c.init(v); e != nil {
		return e
	}
	text := paramsText{present: true, borrowed: true}
	return c.setCore(v.Slot, k, ParamString, 0, spelling, nil, nil, &text)
}

func (c *ParamsContext) SetStringName(v *ParamsRef, k, s *string) error {
	return c.set(v, usymbol.Symbol{}, k, true, ParamString, 0, usymbol.Symbol{}, s, nil)
}
func (c *ParamsContext) SetSymbol(v *ParamsRef, k, s usymbol.Symbol) error {
	return c.set(v, k, nil, false, ParamSymbol, 0, s, nil, nil)
}
func (c *ParamsContext) SetSymbolName(v *ParamsRef, k *string, s usymbol.Symbol) error {
	return c.set(v, usymbol.Symbol{}, k, true, ParamSymbol, 0, s, nil, nil)
}
func (c *ParamsContext) SetRat(v *ParamsRef, k usymbol.Symbol, r *unumeral.OwnedRational) error {
	return c.set(v, k, nil, false, ParamNumeral, 0, usymbol.Symbol{}, nil, r)
}
func (c *ParamsContext) SetRatName(v *ParamsRef, k *string, r *unumeral.OwnedRational) error {
	return c.set(v, usymbol.Symbol{}, k, true, ParamNumeral, 0, usymbol.Symbol{}, nil, r)
}
func (c *ParamsContext) get(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, kind ParamKind, fallback *ParamsRef) (paramEntry, bool, error) {
	x, e := c.find(v, k, name, byName, kind, false)
	if e != nil {
		return paramEntry{}, false, e
	}
	if fallback != nil {
		if x == nil {
			// GET_VALUE2 returns a primary match before it calls the fallback.
			// In particular, a dead fallback has no observable effect on a hit.
			if e = c.check(fallback); e != nil {
				return paramEntry{}, false, e
			}
			x, e = c.find(fallback, usymbol.Symbol{}, name, true, kind, false)
			if e != nil {
				return paramEntry{}, false, e
			}
		}
	}
	if x == nil {
		return paramEntry{}, false, nil
	}
	return *x, true, nil
}
func (c *ParamsContext) getBool(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, f *ParamsRef, d bool) (bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, k, name, byName, ParamBool, f)
	if e != nil || !ok {
		return d, e
	}
	return x.bits != 0, nil
}
func (c *ParamsContext) GetBool(v *ParamsRef, k usymbol.Symbol, d bool) (bool, error) {
	return c.getBool(v, k, nil, false, nil, d)
}
func (c *ParamsContext) GetBoolName(v *ParamsRef, k *string, d bool) (bool, error) {
	return c.getBool(v, usymbol.Symbol{}, k, true, nil, d)
}
func (c *ParamsContext) GetBoolFallbackName(v *ParamsRef, k *string, f *ParamsRef, d bool) (bool, error) {
	return c.getBool(v, usymbol.Symbol{}, k, true, f, d)
}
func (c *ParamsContext) getUInt(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, f *ParamsRef, d uint32) (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, k, name, byName, ParamUInt, f)
	if e != nil || !ok {
		return d, e
	}
	return uint32(x.bits), nil
}
func (c *ParamsContext) GetUInt(v *ParamsRef, k usymbol.Symbol, d uint32) (uint32, error) {
	return c.getUInt(v, k, nil, false, nil, d)
}
func (c *ParamsContext) GetUIntName(v *ParamsRef, k *string, d uint32) (uint32, error) {
	return c.getUInt(v, usymbol.Symbol{}, k, true, nil, d)
}
func (c *ParamsContext) GetUIntFallbackName(v *ParamsRef, k *string, f *ParamsRef, d uint32) (uint32, error) {
	return c.getUInt(v, usymbol.Symbol{}, k, true, f, d)
}
func (c *ParamsContext) getDouble(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, f *ParamsRef, d float64) (float64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, k, name, byName, ParamDouble, f)
	if e != nil || !ok {
		return d, e
	}
	return math.Float64frombits(x.bits), nil
}
func (c *ParamsContext) GetDouble(v *ParamsRef, k usymbol.Symbol, d float64) (float64, error) {
	return c.getDouble(v, k, nil, false, nil, d)
}
func (c *ParamsContext) GetDoubleName(v *ParamsRef, k *string, d float64) (float64, error) {
	return c.getDouble(v, usymbol.Symbol{}, k, true, nil, d)
}
func (c *ParamsContext) GetDoubleFallbackName(v *ParamsRef, k *string, f *ParamsRef, d float64) (float64, error) {
	return c.getDouble(v, usymbol.Symbol{}, k, true, f, d)
}
func (c *ParamsContext) getSymbol(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, f *ParamsRef, d usymbol.Symbol) (usymbol.Symbol, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, k, name, byName, ParamSymbol, f)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	if !ok {
		return d, nil
	}
	return x.symbol, nil
}
func (c *ParamsContext) GetSymbol(v *ParamsRef, k, d usymbol.Symbol) (usymbol.Symbol, error) {
	return c.getSymbol(v, k, nil, false, nil, d)
}
func (c *ParamsContext) GetSymbolName(v *ParamsRef, k *string, d usymbol.Symbol) (usymbol.Symbol, error) {
	return c.getSymbol(v, usymbol.Symbol{}, k, true, nil, d)
}
func (c *ParamsContext) GetSymbolFallbackName(v *ParamsRef, k *string, f *ParamsRef, d usymbol.Symbol) (usymbol.Symbol, error) {
	return c.getSymbol(v, usymbol.Symbol{}, k, true, f, d)
}

// String results are caller-owned copies; bool distinguishes null from empty.
func (c *ParamsContext) getString(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, f *ParamsRef, d *string) (string, bool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, k, name, byName, ParamString, f)
	if e != nil {
		return "", false, e
	}
	if !ok {
		if d == nil {
			return "", false, nil
		}
		return paramsCString(*d), true, nil
	}
	if !x.text.present {
		return "", false, nil
	}
	c.workspace = c.workspace[:0]
	if e = c.appendEntryText(&x); e != nil {
		return "", false, e
	}
	return string(c.workspace), true, nil
}
func (c *ParamsContext) GetString(v *ParamsRef, k usymbol.Symbol, d *string) (string, bool, error) {
	return c.getString(v, k, nil, false, nil, d)
}
func (c *ParamsContext) GetStringName(v *ParamsRef, k, d *string) (string, bool, error) {
	return c.getString(v, usymbol.Symbol{}, k, true, nil, d)
}
func (c *ParamsContext) GetStringFallbackName(v *ParamsRef, k *string, f *ParamsRef, d *string) (string, bool, error) {
	return c.getString(v, usymbol.Symbol{}, k, true, f, d)
}
func (c *ParamsContext) getRat(v *ParamsRef, k usymbol.Symbol, name *string, byName bool, d *unumeral.OwnedRational) (unumeral.OwnedRational, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	x, ok, e := c.get(v, k, name, byName, ParamNumeral, nil)
	if e != nil {
		return unumeral.OwnedRational{}, e
	}
	if !ok {
		// params::get_rat copies the default only after its entry scan misses.
		if _, e := c.NumeralsField.IsRational(d); e != nil {
			return unumeral.OwnedRational{}, e
		}
	}
	r, e := c.NumeralsField.NewValue()
	if e != nil {
		return r, e
	}
	if ok && x.Kind == ParamUInt {
		e = c.NumeralsField.AssignInt(&r, int32(uint32(x.bits)))
	} else {
		src := d
		if ok {
			src = &x.Rat
		}
		e = c.NumeralsField.Copy(&r, src)
	}
	if e != nil {
		if cleanupErr := c.NumeralsField.Del(&r); cleanupErr != nil {
			// The result still owns any numeral storage that cleanup could not
			// release. Return its handle so the caller can finish that cleanup.
			return r, errors.Join(e, cleanupErr)
		}
		return unumeral.OwnedRational{}, e
	}
	return r, nil
}
func (c *ParamsContext) GetRat(v *ParamsRef, k usymbol.Symbol, d *unumeral.OwnedRational) (unumeral.OwnedRational, error) {
	return c.getRat(v, k, nil, false, d)
}
func (c *ParamsContext) GetRatName(v *ParamsRef, k *string, d *unumeral.OwnedRational) (unumeral.OwnedRational, error) {
	return c.getRat(v, usymbol.Symbol{}, k, true, d)
}
func paramsCString(s string) string {
	if i := strings.IndexByte(s, 0); i >= 0 {
		return s[:i]
	}
	return s
}
func (c *ParamsContext) reserveFormat(extra int) error {
	if extra < 0 || extra > int(^uint(0)>>1)-len(c.workspace) {
		return arena.ErrBounds
	}
	want := len(c.workspace) + extra
	if want <= cap(c.workspace) {
		return nil
	}
	n := cap(c.workspace)
	if n == 0 {
		n = 64
	}
	for n < want {
		if n > int(^uint(0)>>1)/2 {
			return arena.ErrBounds
		}
		n *= 2
	}
	next := make([]byte, len(c.workspace), n)
	copy(next, c.workspace)
	c.workspace = next
	return nil
}
func (c *ParamsContext) appendString(s string) error {
	if c.formatWriter != nil {
		_, _ = c.formatWriter.WriteString(s)
		return nil
	}
	if e := c.reserveFormat(len(s)); e != nil {
		return e
	}
	c.workspace = append(c.workspace, s...)
	return nil
}
func (c *ParamsContext) appendByte(b byte) error {
	if c.formatWriter != nil {
		return c.formatWriter.WriteByte(b)
	}
	if e := c.reserveFormat(1); e != nil {
		return e
	}
	c.workspace = append(c.workspace, b)
	return nil
}

// appendEntryText copies actual bytes into the reusable Go format workspace,
// directly from either owned Go text or the borrowed symbol's real backing.
func (c *ParamsContext) appendEntryText(x *paramEntry) error {
	if x.text.borrowed {
		if !x.text.present || x.symbol.Value == 0 || x.symbol.IsNumerical() {
			return ErrParamsPrecondition
		}
		return c.appendSymbol(x.symbol, false, false)
	}
	if !x.text.present {
		return nil
	}
	value, e := x.text.owned.read()
	if e != nil {
		return e
	}
	return c.appendString(value)
}

func (c *ParamsContext) appendSymbol(k usymbol.Symbol, strMode, normalize bool) error {
	m := c.SymbolsField
	if e := m.Validate(k); e != nil {
		return e
	}
	if k.IsNumerical() {
		var a [32]byte
		b := strconv.AppendInt(a[:0], int64(int32(k.Value)), 10)
		if e := c.appendString("k!"); e != nil {
			return e
		}
		if c.formatWriter != nil {
			return c.appendString(string(b))
		}
		if e := c.reserveFormat(len(b)); e != nil {
			return e
		}
		c.workspace = append(c.workspace, b...)
		return nil
	}
	if k.Value == 0 {
		if normalize {
			return ErrParamsPrecondition
		}
		if strMode {
			return c.appendString("<null>")
		}
		return c.appendString("null")
	}
	text, _, e := m.BareString(k)
	if e != nil {
		return e
	}
	for i := 0; i < len(text); i++ {
		b := text[i]
		if normalize {
			if b == '-' {
				b = '_'
			} else if b >= 'A' && b <= 'Z' {
				b = b - 'A' + 'a'
			}
		}
		if e = c.appendByte(b); e != nil {
			return e
		}
	}
	return nil
}
func (c *ParamsContext) appendValue(x *paramEntry) error {
	var a [128]byte
	var b []byte
	switch x.Kind {
	case ParamBool:
		if x.bits != 0 {
			return c.appendString("true")
		}
		return c.appendString("false")
	case ParamUInt:
		b = strconv.AppendUint(a[:0], uint64(uint32(x.bits)), 10)
	case ParamDouble:
		v := math.Float64frombits(x.bits)
		if math.IsNaN(v) {
			if math.Signbit(v) {
				return c.appendString("-nan")
			}
			return c.appendString("nan")
		}
		if math.IsInf(v, 1) {
			return c.appendString("inf")
		}
		if math.IsInf(v, -1) {
			return c.appendString("-inf")
		}
		b = strconv.AppendFloat(a[:0], v, 'g', 6, 64)
	case ParamSymbol:
		return c.appendSymbol(x.symbol, false, false)
	case ParamString:
		if !x.text.present {
			return utext.ErrNullCString
		}
		return c.appendEntryText(x)
	case ParamNumeral:
		for {
			if c.formatWriter != nil {
				c.workspace = c.workspace[:0]
			}
			if e := c.reserveFormat(64); e != nil {
				return e
			}
			old := len(c.workspace)
			dst := c.workspace[old:cap(c.workspace)]
			n, e := c.NumeralsField.FormatInto(&x.Rat, dst)
			if errors.Is(e, unumeral.ErrShortBuffer) {
				if e = c.reserveFormat(cap(c.workspace) - old + 1); e != nil {
					return e
				}
				continue
			}
			if e != nil {
				return e
			}
			if c.formatWriter != nil {
				return c.appendString(string(dst[:n]))
			}
			c.workspace = c.workspace[:old+n]
			return nil
		}
	default:
		return ErrParamsPrecondition
	}
	if c.formatWriter != nil {
		return c.appendString(string(b))
	}
	if e := c.reserveFormat(len(b)); e != nil {
		return e
	}
	c.workspace = append(c.workspace, b...)
	return nil
}
func (c *ParamsContext) format(v *ParamsRef, key *usymbol.Symbol, module *string, d *ParamDescrs) error {
	if e := c.check(v); e != nil {
		return e
	}
	populated := v.Slot != 0 && c.Blocks[v.Slot-1].Size != 0
	if key != nil && populated {
		if e := c.keyCheck(*key); e != nil {
			return e
		}
	}
	if d != nil && populated {
		if d.manager != c.SymbolsField {
			return ErrParamsOwner
		}
		if e := d.open(); e != nil {
			return e
		}
	}
	c.workspace = c.workspace[:0]
	if d == nil && key == nil {
		if e := c.appendString("(params"); e != nil {
			return e
		}
	}
	if v.Slot != 0 {
		b := &c.Blocks[v.Slot-1]
		for i := uint32(0); i < b.Size; i++ {
			x := c.Entry(b, i)
			if key != nil {
				if x.key != *key {
					continue
				}
				// params::display(key) prints this literal for an unknown kind.
				switch x.Kind {
				case ParamBool, ParamUInt, ParamDouble, ParamNumeral, ParamSymbol, ParamString:
				default:
					return c.appendString("internal")
				}
				return c.appendValue(x)
			}
			if d != nil {
				_, found, e := d.find(x.key)
				if e != nil {
					return e
				}
				if !found {
					continue
				}
				if e = c.appendString("(set-option :"); e != nil {
					return e
				}
				if module == nil {
					return utext.ErrNullCString
				}
				if e = c.appendString(paramsCString(*module)); e != nil {
					return e
				}
				if e = c.appendByte('.'); e != nil {
					return e
				}
			} else {
				if e := c.appendByte(' '); e != nil {
					return e
				}
			}
			if e := c.appendSymbol(x.key, false, false); e != nil {
				return e
			}
			switch x.Kind {
			case ParamBool, ParamUInt, ParamDouble, ParamNumeral, ParamSymbol, ParamString:
			default:
				line := int32(434)
				if d != nil {
					line = 469
				}
				if e := udiag.BorrowProcessDebugContext().UnreachableSource("src/util/params.cpp", line); e != nil {
					return e
				}
				if d != nil {
					if e := c.appendString(")\n"); e != nil {
						return e
					}
				}
				continue
			}
			if e := c.appendByte(' '); e != nil {
				return e
			}
			if e := c.appendValue(x); e != nil {
				return e
			}
			if d != nil {
				if e := c.appendString(")\n"); e != nil {
					return e
				}
			}
		}
	}
	if key != nil {
		return c.appendString("default")
	}
	if d == nil {
		return c.appendByte(')')
	}
	return nil
}
func (c *ParamsContext) Display(v *ParamsRef) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.format(v, nil, nil, nil); e != nil {
		if errors.Is(e, utext.ErrNullCString) {
			return string(c.workspace), e
		}
		return "", e
	}
	return string(c.workspace), nil
}
func (c *ParamsContext) FormatInto(v *ParamsRef, dst []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.format(v, nil, nil, nil); e != nil {
		return 0, e
	}
	if len(dst) < len(c.workspace) {
		return len(c.workspace), unumeral.ErrShortBuffer
	}
	return copy(dst, c.workspace), nil
}
func (c *ParamsContext) DisplayKey(v *ParamsRef, k usymbol.Symbol) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.format(v, &k, nil, nil); e != nil {
		if errors.Is(e, utext.ErrNullCString) {
			return string(c.workspace), e
		}
		return "", e
	}
	return string(c.workspace), nil
}
func (c *ParamsContext) DisplaySMT2(v *ParamsRef, module *string, d *ParamDescrs) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return "", e
	}
	// params_ref::display_smt2 leaves an empty reference untouched.
	if v.Slot == 0 || c.Blocks[v.Slot-1].Size == 0 {
		return "", nil
	}
	if d == nil {
		return "", ErrParamsPrecondition
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := c.format(v, nil, module, d); e != nil {
		if errors.Is(e, utext.ErrNullCString) {
			return string(c.workspace), e
		}
		return "", e
	}
	return string(c.workspace), nil
}
func (c *ParamsContext) appendDescrs(d *ParamDescrs) error {
	var names []usymbol.Symbol
	for key := range d.entries.All() {
		names = append(names, key)
	}
	if e := d.sortSymbols(names); e != nil {
		return e
	}
	for _, k := range names {
		if e := c.appendString("  "); e != nil {
			return e
		}
		if e := c.appendSymbol(k, false, true); e != nil {
			return e
		}
		info, found, e := d.find(k)
		if e != nil {
			return e
		}
		if !found {
			return ErrParamDescrsPrecondition
		}
		if e := sourceAssertParamDescription(info.descr.present); e != nil {
			return e
		}
		if e = c.appendString(" ("); e != nil {
			return e
		}
		if e = c.appendString(info.kind.String()); e != nil {
			return e
		}
		if e = c.appendByte(')'); e != nil {
			return e
		}
		if info.def.present {
			if e = c.appendString(" (default: "); e != nil {
				return e
			}
			if e = c.appendString(info.def.value); e != nil {
				return e
			}
			if e = c.appendByte(')'); e != nil {
				return e
			}
		}
		if e = c.appendByte('\n'); e != nil {
			return e
		}
	}
	return nil
}
func (c *ParamsContext) Validate(v *ParamsRef, d *ParamDescrs) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return e
	}
	// An absent or allocated-empty params vector never inspects descriptors.
	if v.Slot == 0 || c.Blocks[v.Slot-1].Size == 0 {
		return nil
	}
	if d == nil || d.manager != c.SymbolsField {
		return ErrParamsOwner
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := d.open(); e != nil {
		return e
	}
	b := &c.Blocks[v.Slot-1]
	for i := uint32(0); i < b.Size; i++ {
		x := c.Entry(b, i)
		expected, e := paramsDescrKindInModule(d, &x.key)
		if e != nil {
			return e
		}
		if expected == x.Kind || (x.Kind == ParamUInt && expected == ParamNumeral) {
			continue
		}
		c.workspace = c.workspace[:0]
		if expected == ParamInvalid {
			if e = c.appendString("unknown parameter '"); e != nil {
				return e
			}
			if e = c.appendSymbol(x.key, true, false); e != nil {
				return e
			}
			if e = c.appendString("'\nLegal parameters are:\n"); e != nil {
				return e
			}
			e = c.appendDescrs(d)
			if e != nil {
				return e
			}
		} else {
			if e = c.appendString("Parameter "); e != nil {
				return e
			}
			if e = c.appendSymbol(x.key, true, false); e != nil {
				return e
			}
			if e = c.appendString(" was given argument of type "); e != nil {
				return e
			}
			if e = c.appendString(x.Kind.String()); e != nil {
				return e
			}
			if e = c.appendString(", expected "); e != nil {
				return e
			}
			if e = c.appendString(expected.String()); e != nil {
				return e
			}
		}
		return z3exception.NewDefaultException(string(c.workspace))
	}
	return nil
}
func (c *ParamsContext) Stats() ParamsStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	var s ParamsStats
	if c.Closed {
		return s
	}
	s.HeaderBytes = c.headerBytes
	s.BlockBytes = paramsBlockBytes(cap(c.Blocks))
	s.ReceiptBytes = paramsReceiptBytes(cap(c.decrementReceipts))
	s.SourceFrameBytes = paramsSourceFrameBytes(cap(c.SourceFrames))
	s.SourceSetBytes = paramsSetFrameBytes(cap(c.SetFrames))
	s.ReplacementBytes = paramsReplacementFrameBytes(cap(c.replacementFrames))
	s.DirectoryBytes = paramsDirectoryBytes(cap(c.pages))
	for _, p := range c.pages {
		s.PageBytes += paramsPageBytes(p)
	}
	for _, b := range c.Blocks {
		if b.Live {
			s.LiveBlocks++
			s.References += b.Refs
			s.Entries += b.Size
		}
	}
	return s
}
func (c *ParamsContext) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Self != c {
		return ErrParamsOwner
	}
	if c.Closed {
		return nil
	}
	for i := range c.replacementFrames {
		if c.replacementFrames[i].active {
			if e := c.continueReplacement(uint32(i+1), false); e != nil {
				return e
			}
		}
	}
	for i := range c.SetFrames {
		if c.SetFrames[i].Active {
			if e := c.cleanupSetFrame(uint32(i + 1)); e != nil {
				return e
			}
		}
	}
	for i := range c.SourceFrames {
		if c.SourceFrames[i].Active {
			if e := c.cleanupSourceFrame(uint32(i + 1)); e != nil {
				return e
			}
		}
	}

	for i := range c.Blocks {
		if c.Blocks[i].Live {
			if c.Blocks[i].resetActive {
				if e := c.continueDecrement(uint32(i + 1)); e != nil {
					return e
				}
			} else if e := c.resetBlock(uint32(i+1), nil); e != nil {
				return e
			}
		}
	}
	for _, p := range c.pages {
		c.BudgetField.Release(paramsPageBytes(p))
	}
	c.BudgetField.Release(paramsDirectoryBytes(cap(c.pages)) + paramsBlockBytes(cap(c.Blocks)) + paramsReceiptBytes(cap(c.decrementReceipts)) + paramsSourceFrameBytes(cap(c.SourceFrames)) + paramsSetFrameBytes(cap(c.SetFrames)) + paramsReplacementFrameBytes(cap(c.replacementFrames)) + c.headerBytes)
	c.Blocks = nil
	c.decrementReceipts = nil
	c.SourceFrames = nil
	c.SetFrames = nil
	c.replacementFrames = nil
	c.pages = nil
	c.workspace = nil
	c.Closed = true
	return nil
}

// DisplayKeyName constructs the source temporary symbol even with null backing.
func (c *ParamsContext) DisplayKeyName(v *ParamsRef, name *string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return "", e
	}
	k, e := c.SymbolsField.InternNullable(name)
	if e != nil {
		return "", e
	}
	if e = c.format(v, &k, nil, nil); e != nil {
		if errors.Is(e, utext.ErrNullCString) {
			return string(c.workspace), e
		}
		return "", e
	}
	return string(c.workspace), nil
}
func (c *ParamsContext) FormatKeyInto(v *ParamsRef, k usymbol.Symbol, dst []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.format(v, &k, nil, nil); e != nil {
		return 0, e
	}
	if len(dst) < len(c.workspace) {
		return len(c.workspace), unumeral.ErrShortBuffer
	}
	return copy(dst, c.workspace), nil
}
func (c *ParamsContext) FormatSMT2Into(v *ParamsRef, module *string, d *ParamDescrs, dst []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.check(v); e != nil {
		return 0, e
	}
	if v.Slot == 0 || c.Blocks[v.Slot-1].Size == 0 {
		return 0, nil
	}
	if d == nil {
		return 0, ErrParamsPrecondition
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if e := c.format(v, nil, module, d); e != nil {
		return 0, e
	}
	if len(dst) < len(c.workspace) {
		return len(c.workspace), unumeral.ErrShortBuffer
	}
	return copy(dst, c.workspace), nil
}

// Caller holds d.mu throughout validation, including prefix publication.
func paramsDescrKindInModule(d *ParamDescrs, name *usymbol.Symbol) (ParamKind, error) {
	k, e := d.kind(*name)
	if e != nil || k != ParamInvalid || name.IsNumerical() {
		return k, e
	}
	if *name == (usymbol.Symbol{}) {
		return ParamInvalid, ErrParamDescrsPrecondition
	}
	prefix, suffix, split, e := d.splitName(*name)
	if e != nil || !split {
		return ParamInvalid, e
	}
	k, e = d.kind(suffix)
	if e != nil || k == ParamInvalid {
		return k, e
	}
	info, found, e := d.find(suffix)
	if e != nil {
		return ParamInvalid, e
	}
	if !found {
		return ParamInvalid, ErrParamDescrsPrecondition
	}
	module, e := d.internText(info.module)
	if e != nil {
		return ParamInvalid, e
	}
	if module == prefix {
		*name = suffix
		return k, nil
	}
	return ParamInvalid, nil
}

// DebugString is the source Z3DEBUG pp text boundary, including its newline.
func (c *ParamsContext) DebugString(v *ParamsRef) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e := c.format(v, nil, nil, nil); e != nil {
		if errors.Is(e, utext.ErrNullCString) {
			return string(c.workspace), e
		}
		return "", e
	}
	if e := c.appendByte('\n'); e != nil {
		return "", e
	}
	return string(c.workspace), nil
}
