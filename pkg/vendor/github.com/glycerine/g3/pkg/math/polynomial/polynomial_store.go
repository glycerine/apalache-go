package polynomial

// Source: src/math/polynomial/polynomial.cpp:2481-2495; src/util/vector.h.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/arena"

	umemory "github.com/glycerine/g3/pkg/util/memory"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

// Active numerals use native storage. Discarded BIG values remain actual
// owning resources until explicit cleanup; Go backing capacity is never charged.
type polynomialNumerals struct {
	values  []unumeral.Int
	retired []unumeral.Int
}

func (v *polynomialNumerals) size(m *MultivariatePolynomialCore) (uint32, error) {
	return uint32(len(v.values)), nil
}
func (v *polynomialNumerals) coefficient(m *MultivariatePolynomialCore, i uint32) (unumeral.Int, error) {
	if uint64(i) >= uint64(len(v.values)) {
		return unumeral.Int{}, arena.ErrBounds
	}
	return v.values[i], nil
}
func (v *polynomialNumerals) putCoefficient(m *MultivariatePolynomialCore, i uint32, a unumeral.Int) error {
	if uint64(i) >= uint64(len(v.values)) {
		return arena.ErrBounds
	}
	v.values[i] = a
	return nil
}
func (v *polynomialNumerals) publishSize(m *MultivariatePolynomialCore, n uint32) error {
	if uint64(n) > uint64(len(v.values)) {
		return arena.ErrBounds
	}
	for _, a := range v.values[n:] {
		if a.NativeCellToken() != 0 {
			v.retired = append(v.retired, a)
		}
	}
	clear(v.values[n:])
	v.values = v.values[:n]
	return nil
}
func (v *polynomialNumerals) reset(m *MultivariatePolynomialCore) error { return v.publishSize(m, 0) }
func (v *polynomialNumerals) appendCoefficient(m *MultivariatePolynomialCore, a unumeral.Int) error {
	return v.appendCoefficientWithAppendScope(m, a, nil, false)
}
func (v *polynomialNumerals) appendCoefficientWithAppendScope(m *MultivariatePolynomialCore, a unumeral.Int, scope *polynomialCheapAppendScope, owned bool) error {
	i := uint32(len(v.values))
	v.values = append(v.values, a)
	// The actual zero NUM slot exists before vector-size publication and the
	// subsequent SET. A stopped SET still belongs to this constructed row.
	if scope != nil {
		scope.buffer.coefficientTransferred[i] = 0
		if owned {
			scope.coefficients = i + 1
			scope.buffer.coefficientOwners[i] = scope.invocation
			scope.coefficientClaims++
		} else {
			scope.foreignMixed = true
			scope.buffer.coefficientOwners[i] = 0
			if scope.retirePhase == 1 || scope.retirePhase == 2 {
				scope.retireAs = i + 1
			}
		}
	}
	return nil
}
func (v *polynomialNumerals) close(m *MultivariatePolynomialCore) error {
	for i := range v.retired {
		if e := m.deleteNumeral(&v.retired[i]); e != nil {
			return e
		}
	}
	v.retired = nil
	for i := range v.values {
		if e := m.deleteNumeral(&v.values[i]); e != nil {
			return e
		}
	}
	v.values = nil
	return nil
}
func (m *MultivariatePolynomialCore) prepareLocation() (uint32, error) {
	for i := range m.locations {
		l := &m.locations[i]
		if !l.live && l.generation != math.MaxUint32 {
			l.generation++
			return uint32(i + 1), nil
		}
	}
	if uint64(len(m.locations)) >= math.MaxUint32 {
		return 0, arena.ErrSlotLimit
	}
	if len(m.locations) == int(^uint(0)>>1) {
		return 0, arena.ErrBounds
	}
	m.locations = append(m.locations, polynomialLocation{generation: 1})
	return uint32(len(m.locations)), nil
}

// The source directory is an ID-indexed, nonowning collection of live handles.
func (m *MultivariatePolynomialCore) publishDirectory(p Polynomial, id uint32) error {
	for uint64(len(m.polynomials)) <= uint64(id) {
		m.polynomials = append(m.polynomials, Polynomial{})
	}
	if e := m.directorySlotSourceAssert(id); e != nil {
		return e
	}
	m.polynomials[id] = p
	return nil
}
func (m *MultivariatePolynomialCore) mkCell(n uint32, as []unumeral.Int, ms []Monomial, buffer *polynomialSOM, single *unumeral.Int) (Polynomial, error) {
	return m.mkCellWithReceipt(n, as, ms, buffer, single, nil)
}
func (m *MultivariatePolynomialCore) mkCellWithReceipt(n uint32, as []unumeral.Int, ms []Monomial, buffer *polynomialSOM, single *unumeral.Int, receipt *polynomialFactoryInvocationReceipt) (Polynomial, error) {
	return m.mkCellWithReceipts(n, as, ms, buffer, single, receipt, nil)
}
func (m *MultivariatePolynomialCore) mkCellWithReceipts(n uint32, as []unumeral.Int, ms []Monomial, buffer *polynomialSOM, single *unumeral.Int, receipt *polynomialFactoryInvocationReceipt, appendReceipt *polynomialCheapAppendInvocationReceipt) (Polynomial, error) {
	if m.pending.raw != (umemory.ObjectRef{}) || m.pending.value != (Polynomial{}) {
		return Polynomial{}, ErrPolynomialPending
	}
	if receipt != nil && m.factoryInvocation.active {
		return Polynomial{}, ErrPolynomialPending
	}
	var appendScope *polynomialCheapAppendScope
	if appendReceipt != nil && appendReceipt.invocation != 0 {
		if buffer == nil || !buffer.matchesAppendInvocation(appendReceipt) {
			return Polynomial{}, ErrPolynomialPending
		}
		appendScope = &buffer.appendScope
		size, e := buffer.as.size(m)
		if e != nil {
			return Polynomial{}, e
		}
		if appendScope.foreignMixed || appendScope.stopped || appendScope.completedTransfer || appendScope.monomials != n || appendScope.coefficients != size || buffer.hasPendingReference {
			return Polynomial{}, ErrPolynomialPending
		}
	}
	// Actual constructor start receipt, after the foreign pending guard.
	if m.creationGeneration == ^uint64(0) {
		return Polynomial{}, arena.ErrBounds
	}
	m.creationGeneration++
	m.pending = polynomialCreation{generation: m.creationGeneration}
	if appendScope != nil {
		m.pending.appendParent = appendScope
		appendScope.factoryInvocation = m.creationGeneration
	} else if buffer != nil && buffer.appendScope.active {
		// Actual unreceipted constructor transfer is foreign to the old append
		// cohort, even when a later vector happens to have identical sizes.
		buffer.appendScope.foreignMixed = true
	}
	if receipt != nil {
		m.factoryInvocation = polynomialFactoryScope{active: true, executing: true, invocation: m.creationGeneration, buffer: buffer}
		*receipt = polynomialFactoryInvocationReceipt{manager: m, frame: &m.factoryInvocation, invocation: m.creationGeneration}
		defer func() { m.factoryInvocation.executing = false }()
	}
	if f := m.factorsOwner; f != nil && f.executing && m.callbackDepth == 0 {
		m.pending.factorsParent = f
		f.factoryGeneration = m.creationGeneration
	}
	if s := m.conversionOwner; s != nil && s.executing && m.callbackDepth == 0 {
		m.pending.conversionParent = s
		s.factoryGeneration = m.creationGeneration
	}
	if m.compositionOwner != nil && m.compositionOwner.executing {
		m.pending.compositionParent = m.compositionOwner
		m.compositionOwner.factoryGeneration = m.creationGeneration
	}
	if m.derivativeOwner != nil && m.derivativeOwner.executing {
		m.pending.derivativeParent = m.derivativeOwner
		m.derivativeOwner.factoryGeneration = m.creationGeneration
	}
	m.pending.substitutionOwned = m.substitution.scope.executing
	m.pending.pscOwner = m.pscExecutingOwner()
	m.pending.translationOwned = m.translation.executing
	requested := polynomialCodecBytes(n)
	observer := m.pending.compositionParent
	var before [3]umemory.Block
	if observer != nil || m.pending.conversionParent != nil || m.pending.factorsParent != nil {
		var e error
		before, e = m.monomials.allocator.PendingRoots()
		if e != nil {
			return Polynomial{}, e
		}
	}
	var raw umemory.ObjectRef
	var e error
	if outcome := m.factoryAllocationOutcome(receipt); outcome != nil {
		var started bool
		if e := m.prepareNormalizationConverterHistoryAtAllocation(receipt, nil); e != nil {
			return Polynomial{}, e
		}
		raw, started, e = m.monomials.allocator.AllocateWithOutcomeStarted(uint64(requested), outcome)
		if started {
			m.factoryInvocation.allocationStarted = true
			m.factoryInvocation.allocationCause = e
			m.factoryInvocation.allocator[0] = outcome.Pending
		}
	} else {
		raw, e = m.monomials.allocator.Allocate(uint64(requested))
	}
	if raw != (umemory.ObjectRef{}) {
		m.pending.raw = raw
		m.pending.requested = requested
	}
	if observer != nil && before == ([3]umemory.Block{}) {
		after, probe := m.monomials.allocator.PendingRoots()
		if after != ([3]umemory.Block{}) {
			observer.monomialRaw = after
		}
		if probe != nil {
			return Polynomial{}, errors.Join(e, probe)
		}
	}
	if owner := m.pending.conversionParent; owner != nil && before == ([3]umemory.Block{}) {
		after, probe := m.monomials.allocator.PendingRoots()
		if after != ([3]umemory.Block{}) {
			owner.monomialRaw = after
		}
		if probe != nil {
			return Polynomial{}, errors.Join(e, probe)
		}
	}
	if f := m.pending.factorsParent; f != nil && before == ([3]umemory.Block{}) {
		after, x := m.monomials.allocator.PendingRoots()
		if after != ([3]umemory.Block{}) {
			f.monomialRaw = after
		}
		if x != nil {
			return Polynomial{}, errors.Join(e, x)
		}
	}
	if e != nil {
		return Polynomial{}, e
	}
	id := m.ids.Mk()
	m.pending.id = id
	m.pending.idConsumed = true
	slot, e := m.prepareLocation()
	if e != nil {
		return Polynomial{}, e
	}
	l := &m.locations[slot-1]
	g := l.generation
	// Actual constructor layout fixes m_ms independently of later mutable SIZE.
	// Retain the allocation count before the uint32 request-size wrap.
	*l = polynomialLocation{raw: raw, requested: requested, generation: g, live: true, monomialBase: 12 + uint64(n)*12}
	p := Polynomial{owner: m.origin, slot: slot, generation: g}
	m.pending.value = p
	m.pending.raw = umemory.ObjectRef{}
	m.pending.buffer = buffer
	if e = m.writeWord(p, 0, 0); e != nil {
		return Polynomial{}, e
	}
	if e = m.writeWord(p, 4, id&0x7fffffff); e != nil {
		return Polynomial{}, e
	}
	if e = m.writeWord(p, 8, n); e != nil {
		return Polynomial{}, e
	}
	max := uint32(0)
	for i := uint32(0); i < n; i++ {
		if e = m.writeCoefficient(p, i, unumeral.Int{}); e != nil {
			return Polynomial{}, e
		}
		l.initializedCoefficients = i + 1
		var a unumeral.Int
		movingAppend := false
		if buffer != nil {
			a, e = buffer.as.coefficient(m, i)
			if e != nil {
				return Polynomial{}, e
			}
			if e = buffer.as.putCoefficient(m, i, unumeral.Int{}); e != nil {
				return Polynomial{}, e
			}
			if buffer.appendScope.active && uint64(i) < uint64(len(buffer.coefficientOwners)) && buffer.coefficientOwners[i] == buffer.appendScope.invocation {
				buffer.appendScope.movingCoefficient = a
				movingAppend = true
				buffer.coefficientOwners[i] = 0
				buffer.coefficientTransferred[i] = buffer.appendScope.invocation
				buffer.appendScope.coefficientClaims--
			}
		} else if single != nil {
			a = *single
			*single = unumeral.Int{}
		} else {
			if uint64(i) >= uint64(len(as)) {
				return Polynomial{}, arena.ErrBounds
			}
			a = as[i]
			as[i] = unumeral.Int{}
		}
		if e = m.writeCoefficient(p, i, a); e != nil {
			return Polynomial{}, e
		}
		if movingAppend {
			buffer.appendScope.movingCoefficient = unumeral.Int{}
		}
		if appendScope != nil {
			appendScope.transferredCoefficients = i + 1
		}
		if e = m.cellSourceAssert(1415, "ms[i]->ref_count() > 0", func() (bool, error) {
			term, e := m.constructorMonomial(buffer, ms, i)
			if e != nil {
				return false, e
			}
			r, e := m.monomialRefCount(term)
			return r > 0, e
		}); e != nil {
			return Polynomial{}, e
		}
		term, e := m.constructorMonomial(buffer, ms, i)
		if e != nil {
			return Polynomial{}, e
		}
		if e = m.writeMonomial(p, i, term); e != nil {
			return Polynomial{}, e
		}
		l.initializedMonomials = i + 1
		if buffer == nil && i == 0 && m.singletonReference && term == m.singletonMonomial {
			if m.pscReceipts.singleton == term {
				m.pscReceipts.singleton = Monomial{}
			}
			if f := m.factorsOwner; f != nil && f.singleton == term && f.singletonGeneration == m.singletonGeneration {
				f.singleton = Monomial{}
			}
			if s := &m.variableLocal; s.active && s.singleton == term && s.singletonGeneration == m.singletonGeneration {
				s.singleton = Monomial{}
			}
			m.singletonReference = false
			m.singletonMonomial = Monomial{}
		}
		if buffer != nil && buffer.appendScope.active && uint64(i) < uint64(len(buffer.monomialOwners)) && buffer.monomialOwners[i] == buffer.appendScope.invocation {
			buffer.monomialOwners[i] = 0
			buffer.monomialTransferred[i] = buffer.appendScope.invocation
			buffer.appendScope.monomialClaims--
		}
		if buffer != nil {
			buffer.transferred = i + 1
			if appendScope != nil {
				appendScope.transferredMonomials = i + 1
			}
		}
		if i > 0 {
			best, e := m.monomialCore(p, max)
			if e != nil {
				return Polynomial{}, e
			}
			greater, e := m.firstGreater(term, best)
			if e != nil {
				return Polynomial{}, e
			}
			if greater {
				max = i
			}
		}
	}
	if e = m.swapTerms(p, 0, max); e != nil {
		return Polynomial{}, e
	}
	if e = m.publishDirectory(p, id); e != nil {
		return Polynomial{}, e
	}
	l.published = true
	if owner := m.pending.conversionParent; owner != nil && owner.factoryGeneration == m.pending.generation {
		owner.factoryGeneration = 0
		owner.result = p
		owner.ownsResult = true
	}
	if m.translation.executing {
		m.translation.freshResult = p
	}
	if m.pending.derivativeParent != nil && m.pending.derivativeParent.factoryGeneration == m.pending.generation {
		m.pending.derivativeParent.factoryGeneration = 0
	}
	if owner := m.pending.compositionParent; owner != nil && owner.factoryGeneration == m.pending.generation {
		owner.factoryGeneration = 0
		owner.freshResult = p
	}
	if f := m.pending.factorsParent; f != nil && f.factoryGeneration == m.pending.generation {
		f.factoryGeneration = 0
		f.freshResult = p
	}
	if receipt != nil {
		m.factoryInvocation.result = p
	}
	m.pending = polynomialCreation{}
	m.live++
	return p, nil
}
