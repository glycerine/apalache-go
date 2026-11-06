package sat

// Translation of src/sat/sat_model_converter.h/.cpp, Leonardo de Moura.

import (
	"errors"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	utext "github.com/glycerine/g3/pkg/util/text"
)

// ErrSATIncompleteGrowth also guards incomplete clause-lifecycle construction.
var ErrSATIncompleteGrowth = errors.New("sat: access to unconstructed nested vector entry")

type satModelConversionKind uint32

const (
	satModelElimVar satModelConversionKind = iota
	satModelBlockLit
)

type satModelConversionEntry struct {
	varKind  uint32 // source unsigned var:31, kind:1.
	literals []Literal
}
type satModelConversionEntryRef struct {
	owner, epoch uint64
	slot         uint32
}

// Entry and literal backing is native Go storage. Checked entry references use
// stable indexes across append growth and retire on reset or owner close.
type satModelConverter struct {
	self                      *satModelConverter
	budget                    *arena.Budget
	traceTags                 *diagnostic.TraceTags
	entries                   []satModelConversionEntry
	origin, epoch, ownerBytes uint64
	constructed, closed       bool
}

// Cold context is a charged Go owning adapter. The declared model-converter
// member stage acquires its checked owner identity without backing allocation.
func newSATColdModelConverter(b *arena.Budget) (*satModelConverter, error) {
	if b == nil {
		return nil, ErrSATOwner
	}
	header := uint64(reflect.TypeOf(satModelConverter{}).Size())
	if e := b.Reserve(header); e != nil {
		return nil, e
	}
	m := &satModelConverter{budget: b, epoch: 1, ownerBytes: header}
	m.self = m
	return m, nil
}
func (m *satModelConverter) constructionBorrow() error {
	if m == nil || m.self != m {
		return ErrSATOwner
	}
	if m.closed {
		return arena.ErrClosed
	}
	return nil
}
func (m *satModelConverter) initializeSource() error {
	if e := m.constructionBorrow(); e != nil {
		return e
	}
	if m.constructed {
		return ErrSATOwner
	}
	id, e := satNextOwner()
	if e != nil {
		return e
	}
	m.origin = id
	m.constructed = true
	return nil
}
func newSATModelConverter(b *arena.Budget) (*satModelConverter, error) {
	m, e := newSATColdModelConverter(b)
	if e != nil {
		return nil, e
	}
	if e = m.initializeSource(); e != nil {
		if cleanup := m.Close(); cleanup != nil {
			return m, cleanup
		}
		return nil, e
	}
	return m, nil
}
func (m *satModelConverter) open() error {
	if e := m.constructionBorrow(); e != nil {
		return e
	}
	if !m.constructed {
		return arena.ErrClosed
	}
	return nil
}
func (m *satModelConverter) entry(r satModelConversionEntryRef) (*satModelConversionEntry, error) {
	if e := m.open(); e != nil {
		return nil, e
	}
	if r.owner != m.origin {
		return nil, ErrSATOwner
	}
	if r.epoch != m.epoch || uint64(r.slot) >= uint64(len(m.entries)) {
		return nil, arena.ErrInvalidSlot
	}
	return &m.entries[r.slot], nil
}
func (m *satModelConverter) EntryAt(i uint32) (satModelConversionEntryRef, error) {
	if e := m.open(); e != nil {
		return satModelConversionEntryRef{}, e
	}
	if uint64(i) >= uint64(len(m.entries)) {
		return satModelConversionEntryRef{}, arena.ErrBounds
	}
	return satModelConversionEntryRef{m.origin, m.epoch, i}, nil
}
func (m *satModelConverter) KindAndVar(r satModelConversionEntryRef) (satModelConversionKind, BoolVar, error) {
	x, e := m.entry(r)
	if e != nil {
		return 0, 0, e
	}
	return satModelConversionKind(x.varKind >> 31), BoolVar(x.varKind & 0x7fffffff), nil
}
func (m *satModelConverter) Size() uint32 {
	if m == nil || m.closed {
		return 0
	}
	return uint32(len(m.entries))
}
func (m *satModelConverter) Empty() bool { return m.Size() == 0 }

func (m *satModelConverter) Mk(k satModelConversionKind, v BoolVar) (satModelConversionEntryRef, error) {
	if e := m.open(); e != nil {
		return satModelConversionEntryRef{}, e
	}
	// The source enum has values0..1; out-of-range C++ enum conversion is
	// outside its defined domain. The Go typed boundary rejects it before action.
	if k > satModelBlockLit {
		return satModelConversionEntryRef{}, ErrSATKind
	}
	if uint64(len(m.entries)) >= uint64(^uint32(0)) {
		return satModelConversionEntryRef{}, arena.ErrBounds
	}
	i := uint32(len(m.entries))
	bits := uint32(v)&0x7fffffff | (uint32(k)&1)<<31
	m.entries = append(m.entries, satModelConversionEntry{varKind: bits})
	r := satModelConversionEntryRef{m.origin, m.epoch, i}
	if satStorageSourceDebug && (BoolVar(bits&0x7fffffff) != v || satModelConversionKind(bits>>31) != k) {
		return r, ErrSATAssertion
	}
	return r, nil
}
func (m *satModelConverter) InsertLiterals(r satModelConversionEntryRef, lits []Literal) error {
	x, e := m.entry(r)
	if e != nil {
		return e
	}
	v := BoolVar(x.varKind & 0x7fffffff)
	if satStorageSourceDebug {
		contains := false
		for _, l := range lits {
			contains = contains || l.variable() == v
		}
		if !contains {
			return ErrSATAssertion
		}
	}
	for _, l := range lits {
		x.literals = append(x.literals, l)
	}
	x.literals = append(x.literals, DefaultLiteral())
	return nil
}
func (m *satModelConverter) InsertBinary(r satModelConversionEntryRef, a, b Literal) error {
	lits := [2]Literal{a, b}
	if err := m.InsertLiterals(r, lits[:]); err != nil {
		return err
	}
	m.traceInsertBinary(a, b)
	return nil
}
func (m *satModelConverter) InsertClause(r satModelConversionEntryRef, a *ClauseAllocator, c StoredClauseRef) error {
	x, e := m.entry(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug {
		ok, e := a.ContainsVar(c, BoolVar(x.varKind&0x7fffffff))
		if e != nil {
			return e
		}
		if !ok {
			return ErrSATAssertion
		}
	}
	n, e := a.Size(c)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := a.Literal(c, i)
		if e != nil {
			return e
		}
		x.literals = append(x.literals, l)
	}
	x.literals = append(x.literals, DefaultLiteral())
	m.traceInsertClause(a, c)
	return nil
}
func (m *satModelConverter) InsertWrapper(r satModelConversionEntryRef, a *ClauseAllocator, c satClauseWrapper) error {
	x, e := m.entry(r)
	if e != nil {
		return e
	}
	if satStorageSourceDebug {
		ok, e := c.ContainsVar(a, BoolVar(x.varKind&0x7fffffff))
		if e != nil {
			return e
		}
		if !ok {
			return ErrSATAssertion
		}
	}
	n, e := c.Size(a)
	if e != nil {
		return e
	}
	for i := uint32(0); i < n; i++ {
		l, e := c.Literal(a, i)
		if e != nil {
			return e
		}
		x.literals = append(x.literals, l)
	}
	x.literals = append(x.literals, DefaultLiteral())
	m.traceInsertWrapper(a, c)
	return nil
}
func (m *satModelConverter) Copy(src *satModelConverter) error {
	if e := m.open(); e != nil {
		return e
	}
	if e := src.open(); e != nil {
		return e
	}
	// Capture the original count so self-copy duplicates each original once.
	end := len(src.entries)
	if uint64(len(m.entries))+uint64(end) > uint64(^uint32(0)) {
		return arena.ErrBounds
	}
	for i := 0; i < end; i++ {
		old := src.entries[i]
		copied := satModelConversionEntry{varKind: old.varKind, literals: append([]Literal(nil), old.literals...)}
		m.entries = append(m.entries, copied)
	}
	return nil
}

func satConverterModelValue(model Model, l Literal) (lbool.LBool, error) {
	if uint64(l.variable()) >= uint64(len(model)) {
		return lbool.Undef, arena.ErrBounds
	}
	v := model[l.variable()]
	if l.sign() {
		v = v.Negated()
	}
	return v, nil
}
func (m *satModelConverter) Apply(model Model) error {
	return m.applyModelSpan(satSolverModelSpan{slice: model})
}
func (m *satModelConverter) applyModelSpan(model satSolverModelSpan) error {
	if e := m.open(); e != nil {
		return e
	}
	for i := len(m.entries); i > 0; {
		i--
		r, e := m.EntryAt(uint32(i))
		if e != nil {
			return e
		}
		x, e := m.entry(r)
		if e != nil {
			return e
		}
		v := BoolVar(x.varKind & 0x7fffffff)
		if satStorageSourceDebug {
			enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
			if e != nil {
				return e
			}
			if enabled && satModelConversionKind(x.varKind>>31) == satModelElimVar {
				value, e := model.read(v)
				if e != nil {
					return e
				}
				if value != lbool.Undef {
					return ErrSATAssertion
				}
			}
		}
		sat, sign := false, false
		n := uint32(len(x.literals))
		for j := uint32(0); j < n; j++ {
			l := x.literals[j]
			if l == DefaultLiteral() {
				if !sat {
					selected := lbool.True
					if sign {
						selected = lbool.False
					}
					if e := model.write(v, selected); e != nil {
						return e
					}
					break
				}
				sat = false
				continue
			}
			if sat {
				continue
			}
			lv := l.variable()
			if lv == v {
				sign = l.sign()
			}
			value, e := model.value(l)
			if e != nil {
				return e
			}
			if value == lbool.True {
				sat = true
			} else if lv != v {
				current, e := model.read(lv)
				if e != nil {
					return e
				}
				if current == lbool.Undef {
					selected := lbool.True
					if l.sign() {
						selected = lbool.False
					}
					if e := model.write(lv, selected); e != nil {
						return e
					}
					sat = true
				}
			}
		}
		if satStorageSourceDebug {
			if e := m.assertEntryModelSpan(x, model); e != nil {
				return e
			}
		}
	}
	return nil
}

// Apply's DEBUG_CODE asserts at each separator, before the next clause.
// CheckModel deliberately keeps its separate accumulating source traversal.
func (m *satModelConverter) assertEntryModel(x *satModelConversionEntry, model Model) error {
	return m.assertEntryModelSpan(x, satSolverModelSpan{slice: model})
}
func (m *satModelConverter) assertEntryModelSpan(x *satModelConversionEntry, model satSolverModelSpan) error {
	n := uint32(len(x.literals))
	sat := false
	for j := uint32(0); j < n; j++ {
		l := x.literals[j]
		if l == DefaultLiteral() {
			enabled, e := diagnostic.SourceAssertionsEnabled(diagnostic.BorrowProcessDebugContext())
			if e != nil {
				return e
			}
			if enabled && !sat {
				return ErrSATAssertion
			}
			sat = false
			continue
		}
		if sat {
			continue
		}
		v, e := model.value(l)
		if e != nil {
			return e
		}
		sat = v == lbool.True
	}
	return nil
}
func (m *satModelConverter) checkEntryModel(x *satModelConversionEntry, model Model) (bool, error) {
	return m.checkEntryModelSpan(x, satSolverModelSpan{slice: model})
}
func (m *satModelConverter) checkEntryModelSpan(x *satModelConversionEntry, model satSolverModelSpan) (bool, error) {
	n := uint32(len(x.literals))
	ok, sat, start := true, false, uint32(0)
	for j := uint32(0); j < n; j++ {
		l := x.literals[j]
		if l == DefaultLiteral() {
			if !sat {
				m.traceFailedClause(x.literals[start:j])
				ok = false
			}
			sat = false
			start = j + 1
			continue
		}
		if sat {
			continue
		}
		v, e := model.value(l)
		if e != nil {
			return false, e
		}
		sat = v == lbool.True
	}
	return ok, nil
}
func (m *satModelConverter) CheckModel(model Model) (bool, error) {
	return m.checkModelSpan(satSolverModelSpan{slice: model})
}
func (m *satModelConverter) checkModelSpan(model satSolverModelSpan) (bool, error) {
	if e := m.open(); e != nil {
		return false, e
	}
	ok := true
	for i := len(m.entries); i > 0; {
		i--
		r, e := m.EntryAt(uint32(i))
		if e != nil {
			return false, e
		}
		x, e := m.entry(r)
		if e != nil {
			return false, e
		}
		good, e := m.checkEntryModelSpan(x, model)
		if e != nil {
			return false, e
		}
		ok = ok && good
	}
	return ok, nil
}
func (m *satModelConverter) CheckInvariant(numVars uint32) (bool, error) {
	if e := m.open(); e != nil {
		return false, e
	}
	for i := uint32(0); i < uint32(len(m.entries)); i++ {
		r, e := m.EntryAt(i)
		if e != nil {
			return false, e
		}
		x, e := m.entry(r)
		if e != nil {
			return false, e
		}
		if !satStorageSourceDebug {
			continue
		}
		v := x.varKind & 0x7fffffff
		if v >= numVars {
			return false, ErrSATAssertion
		}
		if satModelConversionKind(x.varKind>>31) != satModelElimVar {
			continue
		}
		for j := i + 1; j < uint32(len(m.entries)); j++ {
			r, e := m.EntryAt(j)
			if e != nil {
				return false, e
			}
			other, e := m.entry(r)
			if e != nil {
				return false, e
			}
			if other.varKind&0x7fffffff == v {
				return false, ErrSATAssertion
			}
			n := uint32(len(other.literals))
			for k := uint32(0); k < n; k++ {
				l := other.literals[k]
				if uint32(l.variable()) == v {
					m.traceInvariantVar(BoolVar(v))
					return false, ErrSATAssertion
				}
				if l != DefaultLiteral() && uint32(l.variable()) >= numVars {
					return false, ErrSATAssertion
				}
			}
		}
	}
	return true, nil
}

// Caller-owned append buffer is the explicit set-output boundary. Preserve
// source collect_vars insertion order and duplicate suppression, not literals.
func (m *satModelConverter) AppendVars(dst []BoolVar) ([]BoolVar, error) {
	if e := m.open(); e != nil {
		return dst, e
	}
	for i := uint32(0); i < uint32(len(m.entries)); i++ {
		r, e := m.EntryAt(i)
		if e != nil {
			return dst, e
		}
		x, e := m.entry(r)
		if e != nil {
			return dst, e
		}
		v := BoolVar(x.varKind & 0x7fffffff)
		found := false
		for _, old := range dst {
			found = found || old == v
		}
		if !found {
			dst = append(dst, v)
		}
	}
	return dst, nil
}
func (m *satModelConverter) MaxVar(min uint32) (uint32, error) {
	if e := m.open(); e != nil {
		return min, e
	}
	result := min
	for i := uint32(0); i < uint32(len(m.entries)); i++ {
		r, e := m.EntryAt(i)
		if e != nil {
			return result, e
		}
		x, e := m.entry(r)
		if e != nil {
			return result, e
		}
		n := uint32(len(x.literals))
		for j := uint32(0); j < n; j++ {
			l := x.literals[j]
			if l != DefaultLiteral() && uint32(l.variable()) > result {
				result = uint32(l.variable())
			}
		}
	}
	return result, nil
}
func (m *satModelConverter) DisplayInto(out *utext.NumericTextStream) error {
	if e := m.open(); e != nil {
		return e
	}
	if e := out.AppendCString("(sat::model-converter"); e != nil {
		return e
	}
	for i := uint32(0); i < uint32(len(m.entries)); i++ {
		r, e := m.EntryAt(i)
		if e != nil {
			return e
		}
		x, e := m.entry(r)
		if e != nil {
			return e
		}
		if e = out.AppendCString("\n  ("); e != nil {
			return e
		}
		label := "elim"
		if x.varKind>>31 != 0 {
			label = "blocked"
		}
		if e = out.AppendCString(label); e != nil {
			return e
		}
		if e = out.AppendByte(' '); e != nil {
			return e
		}
		if e = out.AppendUint32(x.varKind & 0x7fffffff); e != nil {
			return e
		}
		start := true
		n := uint32(len(x.literals))
		for j := uint32(0); j < n; j++ {
			l := x.literals[j]
			if start {
				if e = out.AppendCString("\n    ("); e != nil {
					return e
				}
				start = false
			} else if l != DefaultLiteral() {
				if e = out.AppendByte(' '); e != nil {
					return e
				}
			}
			if l == DefaultLiteral() {
				if e = out.AppendByte(')'); e != nil {
					return e
				}
				start = true
				continue
			}
			if e = satLiteralInto(out, l); e != nil {
				return e
			}
		}
		if e = out.AppendByte(')'); e != nil {
			return e
		}
	}
	return out.AppendCString(")\n")
}
func (m *satModelConverter) Reset() error {
	if e := m.open(); e != nil {
		return e
	}
	if m.epoch == ^uint64(0) {
		return arena.ErrSlotLimit
	}
	m.entries = nil
	m.epoch++
	return nil
}

func (m *satModelConverter) Close() error {
	if m == nil || m.self != m {
		return ErrSATOwner
	}
	if m.closed {
		return nil
	}
	m.entries = nil
	m.budget.Release(m.ownerBytes)
	m.ownerBytes = 0
	m.closed = true
	return nil
}

// Real source collect_vars: ordered traversal calls sat::uint_set::insert,
// preserving membership reserve/flag/list-allocation failure prefixes.
func (m *satModelConverter) CollectVars(dst *satSolverUintSet) error {
	if e := m.open(); e != nil {
		return e
	}
	for i := uint32(0); i < uint32(len(m.entries)); i++ {
		r, e := m.EntryAt(i)
		if e != nil {
			return e
		}
		x, e := m.entry(r)
		if e != nil {
			return e
		}
		if e = dst.Insert(x.varKind & 0x7fffffff); e != nil {
			return e
		}
	}
	return nil
}
