package polynomial

// Source: src/math/polynomial/polynomial.cpp; src/math/polynomial/polynomial.h.

import (
	"github.com/glycerine/g3/pkg/arena"
	umemory "github.com/glycerine/g3/pkg/util/memory"
)

func (m *MonomialManager) keyWord(k monomialKey, word uint32) (uint32, error) {
	if k.kind == monomialBorrowedTemporaryKey {
		if m.borrowedTemp == nil || k.slot != 0 {
			return 0, ErrMonomialPending
		}
		return m.tempKeyWord(m.borrowedTemp, word)
	}
	if k.kind == monomialTemporaryKey {
		if k.slot >= uint32(len(m.temps)) {
			return 0, arena.ErrBounds
		}
		return m.tempKeyWord(&m.temps[k.slot], word)
	}
	if k.kind != monomialObjectKey || k.slot == 0 || uint64(k.slot) > uint64(len(m.locations)) {
		return 0, ErrMonomialOwner
	}
	l := &m.locations[k.slot-1]
	if !l.live {
		return 0, ErrMonomialOwner
	}
	return m.allocator.ReadUint32(l.raw, uint64(word)*4)
}
func (m *MonomialManager) keyPower(k monomialKey, index uint32) (MonomialPower, error) {
	if k.kind == monomialBorrowedTemporaryKey {
		if m.borrowedTemp == nil || k.slot != 0 {
			return MonomialPower{}, ErrMonomialPending
		}
		return m.temporaryKeyPower(m.borrowedTemp, index)
	}
	if k.kind == monomialTemporaryKey {
		if k.slot >= uint32(len(m.temps)) {
			return MonomialPower{}, arena.ErrBounds
		}
		return m.temporaryKeyPower(&m.temps[k.slot], index)
	}
	if k.kind != monomialObjectKey || k.slot == 0 || uint64(k.slot) > uint64(len(m.locations)) {
		return MonomialPower{}, ErrMonomialOwner
	}
	return m.powerCore(Monomial{m.origin, k.slot, m.locations[k.slot-1].generation}, index)
}

// Source eq_proc receives tmp.get_ptr() as a monomial pointer, so its
// get_power uses logical size228, not just the native temporary capacity.
func (m *MonomialManager) temporaryKeyPower(t *monomialTemp, i uint32) (MonomialPower, error) {
	if e := m.tempConstruct(t); e != nil {
		return MonomialPower{}, e
	}
	enabled, e := monomialSourceAssertionsEnabled()
	if e != nil {
		return MonomialPower{}, e
	}
	if enabled && i >= t.size {
		if e = m.sourceAssertionFailure(228, "idx < size()"); e != nil {
			return MonomialPower{}, e
		}
	}
	return m.tempPower(t, i)
}

func validMonomialPowers(p []MonomialPower) bool {
	for i := 1; i < len(p); i++ {
		if p[i-1].Var >= p[i].Var {
			return false
		}
	}
	return true
}

// Ephemeral outcome of one actual factory call, not persistent ownership.
// Existing interned/unit results do not start a creation, irrespective of RC.
type monomialFactoryOutcome struct {
	created, started    bool
	generation          uint64
	allocator           [3]umemory.Block
	allocatorInvocation monomialAllocatorInvocationReceipt
}

func (m *MonomialManager) MkMonomial(p []MonomialPower) (Monomial, error) {
	return m.mkMonomialCreated(p, nil)
}
func (m *MonomialManager) mkMonomialCreated(p []MonomialPower, outcome *monomialFactoryOutcome) (result Monomial, cause error) {
	if outcome != nil {
		*outcome = monomialFactoryOutcome{}
	}
	defer func() {
		if cause == nil {
			m.creation = monomialCreation{}
		}
	}()
	if e := m.check(); e != nil {
		return Monomial{}, e
	}
	if e := m.sourceScalarAssert(867, "is_valid_power_product(sz, pws)", validMonomialPowers(p)); e != nil {
		return Monomial{}, e
	}
	if m.creation.stage != 0 {
		return Monomial{}, ErrMonomialPending
	}
	if e := m.tempInit(&m.temps[0], p); e != nil {
		return Monomial{}, e
	}
	return m.internTempCreated(0, outcome)
}
func (m *MonomialManager) internTemp(index uint32) (result Monomial, cause error) {
	result, cause = m.internTempCreated(index, nil)
	if cause == nil {
		m.creation = monomialCreation{}
	}
	return
}

// created reports the actual new intern publication. On a reached callee
// postcondition guard, creation remains the callee's real retained owner.
func (m *MonomialManager) internTempCreated(index uint32, outcome *monomialFactoryOutcome) (Monomial, error) {
	if m.borrowedTemp != nil {
		return Monomial{}, ErrMonomialPending
	}
	if index >= uint32(len(m.temps)) {
		return Monomial{}, arena.ErrBounds
	}
	return m.internSourceTempCreated(&m.temps[index], monomialKey{monomialTemporaryKey, index}, outcome)
}

// The same source factory accepts the actual imp-owned temporary. The table
// key carries a scalar provenance tag; the sole cold borrow remains live at a
// stopped insertion until its real callee owning abort removes that binding.
func (m *MonomialManager) internBorrowedTempCreated(t *monomialTemp, outcome *monomialFactoryOutcome) (Monomial, error) {
	if e := m.check(); e != nil {
		return Monomial{}, e
	}
	if m.borrowedTemp != nil || m.creation.stage != 0 {
		return Monomial{}, ErrMonomialPending
	}
	g := m.nextContinuationGeneration()
	m.borrowedTemp = t
	m.borrowedGeneration = g
	r, e := m.internSourceTempCreated(t, monomialKey{monomialBorrowedTemporaryKey, 0}, outcome)
	if e == nil {
		m.creation = monomialCreation{}
		m.borrowedTemp = nil
	}
	return r, e
}
func (m *MonomialManager) internSourceTempCreated(t *monomialTemp, key monomialKey, outcome *monomialFactoryOutcome) (Monomial, error) {
	if outcome != nil {
		*outcome = monomialFactoryOutcome{}
	}
	if m.creation.stage != 0 {
		return Monomial{}, ErrMonomialPending
	}
	if _, e := m.tempHash(t); e != nil {
		return Monomial{}, e
	}
	pos, inserted, e := m.table.insert(m, key)
	if e != nil {
		return Monomial{}, e
	}
	if !inserted {
		old, e := m.table.key(pos)
		if e != nil {
			return Monomial{}, e
		}
		if old.kind != monomialObjectKey || old.slot == 0 || uint64(old.slot) > uint64(len(m.locations)) {
			return Monomial{}, ErrMonomialPending
		}
		l := &m.locations[old.slot-1]
		return Monomial{m.origin, old.slot, l.generation}, nil
	}
	generation := m.nextContinuationGeneration()
	m.creation = monomialCreation{tableIndex: pos, stage: 1, generation: generation}
	if outcome != nil {
		outcome.started = true
		outcome.generation = generation
	}
	slot, e := m.prepareLocation()
	if e != nil {
		return Monomial{}, e
	}
	m.creation.slot = slot
	m.creation.stage = 2
	l := &m.locations[slot-1]
	n, e := m.tempSize(t)
	if e != nil {
		return Monomial{}, e
	}
	requested := monomialObjectSize(n)
	l.requested = requested
	frame := &m.allocatorInvocation
	object, started, e := m.allocator.AllocateWithOutcomeStarted(uint64(requested), &frame.outcome)
	if started {
		frame.owner = m
		frame.invocation = m.allocator.AllocationOutcomeInvocation(&frame.outcome)
		frame.creationGeneration = m.creation.generation
		frame.sourceCause = e
		if outcome != nil {
			outcome.allocatorInvocation = monomialAllocatorInvocationReceipt{m, frame, &frame.outcome, frame.invocation}
			outcome.allocator[0] = frame.outcome.Pending
		}
	}
	if object != (umemory.ObjectRef{}) {
		l.raw = object
		l.live = true
		l.sourceRaw, l.sourceOffset, e = m.allocator.RawLocation(object)
		if e != nil {
			return Monomial{}, e
		}
	}
	if e != nil {
		return Monomial{}, e
	}
	m.creation.stage = 3
	id := m.ids.Mk()
	m.creation.id = id
	m.creation.idConsumed = true
	l.id = id
	m.creation.stage = 4
	h := t.hash
	for i, v := range [5]uint32{0, id, 0, n, h} {
		if e = m.allocator.WriteUint32(object, uint64(i)*4, v); e != nil {
			return Monomial{}, e
		}
	}
	l.headerInitialized = true
	m.creation.stage = 5
	total := uint32(0)
	a := Monomial{m.origin, slot, l.generation}
	for i := uint32(0); i < n; i++ {
		p, e := m.tempPower(t, i)
		if e != nil {
			return Monomial{}, e
		}
		if e = m.allocator.WriteUint32(object, 20+uint64(i)*8, p.Var); e != nil {
			return Monomial{}, e
		}
		if e = m.allocator.WriteUint32(object, 24+uint64(i)*8, p.Degree); e != nil {
			return Monomial{}, e
		}
		if e = m.constructorOrderAssert(a, i); e != nil {
			return Monomial{}, e
		}
		if e = m.constructorDegreeAssert(a, i); e != nil {
			return Monomial{}, e
		}
		degree, e := m.degreeAtCore(a, i)
		if e != nil {
			return Monomial{}, e
		}
		total += degree
		if e = m.allocator.WriteUint32(object, 8, total); e != nil {
			return Monomial{}, e
		}
	}
	if e = m.table.setKey(pos, monomialKey{monomialObjectKey, slot}); e != nil {
		return Monomial{}, e
	}
	l.published = true
	m.publishedObjects++
	if outcome != nil {
		outcome.created = true
	}
	key = monomialKey{monomialObjectKey, slot}
	if e := m.factoryContainsAssert(key); e != nil {
		return Monomial{m.origin, slot, l.generation}, e
	}
	if e := m.factoryFindAssert(key); e != nil {
		return Monomial{m.origin, slot, l.generation}, e
	}
	return Monomial{m.origin, slot, l.generation}, nil
}
func (m *MonomialManager) MkVariable(v, k uint32) (Monomial, error) {
	return m.mkVariableCreatedAt(v, k, nil, 893)
}
func (m *MonomialManager) mkVariableCreated(v, k uint32, outcome *monomialFactoryOutcome) (Monomial, error) {
	return m.mkVariableCreatedAt(v, k, outcome, 893)
}
func (m *MonomialManager) mkVariableCreatedAt(v, k uint32, outcome *monomialFactoryOutcome, line int32) (Monomial, error) {
	if outcome != nil {
		*outcome = monomialFactoryOutcome{}
	}
	if e := m.check(); e != nil {
		return Monomial{}, e
	}
	if k == 0 {
		return m.unit, nil
	}
	if e := m.sourceScalarAssert(line, "is_valid(x)", v < m.nextVar); e != nil {
		return Monomial{}, e
	}
	one := [1]MonomialPower{{v, k}}
	return m.mkMonomialCreated(one[:], outcome)
}
func (m *MonomialManager) Convert(source *MonomialManager, a Monomial) (Monomial, error) {
	return m.convertCreated(source, a, nil)
}
func (m *MonomialManager) convertCreated(source *MonomialManager, a Monomial, outcome *monomialFactoryOutcome) (result Monomial, cause error) {
	if outcome != nil {
		*outcome = monomialFactoryOutcome{}
	}
	defer func() {
		if cause == nil {
			m.creation = monomialCreation{}
		}
	}()
	if e := m.check(); e != nil {
		return Monomial{}, e
	}
	if e := source.check(); e != nil {
		return Monomial{}, e
	}
	info, e := source.sizeInfoCore(a)
	if e != nil {
		return Monomial{}, e
	}
	for i := uint32(0); i < info.Size; i++ {
		v, e := source.varCore(a, i)
		if e != nil {
			return Monomial{}, e
		}
		for v >= m.nextVar {
			if _, e = m.MkVar(); e != nil {
				return Monomial{}, e
			}
		}
		if e = m.sourceScalarAssert(879, "x < num_vars()", v < m.nextVar); e != nil {
			return Monomial{}, e
		}
	}
	if m.creation.stage != 0 {
		return Monomial{}, ErrMonomialPending
	}
	// Source init copies all powers before interning. No hidden spelling slice.
	t := &m.temps[0]
	if e = m.tempSetSize(t, info.Size); e != nil {
		return Monomial{}, e
	}
	for i := uint32(0); i < info.Size; i++ {
		p, e := source.ReadPower(a, i)
		if e != nil {
			return Monomial{}, e
		}
		if e = m.tempSetPower(t, i, p); e != nil {
			return Monomial{}, e
		}
	}
	return m.internTempCreated(0, outcome)
}

// CleanupCreation is an owning Go abort of the actual named unpublished object
// and temporary table binding. It does not roll back IDs or allocation counters.
func (m *MonomialManager) CleanupCreation() error {
	if m.substitutionTraceCallback() || (m.exactDivTraceCallback() || m.lexTraceCallback()) {
		return ErrMonomialPending
	}
	if m == nil || m.self != m || m.closed {
		return ErrMonomialOwner
	}
	c := &m.creation
	if c.stage == 0 {
		m.completeBorrowedContinuation()
		return nil
	}
	if !c.tableRemoved {
		k, e := m.table.key(c.tableIndex)
		if e != nil {
			return e
		}
		m.table.removeToken(k)

		c.tableRemoved = true
	}
	if c.slot != 0 {
		l := &m.locations[c.slot-1]
		if l.raw != (umemory.ObjectRef{}) {
			progress, e := m.allocator.DeallocateTracked(uint64(l.requested), l.raw)
			if progress.PhysicalReleased || progress.PooledLinked {
				l.raw = umemory.ObjectRef{}
			}
			if e != nil {
				return e
			}
		}
		if l.published {
			m.publishedObjects--
			l.published = false
		}
		l.live = false
	}
	m.completeCreationContinuation()
	*c = monomialCreation{}
	m.completeBorrowedContinuation()
	return nil
}

// Cold continuation identity is distinct from source uint32 IDs/statistics.
// It is charged within the actual owner; no per-call allocation or ticket.
func (m *MonomialManager) nextContinuationGeneration() uint64 {
	// A new creation is possible only after the sole prior continuation is
	// completed, which clears its exact parent receipt. No completed generation
	// remains an authority. Skip zero when wrapping this cold adapter sequence.
	m.continuationGeneration++
	if m.continuationGeneration == 0 {
		m.continuationGeneration = 1
	}
	return m.continuationGeneration
}
func (m *MonomialManager) completeCreationContinuation() {
	c := &m.creation
	if s := c.coefficientParent; s != nil && s.active && s.invocation == c.coefficientInvocation && s.calleeGeneration == c.generation {
		s.calleeGeneration = 0
	}
	c.coefficientParent = nil
	c.coefficientInvocation = 0
	if p := c.parent; p != nil && p.generatedCalleeGeneration == c.generation {
		p.generatedCalleePending = false
		p.generatedCalleeGeneration = 0
		// This exact monomial continuation has ended. Retire its legacy PSC
		// receipt here, so typed lexical cleanup needs no generation-wide scan.
		if p.pscReceipts.factory == c.generation {
			p.pscReceipts.factory = 0
		}
		if p.substitution.generatedCalleeReceipt == c.generation {
			p.substitution.generatedCalleeReceipt = 0
		}
	}
	c.parent = nil
}
func (m *MonomialManager) completeBorrowedContinuation() {
	if p := m.borrowedParent; p != nil && p.substitution.borrowedReceipt == m.borrowedGeneration {
		p.substitution.borrowedReceipt = 0
	}
	m.borrowedParent = nil
	m.borrowedTemp = nil
}

// Only size and cached hash are read from a temporary by retained table users.
func (m *MonomialManager) tempKeyWord(t *monomialTemp, word uint32) (uint32, error) {
	switch word {
	case 3:
		return m.tempSize(t)
	case 4:
		return t.hash, nil
	}
	return 0, arena.ErrBounds
}
