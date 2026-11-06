package polynomial

// Source: src/math/polynomial/upolynomial.cpp:1324-1329,432-513.
// Source: src/util/vector.h:59-100; src/math/polynomial/upolynomial.cpp:164-203.
// Concrete typed storage for actual svector<mpz> coefficient ownership.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// UPolynomial is an explicit scalar owner. Never copy it as another owning
// polynomial: Copy performs the source numeral copies through its manager.
type UPolynomial struct {
	origin           uint64
	slot, generation uint32
}

// UPolynomialView borrows actual physical coefficient storage. Swapping vector
// ownership preserves it; growth/free invalidates that physical binding.
type UPolynomialView struct {
	origin                            uint64
	backing, generation, start, count uint32
}

// Backings retain actual constructed numeral fields in native slices. Logical
// polynomial size is distinct from the extent still owned for cleanup.
type upolyVectorState struct {
	origin                    uint64
	backing, generation, size uint32
}
type upolyDescriptor struct {
	state      upolyVectorState
	generation uint32
	live       bool
}
type upolyDescriptorPage struct{ rows [32]upolyDescriptor }
type upolyBacking struct {
	values     []unumeral.Int
	generation uint32
	live       bool
}

func (m *UPolynomialManager) values(s *upolyVectorState) []unumeral.Int {
	if s.backing == 0 {
		return nil
	}
	return m.stateStore(s).backings[s.backing-1].values
}
func (m *UPolynomialManager) backingSlot() (uint32, error) {
	for i := range m.store.backings {
		b := &m.store.backings[i]
		if !b.live && b.generation != math.MaxUint32 {
			return uint32(i + 1), nil
		}
	}
	if uint64(len(m.store.backings)) >= math.MaxUint32 {
		return 0, arena.ErrSlotLimit
	}
	m.store.backings = append(m.store.backings, upolyBacking{generation: 1})
	return uint32(len(m.store.backings)), nil
}

// Reserve extends the native constructed fields, then publishes logical size.
// A real relocation invalidates the old backing binding; swapping ownership or
// extending within native capacity preserves it. No numeric allocation occurs.
func (m *UPolynomialManager) reserve(s *upolyVectorState, n uint32) error {
	if n <= s.size {
		return nil
	}
	if uint64(n) > uint64(^uint(0)>>1) {
		return arena.ErrBounds
	}
	a := m.values(s)
	if s.backing == 0 || uint64(n) > uint64(cap(a)) {
		oldSlot, oldStore := s.backing, m.stateStore(s)
		slot, e := m.backingSlot()
		if e != nil {
			return e
		}
		a = append(a, make([]unumeral.Int, int(n)-len(a))...)
		if oldSlot != 0 {
			old := &oldStore.backings[oldSlot-1]
			old.values = nil
			old.live = false
			if old.generation != math.MaxUint32 {
				old.generation++
			}
		}
		b := &m.store.backings[slot-1]
		b.values, b.live = a, true
		s.origin, s.backing, s.generation = m.store.origin, slot, b.generation
	} else if int(n) > len(a) {
		a = append(a, make([]unumeral.Int, int(n)-len(a))...)
		m.stateStore(s).backings[s.backing-1].values = a
	}
	clear(a[s.size:n])
	s.size = n
	return nil
}
func (m *UPolynomialManager) freeBacking(s *upolyVectorState) {
	if s.backing != 0 {
		b := &m.stateStore(s).backings[s.backing-1]
		b.values = nil
		b.live = false
		if b.generation != math.MaxUint32 {
			b.generation++
		}
	}
	*s = upolyVectorState{}
}
func (m *UPolynomialManager) trim(s *upolyVectorState) error {
	a := m.values(s)
	n := s.size
	for n > 0 {
		zero, e := m.modular.IsZero(&a[n-1])
		if e != nil {
			return e
		}
		if !zero {
			break
		}
		if e = m.modular.Del(&a[n-1]); e != nil {
			return e
		}
		n--
	}
	s.size = n
	return nil
}
func (m *UPolynomialManager) setSize(s *upolyVectorState, n uint32) error {
	enabled, e := refinementSourceAssertionsEnabled()
	if e != nil {
		return e
	}
	if enabled && n > s.size {
		if e := m.scalarDivisionAssertionFailure(177, "old_sz >= sz"); e != nil {
			return e
		}
	}
	if s.backing == 0 {
		return nil
	}
	if uint64(n) > uint64(len(m.values(s))) {
		return arena.ErrBounds
	}
	a := m.values(s)
	for i := n; i < s.size; i++ {
		if e := m.modular.Del(&a[i]); e != nil {
			return e
		}
	}
	s.size = n
	return m.trim(s)
}
func (m *UPolynomialManager) reset(s *upolyVectorState) error { return m.setSize(s, 0) }
func (m *UPolynomialManager) Size(p UPolynomial) (uint32, error) {
	d, e := m.descriptor(p)
	if e != nil {
		return 0, e
	}
	return d.state.size, nil
}
func (m *UPolynomialManager) Capacity(p UPolynomial) (uint32, error) {
	d, e := m.descriptor(p)
	if e != nil {
		return 0, e
	}
	return uint32(cap(m.values(&d.state))), nil
}
func (m *UPolynomialManager) Degree(p UPolynomial) (uint32, error) {
	n, e := m.Size(p)
	if e != nil {
		return 0, e
	}
	if n == 0 {
		return 0, nil
	}
	return n - 1, nil
}
func (m *UPolynomialManager) IsZero(p UPolynomial) (bool, error) {
	d, e := m.descriptor(p)
	if e != nil {
		return false, e
	}
	if e = m.trim(&d.state); e != nil {
		return false, e
	}
	return d.state.size == 0, nil
}
func (m *UPolynomialManager) IsConst(p UPolynomial) (bool, error) {
	d, e := m.descriptor(p)
	if e != nil {
		return false, e
	}
	if e = m.trim(&d.state); e != nil {
		return false, e
	}
	return d.state.size <= 1, nil
}
func (m *UPolynomialManager) Reset(p UPolynomial) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.reset(&d.state)
}
func (m *UPolynomialManager) Trim(p UPolynomial) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.trim(&d.state)
}
func (m *UPolynomialManager) SetSize(p UPolynomial, n uint32) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.setSize(&d.state, n)
}
func (m *UPolynomialManager) Reserve(p UPolynomial, n uint32) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.reserve(&d.state, n)
}
func (m *UPolynomialManager) Swap(p, q UPolynomial) error {
	a, e := m.descriptor(p)
	if e != nil {
		return e
	}
	b, e := m.descriptor(q)
	if e != nil {
		return e
	}
	a.state, b.state = b.state, a.state
	return nil
}
func (m *UPolynomialManager) View(p UPolynomial, start, count uint32) (UPolynomialView, error) {
	d, e := m.descriptor(p)
	if e != nil {
		return UPolynomialView{}, e
	}
	s := &d.state
	if uint64(start)+uint64(count) > uint64(s.size) {
		return UPolynomialView{}, arena.ErrBounds
	}
	return UPolynomialView{m.stateOrigin(s), s.backing, s.generation, start, count}, nil
}
func (m *UPolynomialManager) ReadView(v UPolynomialView, index uint32, out *unumeral.Int) error {
	a, e := m.read(v, index)
	if e != nil {
		return e
	}
	return m.modular.Copy(out, a)
}
func (m *UPolynomialManager) ReadCoefficient(p UPolynomial, index uint32, out *unumeral.Int) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	if index >= d.state.size {
		return arena.ErrBounds
	}
	return m.modular.Copy(out, &m.values(&d.state)[index])
}
func (m *UPolynomialManager) SetCoefficient(p UPolynomial, index uint32, n *unumeral.Int) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	if index >= d.state.size {
		return arena.ErrBounds
	}
	return m.modular.Copy(&m.values(&d.state)[index], n)
}
func (m *UPolynomialManager) set(s *upolyVectorState, a []unumeral.Int) error {
	if uint64(len(a)) > math.MaxUint32 {
		return arena.ErrBounds
	}
	n := uint32(len(a))
	if e := m.reserve(s, n); e != nil {
		return e
	}
	dst := m.values(s)
	for i := uint32(0); i < n; i++ {
		if e := m.modular.Copy(&dst[i], &a[i]); e != nil {
			return e
		}
	}
	return m.setSize(s, n)
}
func (m *UPolynomialManager) SetInts(p UPolynomial, a []unumeral.Int) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.set(&d.state, a)
}

// Borrowed input is checked at the actual p[i] access, after source reserve.
func (m *UPolynomialManager) SetView(p UPolynomial, v UPolynomialView) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	return m.setView(&d.state, v)
}
func (m *UPolynomialManager) setViewWithAllocationContext(p UPolynomial, v UPolynomialView, context unumeral.NumericAllocationContext) error {
	d, e := m.descriptor(p)
	if e != nil {
		return e
	}
	s := &d.state
	if m.alias(v, s) {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && s.size != v.count {
			return m.scalarDivisionAssertionFailure(194, "buffer.size() == sz")
		}
		return nil
	}
	if e = m.reserve(s, v.count); e != nil {
		return e
	}
	ops := m.modular.WithAllocationContext(context)
	for i := uint32(0); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		if e = ops.Copy(&m.values(s)[i], a); e != nil {
			return e
		}
	}
	return m.setSize(s, v.count)
}
func (m *UPolynomialManager) alias(v UPolynomialView, s *upolyVectorState) bool {
	return s.backing != 0 && v.backing == s.backing && v.generation == s.generation && v.start == 0 && v.origin == m.stateOrigin(s)
}
func (m *UPolynomialManager) stateView(s *upolyVectorState) UPolynomialView {
	return UPolynomialView{m.stateOrigin(s), s.backing, s.generation, 0, s.size}
}
func (m *UPolynomialManager) read(v UPolynomialView, index uint32) (*unumeral.Int, error) {
	if e := m.check(); e != nil {
		return nil, e
	}
	store := m.boundStore(v.origin)
	if store == nil || v.backing == 0 || uint64(v.backing) > uint64(len(store.backings)) {
		return nil, ErrUPolynomialOwner
	}
	b := &store.backings[v.backing-1]
	if !b.live || b.generation != v.generation {
		return nil, ErrUPolynomialOwner
	}
	if index >= v.count || uint64(v.start)+uint64(index) >= uint64(len(b.values)) {
		return nil, arena.ErrBounds
	}
	return &b.values[v.start+index], nil
}
func (m *UPolynomialManager) setView(s *upolyVectorState, v UPolynomialView) error {
	return m.setViewWithHensel(s, v, nil)
}
func (m *UPolynomialManager) setViewWithHensel(s *upolyVectorState, v UPolynomialView, receipt *upolyHenselPairInvocationReceipt) error {
	context := numericIntegerModularManager_henselPairAllocationContext(m.modular, receipt)
	if receipt != nil && context.henselPairReceipt(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	return m.setViewWithSourceContext(s, v, context)
}
func (m *UPolynomialManager) setViewWithSourceContext(s *upolyVectorState, v UPolynomialView, context integerAllocationInvocationContext) error {
	if context.sequenceSturm != nil && context.sequenceSturmFrame(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	if context.rootID != nil && context.rootIDFrame(m.config.Integer) == nil {
		return ErrUPolynomialOwner
	}
	if m.alias(v, s) {
		enabled, e := refinementSourceAssertionsEnabled()
		if e != nil {
			return e
		}
		if enabled && s.size != v.count {
			if e := m.scalarDivisionAssertionFailure(194, "buffer.size() == sz"); e != nil {
				return e
			}
		}
		return nil
	}
	if e := m.reserve(s, v.count); e != nil {
		return e
	}
	for i := uint32(0); i < v.count; i++ {
		a, e := m.read(v, i)
		if e != nil {
			return e
		}
		if q := context.rootIDFrame(m.config.Integer); q != nil {
			e = m.modular.CopyFromOwnerTracked(&m.values(s)[i], m.config.Integer, a, &q.remainderCopy, &q.remainderNormalize)
		} else if q := context.sequenceSturmFrame(m.config.Integer); q != nil {
			e = m.modular.CopyFromOwnerTracked(&m.values(s)[i], m.config.Integer, a, &q.remainderCopy, &q.remainderNormalize)
		} else {
			e = m.modular.WithAllocationContext(context.utility(m.modular.Integers())).Copy(&m.values(s)[i], a)
		}
		if e != nil {
			if q := context.removeOneHalfRootFrame(m.config.Integer); q != nil {
				q.recordFailedRow(s, i)
			}
			return e
		}
	}
	return m.setSize(s, v.count)
}
func (m *UPolynomialManager) Copy(dst, src UPolynomial) error {
	d, e := m.descriptor(src)
	if e != nil {
		return e
	}
	v, e := m.View(src, 0, d.state.size)
	if e != nil {
		return e
	}
	return m.SetView(dst, v)
}
