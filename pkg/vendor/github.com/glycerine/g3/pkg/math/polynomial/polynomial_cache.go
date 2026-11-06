package polynomial

// Source: src/math/polynomial/polynomial_cache.cpp:35-92,129-227.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
)

// PolynomialCache owns canonical representatives and memoized results used by
// the retained NLSAT cache.
type PolynomialCache struct {
	hashLex        polynomialLexSortInvocationReceipt
	nextPending    *PolynomialCache
	pendingBound   bool
	pendingCause   error
	previous       *PolynomialCache
	executing      bool
	pscCall        polynomialPSCInvocationReceipt
	factorCall     polynomialFactorCoreInvocationReceipt
	self           *PolynomialCache
	manager        *MultivariatePolynomialManager
	refs           polynomialReferenceVector
	buckets        map[uint32][]Polynomial
	ids            map[uint32]bool
	psc            map[polynomialPSCKey]*polynomialPSCEntry
	factors        map[Polynomial]*polynomialFactorCacheEntry
	pendingFactors *PolynomialFactors
	pendingPSC     bool
	header         uint64
	imp            *polynomialCacheImpOwner
	resetting      bool
	closing        bool
	closed         bool

	factorLocalSourceOwed bool
}

type polynomialPSCKey struct {
	p, q Polynomial
	x    uint32
}

type polynomialPSCEntry struct {
	result   []Polynomial
	complete bool
}

type polynomialFactorCacheEntry struct {
	result   []Polynomial
	complete bool
}

func NewPolynomialCache(m *MultivariatePolynomialManager) (*PolynomialCache, error) {
	if e := m.ready(); e != nil {
		return nil, e
	}
	header := uint64(reflect.TypeOf(PolynomialCache{}).Size())
	if e := m.core.integer.Budget().Reserve(header); e != nil {
		return nil, e
	}
	c := &PolynomialCache{manager: m, header: header, buckets: make(map[uint32][]Polynomial), ids: make(map[uint32]bool), psc: make(map[polynomialPSCKey]*polynomialPSCEntry), factors: make(map[Polynomial]*polynomialFactorCacheEntry)}
	c.self = c
	// The source alloc(imp,m) memory trace executes before this constructor
	// returns. Keep its borrowed manager alive across that actual callback.
	c.beginExecution()
	defer c.endExecution()
	if e := c.newImp(207); e != nil {
		m.core.integer.Budget().Release(header)
		return nil, e
	}
	return c, nil
}

// Factor returns distinct factors, dropping multiplicities and the source
// factorization constant. The result cache is keyed by the canonical input.
func (c *PolynomialCache) Factor(p Polynomial, out *PolynomialRefVector) error {
	if e := c.ready(); e != nil {
		return e
	}
	c.beginExecution()
	defer c.endExecution()
	if out == nil || out.owner != c.manager.core {
		return ErrPolynomialOwner
	}
	if e := out.Reset(); e != nil {
		return e
	}
	var e error
	p, e = c.mkUnique(p)
	if e != nil {
		return e
	}
	if entry, ok := c.factors[p]; ok {
		// Source entries start with result size0; interruption alone does not
		// invalidate a later empty hit. Unwritten published slots are not values.
		if !entry.complete && len(entry.result) != 0 {
			return ErrPolynomialPending
		}
		if e := out.Reset(); e != nil {
			return e
		}
		for _, value := range entry.result {
			if e := out.Push(value); e != nil {
				return e
			}
		}
		return nil
	}
	entry := &polynomialFactorCacheEntry{}
	c.factors[p] = entry
	c.factorCall = polynomialFactorCoreInvocationReceipt{}
	fs, e := NewPolynomialFactors(c.manager)
	if e != nil {
		if fs != nil {
			return c.finishFactorOwner(fs, e)
		}
		return e
	}
	if e := c.manager.factorWithReceipt(p, fs, DefaultUPolynomialFactorParams(), &c.factorCall); e != nil {
		return c.finishFactorOwner(fs, e)
	}
	n, e := fs.Distinct()
	if e != nil {
		return c.finishFactorOwner(fs, e)
	}
	entry.result = make([]Polynomial, n)
	for i := uint32(0); i < n; i++ {
		value, e := fs.factor(i)
		if e != nil {
			return c.finishFactorOwner(fs, e)
		}
		value, e = c.mkUnique(value)
		if e != nil {
			return c.finishFactorOwner(fs, e)
		}
		if e := out.Push(value); e != nil {
			return c.finishFactorOwner(fs, e)
		}
		entry.result[i] = value
	}
	// The cache is filled before the local source factors destructor runs.
	entry.complete = true
	if e := c.finishFactorOwner(fs, nil); e != nil {
		return e
	}
	return nil
}

func (c *PolynomialCache) finishFactorOwner(fs *PolynomialFactors, body error) error {
	if body != nil {
		if e := c.cleanupFactorInvocation(); e != nil {
			c.pendingFactors = fs
			c.factorLocalSourceOwed = fs.constructed
			return errors.Join(body, e)
		}
	}
	// A completed constructor grants the local its source destructor even
	// when the body fails. A refused constructor grants no class destructor.
	// CloseChecked alone is owning abort and omits reset/member DT.
	if fs.constructed {
		if e := fs.SourceFinalize(); e != nil {
			c.pendingFactors = fs
			return errors.Join(body, e)
		}
	}
	if e := fs.CloseChecked(); e != nil {
		c.pendingFactors = fs
		return errors.Join(body, e)
	}
	return body
}

// CleanupFactor completes a temporary factor owner after an interrupted call.
func (c *PolynomialCache) CleanupFactor() error {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	if c.executing {
		return ErrPolynomialPending
	}
	if c.closed {
		return arena.ErrClosed
	}
	c.beginExecution()
	defer c.endExecution()
	return c.cleanupFactor()
}
func (c *PolynomialCache) cleanupFactor() error {
	if e := c.manager.cleanupLexSortInvocation(&c.hashLex); e != nil {
		return e
	}
	if c.pendingFactors == nil {
		return nil
	}
	if e := c.cleanupFactorInvocation(); e != nil {
		return e
	}
	// Callee abort may have stopped before the constructed cache local got
	// its first source-destructor attempt. Consume that exact obligation once;
	// a stopped source destructor subsequently receives only owning abort.
	if c.factorLocalSourceOwed {
		c.factorLocalSourceOwed = false
		if e := c.pendingFactors.SourceFinalize(); e != nil {
			return e
		}
	}
	if e := c.pendingFactors.CloseChecked(); e != nil {
		return e
	}
	c.pendingFactors = nil
	return nil
}

func (c *PolynomialCache) finishPSC(body error) error {
	if e := c.cleanupPSCInvocation(); e != nil {
		c.pendingPSC = true
		return errors.Join(body, e)
	}
	return body
}

// CleanupPSC completes the manager's source PSC scope after an interrupted
// cache miss. The caller still owns its output reference vector.
func (c *PolynomialCache) CleanupPSC() error {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	if c.executing {
		return ErrPolynomialPending
	}
	if c.closed {
		return arena.ErrClosed
	}
	c.beginExecution()
	defer c.endExecution()
	return c.cleanupPSC()
}
func (c *PolynomialCache) cleanupPSC() error {
	if e := c.manager.cleanupLexSortInvocation(&c.hashLex); e != nil {
		return e
	}
	if !c.pendingPSC {
		return nil
	}
	if e := c.cleanupPSCInvocation(); e != nil {
		return e
	}
	c.pendingPSC = false
	return nil
}

// PSCChain memoizes the source ordered pair and variable after canonicalizing
// both inputs. A pre-publication failure leaves the source empty cache entry.
// Published arrays with unwritten elements remain unavailable for safe replay.
func (c *PolynomialCache) PSCChain(p, q Polynomial, x uint32, out *PolynomialRefVector) error {
	if e := c.ready(); e != nil {
		return e
	}
	c.beginExecution()
	defer c.endExecution()
	var e error
	p, e = c.mkUnique(p)
	if e != nil {
		return e
	}
	q, e = c.mkUnique(q)
	if e != nil {
		return e
	}
	key := polynomialPSCKey{p: p, q: q, x: x}
	if entry, ok := c.psc[key]; ok {
		// Preserve the source's size0 hit before result-array publication.
		if !entry.complete && len(entry.result) != 0 {
			return ErrPolynomialPending
		}
		if out == nil || out.owner != c.manager.core {
			return ErrPolynomialOwner
		}
		if e := out.Reset(); e != nil {
			return e
		}
		for _, value := range entry.result {
			if e := out.Push(value); e != nil {
				return e
			}
		}
		return nil
	}
	entry := &polynomialPSCEntry{}
	c.psc[key] = entry
	c.pscCall = polynomialPSCInvocationReceipt{}
	if e := c.manager.pscChainWithReceipt(p, q, x, out, &c.pscCall); e != nil {
		return c.finishPSC(e)
	}
	n, e := out.Size()
	if e != nil {
		return c.finishPSC(e)
	}
	entry.result = make([]Polynomial, n)
	for i := uint32(0); i < n; i++ {
		value, e := out.Get(i)
		if e != nil {
			return c.finishPSC(e)
		}
		value, e = c.mkUnique(value)
		if e != nil {
			return c.finishPSC(e)
		}
		if e := out.setWithReceipt(i, value, nil); e != nil {
			return c.finishPSC(e)
		}
		entry.result[i] = value
	}
	entry.complete = true
	return nil
}

func (c *PolynomialCache) ready() error {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	if c.executing {
		return ErrPolynomialPending
	}
	if c.closed || c.closing {
		return arena.ErrClosed
	}
	if c.resetting {
		return ErrPolynomialPending
	}
	if c.manager.matchesLexSortInvocation(&c.hashLex) {
		return ErrPolynomialPending
	}
	if c.pendingFactors != nil {
		return ErrPolynomialPending
	}
	if c.pendingPSC {
		return ErrPolynomialPending
	}
	return c.manager.ready()
}

// MkUnique returns the first equal representative inserted since Reset. The
// returned handle is borrowed; the cache's reference keeps it alive.
func (c *PolynomialCache) MkUnique(p Polynomial) (Polynomial, error) {
	if e := c.ready(); e != nil {
		return Polynomial{}, e
	}
	c.beginExecution()
	defer c.endExecution()
	return c.mkUnique(p)
}

// CleanupUnique acknowledges only a lexical sort actually accepted by this
// cache's canonicalization call. It does not reset canonical representatives or
// clear another caller's independent pending lexical sort.
func (c *PolynomialCache) CleanupUnique() error {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	if c.executing {
		return ErrPolynomialPending
	}
	if c.closed {
		return arena.ErrClosed
	}
	c.beginExecution()
	defer c.endExecution()
	return c.manager.cleanupLexSortInvocation(&c.hashLex)
}
func (c *PolynomialCache) mkUnique(p Polynomial) (Polynomial, error) {
	// cpp143/146: pid reaches only polynomial::id, not the reference
	// count or term count exposed by the whole-header Info adaptation.
	if e := c.manager.core.check(); e != nil {
		return Polynomial{}, e
	}
	flags, e := c.manager.core.readWord(p, 4)
	if e != nil {
		return Polynomial{}, e
	}
	id := flags & 0x7fffffff
	if c.ids[id] {
		return p, nil
	}
	h, e := c.manager.hashWithLexReceipt(p, &c.hashLex)
	if e != nil {
		return Polynomial{}, e
	}
	for _, old := range c.buckets[h] {
		equal, e := c.manager.core.Equal(p, old)
		if e != nil {
			return Polynomial{}, e
		}
		if equal {
			return old, nil
		}
	}
	// cpp145-148 publishes insert_if_not_there before the owning reference
	// push. A stopped INC leaves that table entry but no cached ID/reference.
	c.buckets[h] = append(c.buckets[h], p)
	if e := c.refs.push(c.manager.core, p); e != nil {
		return Polynomial{}, e
	}
	c.ids[id] = true
	return p, nil
}

// Reset releases cached representatives and permits new first representatives.
// CleanupReset completes a stopped owning release before further cache use.
func (c *PolynomialCache) Reset() error {
	if e := c.ready(); e != nil {
		return e
	}
	c.beginExecution()
	defer c.endExecution()
	clear(c.psc)
	clear(c.factors)
	if e := c.refs.reset(c.manager.core); e != nil {
		c.resetting = true
		return e
	}
	clear(c.buckets)
	clear(c.ids)
	c.releaseImp(232)
	if e := c.newImp(233); e != nil {
		c.resetting = true
		return e
	}
	return nil
}

func (c *PolynomialCache) CleanupReset() error {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	if c.executing {
		return ErrPolynomialPending
	}
	if c.closed {
		return arena.ErrClosed
	}
	c.beginExecution()
	defer c.endExecution()
	if !c.resetting {
		return nil
	}
	if e := c.refs.resetMode(c.manager.core, true); e != nil {
		return e
	}
	clear(c.buckets)
	clear(c.ids)
	clear(c.psc)
	clear(c.factors)
	c.releaseImp(232)
	if c.imp == nil {
		if e := c.newImp(233); e != nil {
			return e
		}
	}
	c.resetting = false
	return nil
}

func (c *PolynomialCache) Close() error {
	if c == nil || c.self != c {
		return ErrPolynomialOwner
	}
	if c.executing {
		return ErrPolynomialPending
	}
	if c.closed {
		return nil
	}
	c.beginExecution()
	defer c.endExecution()
	c.closing = true
	if e := c.cleanupPSC(); e != nil {
		return e
	}
	if e := c.cleanupFactor(); e != nil {
		return e
	}
	if e := c.refs.close(c.manager.core); e != nil {
		return e
	}
	c.releaseImp(211)
	c.manager.core.integer.Budget().Release(c.header)
	c.header = 0
	c.buckets = nil
	c.ids = nil
	c.psc = nil
	c.factors = nil
	c.closed = true
	return nil
}

// A refused cache call conveys no authority over the manager's public root.
func (c *PolynomialCache) cleanupFactorInvocation() error {
	if e := c.manager.cleanupFactorCoreInvocation(&c.factorCall, false); e != nil {
		return e
	}
	return c.manager.cleanupFactorCoreInvocation(&c.factorCall, true)
}

// Only the accepted PSC frame and its concrete child receipts grant cleanup
// authority. Generation-wide compatibility cleanup also visits independent
// Lazard/next scopes and must not be used by this lexical caller.
func (c *PolynomialCache) cleanupPSCInvocation() error {
	core := c.manager.core
	if e := core.cleanupPSCInvocation(&c.pscCall, false); e != nil {
		return e
	}
	return core.cleanupPSCInvocation(&c.pscCall, true)
}

// Manager borrows the source cache::m() polynomial manager.
func (c *PolynomialCache) Manager() (*MultivariatePolynomialManager, error) {
	if e := c.ready(); e != nil {
		return nil, e
	}
	return c.manager, nil
}

// PM is the source cache::pm() alias.
func (c *PolynomialCache) PM() (*MultivariatePolynomialManager, error) { return c.Manager() }

// Bind only synchronous accepted calls, not a registry of idle caches. Nested
// distinct caches restore the enclosing borrow before returning to its callback.
func (c *PolynomialCache) beginExecution() {
	core := c.manager.core
	c.previous = core.cacheCaller
	c.executing = true
	core.cacheCaller = c
}
func (c *PolynomialCache) endExecution() {
	core := c.manager.core
	if core.cacheCaller != c {
		panic(ErrPolynomialOwner)
	}
	core.cacheCaller = c.previous
	c.previous = nil
	c.executing = false
	c.syncPendingBorrow()
}
func (m *MultivariatePolynomialCore) cacheExecuting() bool {
	if m == nil || m.self != m {
		return false
	}
	for c := m.cacheCaller; c != nil; c = c.previous {
		if c.self == c && c.manager != nil && c.manager.self == c.manager && c.manager.core == m && c.executing {
			return true
		}
	}
	return false
}

func (c *PolynomialCache) hasPendingWork() bool {
	return !c.closed && (c.manager.matchesLexSortInvocation(&c.hashLex) || c.pendingFactors != nil || c.pendingPSC || c.resetting || c.closing)
}
func (c *PolynomialCache) currentPendingCause() error {
	if e := c.manager.lexSortInvocationSourceCause(&c.hashLex); e != nil {
		return e
	}
	if c.pendingFactors != nil {
		if c.pendingFactors.sourceCause != nil {
			return c.pendingFactors.sourceCause
		}
		if c.pendingFactors.cleanupCause != nil {
			return c.pendingFactors.cleanupCause
		}
	}
	if c.pendingFactors != nil && c.manager.matchesFactorCore(&c.factorCall) {
		if e := c.factorCall.frame.sourceCause; e != nil {
			return e
		}
		if e := c.factorCall.frame.cleanupCause; e != nil {
			return e
		}
	}
	if c.pendingPSC && c.manager.core.matchesPSCInvocation(&c.pscCall) {
		if e := c.manager.core.pscInvocationSourceCause(&c.pscCall); e != nil {
			return e
		}
		if e := c.pscCall.frame.cleanupCause; e != nil {
			return e
		}
	}
	if c.resetting || c.closing {
		return c.refs.cleanupCause
	}
	return nil
}

// Only actually stopped cache invocations enter this list. No idle-cache scan
// or manager-global callee state supplies ownership or cleanup authority.
func (c *PolynomialCache) syncPendingBorrow() {
	core := c.manager.core
	if c.hasPendingWork() {
		if !c.pendingBound {
			c.pendingCause = c.currentPendingCause()
			c.nextPending = core.pendingCaches
			core.pendingCaches = c
			c.pendingBound = true
		}
		return
	}
	if !c.pendingBound {
		return
	}
	link := &core.pendingCaches
	for *link != nil {
		if *link == c {
			*link = c.nextPending
			c.nextPending = nil
			c.pendingBound = false
			return
		}
		link = &(*link).nextPending
	}
	panic(ErrPolynomialOwner)
}
func (m *MultivariatePolynomialCore) pendingCacheCause() error {
	if m == nil || m.self != m {
		return nil
	}
	for c := m.pendingCaches; c != nil; c = c.nextPending {
		if c.self != c || c.manager == nil || c.manager.self != c.manager || c.manager.core != m || !c.pendingBound || !c.hasPendingWork() {
			return ErrPolynomialOwner
		}
		if c.pendingCause != nil {
			return c.pendingCause
		}
		return ErrPolynomialPending
	}
	return nil
}
