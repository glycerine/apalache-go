package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:25-350;
// src/tactic/arith/bv2real_rewriter.h:33-141.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type bv2RealKey struct {
	first, second uint32
	divisor, root string
}
type bv2RealSignature struct {
	first, second uint32
	divisor, root *unumeral.OwnedRational
}

// BV2RealUtil owns encoding declarations and side conditions for the retained
// NLA-to-BV consumer. Its defaults come from that consumer's uint parameters.
// Reset clears side conditions; it deliberately preserves declaration caches.
type BV2RealUtil struct {
	cacheOwners                []*bv2RealCacheOwner
	cacheRetiring              bool
	self                       *BV2RealUtil
	manager                    *ast.ASTManager
	arith, bv                  ast.FamilyID
	defaults                   [3]unumeral.OwnedRational
	posLT, posLE               ast.ASTRef
	decls, side                []ast.ASTHandle
	declBytes, sideBytes       uint64
	vectorPending              []ast.ASTHandle
	forward                    map[bv2RealKey]ast.ASTHandle
	reverse                    map[ast.ASTHandle]bv2RealSignature
	pending                    []*bv2RealCall
	trace                      *diagnostic.TraceTags
	tracePending               []*bv2RealTrace
	maxBits                    uint32
	header                     uint64
	borrowers                  uint32
	resetting, closing, closed bool
	ready                      bool
}

// A nonnil owner returned with an error must be closed.
func NewBV2RealUtil(m *ast.ASTManager, root, divisor, maxBits uint32) (*BV2RealUtil, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	a, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	b, err := m.RegisterBitVectorPlugin()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(BV2RealUtil{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	u := &BV2RealUtil{manager: m, arith: a, bv: b, maxBits: maxBits, header: bytes, forward: make(map[bv2RealKey]ast.ASTHandle), reverse: make(map[ast.ASTHandle]bv2RealSignature)}
	u.self = u
	if err = u.initializeDefaults(root, divisor); err != nil {
		return u, err
	}
	real, err := m.MkFamilySort(a, 0, nil)
	if err != nil {
		return u, err
	}
	boolean, err := m.MkBoolSort()
	if err != nil {
		return u, err
	}
	empty, err := m.Symbols.Intern("")
	if err != nil {
		return u, err
	}
	for i, name := range []string{"<", "<="} {
		prefix, err := m.Symbols.Intern(name)
		if err != nil {
			return u, err
		}
		decl, err := m.MkFreshFuncDecl(prefix, empty, []ast.ASTHandle{real, real}, boolean)
		if err != nil {
			return u, err
		}
		ref, err := m.NewRef(decl)
		if err != nil {
			return u, err
		}
		if i == 0 {
			u.posLT = ref
		} else {
			u.posLE = ref
		}
	}
	for _, ref := range []ast.ASTRef{u.posLT, u.posLE} {
		h, err := m.RefGet(ref)
		if err != nil {
			return u, err
		}
		if err = u.addAuxDecl(h); err != nil {
			return u, err
		}
	}
	u.ready = true
	return u, nil
}
func (u *BV2RealUtil) owner() error {
	if u == nil || u.self != u || u.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (u *BV2RealUtil) open() error {
	if err := u.owner(); err != nil {
		return err
	}
	if !u.ready || u.closing || u.resetting || u.cacheRetiring || len(u.pending) > 0 || len(u.tracePending) > 0 || len(u.vectorPending) > 0 {
		return ast.ErrASTPending
	}
	return nil
}
func (u *BV2RealUtil) AddAuxDecl(h ast.ASTHandle) error {
	if err := u.open(); err != nil {
		return err
	}
	return u.addAuxDecl(h)
}
func (u *BV2RealUtil) addAuxDecl(h ast.ASTHandle) error {
	return u.appendVectorRef(&u.decls, &u.declBytes, h)
}
func (u *BV2RealUtil) AddSideCondition(h ast.ASTHandle) error {
	if err := u.open(); err != nil {
		return err
	}
	return u.appendVectorRef(&u.side, &u.sideBytes, h)
}
func (u *BV2RealUtil) SideConditions() ([]ast.ASTHandle, error) {
	if err := u.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), u.side...), nil
}
func (u *BV2RealUtil) AuxDecls() ([]ast.ASTHandle, error) {
	if err := u.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), u.decls...), nil
}
func (u *BV2RealUtil) Defaults() (root, divisor, maxDivisor big.Rat, err error) {
	if err = u.open(); err != nil {
		return
	}
	for i, out := range []*big.Rat{&root, &divisor, &maxDivisor} {
		text, e := u.manager.NumeralService().String(&u.defaults[i])
		if e != nil {
			err = e
			return
		}
		if _, ok := out.SetString(text); !ok {
			err = ast.ErrASTDomain
			return
		}
	}
	return
}
func (u *BV2RealUtil) MaxNumBits() (uint32, error) {
	if err := u.open(); err != nil {
		return 0, err
	}
	return u.maxBits, nil
}
func (u *BV2RealUtil) PositiveLTDecl() (ast.ASTHandle, error) {
	if err := u.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return u.manager.RefGet(u.posLT)
}
func (u *BV2RealUtil) PositiveLEDecl() (ast.ASTHandle, error) {
	if err := u.open(); err != nil {
		return ast.ASTHandle{}, err
	}
	return u.manager.RefGet(u.posLE)
}
func (u *BV2RealUtil) release(values *[]ast.ASTHandle) error {
	for i, h := range *values {
		if h.IsNull() {
			continue
		}
		consumed, err := u.manager.DecRefConsumed(h)
		if consumed {
			(*values)[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	*values = (*values)[:0]
	return nil
}
func (u *BV2RealUtil) Reset() error {
	if err := u.owner(); err != nil {
		return err
	}
	if u.closing || u.cacheRetiring || len(u.pending) > 0 || len(u.tracePending) > 0 || len(u.vectorPending) > 0 {
		return ast.ErrASTPending
	}
	u.resetting = true
	if err := u.release(&u.side); err != nil {
		return err
	}
	u.resetting = false
	return nil
}
func (u *BV2RealUtil) Close() error {
	if u != nil && u.self == u && u.closed {
		return nil
	}
	if err := u.owner(); err != nil {
		return err
	}
	if u.borrowers != 0 {
		return ast.ErrASTPending
	}
	u.closing = true
	if err := u.release(&u.vectorPending); err != nil {
		return err
	}
	if err := u.cleanupCacheFailures(); err != nil {
		return err
	}
	for len(u.tracePending) > 0 {
		if err := u.tracePending[0].close(); err != nil {
			return err
		}
		u.tracePending = u.tracePending[1:]
	}
	for len(u.pending) > 0 {
		c := u.pending[0]
		if err := c.close(); err != nil {
			return err
		}
		u.pending = u.pending[1:]
	}
	for i := len(u.defaults) - 1; i >= 0; i-- {
		if u.defaults[i] != (unumeral.OwnedRational{}) {
			if err := u.manager.NumeralService().Del(&u.defaults[i]); err != nil {
				return err
			}
		}
	}
	if err := u.closeCacheOwners(); err != nil {
		return err
	}
	clear(u.reverse)
	clear(u.forward)
	if err := u.release(&u.side); err != nil {
		return err
	}
	u.manager.Budget().Release(u.sideBytes)
	u.side, u.sideBytes = nil, 0
	for _, ref := range []*ast.ASTRef{&u.posLT, &u.posLE} {
		if *ref != (ast.ASTRef{}) {
			if err := u.manager.DelRef(ref); err != nil {
				return err
			}
		}
	}
	if err := u.release(&u.decls); err != nil {
		return err
	}
	u.manager.Budget().Release(u.declBytes)
	u.decls, u.declBytes = nil, 0
	u.manager.Budget().Release(u.header)
	u.header = 0
	u.closed = true
	return nil
}

type bv2RealCall struct {
	u              *BV2RealUtil
	refs           []ast.ASTRef
	numbers        []*unumeral.OwnedRational
	err            error
	traceLine      int
	output         ast.ASTRef
	outputAssigned bool
	readPending    []*bv2RealArithmeticRead
}

type bv2RealCallCheckpoint struct {
	refs, numbers int
}

func (c *bv2RealCall) checkpoint() bv2RealCallCheckpoint {
	return bv2RealCallCheckpoint{refs: len(c.refs), numbers: len(c.numbers)}
}

// releaseNumbersTo ends only the rational portion of a lexical scope.
// A stopped release stays attached to the call for owner cleanup.
func (c *bv2RealCall) releaseNumbersTo(numberCount int) error {
	for len(c.numbers) > numberCount {
		index := len(c.numbers) - 1
		if c.numbers[index] != nil {
			if err := c.u.manager.NumeralService().Del(c.numbers[index]); err != nil {
				return err
			}
			c.numbers[index] = nil
		}
		c.numbers = c.numbers[:index]
	}
	return nil
}

// releaseTo ends a complete source expr_ref/rational scope.
func (c *bv2RealCall) releaseTo(mark bv2RealCallCheckpoint) error {
	if err := c.releaseNumbersTo(mark.numbers); err != nil {
		return err
	}
	for len(c.refs) > mark.refs {
		index := len(c.refs) - 1
		if c.refs[index] != (ast.ASTRef{}) {
			if err := c.u.manager.DelRef(&c.refs[index]); err != nil {
				return err
			}
		}
		c.refs = c.refs[:index]
	}
	return nil
}

func (c *bv2RealCall) close() error {
	for len(c.readPending) > 0 {
		if err := c.readPending[0].close(); err != nil {
			return err
		}
		c.readPending = c.readPending[1:]
	}
	return c.releaseTo(bv2RealCallCheckpoint{})
}
func (u *BV2RealUtil) finish(c *bv2RealCall, err *error) {
	if cleanup := c.close(); cleanup != nil {
		u.pending = append(u.pending, c)
		*err = errors.Join(*err, cleanup)
	}
	*err = errors.Join(*err, c.err)
}
func (c *bv2RealCall) hold(h ast.ASTHandle, err error) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	ref, err := c.u.manager.NewRef(h)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	c.refs = append(c.refs, ref)
	return h
}
func (c *bv2RealCall) number() *unumeral.OwnedRational {
	if c.err != nil {
		return nil
	}
	q, err := c.u.manager.NumeralService().NewValue()
	if err != nil {
		c.err = err
		return nil
	}
	c.numbers = append(c.numbers, &q)
	return &q
}
func (c *bv2RealCall) width(h ast.ASTHandle) uint32 {
	if c.err != nil {
		return 0
	}
	w, err := c.u.manager.BitVectorSize(h)
	c.err = err
	return w
}
func (c *bv2RealCall) app(f ast.FamilyID, k int32, args ...ast.ASTHandle) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.u.manager.MkFamilyApp(f, k, nil, args))
}
func (c *bv2RealCall) bv(k int32, args ...ast.ASTHandle) ast.ASTHandle {
	return c.app(c.u.bv, k, args...)
}
func (c *bv2RealCall) arith(k int32, args ...ast.ASTHandle) ast.ASTHandle {
	return c.app(c.u.arith, k, args...)
}
func (c *bv2RealCall) basic(k int32, args ...ast.ASTHandle) ast.ASTHandle {
	return c.app(c.u.manager.BasicFamilyID(), k, args...)
}
func (c *bv2RealCall) bvNumeral(h ast.ASTHandle) (*big.Rat, bool) {
	// is_zero/mk_extend borrow a numeral only for their local recognition.
	// The returned Go value is independent of that project rational owner.
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	q := c.number()
	if q == nil {
		return nil, false
	}
	var width uint32
	yes, err := c.u.manager.BitVectorNumeralValue(h, q, &width)
	if err != nil {
		c.err = err
		return nil, false
	}
	if !yes {
		return nil, false
	}
	text, err := c.u.manager.NumeralService().String(q)
	if err != nil {
		c.err = err
		return nil, false
	}
	value, ok := new(big.Rat).SetString(text)
	if !ok {
		c.err = ast.ErrASTDomain
	}
	return value, ok
}
func (c *bv2RealCall) numeral(q *big.Rat, width uint32) ast.ASTHandle {
	// The AST parameter copies the numeral; its construction argument ends here.
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	n := c.number()
	if n == nil {
		return ast.ASTHandle{}
	}
	if err := c.u.manager.NumeralService().SetString(n, q.RatString()); err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	return c.hold(c.u.manager.BitVectorNumeral(n, width))
}
func (c *bv2RealCall) realNumeral(q *big.Rat) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.u.manager.ArithmeticNumeralRat(q, false))
}
func (c *bv2RealCall) align(a, b ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle) {
	wa, wb := c.width(a), c.width(b)
	if wa > wb {
		next := c.extend(wa-wb, b)
		if c.err == nil {
			b = next
		}
	} else if wa < wb {
		next := c.extend(wb-wa, a)
		if c.err == nil {
			a = next
		}
	}
	return a, b
}
func (c *bv2RealCall) add(a, b ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	c.assertBV(a, 217, "m_bv.is_bv(s)")
	c.assertBV(b, 218, "m_bv.is_bv(t)")
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if c.isZero(a) {
		return b
	}
	if c.isZero(b) {
		return a
	}
	a, b = c.align(a, b)
	return c.bv(4, c.extend(1, a), c.extend(1, b))
}
func (c *bv2RealCall) sub(a, b ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	a, b = c.align(a, b)
	return c.bv(5, c.extend(1, a), c.extend(1, b))
}
func (c *bv2RealCall) side(h ast.ASTHandle) {
	if c.err == nil {
		c.err = c.u.AddSideCondition(h)
	}
}
func (c *bv2RealCall) mul(a, b ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	c.assertBV(a, 179, "m_bv.is_bv(s)")
	c.assertBV(b, 180, "m_bv.is_bv(t)")
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if c.isZero(a) {
		return a
	}
	if c.isZero(b) {
		return b
	}
	a, b = c.align(a, b)
	n := c.width(b)
	max := c.u.maxBits
	overflow := 2*n > max
	if n < max {
		extra := n
		if 2*n > max {
			extra = max - n
		}
		a, b = c.extend(extra, a), c.extend(extra, b)
	}
	if overflow && c.err == nil {
		c.side(c.hold(c.u.manager.MkBitVectorSignedMulNoOverflow(a, b)))
		if c.err == nil {
			c.side(c.hold(c.u.manager.MkBitVectorSignedMulNoUnderflow(a, b)))
		}
	}
	return c.bv(6, a, b)
}
func (c *bv2RealCall) multiplyScalar(q *big.Rat, b ast.ASTHandle) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	if q.Cmp(big.NewRat(1, 1)) == 0 {
		return b
	}
	return c.mul(c.signedNumeral(q), b)
}
func (c *bv2RealCall) extract(high, low uint32, h ast.ASTHandle) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.u.manager.MkBitVectorExtract(high, low, h))
}
func (c *bv2RealCall) encoding(a, b ast.ASTHandle, d, r *big.Rat) ast.ASTHandle {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	divisor, root := c.rational(d), c.rational(r)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.encodingOwned(a, b, divisor, root)
}
func (c *bv2RealCall) encodingOwned(a, b ast.ASTHandle, d, r *unumeral.OwnedRational) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	wa, wb := c.width(a), c.width(b)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	numberCount := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(numberCount)) }()
	sourceD, sourceR := c.number(), c.number()
	if c.err != nil {
		return ast.ASTHandle{}
	}
	c.err = c.u.manager.NumeralService().Copy(sourceD, d)
	if c.err == nil {
		c.err = c.u.manager.NumeralService().Copy(sourceR, r)
	}
	if c.err != nil {
		return ast.ASTHandle{}
	}
	keyD, err := c.u.manager.NumeralService().String(sourceD)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	keyR, err := c.u.manager.NumeralService().String(sourceR)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	key := bv2RealKey{wa, wb, keyD, keyR}
	decl, ok := c.u.forward[key]
	if !ok {
		m := c.u.manager
		s1, err := m.GetSort(a)
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		s2, err := m.GetSort(b)
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		real, err := m.MkFamilySort(c.u.arith, 0, nil)
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		prefix, err := m.Symbols.Intern("bv2real")
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		empty, err := m.Symbols.Intern("")
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		decl, err = m.MkFreshFuncDecl(prefix, empty, []ast.ASTHandle{s1, s2}, real)
		if err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		if err = c.u.AddAuxDecl(decl); err != nil {
			c.err = err
			return ast.ASTHandle{}
		}
		if c.err = c.u.publishEncoding(key, decl, wa, wb, sourceD, sourceR); c.err != nil {
			return ast.ASTHandle{}
		}
	}
	return c.hold(c.u.manager.MkApp(decl, []ast.ASTHandle{a, b}))
}
func (c *bv2RealCall) reduced(a, b ast.ASTHandle, d, r *big.Rat) (result ast.ASTHandle) {
	mark := c.checkpoint()
	defer c.endExpressionScope(mark, &result)
	divisor, root := c.rational(d), c.rational(r)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.reducedOwned(a, b, divisor, root)
}
func (u *BV2RealUtil) IsEncodingDecl(decl ast.ASTHandle) (bool, error) {
	if err := u.open(); err != nil {
		return false, err
	}
	_, ok := u.reverse[decl]
	return ok, nil
}
func (u *BV2RealUtil) MkBV2RealC(a, b ast.ASTHandle, d, r *big.Rat, out ast.ASTRef) (err error) {
	if err = u.open(); err != nil {
		return err
	}
	if d == nil || r == nil {
		return ast.ErrASTDomain
	}
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	h := c.encoding(a, b, d, r)
	if c.err != nil {
		return c.err
	}
	return u.manager.SetRef(out, h)
}
func (u *BV2RealUtil) MkBV2Real(a, b ast.ASTHandle, d, r *big.Rat, out ast.ASTRef) (ok bool, err error) {
	if err = u.open(); err != nil {
		return false, err
	}
	if d == nil || r == nil {
		return false, ast.ErrASTDomain
	}
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	a, b, ok = c.alignDivisor(a, b, d)
	if !ok || c.err != nil {
		return false, c.err
	}
	h := c.encoding(a, b, d, r)
	if c.err != nil {
		return false, c.err
	}
	return true, u.manager.SetRef(out, h)
}
func (u *BV2RealUtil) MkReduced(a, b ast.ASTHandle, d, r *big.Rat, out ast.ASTRef) (err error) {
	if err = u.open(); err != nil {
		return err
	}
	if d == nil || r == nil {
		return ast.ErrASTDomain
	}
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	h := c.reduced(a, b, d, r)
	if c.err != nil {
		return c.err
	}
	return u.manager.SetRef(out, h)
}

func (u *BV2RealUtil) MkDefault(a, b ast.ASTHandle, out ast.ASTRef) (err error) {
	if err = u.open(); err != nil {
		return err
	}
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	h := c.encodingOwned(a, b, &u.defaults[1], &u.defaults[0])
	if c.err != nil {
		return c.err
	}
	return u.manager.SetRef(out, h)
}
func (u *BV2RealUtil) MkReducedDefault(a, b ast.ASTHandle, out ast.ASTRef) (err error) {
	if err = u.open(); err != nil {
		return err
	}
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	h := c.reducedOwned(a, b, &u.defaults[1], &u.defaults[0])
	if c.err != nil {
		return c.err
	}
	return u.manager.SetRef(out, h)
}
func (c *bv2RealCall) match(h ast.ASTHandle) (a, b ast.ASTHandle, d, r big.Rat, found bool) {
	count := len(c.numbers)
	defer func() { c.err = errors.Join(c.err, c.releaseNumbersTo(count)) }()
	term, found := c.matchTerm(h)
	if !found || c.err != nil {
		return
	}
	divisor, root := c.rationalValue(term.divisor), c.rationalValue(term.root)
	if c.err != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, d, r, false
	}
	d.Set(divisor)
	r.Set(root)
	return term.first, term.second, d, r, true
}
func (u *BV2RealUtil) Match(h ast.ASTHandle, aout, bout ast.ASTRef, d, r *big.Rat) (found bool, err error) {
	if err = u.open(); err != nil {
		return false, err
	}
	if d == nil || r == nil {
		return false, ast.ErrASTDomain
	}
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	a, b, divisor, root, found := c.match(h)
	if !found || c.err != nil {
		return false, c.err
	}
	if err = u.manager.SetRef(aout, a); err != nil {
		return false, err
	}
	if err = u.manager.SetRef(bout, b); err != nil {
		return false, err
	}
	d.Set(&divisor)
	r.Set(&root)
	return true, nil
}

// ContainsBV2Real follows the concrete source postorder traversal, including
// quantifier bodies, patterns and no-patterns. Only shared expressions are marked.
func (u *BV2RealUtil) ContainsBV2Real(root ast.ASTHandle) (bool, error) {
	if err := u.open(); err != nil {
		return false, err
	}
	type frame struct {
		h    ast.ASTHandle
		next uint32
	}
	m := u.manager
	stack := []frame{{h: root}}
	marked := map[ast.ASTHandle]bool{}
	mark := func(h ast.ASTHandle) (bool, error) {
		refs, err := m.GetRefCount(h)
		if err != nil {
			return false, err
		}
		if refs <= 1 {
			return false, nil
		}
		if marked[h] {
			return true, nil
		}
		marked[h] = true
		return false, nil
	}
	if _, err := mark(root); err != nil {
		return false, err
	}
	for len(stack) > 0 {
		i := len(stack) - 1
		f := &stack[i]
		kind, err := m.GetKind(f.h)
		if err != nil {
			return false, err
		}
		var children uint32
		switch kind {
		case ast.ASTApp:
			children, err = m.GetNumArgs(f.h)
		case ast.ASTQuantifier:
			var patterns, noPatterns uint32
			patterns, err = m.GetNumPatterns(f.h)
			if err == nil {
				noPatterns, err = m.GetNumNoPatterns(f.h)
			}
			children = 1 + patterns + noPatterns
		case ast.ASTVar:
		default:
			return false, ast.ErrASTDomain
		}
		if err != nil {
			return false, err
		}
		if f.next < children {
			index := f.next
			f.next++
			var child ast.ASTHandle
			if kind == ast.ASTApp {
				child, err = m.GetArg(f.h, index)
			} else if index == 0 {
				child, err = m.GetQuantifierBody(f.h)
			} else {
				var patterns uint32
				patterns, err = m.GetNumPatterns(f.h)
				if err == nil {
					if index <= patterns {
						child, err = m.GetPattern(f.h, index-1)
					} else {
						child, err = m.GetNoPattern(f.h, index-1-patterns)
					}
				}
			}
			if err != nil {
				return false, err
			}
			seen, err := mark(child)
			if err != nil {
				return false, err
			}
			if !seen {
				stack = append(stack, frame{h: child})
			}
			continue
		}
		if kind == ast.ASTApp {
			decl, err := m.GetDecl(f.h)
			if err != nil {
				return false, err
			}
			if _, ok := u.reverse[decl]; ok {
				return true, nil
			}
		}
		stack = stack[:i]
	}
	return false, nil
}
