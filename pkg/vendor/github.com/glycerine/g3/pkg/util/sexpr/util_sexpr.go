package sexpr

// Source: src/util/sexpr.h; src/util/sexpr.cpp:28-55.
// Concrete S-expression records retain source reference counting with Go storage.

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
	utext "github.com/glycerine/g3/pkg/util/text"
)

var (
	ErrSexprKind                = errors.New("g3: wrong sexpr kind")
	ErrSexprOwner               = errors.New("g3: sexpr owner mismatch")
	ErrSexprRefCount            = errors.New("g3: sexpr zero reference count")
	ErrSexprDisplayContinuation = errors.New("g3: source sexpr atom display continuation is undefined")
)

type SexprKind uint32

const (
	SexprComposite SexprKind = iota
	SexprNumeral
	SexprBVNumeral
	SexprString
	SexprKeyword
	SexprSymbol
)
const SexprDefaultPosition uint32 = math.MaxUint32

// A handle borrows a record; factory results have reference count zero.
type SexprHandle struct {
	owner            uint64
	slot, generation uint32
}

func (h SexprHandle) IsNull() bool { return h == SexprHandle{} }

type SexprConfig struct {
	Budget              *arena.Budget
	PageSize, SlotLimit uint32
	Symbols             *usymbol.Manager
	Rationals           *unumeral.RationalService
}
type sexprRecord struct {
	Kind                   SexprKind
	refs, line, pos, width uint32
	Numeral                unumeral.OwnedRational
	symbol                 usymbol.Symbol
	children               []SexprHandle
	text                   string
	length                 uint32
	generation, next       uint32
	Live, retired          bool
}
type sexprPage struct{ Nodes []sexprRecord }
type sexprFrame struct {
	h     SexprHandle
	index uint32
}

// SexprStats reports budgeted record storage; Go payloads and scratch are excluded.
type SexprStats struct {
	Live, Slots, Retired, Pages                           uint32
	OwnerBytes, RecordBytes, DirectoryBytes, ScratchBytes uint64
}

// Borrowed symbol and rational owners must outlive this manager. The shared
// Budget remains owner-confined even when these managers use separate locks.
type SexprManager struct {
	borrowedNodes               SexprBorrowedNodes
	mu                          sync.Mutex
	self                        *SexprManager
	Config                      SexprConfig
	owner                       uint64
	nextVector                  uint64
	Pages                       []sexprPage
	free, slots, live, retired  uint32
	directoryBytes, recordBytes uint64
	deletion                    []SexprHandle
	frames                      []sexprFrame
	output, stage               []byte
	bvDigits                    []byte
	closed                      bool
}

var sexprNextOwner uint64

func nextSexprOwner() (uint64, error) {
	for {
		old := atomic.LoadUint64(&sexprNextOwner)
		if old == math.MaxUint64 {
			return 0, arena.ErrBounds
		}
		if atomic.CompareAndSwapUint64(&sexprNextOwner, old, old+1) {
			return old + 1, nil
		}
	}
}
func NewSexprManager(c SexprConfig) (*SexprManager, error) {
	if c.Symbols == nil || c.Rationals == nil {
		return nil, arena.ErrConfig
	}
	if c.PageSize == 0 {
		c.PageSize = 1024
	}
	if c.SlotLimit == 0 {
		c.SlotLimit = math.MaxUint32
	}
	if c.PageSize&(c.PageSize-1) != 0 {
		return nil, arena.ErrConfig
	}
	id, e := nextSexprOwner()
	if e != nil {
		return nil, e
	}
	header := uint64(reflect.TypeOf(SexprManager{}).Size())
	if e = c.Budget.Reserve(header); e != nil {
		return nil, e
	}
	m := &SexprManager{Config: c, owner: id}
	m.self = m
	return m, nil
}
func (m *SexprManager) lock() error {
	if m == nil || m.self != m {
		return ErrSexprOwner
	}
	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return arena.ErrClosed
	}
	return nil
}
func (m *SexprManager) record(h SexprHandle) (*sexprRecord, error) {
	if h.owner != m.owner {
		return nil, ErrSexprOwner
	}
	if h.slot == 0 || h.slot > m.slots {
		return nil, arena.ErrInvalidSlot
	}
	r := &m.Pages[(h.slot-1)/m.Config.PageSize].Nodes[(h.slot-1)%m.Config.PageSize]
	if !r.Live || r.generation != h.generation {
		return nil, arena.ErrInvalidSlot
	}
	return r, nil
}
func (m *SexprManager) handle(slot uint32) SexprHandle {
	r := &m.Pages[(slot-1)/m.Config.PageSize].Nodes[(slot-1)%m.Config.PageSize]
	return SexprHandle{m.owner, slot, r.generation}
}
func sxSize(count uint64, size uintptr) (uint64, error) {
	if count > uint64(^uint(0)>>1) || size != 0 && count > uint64(^uint(0)>>1)/uint64(size) {
		return 0, arena.ErrBounds
	}
	return count * uint64(size), nil
}
func (m *SexprManager) growPage() error {
	count := uint64(len(m.Pages) + 1)
	total := count * uint64(m.Config.PageSize)
	if total > math.MaxUint32 {
		return arena.ErrBounds
	}
	dir, e := sxSize(count, reflect.TypeOf(sexprPage{}).Size())
	if e != nil {
		return e
	}
	records, e := sxSize(uint64(m.Config.PageSize), reflect.TypeOf(sexprRecord{}).Size())
	if e != nil {
		return e
	}
	charge := dir + records
	if e = m.Config.Budget.Reserve(charge); e != nil {
		return e
	}
	pages := make([]sexprPage, int(count))
	copy(pages, m.Pages)
	pages[len(pages)-1].Nodes = make([]sexprRecord, m.Config.PageSize)
	old := m.directoryBytes
	m.Pages = pages
	m.directoryBytes = dir
	m.recordBytes += records
	m.Config.Budget.Release(old)
	return nil
}
func (m *SexprManager) allocate(kind SexprKind, line, pos uint32) (SexprHandle, error) {
	slot := m.free
	if slot == 0 {
		if m.slots == m.Config.SlotLimit {
			return SexprHandle{}, arena.ErrSlotLimit
		}
		slot = m.slots + 1
	}
	if uint64(slot) > uint64(len(m.Pages))*uint64(m.Config.PageSize) {
		if e := m.growPage(); e != nil {
			return SexprHandle{}, e
		}
	}
	r := &m.Pages[(slot-1)/m.Config.PageSize].Nodes[(slot-1)%m.Config.PageSize]
	gen := r.generation
	if slot == m.free {
		m.free = r.next
	} else {
		m.slots = slot
		gen = 1
	}
	*r = sexprRecord{Kind: kind, line: line, pos: pos, generation: gen, Live: true}
	m.live++
	return m.handle(slot), nil
}
func (m *SexprManager) recycle(h SexprHandle) {
	r, _ := m.record(h)
	gen := r.generation
	*r = sexprRecord{generation: gen}
	m.live--
	if gen == math.MaxUint32 {
		r.retired = true
		m.retired++
		return
	}
	r.generation++
	r.next = m.free
	m.free = h.slot
}
func (m *SexprManager) inc(h SexprHandle) error {
	r, e := m.record(h)
	if e != nil {
		return e
	}
	r.refs++
	return nil
}
func (m *SexprManager) dec(h SexprHandle) error {
	_, e := m.decConsumed(h)
	return e
}
func (m *SexprManager) decConsumed(h SexprHandle) (bool, error) {
	r, e := m.record(h)
	if e != nil {
		return false, e
	}
	if r.refs == 0 {
		return false, ErrSexprRefCount
	}
	r.refs--
	if r.refs != 0 {
		return true, nil
	}
	m.deletion = append(m.deletion[:0], h)
	defer func() { m.deletion = m.deletion[:0] }()
	for len(m.deletion) > 0 {
		last := len(m.deletion) - 1
		curr := m.deletion[last]
		m.deletion = m.deletion[:last]
		n, e := m.record(curr)
		if e != nil {
			return true, e
		}
		switch n.Kind {
		case SexprComposite:
			for i := uint32(0); i < n.length; i++ {
				child, e := m.child(n, i)
				if e != nil {
					return true, e
				}
				c, e := m.record(child)
				if e != nil {
					return true, e
				}
				if c.refs == 0 {
					return true, ErrSexprRefCount
				}
				c.refs--
				if c.refs == 0 {
					m.deletion = append(m.deletion, child)
				}
			}
		case SexprNumeral, SexprBVNumeral:
			if e = m.Config.Rationals.Del(&n.Numeral); e != nil {
				return true, e
			}
		}
		m.recycle(curr)
	}
	return true, nil
}
func (m *SexprManager) IncRef(h SexprHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	return m.inc(h)
}
func (m *SexprManager) DecRef(h SexprHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	return m.dec(h)
}
func (m *SexprManager) MkComposite(children []SexprHandle, line, pos uint32) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	if uint64(len(children)) > math.MaxUint32 {
		return SexprHandle{}, arena.ErrBounds
	}
	for _, h := range children {
		if _, e := m.record(h); e != nil {
			return SexprHandle{}, e
		}
	}
	h, e := m.allocate(SexprComposite, line, pos)
	if e != nil {
		return SexprHandle{}, e
	}
	r, _ := m.record(h)
	r.children = append([]SexprHandle(nil), children...)
	r.length = uint32(len(children))
	for _, c := range children {
		m.inc(c)
	}
	return h, nil
}
func (m *SexprManager) mkNumeral(v *unumeral.OwnedRational, width, line, pos uint32, kind SexprKind) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	h, e := m.allocate(kind, line, pos)
	if e != nil {
		return SexprHandle{}, e
	}
	owned, e := m.Config.Rationals.NewValue()
	if e != nil {
		m.recycle(h)
		return SexprHandle{}, e
	}
	if e = m.Config.Rationals.Copy(&owned, v); e != nil {
		m.Config.Rationals.Del(&owned)
		m.recycle(h)
		return SexprHandle{}, e
	}
	r, _ := m.record(h)
	r.Numeral = owned
	r.width = width
	return h, nil
}
func (m *SexprManager) MkNumeral(v *unumeral.OwnedRational, line, pos uint32) (SexprHandle, error) {
	return m.mkNumeral(v, 0, line, pos, SexprNumeral)
}
func (m *SexprManager) MkBVNumeral(v *unumeral.OwnedRational, width, line, pos uint32) (SexprHandle, error) {
	return m.mkNumeral(v, width, line, pos, SexprBVNumeral)
}
func (m *SexprManager) MkString(s string, line, pos uint32) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	if uint64(len(s)) > math.MaxUint32 {
		return SexprHandle{}, arena.ErrBounds
	}
	h, e := m.allocate(SexprString, line, pos)
	if e != nil {
		return SexprHandle{}, e
	}
	r, _ := m.record(h)
	r.text = s
	r.length = uint32(len(s))
	return h, nil
}
func (m *SexprManager) MkCString(s string, line, pos uint32) (SexprHandle, error) {
	return m.MkString(s[:utext.CStringLength(s)], line, pos)
}
func (m *SexprManager) mkSymbol(s usymbol.Symbol, line, pos uint32, kind SexprKind) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	e := m.Config.Symbols.Validate(s)
	if e != nil {
		return SexprHandle{}, e
	}
	h, e := m.allocate(kind, line, pos)
	if e == nil {
		r, _ := m.record(h)
		r.symbol = s
	}
	return h, e
}
func (m *SexprManager) MkSymbol(s usymbol.Symbol, line, pos uint32) (SexprHandle, error) {
	return m.mkSymbol(s, line, pos, SexprSymbol)
}
func (m *SexprManager) MkKeyword(s usymbol.Symbol, line, pos uint32) (SexprHandle, error) {
	return m.mkSymbol(s, line, pos, SexprKeyword)
}
func (m *SexprManager) Kind(h SexprHandle) (SexprKind, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	return r.Kind, nil
}
func (m *SexprManager) RefCount(h SexprHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	return r.refs, nil
}
func (m *SexprManager) Line(h SexprHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	return r.line, nil
}
func (m *SexprManager) Pos(h SexprHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	return r.pos, nil
}
func (m *SexprManager) is(h SexprHandle, k SexprKind) (bool, error) {
	v, e := m.Kind(h)
	return v == k, e
}
func (m *SexprManager) IsComposite(h SexprHandle) (bool, error) { return m.is(h, SexprComposite) }
func (m *SexprManager) IsNumeral(h SexprHandle) (bool, error)   { return m.is(h, SexprNumeral) }
func (m *SexprManager) IsBVNumeral(h SexprHandle) (bool, error) { return m.is(h, SexprBVNumeral) }
func (m *SexprManager) IsString(h SexprHandle) (bool, error)    { return m.is(h, SexprString) }
func (m *SexprManager) IsKeyword(h SexprHandle) (bool, error)   { return m.is(h, SexprKeyword) }
func (m *SexprManager) IsSymbol(h SexprHandle) (bool, error)    { return m.is(h, SexprSymbol) }
func (m *SexprManager) Numeral(h SexprHandle) (unumeral.OwnedRational, error) {
	if e := m.lock(); e != nil {
		return unumeral.OwnedRational{}, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return unumeral.OwnedRational{}, e
	}
	if r.Kind != SexprNumeral && r.Kind != SexprBVNumeral {
		return unumeral.OwnedRational{}, ErrSexprKind
	}
	v, e := m.Config.Rationals.NewValue()
	if e != nil {
		return v, e
	}
	if e = m.Config.Rationals.Copy(&v, &r.Numeral); e != nil {
		m.Config.Rationals.Del(&v)
		return unumeral.OwnedRational{}, e
	}
	return v, nil
}
func (m *SexprManager) BVSize(h SexprHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	if r.Kind != SexprBVNumeral {
		return 0, ErrSexprKind
	}
	return r.width, nil
}
func (m *SexprManager) Symbol(h SexprHandle) (usymbol.Symbol, error) {
	if e := m.lock(); e != nil {
		return usymbol.Symbol{}, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return usymbol.Symbol{}, e
	}
	if r.Kind != SexprSymbol && r.Kind != SexprKeyword {
		return usymbol.Symbol{}, ErrSexprKind
	}
	return r.symbol, nil
}
func (m *SexprManager) String(h SexprHandle) (string, error) {
	if e := m.lock(); e != nil {
		return "", e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return "", e
	}
	if r.Kind != SexprString {
		return "", ErrSexprKind
	}
	return r.text, nil
}
func (m *SexprManager) NumChildren(h SexprHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	if r.Kind != SexprComposite {
		return 0, ErrSexprKind
	}
	return r.length, nil
}
func (m *SexprManager) child(r *sexprRecord, i uint32) (SexprHandle, error) {
	if r.Kind != SexprComposite {
		return SexprHandle{}, ErrSexprKind
	}
	if i >= r.length {
		return SexprHandle{}, arena.ErrBounds
	}
	return r.children[i], nil
}
func (m *SexprManager) Child(h SexprHandle, i uint32) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return SexprHandle{}, e
	}
	return m.child(r, i)
}
func (m *SexprManager) ChildrenInto(h SexprHandle, dst []SexprHandle) (uint32, error) {
	if e := m.lock(); e != nil {
		return 0, e
	}
	defer m.mu.Unlock()
	r, e := m.record(h)
	if e != nil {
		return 0, e
	}
	if r.Kind != SexprComposite {
		return 0, ErrSexprKind
	}
	if uint64(len(dst)) < uint64(r.length) {
		return r.length, arena.ErrBounds
	}
	for i := uint32(0); i < r.length; i++ {
		dst[i], e = m.child(r, i)
		if e != nil {
			return 0, e
		}
	}
	return r.length, nil
}

func (m *SexprManager) put(n *uint32, v byte) error {
	if *n == math.MaxUint32 {
		return arena.ErrBounds
	}
	if uint64(*n) == uint64(len(m.output)) {
		m.output = append(m.output, v)
	} else {
		m.output[*n] = v
	}
	*n++
	return nil
}
func (m *SexprManager) literal(n *uint32, s string) error {
	for i := range s {
		if e := m.put(n, s[i]); e != nil {
			return e
		}
	}
	return nil
}
func (m *SexprManager) formatNumeral(v *unumeral.OwnedRational, n *uint32) error {
	q := m.Config.Rationals
	length, e := q.FormatInto(v, m.stage)
	if errors.Is(e, unumeral.ErrShortBuffer) {
		if uint64(length) > math.MaxUint32 {
			return arena.ErrBounds
		}
		m.stage = make([]byte, length)
		length, e = q.FormatInto(v, m.stage)
	}
	if e != nil {
		return e
	}
	for i := 0; i < length; i++ {
		if e = m.put(n, m.stage[i]); e != nil {
			return e
		}
	}
	return nil
}
func (m *SexprManager) formatSymbol(s usymbol.Symbol, n *uint32) error {
	owner := m.Config.Symbols
	if e := owner.Validate(s); e != nil {
		return e
	}
	if s.IsNumerical() {
		if e := m.literal(n, "k!"); e != nil {
			return e
		}
		value := int64(int32(s.Value))
		if value < 0 {
			if e := m.put(n, '-'); e != nil {
				return e
			}
			value = -value
		}
		var digits [10]byte
		count := 0
		for {
			digits[count] = byte(value%10) + '0'
			count++
			value /= 10
			if value == 0 {
				break
			}
		}
		for count > 0 {
			count--
			if e := m.put(n, digits[count]); e != nil {
				return e
			}
		}
		return nil
	}
	if s.Value == 0 {
		return m.literal(n, "null")
	}
	text, _, e := owner.BareString(s)
	if e != nil {
		return e
	}
	for i := 0; i < len(text); i++ {
		if e = m.put(n, text[i]); e != nil {
			return e
		}
	}
	return nil
}
func (m *SexprManager) scalar(n int64) (unumeral.OwnedRational, error) {
	q := m.Config.Rationals
	v, e := q.NewValue()
	if e != nil {
		return v, e
	}
	if e = q.SetInt64(&v, n); e != nil {
		q.Del(&v)
		return unumeral.OwnedRational{}, e
	}
	return v, nil
}
func (m *SexprManager) bvDigit(val *unumeral.OwnedRational, hex bool) (byte, unumeral.OwnedRational, error) {
	q := m.Config.Rationals
	base := int64(2)
	if hex {
		base = 16
	}
	divisor, e := m.scalar(base)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	c, e := q.Rem(val, &divisor)
	q.Del(&divisor)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	keep := false
	defer func() {
		if !keep {
			q.Del(&c)
		}
	}()
	divisor, e = m.scalar(base)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	next, e := q.IDiv(val, &divisor)
	if e == nil {
		e = q.Copy(val, &next)
		q.Del(&next)
	}
	q.Del(&divisor)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	if e = m.sexprBVDiagnostic(&c, base); e != nil {
		return 0, unumeral.OwnedRational{}, e
	}

	if !hex {
		zero, e := q.IsZero(&c)
		if e != nil {
			return 0, unumeral.OwnedRational{}, e
		}
		if zero {
			keep = true
			return '0', c, nil
		}
		keep = true
		return '1', c, nil
	}
	nine, e := m.scalar(9)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	small, e := q.LessEqual(&c, &nine)
	q.Del(&nine)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	digit, e := q.Unsigned(&c)
	if e != nil {
		return 0, unumeral.OwnedRational{}, e
	}
	if small {
		keep = true
		return '0' + byte(digit), c, nil
	}
	keep = true
	return 'a' + byte(digit-10), c, nil
}

// Source: src/util/sexpr.cpp:142-194.
func (m *SexprManager) displayBV(r *sexprRecord, n *uint32) error {
	if e := m.put(n, '#'); e != nil {
		return e
	}
	q := m.Config.Rationals
	val, e := q.NewValue()
	if e != nil {
		return e
	}
	defer q.Del(&val)
	if e = q.Copy(&val, &r.Numeral); e != nil {
		return e
	}
	m.bvDigits = m.bvDigits[:0]
	defer func() { m.bvDigits = m.bvDigits[:0] }()
	hex := r.width%4 == 0
	prefix := byte('b')
	step := uint32(1)
	if hex {
		prefix = 'x'
		step = 4
	}
	if e = m.put(n, prefix); e != nil {
		return e
	}
	var size uint32
	for {
		positive, e := q.IsPos(&val)
		if e != nil {
			return e
		}
		if !positive {
			break
		}
		digit, c, e := m.bvDigit(&val, hex)
		if e != nil {
			return e
		}
		m.bvDigits = append(m.bvDigits, digit)
		q.Del(&c)
		size += step
	}
	for size < r.width {
		m.bvDigits = append(m.bvDigits, '0')
		size += step
	}
	for i := len(m.bvDigits); i > 0; i-- {
		if e = m.put(n, m.bvDigits[i-1]); e != nil {
			return e
		}
	}
	return nil
}

func (m *SexprManager) displayAtom(h SexprHandle, n *uint32) error {
	r, e := m.record(h)
	if e != nil {
		return e
	}
	switch r.Kind {
	case SexprNumeral:
		return m.formatNumeral(&r.Numeral, n)
	case SexprBVNumeral:
		return m.displayBV(r, n)
	case SexprSymbol, SexprKeyword:
		return m.formatSymbol(r.symbol, n)
	case SexprString:
		if e = m.put(n, '"'); e != nil {
			return e
		}
		for i := uint32(0); i < r.length; i++ {
			c := r.text[i]
			if c == 0 {
				break
			}
			if c == '"' {
				if e = m.put(n, '\\'); e != nil {
					return e
				}
			}
			if e = m.put(n, c); e != nil {
				return e
			}
		}
		return m.put(n, '"')
	default:
		return ErrSexprKind
	}
}
func (m *SexprManager) DisplayAtom(h SexprHandle) (string, error) {
	if e := m.lock(); e != nil {
		return "", e
	}
	defer m.mu.Unlock()
	var n uint32
	e := m.displayAtom(h, &n)
	return string(m.output[:n]), e
}

// Source display has no return after display_atom and then performs an invalid
// derived-class cast. Expose its defined prefix, then stop with a typed error.
func (m *SexprManager) Display(h SexprHandle) (string, error) {
	if e := m.lock(); e != nil {
		return "", e
	}
	defer m.mu.Unlock()
	var n uint32
	r, e := m.record(h)
	if e != nil {
		return "", e
	}
	if r.Kind != SexprComposite {
		e = m.displayAtom(h, &n)
		if e == nil {
			e = ErrSexprDisplayContinuation
		}
		return string(m.output[:n]), e
	}
	if len(m.frames) == 0 {
		m.frames = append(m.frames, sexprFrame{h: h})
	} else {
		m.frames[0] = sexprFrame{h: h}
	}
	size := uint32(1)
	for size > 0 {
		frame := &m.frames[size-1]
		node, _ := m.record(frame.h)
		if frame.index == node.length {
			if e = m.put(&n, ')'); e != nil {
				break
			}
			size--
			continue
		}
		separator := byte(' ')
		if frame.index == 0 {
			separator = '('
		}
		if e = m.put(&n, separator); e != nil {
			break
		}
		child, err := m.child(node, frame.index)
		if err != nil {
			e = err
			break
		}
		frame.index++
		childNode, err := m.record(child)
		if err != nil {
			e = err
			break
		}
		if childNode.Kind == SexprComposite {
			if size == math.MaxUint32 {
				e = arena.ErrBounds
				break
			}
			if uint64(size) == uint64(len(m.frames)) {
				m.frames = append(m.frames, sexprFrame{h: child})
			} else {
				m.frames[size] = sexprFrame{h: child}
			}
			size++
		} else {
			if e = m.displayAtom(child, &n); e != nil {
				break
			}
		}
	}
	return string(m.output[:n]), e
}
func (m *SexprManager) Stats() SexprStats {
	if m == nil {
		return SexprStats{}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	s := SexprStats{Live: m.live, Slots: m.slots, Retired: m.retired, Pages: uint32(len(m.Pages)), RecordBytes: m.recordBytes, DirectoryBytes: m.directoryBytes}
	if !m.closed {
		s.OwnerBytes = uint64(reflect.TypeOf(SexprManager{}).Size())
	}
	return s
}

// Source allocator teardown does not destroy outstanding logical records.
// Explicit Go owner teardown drops each still-live owned numeral exactly once.
func (m *SexprManager) Close() error {
	if m == nil {
		return nil
	}
	if m.self != m {
		return ErrSexprOwner
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	for i := range m.Pages {
		for j := range m.Pages[i].Nodes {
			r := &m.Pages[i].Nodes[j]
			if r.Live && (r.Kind == SexprNumeral || r.Kind == SexprBVNumeral) {
				if e := m.Config.Rationals.Del(&r.Numeral); e != nil {
					return e
				}
			}
		}
	}
	m.Config.Budget.Release(uint64(reflect.TypeOf(SexprManager{}).Size()) + m.directoryBytes + m.recordBytes)
	m.Pages = nil
	m.deletion = nil
	m.frames = nil
	m.output = nil
	m.stage = nil
	m.bvDigits = nil
	m.recordBytes = 0
	m.directoryBytes = 0
	m.live = 0
	m.closed = true
	return nil
}

// SexprRef is a scalar owning reference; copying its Go value does not acquire
// ownership. Use CloneRef or MoveRef. Null callbacks are source-defined here.
type SexprRef struct {
	owner   uint64
	h       SexprHandle
	owned   bool
	pending []SexprHandle
}

func (r SexprRef) IsNull() bool { return r.h.IsNull() }
func (m *SexprManager) checkRef(r *SexprRef) error {
	if r == nil || r.owner != m.owner {
		return ErrSexprOwner
	}
	if !r.h.IsNull() && r.owned {
		_, e := m.record(r.h)
		return e
	}
	return nil
}
func (m *SexprManager) NewRef(h SexprHandle) (SexprRef, error) {
	if e := m.lock(); e != nil {
		return SexprRef{}, e
	}
	defer m.mu.Unlock()
	if !h.IsNull() {
		if e := m.inc(h); e != nil {
			return SexprRef{}, e
		}
	}
	return SexprRef{owner: m.owner, h: h, owned: !h.IsNull()}, nil
}
func (m *SexprManager) CloneRef(src *SexprRef) (SexprRef, error) {
	if e := m.lock(); e != nil {
		return SexprRef{}, e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(src); e != nil {
		return SexprRef{}, e
	}
	if !src.h.IsNull() {
		if e := m.inc(src.h); e != nil {
			return SexprRef{}, e
		}
	}
	return SexprRef{owner: m.owner, h: src.h, owned: !src.h.IsNull()}, nil
}
func (m *SexprManager) MoveRef(src *SexprRef) (SexprRef, error) {
	if e := m.lock(); e != nil {
		return SexprRef{}, e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(src); e != nil {
		return SexprRef{}, e
	}
	out := *src
	src.h = SexprHandle{}
	src.owned = false
	src.pending = nil
	return out, nil
}
func (m *SexprManager) GetRef(r *SexprRef) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(r); e != nil {
		return SexprHandle{}, e
	}
	return r.h, nil
}
func (m *SexprManager) AssignRef(r *SexprRef, h SexprHandle) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(r); e != nil {
		return e
	}
	if !h.IsNull() {
		if e := m.inc(h); e != nil {
			return e
		}
	}
	if !r.h.IsNull() {
		if !r.owned {
			if !h.IsNull() {
				r.pending = append(r.pending, h)
			}
			return arena.ErrInvalidSlot
		}
		consumed, e := m.decConsumed(r.h)
		if e != nil || !consumed {
			if !h.IsNull() {
				r.pending = append(r.pending, h)
			}
			if consumed {
				r.owned = false
			}
			if e == nil {
				return ErrSexprOwner
			}
			return e
		}
	}
	r.h = h
	r.owned = !h.IsNull()
	return nil
}
func (m *SexprManager) AssignRefFrom(r, src *SexprRef) error {
	h, e := m.GetRef(src)
	if e != nil {
		return e
	}
	return m.AssignRef(r, h)
}
func (m *SexprManager) ResetRef(r *SexprRef) error { return m.AssignRef(r, SexprHandle{}) }
func (m *SexprManager) CloseRef(r *SexprRef) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(r); e != nil {
		return e
	}
	var failure error
	if r.owned {
		consumed, e := m.decConsumed(r.h)
		if consumed {
			r.owned = false
		}
		if !consumed {
			if e != nil {
				return e
			}
			return ErrSexprOwner
		}
		failure = e
	}
	for len(r.pending) != 0 {
		last := len(r.pending) - 1
		consumed, e := m.decConsumed(r.pending[last])
		if consumed {
			r.pending = r.pending[:last]
		}
		if failure == nil {
			failure = e
		}
		if !consumed {
			if e != nil {
				return e
			}
			return ErrSexprOwner
		}
	}
	r.h = SexprHandle{}
	return failure
}
func (m *SexprManager) SwapRef(a, b *SexprRef) error {
	if e := m.lock(); e != nil {
		return e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(a); e != nil {
		return e
	}
	if e := m.checkRef(b); e != nil {
		return e
	}
	a.h, b.h = b.h, a.h
	a.owned, b.owned = b.owned, a.owned
	a.pending, b.pending = b.pending, a.pending
	return nil
}
func (m *SexprManager) StealRef(r *SexprRef) (SexprHandle, error) {
	if e := m.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(r); e != nil {
		return SexprHandle{}, e
	}
	h := r.h
	r.h = SexprHandle{}
	r.owned = false
	return h, nil
}
func (m *SexprManager) EqualRef(a, b *SexprRef) (bool, error) {
	if e := m.lock(); e != nil {
		return false, e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(a); e != nil {
		return false, e
	}
	if e := m.checkRef(b); e != nil {
		return false, e
	}
	return a.h == b.h, nil
}

// SexprRefVector adapts undefined source null callback dereferences to no-ops,
// allowing reserve/resize null placeholders followed by Set. Manager raw null
// callbacks remain typed errors. Owned handles use a native typed slice; its
// Go storage is outside arena accounting.
type SexprRefVector struct {
	id      uint64
	manager *SexprManager
	self    *SexprRefVector
	nodes   []SexprHandle
	owned   []bool
	pending []SexprHandle
	epoch   uint64
	closed  bool
}
type SexprVectorElement struct {
	vectorID uint64
	owner    uint64
	epoch    uint64
	index    uint32
}

func (m *SexprManager) NewRefVector() (*SexprRefVector, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	if m.nextVector == math.MaxUint64 {
		return nil, arena.ErrBounds
	}
	m.nextVector++
	id := m.nextVector
	v := &SexprRefVector{id: id, manager: m, epoch: 1}
	v.self = v
	return v, nil
}
func (v *SexprRefVector) lock() error {
	if v == nil || v.self != v {
		return ErrSexprOwner
	}
	if e := v.manager.lock(); e != nil {
		return e
	}
	if v.closed {
		v.manager.mu.Unlock()
		return arena.ErrClosed
	}
	return nil
}
func (v *SexprRefVector) canBump() error {
	if v.epoch == math.MaxUint64 {
		return arena.ErrBounds
	}
	return nil
}
func (v *SexprRefVector) Size() uint32              { return uint32(len(v.nodes)) }
func (v *SexprRefVector) Capacity() uint32          { return uint32(cap(v.nodes)) }
func (v *SexprRefVector) Empty() bool               { return v.Size() == 0 }
func (v *SexprRefVector) GetManager() *SexprManager { return v.manager }
func (v *SexprRefVector) Get(i uint32) (SexprHandle, error) {
	if e := v.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer v.manager.mu.Unlock()
	if i >= v.Size() {
		return SexprHandle{}, arena.ErrBounds
	}
	return v.nodes[i], nil
}
func (v *SexprRefVector) GetDefault(i uint32, d SexprHandle) (SexprHandle, error) {
	if e := v.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer v.manager.mu.Unlock()
	if i >= v.Size() {
		return d, nil
	}
	return v.nodes[i], nil
}
func (v *SexprRefVector) Back() (SexprHandle, error) {
	if v.Size() == 0 {
		return SexprHandle{}, arena.ErrBounds
	}
	return v.Get(v.Size() - 1)
}
func (v *SexprRefVector) inc(h SexprHandle) error {
	if h.IsNull() {
		return nil
	}
	return v.manager.inc(h)
}
func (v *SexprRefVector) decConsumed(h SexprHandle) (bool, error) {
	if h.IsNull() {
		return true, nil
	}
	return v.manager.decConsumed(h)
}
func (v *SexprRefVector) push(h SexprHandle) error {
	if v.Size() == math.MaxUint32 {
		return arena.ErrBounds
	}
	if e := v.canBump(); e != nil {
		return e
	}
	if !h.IsNull() {
		if _, e := v.manager.record(h); e != nil {
			return e
		}
	}
	if e := v.inc(h); e != nil {
		return e
	}
	v.nodes = append(v.nodes, h)
	v.owned = append(v.owned, !h.IsNull())
	v.epoch++
	return nil
}
func (v *SexprRefVector) PushBack(h SexprHandle) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	return v.push(h)
}
func (v *SexprRefVector) shrink(size uint32) error {
	if size > v.Size() {
		return arena.ErrBounds
	}
	if size == v.Size() {
		return nil
	}
	if e := v.canBump(); e != nil {
		return e
	}
	for i := size; i < v.Size(); i++ {
		if !v.nodes[i].IsNull() && !v.owned[i] {
			return arena.ErrInvalidSlot
		}
		consumed, e := v.decConsumed(v.nodes[i])
		if consumed {
			v.owned[i] = false
		}
		if e != nil {
			return e
		}
	}
	v.nodes = v.nodes[:size]
	v.owned = v.owned[:size]
	v.epoch++
	return nil
}
func (v *SexprRefVector) Shrink(size uint32) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	return v.shrink(size)
}
func (v *SexprRefVector) Resize(size uint32) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if size <= v.Size() {
		return v.shrink(size)
	}
	if e := v.canBump(); e != nil {
		return e
	}
	v.nodes = append(v.nodes, make([]SexprHandle, int(size)-len(v.nodes))...)
	v.owned = append(v.owned, make([]bool, int(size)-len(v.owned))...)
	v.epoch++
	return nil
}
func (v *SexprRefVector) ResizeFill(size uint32, h SexprHandle) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if size <= v.Size() {
		return v.shrink(size)
	}
	for v.Size() < size {
		if e := v.push(h); e != nil {
			return e
		}
	}
	return nil
}

// Source reserve changes size, rather than reserving capacity only.
func (v *SexprRefVector) Reserve(size uint32) error {
	if size <= v.Size() {
		return nil
	}
	return v.Resize(size)
}
func (v *SexprRefVector) Reset() error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	return v.shrink(0)
}
func (v *SexprRefVector) Finalize() error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if cap(v.nodes) == 0 {
		return nil
	}
	if e := v.canBump(); e != nil {
		return e
	}
	for i := uint32(0); i < v.Size(); i++ {
		if !v.nodes[i].IsNull() && !v.owned[i] {
			return arena.ErrInvalidSlot
		}
		consumed, e := v.decConsumed(v.nodes[i])
		if consumed {
			v.owned[i] = false
		}
		if e != nil {
			return e
		}
	}
	v.nodes = nil
	v.owned = nil
	v.epoch++
	return nil
}

func (v *SexprRefVector) PopBack() error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if v.Size() == 0 {
		return arena.ErrBounds
	}
	if e := v.canBump(); e != nil {
		return e
	}
	last := len(v.nodes) - 1
	h := v.nodes[last]
	owned := v.owned[last]
	v.nodes[last] = SexprHandle{}
	v.nodes = v.nodes[:last]
	v.owned = v.owned[:last]
	v.epoch++
	if !h.IsNull() && !owned {
		return arena.ErrInvalidSlot
	}
	consumed, e := v.decConsumed(h)
	if !consumed && !h.IsNull() {
		v.pending = append(v.pending, h)
	}
	return e
}
func (v *SexprRefVector) set(i uint32, h SexprHandle) error {
	if i >= v.Size() {
		return arena.ErrBounds
	}
	if e := v.inc(h); e != nil {
		return e
	}
	if !v.nodes[i].IsNull() && !v.owned[i] {
		if !h.IsNull() {
			v.pending = append(v.pending, h)
		}
		return arena.ErrInvalidSlot
	}
	consumed, e := v.decConsumed(v.nodes[i])
	if consumed {
		v.owned[i] = false
	}
	if e != nil {
		if !h.IsNull() {
			v.pending = append(v.pending, h)
		}
		return e
	}
	v.nodes[i] = h
	v.owned[i] = !h.IsNull()
	return nil
}
func (v *SexprRefVector) Set(i uint32, h SexprHandle) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	return v.set(i, h)
}
func (v *SexprRefVector) erase(i uint32) error {
	if i >= v.Size() {
		return arena.ErrBounds
	}
	if e := v.canBump(); e != nil {
		return e
	}
	h := v.nodes[i]
	owned := v.owned[i]
	copy(v.nodes[i:], v.nodes[i+1:])
	copy(v.owned[i:], v.owned[i+1:])
	last := len(v.nodes) - 1
	v.nodes[last] = SexprHandle{}
	v.nodes = v.nodes[:last]
	v.owned = v.owned[:last]
	v.epoch++
	if !h.IsNull() && !owned {
		return arena.ErrInvalidSlot
	}
	consumed, e := v.decConsumed(h)
	if !consumed && !h.IsNull() {
		v.pending = append(v.pending, h)
	}
	return e
}
func (v *SexprRefVector) EraseIndex(i uint32) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	return v.erase(i)
}
func (v *SexprRefVector) EraseHandle(h SexprHandle) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	for i := uint32(0); i < v.Size(); i++ {
		if v.nodes[i] == h {
			return v.erase(i)
		}
	}
	return nil
}
func (v *SexprRefVector) Contains(h SexprHandle) (bool, error) {
	if e := v.lock(); e != nil {
		return false, e
	}
	defer v.manager.mu.Unlock()
	for i := uint32(0); i < v.Size(); i++ {
		if v.nodes[i] == h {
			return true, nil
		}
	}
	return false, nil
}
func (v *SexprRefVector) Append(data []SexprHandle) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	for _, h := range data {
		if e := v.push(h); e != nil {
			return e
		}
	}
	return nil
}
func (v *SexprRefVector) AppendVector(other *SexprRefVector) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if other == nil || other.self != other || other.manager != v.manager {
		return ErrSexprOwner
	}
	if other.closed {
		return arena.ErrClosed
	}
	if other == v && v.Size() > 0 {
		return arena.ErrBounds
	}
	for i := uint32(0); i < other.Size(); i++ {
		if !other.nodes[i].IsNull() && !other.owned[i] {
			return arena.ErrInvalidSlot
		}
		if e := v.push(other.nodes[i]); e != nil {
			return e
		}
	}
	return nil
}
func (v *SexprRefVector) SwapIndices(a, b uint32) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if a >= v.Size() || b >= v.Size() {
		return arena.ErrBounds
	}
	v.nodes[a], v.nodes[b] = v.nodes[b], v.nodes[a]
	v.owned[a], v.owned[b] = v.owned[b], v.owned[a]
	return nil
}
func (v *SexprRefVector) Reverse() error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	for i := uint32(0); i < v.Size()/2; i++ {
		j := v.Size() - i - 1
		v.nodes[i], v.nodes[j] = v.nodes[j], v.nodes[i]
		v.owned[i], v.owned[j] = v.owned[j], v.owned[i]
	}
	return nil
}
func (v *SexprRefVector) Clone() (*SexprRefVector, error) {
	out, e := v.manager.NewRefVector()
	if e != nil {
		return nil, e
	}
	if e = out.AppendVector(v); e != nil {
		out.Close()
		return nil, e
	}
	return out, nil
}
func (v *SexprRefVector) Move() (*SexprRefVector, error) {
	out, e := v.manager.NewRefVector()
	if e != nil {
		return nil, e
	}
	if e = out.Swap(v); e != nil {
		out.Close()
		return nil, e
	}
	return out, nil
}
func (v *SexprRefVector) Assign(other *SexprRefVector) error {
	if v == nil || v.self != v {
		return ErrSexprOwner
	}
	if v.closed {
		return arena.ErrClosed
	}
	if other == v {
		return nil
	}
	if other == nil || other.self != other || other.manager != v.manager || other.closed {
		return ErrSexprOwner
	}
	if e := v.Reset(); e != nil {
		return e
	}
	return v.AppendVector(other)
}
func (v *SexprRefVector) Swap(other *SexprRefVector) error {
	if e := v.lock(); e != nil {
		return e
	}
	defer v.manager.mu.Unlock()
	if other == nil || other.self != other || other.manager != v.manager {
		return ErrSexprOwner
	}
	if other.closed {
		return arena.ErrClosed
	}
	if v == other {
		return nil
	}
	if e := v.canBump(); e != nil {
		return e
	}
	if e := other.canBump(); e != nil {
		return e
	}
	v.nodes, other.nodes = other.nodes, v.nodes
	v.owned, other.owned = other.owned, v.owned
	v.pending, other.pending = other.pending, v.pending
	v.epoch++
	other.epoch++
	return nil
}
func (v *SexprRefVector) CopyInto(dst []SexprHandle) (uint32, error) {
	if e := v.lock(); e != nil {
		return 0, e
	}
	defer v.manager.mu.Unlock()
	if uint64(len(dst)) < uint64(v.Size()) {
		return v.Size(), arena.ErrBounds
	}
	copy(dst, v.nodes[:v.Size()])
	return v.Size(), nil
}
func (v *SexprRefVector) Element(i uint32) (SexprVectorElement, error) {
	if e := v.lock(); e != nil {
		return SexprVectorElement{}, e
	}
	defer v.manager.mu.Unlock()
	if i >= v.Size() {
		return SexprVectorElement{}, arena.ErrBounds
	}
	return SexprVectorElement{vectorID: v.id, owner: v.manager.owner, epoch: v.epoch, index: i}, nil
}
func (v *SexprRefVector) GetElement(e SexprVectorElement) (SexprHandle, error) {
	if err := v.lock(); err != nil {
		return SexprHandle{}, err
	}
	defer v.manager.mu.Unlock()
	if e.owner != v.manager.owner || e.vectorID != v.id || e.epoch != v.epoch || e.index >= v.Size() {
		return SexprHandle{}, arena.ErrInvalidSlot
	}
	return v.nodes[e.index], nil
}
func (v *SexprRefVector) SetElement(e SexprVectorElement, h SexprHandle) error {
	if err := v.lock(); err != nil {
		return err
	}
	defer v.manager.mu.Unlock()
	if e.owner != v.manager.owner || e.vectorID != v.id || e.epoch != v.epoch || e.index >= v.Size() {
		return arena.ErrInvalidSlot
	}
	return v.set(e.index, h)
}
func (v *SexprRefVector) AssignElement(e SexprVectorElement, other *SexprRefVector, src SexprVectorElement) error {
	h, err := other.GetElement(src)
	if err != nil {
		return err
	}
	return v.SetElement(e, h)
}
func (v *SexprRefVector) EqualElement(e SexprVectorElement, h SexprHandle) (bool, error) {
	got, err := v.GetElement(e)
	return got == h, err
}
func (v *SexprRefVector) Close() error {
	if v == nil {
		return nil
	}
	if v.self != v {
		return ErrSexprOwner
	}
	m := v.manager
	m.mu.Lock()
	defer m.mu.Unlock()
	if v.closed {
		return nil
	}
	if !m.closed {
		var first error
		for i := uint32(0); i < v.Size(); i++ {
			if !v.owned[i] {
				continue
			}
			consumed, e := v.decConsumed(v.nodes[i])
			if consumed {
				v.owned[i] = false
			}
			if e != nil && first == nil {
				first = e
			}
		}
		for i, h := range v.pending {
			if h.IsNull() {
				continue
			}
			consumed, e := v.decConsumed(h)
			if consumed {
				v.pending[i] = SexprHandle{}
			}
			if e != nil && first == nil {
				first = e
			}
		}
		for _, owned := range v.owned {
			if owned {
				return first
			}
		}
		for _, h := range v.pending {
			if !h.IsNull() {
				return first
			}
		}
		v.nodes, v.owned, v.pending = nil, nil, nil
		v.closed = true
		return first
	}
	v.nodes = nil
	v.owned = nil
	v.pending = nil
	v.closed = true
	return nil
}

func (m *SexprManager) RefManager(r *SexprRef) (*SexprManager, error) {
	if e := m.lock(); e != nil {
		return nil, e
	}
	defer m.mu.Unlock()
	if e := m.checkRef(r); e != nil {
		return nil, e
	}
	return m, nil
}
func (m *SexprManager) NewRefVectorFrom(data []SexprHandle) (*SexprRefVector, error) {
	v, e := m.NewRefVector()
	if e != nil {
		return nil, e
	}
	if e = v.Append(data); e != nil {
		v.Close()
		return nil, e
	}
	return v, nil
}

// Iterators are scalar non-owning borrows, invalidated by structural mutation.
// They replace source pointer arithmetic without retaining record pointers.
type SexprVectorIterator struct {
	owner, vectorID, epoch uint64
	index                  uint32
}

func (v *SexprRefVector) Begin() (SexprVectorIterator, error) {
	if e := v.lock(); e != nil {
		return SexprVectorIterator{}, e
	}
	defer v.manager.mu.Unlock()
	return SexprVectorIterator{owner: v.manager.owner, vectorID: v.id, epoch: v.epoch}, nil
}
func (v *SexprRefVector) End() (SexprVectorIterator, error) {
	i, e := v.Begin()
	i.index = v.Size()
	return i, e
}
func (v *SexprRefVector) iterator(i SexprVectorIterator) error {
	if i.owner != v.manager.owner || i.vectorID != v.id || i.epoch != v.epoch {
		return arena.ErrInvalidSlot
	}
	if i.index > v.Size() {
		return arena.ErrBounds
	}
	return nil
}
func (v *SexprRefVector) IteratorGet(i SexprVectorIterator) (SexprHandle, error) {
	if e := v.lock(); e != nil {
		return SexprHandle{}, e
	}
	defer v.manager.mu.Unlock()
	if e := v.iterator(i); e != nil {
		return SexprHandle{}, e
	}
	if i.index == v.Size() {
		return SexprHandle{}, arena.ErrBounds
	}
	return v.nodes[i.index], nil
}
func (v *SexprRefVector) IteratorNext(i SexprVectorIterator) (SexprVectorIterator, error) {
	if e := v.lock(); e != nil {
		return SexprVectorIterator{}, e
	}
	defer v.manager.mu.Unlock()
	if e := v.iterator(i); e != nil {
		return SexprVectorIterator{}, e
	}
	if i.index == v.Size() {
		return SexprVectorIterator{}, arena.ErrBounds
	}
	i.index++
	return i, nil
}
