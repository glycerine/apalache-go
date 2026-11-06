package symbol

// Source: src/util/symbol.h; src/util/symbol.cpp.

import (
	"errors"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

var ErrInvalid = errors.New("g3: invalid source symbol precondition")
var ErrOwner = errors.New("g3: symbol belongs to another owner")

// Symbol is a pointer-free source symbol identity. Tag0 is a string or null,
// tag1 a uint32 numerical value, and tags2/3 the source marked forms. Physical
// IDs replace addresses; owner and IDs are never recycled. Generation remains
// in the encoded handle for compatibility and is validated on access.
type Symbol struct {
	owner      uint64
	Value      uint32
	Generation arena.Generation
	Tag        uint32
}

func Numerical(value uint32) Symbol { return Symbol{Value: value, Tag: 1} }
func Dummy() Symbol                 { return Symbol{Tag: 2} }
func (s Symbol) IsNumerical() bool  { return s.Tag == 1 }
func (s Symbol) IsMarked() bool     { return s.Tag > 1 }
func (s Symbol) Number() (uint32, error) {
	if !s.IsNumerical() {
		return 0, ErrInvalid
	}
	return s.Value, nil
}

// Marking is private in the source and is used by the scoped symbol table.
func (s Symbol) Mark() (Symbol, error) {
	if s.IsMarked() {
		return Symbol{}, ErrInvalid
	}
	s.Tag += 2
	return s, nil
}
func (s Symbol) unmark() (Symbol, error) {
	if !s.IsMarked() || s.Tag > 3 {
		return Symbol{}, ErrInvalid
	}
	s.Tag -= 2
	return s, nil
}

// Token replaces the source raw pointer conversion; it is opaque and
// comparable, and its string identity can only be restored by its live owner.
type Token struct{ symbol Symbol }

func (s Symbol) Token() Token { return Token{symbol: s} }

type Config struct {
	Budget      *arena.Budget
	onAllocated func(*Manager)
	// Debug is borrowed for reached source assertions; the caller keeps it alive.
	Debug *diagnostic.DebugContext
	// Legacy arena geometry is ignored: spellings use Go-managed storage.
	PageSize, Alignment, DescriptorPageSize uint32
	DirectOnly                              bool
}
type Stats struct {
	OwnerBytes uint64
	Symbols    uint32
	Region     arena.BytesRegionStats
	Table      InternStats
}

// InternStats retains the existing accounting view while interning is
// implemented by Go maps. Map storage is managed by the Go runtime.
type InternStats struct {
	OwnerBytes, TableBytes, DirectoryBytes, RetainedBytes uint64
	Size, Deleted, Capacity                               uint32
}

// Manager owns canonical immutable Go spellings. IDs are never recycled
// within an owner; the mutex serializes interning and owner teardown.
type Manager struct {
	mu      sync.Mutex
	byText  map[string]uint32
	entries []symbolEntry
	Budget  *arena.Budget
	debug   *diagnostic.DebugContext
	owner   uint64
	closed  bool
}
type symbolEntry struct {
	text string
	hash uint32
}

var symbolOwnerSequence atomic.Uint64

func NewManager(c Config) (*Manager, error) {
	header := uint64(reflect.TypeOf(Manager{}).Size())
	if e := c.Budget.Reserve(header); e != nil {
		return nil, e
	}
	// Saturate instead of recycling identities on counter exhaustion.
	var owner uint64
	for {
		previous := symbolOwnerSequence.Load()
		if previous == math.MaxUint64 {
			c.Budget.Release(header)
			return nil, arena.ErrBounds
		}
		if symbolOwnerSequence.CompareAndSwap(previous, previous+1) {
			owner = previous + 1
			break
		}
	}
	m := &Manager{Budget: c.Budget, debug: c.Debug, owner: owner}
	if c.onAllocated != nil {
		c.onAllocated(m) // source alloc(internal_symbol_table) precedes member construction
	}
	m.byText = make(map[string]uint32)
	return m, nil
}

// BindDebug borrows the generated process assertion owner after the symbol
// table is constructed. The source SASSERT reads that process switch when
// bare_str is called, so later changes to the switch remain visible.
func (m *Manager) BindDebug(d *diagnostic.DebugContext) error {
	if m == nil || d == nil {
		return arena.ErrConfig
	}
	if _, e := d.AssertionsEnabled(); e != nil {
		return e
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return arena.ErrClosed
	}
	m.debug = d
	return nil
}

// The source SASSERT is reached before bare_str's numerical-empty-string
// continuation. A nil binding keeps the existing explicit Go refusal.
func (m *Manager) bareStringNumericalAssertion() error {
	on, e := diagnostic.SourceAssertionsEnabled(m.debug)
	if e != nil || !on {
		return e
	}
	if m.debug == nil || !m.debug.HasAssertionActions() {
		return ErrInvalid
	}
	return diagnostic.DebugAssertionFailure(m.debug, "src/util/symbol.h", 96, "!is_numerical()")
}
func cSymbolLength(text string) int {
	if i := strings.IndexByte(text, 0); i >= 0 {
		return i
	}
	return len(text)
}
func (m *Manager) check(s Symbol) error {
	if m.closed {
		return arena.ErrClosed
	}
	if s.Tag > 3 {
		return ErrInvalid
	}
	if s.Tag == 1 || s.Tag == 3 {
		if s.owner != 0 || s.Generation != 0 {
			return ErrInvalid
		}
		return nil
	}
	if s.Value == 0 {
		if s.owner != 0 || s.Generation != 0 {
			return ErrInvalid
		}
		return nil
	}
	if s.owner != m.owner {
		return ErrOwner
	}
	if s.Generation != 1 || uint64(s.Value) > uint64(len(m.entries)) {
		return arena.ErrInvalidSlot
	}
	return nil
}
func (m *Manager) checkPlain(s Symbol) error {
	if e := m.check(s); e != nil {
		return e
	}
	if s.IsMarked() {
		return ErrInvalid
	}
	return nil
}
func (m *Manager) symbol(id uint32) Symbol {
	return Symbol{owner: m.owner, Value: id, Generation: 1}
}

// Intern canonicalizes the C-string spelling using a Go map. The typed entry
// retains the source hash without reproducing its raw size_t prefix or NUL.
func (m *Manager) Intern(text string) (Symbol, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.intern(text)
}

func (m *Manager) intern(text string) (Symbol, error) {
	if m.closed {
		return Symbol{}, arena.ErrClosed
	}
	length := cSymbolLength(text)
	text = text[:length]
	if r, found := m.byText[text]; found {
		return m.symbol(r), nil
	}
	if uint64(length) > math.MaxUint32 || uint64(len(m.entries)) == math.MaxUint32 {
		return Symbol{}, arena.ErrBounds
	}
	// Own only the spelling, rather than retaining a possibly large input suffix.
	text = strings.Clone(text)
	id := uint32(len(m.entries)) + 1
	m.entries = append(m.entries, symbolEntry{text: text, hash: spellingHash(text, 17)})
	m.byText[text] = id
	return m.symbol(id), nil
}

// A null constructor needs no interned spelling, as in the source.
func (m *Manager) InternNullable(text *string) (Symbol, error) {
	if text == nil {
		return Symbol{}, nil
	}
	return m.Intern(*text)
}
func (m *Manager) AssignString(out *Symbol, text *string) error {
	if out == nil || text == nil {
		return ErrInvalid
	}
	s, e := m.Intern(*text)
	if e != nil {
		return e
	}
	*out = s
	return nil
}
func (m *Manager) FromToken(t Token) (Symbol, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.check(t.symbol); e != nil {
		return Symbol{}, e
	}
	return t.symbol, nil
}
func (m *Manager) spelling(s Symbol) (string, error) {
	if s.Value == 0 || uint64(s.Value) > uint64(len(m.entries)) || s.Generation != 1 {
		return "", arena.ErrInvalidSlot
	}
	return m.entries[s.Value-1].text, nil
}
func (m *Manager) text(s Symbol, display bool) (string, error) {
	if e := m.checkPlain(s); e != nil {
		return "", e
	}
	if s.IsNumerical() {
		return "k!" + strconv.FormatInt(int64(int32(s.Value)), 10), nil
	}
	if s.Value == 0 {
		if display {
			return "null", nil
		}
		return "<null>", nil
	}
	return m.spelling(s)
}
func (m *Manager) String(s Symbol) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return "", e
	}
	if e := m.sourceAssertion(89, !s.IsMarked(), "!is_marked()"); e != nil {
		return "", e
	}
	return m.text(s, false)
}
func (m *Manager) Display(s Symbol) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.text(s, true)
}

func writeSymbolText(w io.Writer, s string) error {
	n, err := io.WriteString(w, s)
	if err == nil && n != len(s) {
		return io.ErrShortWrite
	}
	return err
}

// WriteDisplay is the retained symbol ostream insertion. A numerical symbol
// writes its prefix before the number, so a failed second write leaves "k!".
// Resolve the immutable spelling before calling a writer that may reenter us.
func (m *Manager) WriteDisplay(w io.Writer, s Symbol) error {
	if w == nil {
		return ErrInvalid
	}
	m.mu.Lock()
	if e := m.checkPlain(s); e != nil {
		m.mu.Unlock()
		return e
	}
	if s.IsNumerical() {
		m.mu.Unlock()
		if e := writeSymbolText(w, "k!"); e != nil {
			return e
		}
		return writeSymbolText(w, strconv.FormatInt(int64(int32(s.Value)), 10))
	}
	if s.Value == 0 {
		m.mu.Unlock()
		return writeSymbolText(w, "null")
	}
	text, e := m.spelling(s)
	m.mu.Unlock()
	if e != nil {
		return e
	}
	return writeSymbolText(w, text)
}

// BareString preserves source nullability through present=false. Its immutable
// Go string remains safe to use after the manager closes.
func (m *Manager) BareString(s Symbol) (text string, present bool, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return "", false, e
	}
	if s.IsNumerical() {
		if e := m.bareStringNumericalAssertion(); e != nil {
			return "", false, e
		}
		// symbol.h:96 returns a non-null empty C string after SASSERT is erased.
		return "", true, nil
	}
	if s.Value == 0 {
		return "", false, nil
	}
	text, err = m.spelling(s)
	return text, err == nil, err
}
func (m *Manager) Hash(s Symbol) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return 0, e
	}
	if s.IsNumerical() {
		return s.Value, nil
	}
	if s.Value == 0 {
		return 0x9e3779d9, nil
	}
	return m.entries[s.Value-1].hash, nil
}
func (m *Manager) Contains(s Symbol, ch byte) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return false, e
	}
	if e := m.sourceAssertion(104, !s.IsMarked(), "!is_marked()"); e != nil {
		return false, e
	}
	if s.IsNumerical() {
		return false, nil
	}
	if s.Value == 0 {
		return false, ErrInvalid
	}
	text, e := m.spelling(s)
	if e != nil {
		return false, e
	}
	return ch == 0 || strings.IndexByte(text, ch) >= 0, nil
}
func (m *Manager) Size(s Symbol) (uint32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return 0, e
	}
	if e := m.sourceAssertion(114, !s.IsMarked(), "!is_marked()"); e != nil {
		return 0, e
	}
	if s.IsNumerical() {
		n, v := uint32(4), s.Value>>1
		for v > 0 {
			n++
			v >>= 1
		}
		return n, nil
	}
	if s.Value == 0 {
		return 0, ErrInvalid
	}
	text, e := m.spelling(s)
	return uint32(len(text)), e
}
func (m *Manager) EqualString(s Symbol, text *string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.checkPlain(s); e != nil {
		return false, e
	}
	if s.Tag == 0 && s.Value == 0 {
		return text == nil, nil
	}
	if text == nil {
		return false, nil
	}
	v := (*text)[:cSymbolLength(*text)]
	if s.IsNumerical() {
		rendered, e := m.text(s, false)
		return rendered == v, e
	}
	spelling, e := m.spelling(s)
	if e != nil {
		return false, e
	}
	return spelling == v, nil
}
func (m *Manager) Less(a, b Symbol) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if e := m.check(a); e != nil {
		return false, e
	}
	if e := m.check(b); e != nil {
		return false, e
	}
	if a == b {
		return false, nil
	}
	if a.IsMarked() || b.IsMarked() {
		return false, ErrInvalid
	}
	if a.IsNumerical() {
		if !b.IsNumerical() {
			return true, nil
		}
		return a.Value < b.Value, nil
	}
	if b.IsNumerical() {
		if e := m.sourceAssertion(139, !a.IsNumerical(), "!s1.is_numerical()"); e != nil {
			return false, e
		}
		return false, nil
	}
	if e := m.sourceAssertion(142, !a.IsNumerical() && !b.IsNumerical(), "!s1.is_numerical() && !s2.is_numerical()"); e != nil {
		return false, e
	}
	if a.Value == 0 || b.Value == 0 {
		return false, ErrInvalid
	}
	if e := m.sourceAssertion(96, !a.IsNumerical(), "!is_numerical()"); e != nil {
		return false, e
	}
	x, e := m.spelling(a)
	if e != nil {
		return false, e
	}
	if e := m.sourceAssertion(96, !b.IsNumerical(), "!is_numerical()"); e != nil {
		return false, e
	}
	y, e := m.spelling(b)
	if e != nil {
		return false, e
	}
	for i := 0; ; i++ {
		var p, q byte
		if i < len(x) {
			p = x[i]
		}
		if i < len(y) {
			q = y[i]
		}
		if int8(p) < int8(q) {
			return true, nil
		}
		if p != q {
			if e := m.sourceAssertion(162, int8(p) > int8(q), "*str1 > *str2"); e != nil {
				return false, e
			}
			return false, nil
		}
		if i+1 >= len(x) {
			if e := m.sourceAssertion(153, i+1 < len(y), "*str2"); e != nil {
				return false, e
			}
			return true, nil
		}
		if i+1 >= len(y) {
			if e := m.sourceAssertion(157, i+1 < len(x), "*str1"); e != nil {
				return false, e
			}
			return false, nil
		}
	}
}

func (m *Manager) sourceAssertion(line int32, predicate bool, expression string) error {
	if !sourceDebug {
		return nil
	}
	enabled, e := diagnostic.SourceAssertionsEnabled(m.debug)
	if e != nil || !enabled || predicate {
		return e
	}
	return diagnostic.DebugAssertionFailure(m.debug, "src/util/symbol.cpp", line, expression)
}
func (m *Manager) Stats() Stats {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Stats{}
	if !m.closed {
		s.OwnerBytes = uint64(reflect.TypeOf(Manager{}).Size())
		s.Symbols = uint32(len(m.byText))
		s.Table.Size = s.Symbols
	}
	return s
}
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return nil
	}
	m.Budget.Release(uint64(reflect.TypeOf(Manager{}).Size()))
	m.byText = nil
	m.entries = nil
	m.closed = true
	return nil
}

// Validate checks a live, unmarked symbol without exposing manager storage.
func (m *Manager) Validate(s Symbol) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.checkPlain(s)
}

// Owns compares identity domains only. Consumers that delay reading a borrowed
// spelling still validate its lifetime when they reach the actual read.
func (m *Manager) Owns(s Symbol) bool { return s.owner == m.owner }
