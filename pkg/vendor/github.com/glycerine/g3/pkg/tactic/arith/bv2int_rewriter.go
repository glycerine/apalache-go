package arith

// Source: src/tactic/arith/bv2int_rewriter.cpp:95-631;
// src/tactic/arith/bv2int_rewriter.h:53-118.

import (
	"errors"
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type bv2IntConfiguration struct {
	rewriter.DefaultConfiguration
	owner *BV2IntRewriter
}

func (*bv2IntConfiguration) RewritePatterns() bool                 { return false }
func (*bv2IntConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (c *bv2IntConfiguration) ReduceApp(d ast.ASTHandle, a ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	if err := c.owner.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return rewriter.RewriteFailed, err
	}
	return c.owner.MkAppCore(d, a, out)
}

// BV2IntRewriter borrows its shared context. Close it before the context.
// Traversal always runs without proofs, as in bv2int_rewriter_star.
type BV2IntRewriter struct {
	self            *BV2IntRewriter
	manager         *ast.ASTManager
	context         *BV2IntContext
	arith, bv       ast.FamilyID
	cfg             bv2IntConfiguration
	traversal       *rewriter.Traversal
	pending         []*bv2IntCall
	header          uint64
	closing, closed bool
}

// NewBV2IntRewriter returns a partial owner if traversal construction fails.
// Close every nonnil returned owner, including when err is nonnil.
func NewBV2IntRewriter(ctx *BV2IntContext) (*BV2IntRewriter, error) {
	if err := ctx.open(); err != nil {
		return nil, err
	}
	m := ctx.manager
	a, err := m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	b, err := m.RegisterBitVectorPlugin()
	if err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(BV2IntRewriter{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &BV2IntRewriter{manager: m, context: ctx, arith: a, bv: b, header: n}
	r.self = r
	r.cfg.owner = r
	r.traversal, err = rewriter.NewTraversal(m, &r.cfg)
	return r, err
}
func (r *BV2IntRewriter) open() error {
	if r == nil || r.self != r || r.closed {
		return ast.ErrASTHandle
	}
	if r.closing || len(r.pending) != 0 || r.traversal == nil {
		return ast.ErrASTPending
	}
	return r.context.open()
}
func (r *BV2IntRewriter) Rewrite(root ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(root, out)
}

// RewriteWithProof preserves the caller's proof slot: the source star
// explicitly instantiates rewriter_tpl with proof generation disabled.
func (r *BV2IntRewriter) RewriteWithProof(root ast.ASTHandle, out, _ ast.ASTRef) error {
	return r.Rewrite(root, out)
}
func (r *BV2IntRewriter) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Reset()
}
func (r *BV2IntRewriter) Close() error {
	if r != nil && r.self == r && r.closed {
		return nil
	}
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	r.closing = true
	for len(r.pending) > 0 {
		c := r.pending[0]
		if err := c.close(); err != nil {
			return err
		}
		r.pending = r.pending[1:]
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}

// Concrete per-reduction owners retain generated ASTs through result handoff.
// A refused cleanup stays attached to the rewriter for Close to finish.
type bv2IntCall struct {
	r         *BV2IntRewriter
	refs      []ast.ASTRef
	numerals  []*unumeral.OwnedRational
	err       error
	traceLine int
}

func (c *bv2IntCall) close() error {
	for i := len(c.numerals) - 1; i >= 0; i-- {
		if c.numerals[i] != nil {
			if err := c.r.manager.NumeralService().Del(c.numerals[i]); err != nil {
				return err
			}
			c.numerals[i] = nil
		}
	}
	for i := len(c.refs) - 1; i >= 0; i-- {
		if err := c.r.manager.DelRef(&c.refs[i]); err != nil {
			return err
		}
	}
	return nil
}
func (c *bv2IntCall) hold(h ast.ASTHandle, err error) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	ref, err := c.r.manager.NewRef(h)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	c.refs = append(c.refs, ref)
	return h
}
func (c *bv2IntCall) app(f ast.FamilyID, k int32, args ...ast.ASTHandle) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.r.manager.MkFamilyApp(f, k, nil, args))
}
func (c *bv2IntCall) ai(k int32, a ...ast.ASTHandle) ast.ASTHandle { return c.app(c.r.arith, k, a...) }
func (c *bv2IntCall) bi(k int32, a ...ast.ASTHandle) ast.ASTHandle { return c.app(c.r.bv, k, a...) }
func (c *bv2IntCall) basic(k int32, a ...ast.ASTHandle) ast.ASTHandle {
	return c.app(c.r.manager.BasicFamilyID(), k, a...)
}
func (c *bv2IntCall) arg(h ast.ASTHandle, i uint32) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	v, err := c.r.manager.GetArg(h, i)
	c.err = err
	return v
}
func (c *bv2IntCall) is(h ast.ASTHandle, f ast.FamilyID, k int32, n uint32) bool {
	if c.err != nil {
		return false
	}
	kind, err := c.r.manager.GetKind(h)
	if err != nil {
		c.err = err
		return false
	}
	if kind != ast.ASTApp {
		return false
	}
	family, err := c.r.manager.GetFamilyID(h)
	if err != nil {
		c.err = err
		return false
	}
	if family != f {
		return false
	}
	op, err := c.r.manager.GetDeclKind(h)
	if err != nil {
		c.err = err
		return false
	}
	if op != k {
		return false
	}
	count, err := c.r.manager.GetNumArgs(h)
	c.err = err
	return count == n && err == nil
}
func (c *bv2IntCall) width(h ast.ASTHandle) uint32 {
	if c.err != nil {
		return 0
	}
	n, err := c.r.manager.BitVectorSize(h)
	c.err = err
	return n
}
func (c *bv2IntCall) numeral() *unumeral.OwnedRational {
	if c.err != nil {
		return nil
	}
	q, err := c.r.manager.NumeralService().NewValue()
	if err != nil {
		c.err = err
		return nil
	}
	c.numerals = append(c.numerals, &q)
	return &q
}
func (c *bv2IntCall) number(h ast.ASTHandle) (*big.Rat, bool) {
	if c.err != nil {
		return nil, false
	}
	text, integer, ok, err := c.r.manager.ArithmeticNumeralText(h)
	if err != nil {
		c.err = err
		return nil, false
	}
	if !ok || !integer {
		return nil, false
	}
	q, ok := new(big.Rat).SetString(text)
	if !ok {
		c.err = ast.ErrASTDomain
	}
	return q, ok
}
func (c *bv2IntCall) bvNumber(h ast.ASTHandle) (*big.Rat, bool) {
	q := c.numeral()
	if q == nil {
		return nil, false
	}
	var width uint32
	ok, err := c.r.manager.BitVectorNumeralValue(h, q, &width)
	if err != nil {
		c.err = err
		return nil, false
	}
	if !ok {
		return nil, false
	}
	text, err := c.r.manager.NumeralService().String(q)
	if err != nil {
		c.err = err
		return nil, false
	}
	v, ok := new(big.Rat).SetString(text)
	if !ok {
		c.err = ast.ErrASTDomain
	}
	return v, ok
}
func (c *bv2IntCall) bn(q *big.Rat, n uint32) ast.ASTHandle {
	v := c.numeral()
	if v == nil {
		return ast.ASTHandle{}
	}
	if err := c.r.manager.NumeralService().SetString(v, q.RatString()); err != nil {
		c.err = err
		return ast.ASTHandle{}
	}
	return c.hold(c.r.manager.BitVectorNumeral(v, n))
}
func (c *bv2IntCall) small(v int64, n uint32) ast.ASTHandle { return c.bn(big.NewRat(v, 1), n) }
func (c *bv2IntCall) integer(q *big.Rat) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.r.manager.ArithmeticNumeralRat(q, true))
}
func (c *bv2IntCall) toInt(h ast.ASTHandle) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.r.manager.MkBitVectorToInt(h))
}
func (c *bv2IntCall) extend(n uint32, h ast.ASTHandle, signed bool) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if n == 0 {
		return h
	}
	if signed {
		return c.hold(c.r.manager.MkBitVectorSignExtend(n, h))
	}
	if q, ok := c.bvNumber(h); ok {
		return c.bn(q, c.width(h)+n)
	}
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.r.manager.MkBitVectorZeroExtend(n, h))
}
func (c *bv2IntCall) align(s, t ast.ASTHandle, signed bool) (ast.ASTHandle, ast.ASTHandle) {
	a, b := c.width(s), c.width(t)
	if a > b {
		t = c.extend(a-b, t, signed)
	}
	if a < b {
		s = c.extend(b-a, s, signed)
	}
	return s, t
}
func (c *bv2IntCall) unsigned(h ast.ASTHandle) (ast.ASTHandle, bool) {
	if c.is(h, c.r.bv, 58, 1) {
		return c.arg(h, 0), true
	}
	if q, ok := c.number(h); ok && q.Sign() >= 0 {
		n := uint32(q.Num().BitLen())
		if n == 0 {
			n = 1
		}
		return c.bn(q, n), true
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) difference(h ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, bool) {
	if s, ok := c.unsigned(h); ok {
		return s, c.small(0, 1), true
	}
	if q, ok := c.number(h); ok {
		c.err = bv2IntSourceAssert(515, "k.is_neg()", q.Sign() < 0)
		if c.err != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, false
		}
		q.Neg(q)
		return c.small(0, 1), c.bn(q, uint32(q.Num().BitLen())), true
	}
	if c.is(h, c.r.arith, 7, 2) {
		s, a := c.unsigned(c.arg(h, 0))
		if a {
			t, b := c.unsigned(c.arg(h, 1))
			if b {
				return s, t, true
			}
		}
	}
	if c.is(h, c.r.arith, 6, 2) {
		a, b := c.arg(h, 0), c.arg(h, 1)
		if q, ok := c.number(a); ok && q.Sign() < 0 {
			if s, ok := c.unsigned(b); ok {
				q.Neg(q)
				return s, c.bn(q, uint32(q.Num().BitLen())), true
			}
		}
		if q, ok := c.number(b); ok && q.Sign() < 0 {
			if s, ok := c.unsigned(a); ok {
				q.Neg(q)
				return s, c.bn(q, uint32(q.Num().BitLen())), true
			}
		}
	}
	return ast.ASTHandle{}, ast.ASTHandle{}, false
}
func (c *bv2IntCall) shl1(h ast.ASTHandle) (ast.ASTHandle, bool) {
	if !c.is(h, c.r.bv, 58, 1) {
		return ast.ASTHandle{}, false
	}
	b := c.arg(h, 0)
	if !c.is(b, c.r.bv, 45, 2) {
		return ast.ASTHandle{}, false
	}
	if q, ok := c.bvNumber(c.arg(b, 0)); ok && q.Cmp(big.NewRat(1, 1)) == 0 {
		return c.arg(b, 1), true
	}
	return ast.ASTHandle{}, false
}
func (c *bv2IntCall) extract(h ast.ASTHandle) (lo, hi uint32, base ast.ASTHandle, ok bool) {
	if c.err != nil {
		return
	}
	var err error
	ok, err = c.r.manager.BitVectorExtractValue(h, &lo, &hi, &base)
	c.err = err
	return
}
func (c *bv2IntCall) signed(h ast.ASTHandle) (ast.ASTHandle, bool) {
	if s, ok := c.unsigned(h); ok {
		if c.err != nil {
			return ast.ASTHandle{}, false
		}
		return c.hold(c.r.manager.MkBitVectorZeroExtend(1, s)), true
	}
	if a, b, ok := c.difference(h); ok {
		a, b = c.align(a, b, false)
		return c.bi(5, c.extend(1, a, false), c.extend(1, b, false)), true
	}
	if !c.is(h, c.r.manager.BasicFamilyID(), ast.ASTOpIte, 3) {
		return ast.ASTHandle{}, false
	}
	cond, then, other := c.arg(h, 0), c.arg(h, 1), c.arg(h, 2)
	if !c.is(cond, c.r.manager.BasicFamilyID(), ast.ASTOpEq, 2) {
		return ast.ASTHandle{}, false
	}
	one, sign := c.arg(cond, 0), c.arg(cond, 1)
	if q, ok := c.bvNumber(one); !ok || q.Cmp(big.NewRat(1, 1)) != 0 || c.width(one) != 1 {
		return ast.ASTHandle{}, false
	}
	lo, hi, base, ok := c.extract(sign)
	if !ok || lo != hi || lo != c.width(base)-1 || !c.is(then, c.r.arith, 7, 2) || other != c.arg(then, 0) || !c.is(other, c.r.bv, 58, 1) {
		return ast.ASTHandle{}, false
	}
	low, high, original, ok := c.extract(c.arg(other, 0))
	if !ok || low != 0 || high != hi-1 {
		return ast.ASTHandle{}, false
	}
	q, ok := c.number(c.arg(then, 1))
	power := new(big.Int).Lsh(big.NewInt(1), uint(hi))
	if !ok || q.Cmp(new(big.Rat).SetInt(power)) != 0 {
		return ast.ASTHandle{}, false
	}
	return original, true
}
func (c *bv2IntCall) extractApp(hi, lo uint32, h ast.ASTHandle) ast.ASTHandle {
	if c.err != nil {
		return ast.ASTHandle{}
	}
	return c.hold(c.r.manager.MkBitVectorExtract(hi, lo, h))
}
func (c *bv2IntCall) signedInt(h ast.ASTHandle) ast.ASTHandle {
	n := c.width(h)
	if c.err != nil {
		return ast.ASTHandle{}
	}
	if n < 2 {
		c.err = ast.ErrASTDomain
		return ast.ASTHandle{}
	}
	cond := c.basic(ast.ASTOpEq, c.small(1, 1), c.extractApp(n-1, n-1, h))
	e := c.toInt(c.extractApp(n-2, 0, h))
	power := new(big.Int).Lsh(big.NewInt(1), uint(n-1))
	t := c.ai(7, e, c.integer(new(big.Rat).SetInt(power)))
	return c.basic(ast.ASTOpIte, cond, t, e)
}
func (c *bv2IntCall) addBV(s, t ast.ASTHandle, signed bool) ast.ASTHandle {
	c.assertBV(s, 321, "m_bv.is_bv(s)")
	c.assertBV(t, 322, "m_bv.is_bv(t)")
	if q, ok := c.bvNumber(s); ok && q.Sign() == 0 {
		return t
	}
	if q, ok := c.bvNumber(t); ok && q.Sign() == 0 {
		return s
	}
	s, t = c.align(s, t, signed)
	return c.bi(4, c.extend(1, s, signed), c.extend(1, t, signed))
}
func (c *bv2IntCall) mulBV(s, t ast.ASTHandle, signed bool) ast.ASTHandle {
	c.assertBV(s, 372, "m_bv.is_bv(s)")
	c.assertBV(t, 373, "m_bv.is_bv(t)")
	if q, ok := c.bvNumber(s); ok && q.Sign() == 0 {
		return s
	}
	if q, ok := c.bvNumber(t); ok && q.Sign() == 0 {
		return t
	}
	if q, ok := c.bvNumber(s); ok && q.Cmp(big.NewRat(1, 1)) == 0 {
		return t
	}
	if q, ok := c.bvNumber(t); ok && q.Cmp(big.NewRat(1, 1)) == 0 {
		return s
	}
	s, t = c.align(s, t, signed)
	n := c.width(t)
	max := c.r.context.maxSize
	overflow := 2*n > max
	if n < max {
		extra := n
		if 2*n > max {
			extra = max - n
		}
		s, t = c.extend(extra, s, signed), c.extend(extra, t, signed)
	}
	if overflow && c.err == nil {
		var cond ast.ASTHandle
		if signed {
			cond = c.hold(c.r.manager.MkBitVectorSignedMulNoOverflow(s, t))
		} else {
			cond = c.hold(c.r.manager.MkBitVectorUnsignedMulNoOverflow(s, t))
		}
		if c.err == nil {
			c.err = c.r.context.AddSideCondition(cond)
		}
		if signed && c.err == nil {
			cond = c.hold(c.r.manager.MkBitVectorSignedMulNoUnderflow(s, t))
			if c.err == nil {
				c.err = c.r.context.AddSideCondition(cond)
			}
		}
	}
	return c.bi(6, s, t)
}

func (r *BV2IntRewriter) MkAppCore(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return rewriter.RewriteFailed, err
	}
	c := &bv2IntCall{r: r}
	defer func() {
		cleanup := c.close()
		if cleanup != nil {
			r.pending = append(r.pending, c)
		}
		err = errors.Join(err, c.err, cleanup)
	}()
	family, err := r.manager.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	kind, err := r.manager.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	a := make([]ast.ASTHandle, args.Count())
	for i := range a {
		a[i], err = args.At(uint32(i))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	h, status := c.reduce(family, kind, a)
	if c.err == nil && !h.IsNull() {
		err = r.manager.SetRef(out, h)
		if err == nil && c.traceLine != 0 {
			err = r.traceMod(h, c.traceLine)
		}
	}
	return status, err
}
func (r *BV2IntRewriter) MkApp(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) error {
	status, err := r.MkAppCore(decl, args, out)
	if err != nil || status != rewriter.RewriteFailed {
		return err
	}
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		values[i], err = args.At(uint32(i))
		if err != nil {
			return err
		}
	}
	h, err := r.manager.MkApp(decl, values)
	if err != nil {
		return err
	}
	return r.manager.SetRef(out, h)
}
