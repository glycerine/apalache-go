package arith

// Source: src/tactic/arith/bv2real_rewriter.cpp:355-695;
// src/tactic/arith/bv2real_rewriter.h:144-231.

import (
	"math/big"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
)

type bv2RealConfiguration struct {
	rewriter.DefaultConfiguration
	owner *BV2RealRewriter
}

func (*bv2RealConfiguration) RewritePatterns() bool                 { return false }
func (*bv2RealConfiguration) FlatAssoc(ast.ASTHandle) (bool, error) { return false, nil }
func (c *bv2RealConfiguration) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	if err := c.owner.utility.manager.SetRef(proof, ast.ASTHandle{}); err != nil {
		return rewriter.RewriteFailed, err
	}
	return c.owner.MkAppCore(d, args, out)
}

// BV2RealRewriter owns one of the two concrete source traversals. The utility
// is borrowed and must outlive the rewriter. Both traversals disable proofs.
type BV2RealRewriter struct {
	self      *BV2RealRewriter
	utility   *BV2RealUtil
	cfg       bv2RealConfiguration
	traversal *rewriter.Traversal
	eliminate bool
	header    uint64
	closed    bool
}

func NewBV2RealRewriter(u *BV2RealUtil) (*BV2RealRewriter, error) {
	return newBV2RealRewriter(u, false)
}
func NewBV2RealElimRewriter(u *BV2RealUtil) (*BV2RealRewriter, error) {
	return newBV2RealRewriter(u, true)
}
func newBV2RealRewriter(u *BV2RealUtil, eliminate bool) (*BV2RealRewriter, error) {
	if err := u.open(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(BV2RealRewriter{}).Size())
	if err := u.manager.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	r := &BV2RealRewriter{utility: u, eliminate: eliminate, header: bytes}
	r.self = r
	r.cfg.owner = r
	var err error
	r.traversal, err = rewriter.NewTraversal(u.manager, &r.cfg)
	return r, err
}
func (r *BV2RealRewriter) open() error {
	if r == nil || r.self != r || r.closed {
		return ast.ErrASTHandle
	}
	if r.traversal == nil {
		return ast.ErrASTPending
	}
	return r.utility.open()
}
func (r *BV2RealRewriter) Close() error {
	if r != nil && r.self == r && r.closed {
		return nil
	}
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.traversal != nil {
		if err := r.traversal.Close(); err != nil {
			return err
		}
		r.traversal = nil
	}
	r.utility.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
func (r *BV2RealRewriter) Rewrite(h ast.ASTHandle, out ast.ASTRef) error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Rewrite(h, out)
}
func (r *BV2RealRewriter) RewriteWithProof(h ast.ASTHandle, out, _ ast.ASTRef) error {
	return r.Rewrite(h, out)
}
func (r *BV2RealRewriter) Reset() error {
	if err := r.open(); err != nil {
		return err
	}
	return r.traversal.Reset()
}
func (r *BV2RealRewriter) MkAppCore(decl ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	if err = r.open(); err != nil {
		return rewriter.RewriteFailed, err
	}
	u := r.utility
	c := &bv2RealCall{u: u}
	defer u.finish(c, &err)
	values := make([]ast.ASTHandle, args.Count())
	for i := range values {
		values[i], err = args.At(uint32(i))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	var h ast.ASTHandle
	if !r.eliminate {
		if err = u.traceApp(decl, values); err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	if r.eliminate {
		if sig, ok := u.reverse[decl]; ok {
			if err = bv2RealSourceAssert(55, "num_args == 2", len(values) == 2); err != nil {
				return rewriter.RewriteFailed, err
			}
			if len(values) != 2 {
				return rewriter.RewriteFailed, ast.ErrASTDomain
			}
			if err = c.checkSignature(values[0], values[1], &sig); err != nil {
				return rewriter.RewriteFailed, err
			}
			h = c.reduced(values[0], values[1], &sig.divisor, &sig.root)
			status = rewriter.RewriteFull
		} else {
			status = rewriter.RewriteFailed
		}
	} else {
		h, status = c.reduce(decl, values)
	}
	if c.err != nil {
		return status, c.err
	}
	if !h.IsNull() {
		err = u.manager.SetRef(out, h)
		if err == nil && c.traceLine != 0 {
			err = u.traceLE(c.traceLine, "\n")
		}
	}
	return status, err
}
func (c *bv2RealCall) pair(s, t ast.ASTHandle) (s1, s2, t1, t2 ast.ASTHandle, d1, d2, r big.Rat, ok bool) {
	s1, s2, d1, r, ok = c.match(s)
	if !ok {
		return
	}
	var r2 big.Rat
	t1, t2, d2, r2, ok = c.match(t)
	ok = ok && r.Cmp(&r2) == 0
	return
}
func (c *bv2RealCall) wrap(a, b ast.ASTHandle, d, r *big.Rat) (ast.ASTHandle, bool) {
	a, b, ok := c.alignDivisor(a, b, d)
	if !ok {
		return ast.ASTHandle{}, false
	}
	return c.encoding(a, b, d, r), true
}
func (c *bv2RealCall) binary(op int32, s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	a, b, x, y, d1, d2, r, ok := c.pair(s, t)
	if !ok {
		return ast.ASTHandle{}, false
	}
	if op == 9 {
		first := c.add(c.mul(a, x), c.multiplyScalar(&r, c.mul(y, b)))
		second := c.add(c.mul(a, y), c.mul(b, x))
		d1.Mul(&d1, &d2)
		return c.wrap(first, second, &d1, &r)
	}
	a, b, x, y = c.alignDivisors(a, b, x, y, &d1, &d2)
	switch op {
	case 6:
		return c.wrap(c.add(a, x), c.add(y, b), &d1, &r)
	case 7:
		return c.wrap(c.sub(a, x), c.sub(b, y), &d1, &r)
	}
	return ast.ASTHandle{}, false
}
func (c *bv2RealCall) equal(s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	a, b, x, y, d1, d2, _, ok := c.pair(s, t)
	if !ok {
		return ast.ASTHandle{}, false
	}
	a, b, x, y = c.alignDivisors(a, b, x, y, &d1, &d2)
	a, x = c.align(a, x)
	b, y = c.align(b, y)
	return c.basic(ast.ASTOpAnd, c.basic(ast.ASTOpEq, a, x), c.basic(ast.ASTOpEq, b, y)), true
}
func (c *bv2RealCall) ite(cond, s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	a, b, x, y, d1, d2, r, ok := c.pair(s, t)
	if !ok {
		return ast.ASTHandle{}, false
	}
	a, b, x, y = c.alignDivisors(a, b, x, y, &d1, &d2)
	a, x = c.align(a, x)
	b, y = c.align(b, y)
	return c.wrap(c.basic(ast.ASTOpIte, cond, a, x), c.basic(ast.ASTOpIte, cond, b, y), &d1, &r)
}
func (c *bv2RealCall) negate(s ast.ASTHandle) (ast.ASTHandle, bool) {
	a, b, d, r, ok := c.match(s)
	if !ok {
		return ast.ASTHandle{}, false
	}
	a = c.extend(1, a)
	b = c.extend(1, b)
	return c.wrap(c.bv(3, a), c.bv(3, b), &d, &r)
}
func (c *bv2RealCall) approxLE(s, t ast.ASTHandle, positive, negative bool) (ast.ASTHandle, bool) {
	if c.err == nil {
		c.err = bv2RealSourceAssert(408, "is_pos || is_neg", positive || negative)
	}
	if c.err != nil {
		return ast.ASTHandle{}, false
	}
	a, b, x, y, d1, d2, r, ok := c.pair(s, t)
	if !ok || r.Cmp(big.NewRat(2, 1)) != 0 {
		return ast.ASTHandle{}, false
	}
	a, b, x, y = c.alignDivisors(a, b, x, y, &d1, &d2)
	a = c.sub(a, x)
	b = c.sub(b, y)
	width := c.width(b)
	if c.err != nil {
		return ast.ASTHandle{}, false
	}
	boolean, err := c.u.manager.MkBoolSort()
	if err != nil {
		c.err = err
		return ast.ASTHandle{}, false
	}
	le := c.hold(c.u.manager.MkFreshConst("le_proxy", boolean))
	if c.err != nil {
		return ast.ASTHandle{}, false
	}
	decl, err := c.u.manager.GetDecl(le)
	if err != nil {
		c.err = err
		return ast.ASTHandle{}, false
	}
	if err = c.u.AddAuxDecl(decl); err != nil {
		c.err = err
		return ast.ASTHandle{}, false
	}
	gt := c.basic(ast.ASTOpNot, le)
	nonpos := c.bv(23, b, c.numeral(big.NewRat(0, 1), width))
	under := c.add(c.multiplyScalar(big.NewRat(4, 1), a), c.multiplyScalar(big.NewRat(5, 1), b))
	z1 := c.numeral(big.NewRat(0, 1), c.width(under))
	leUnder := c.bv(23, under, z1)
	over := c.add(c.multiplyScalar(big.NewRat(2, 1), a), c.multiplyScalar(big.NewRat(3, 1), b))
	z2 := c.numeral(big.NewRat(0, 1), c.width(over))
	leOver := c.bv(23, over, z2)
	if positive {
		c.side(c.basic(ast.ASTOpImplies, c.basic(ast.ASTOpAnd, le, nonpos), leUnder))
		c.side(c.basic(ast.ASTOpImplies, c.basic(ast.ASTOpAnd, le, c.basic(ast.ASTOpNot, nonpos)), leOver))
	}
	if negative {
		c.side(c.basic(ast.ASTOpImplies, c.basic(ast.ASTOpAnd, gt, c.basic(ast.ASTOpNot, nonpos)), c.basic(ast.ASTOpNot, leUnder)))
		c.side(c.basic(ast.ASTOpImplies, c.basic(ast.ASTOpAnd, gt, nonpos), c.basic(ast.ASTOpNot, leOver)))
	}
	if c.err == nil {
		c.err = c.u.traceLE(464, "mk_le\n")
	}
	if positive {
		return le, true
	}
	return gt, true
}
func (c *bv2RealCall) le(s, t ast.ASTHandle) (ast.ASTHandle, bool) {
	if h, ok := c.approxLE(s, t, true, true); ok {
		return h, true
	}
	a, b, x, y, d1, d2, r, ok := c.pair(s, t)
	if !ok {
		return ast.ASTHandle{}, false
	}
	a, b, x, y = c.alignDivisors(a, b, x, y, &d1, &d2)
	x = c.sub(x, a)
	y = c.sub(y, b)
	z1, z2 := c.numeral(big.NewRat(0, 1), c.width(x)), c.numeral(big.NewRat(0, 1), c.width(y))
	gz1, lz1 := c.bv(23, z1, x), c.bv(23, x, z1)
	gz2, lz2 := c.bv(23, z2, y), c.bv(23, y, z2)
	xx := c.mul(x, x)
	yy := c.multiplyScalar(&r, c.mul(y, y))
	xx, yy = c.align(xx, yy)
	ge, le := c.bv(23, yy, xx), c.bv(23, xx, yy)
	e1 := c.basic(ast.ASTOpOr, gz1, gz2)
	e2 := c.basic(ast.ASTOpOr, c.basic(ast.ASTOpNot, gz1), c.basic(ast.ASTOpNot, lz2), ge)
	e3 := c.basic(ast.ASTOpOr, c.basic(ast.ASTOpNot, gz2), c.basic(ast.ASTOpNot, lz1), le)
	c.traceLine = 541
	return c.basic(ast.ASTOpAnd, e1, e2, e3), true
}
func (c *bv2RealCall) reduce(decl ast.ASTHandle, a []ast.ASTHandle) (h ast.ASTHandle, status rewriter.RewriteStatus) {
	status = rewriter.RewriteFailed
	m := c.u.manager
	family, err := m.GetFamilyID(decl)
	if err != nil {
		c.err = err
		return
	}
	kind, err := m.GetDeclKind(decl)
	if err != nil {
		c.err = err
		return
	}
	if c.err = c.assertArity(family, kind, len(a)); c.err != nil {
		return
	}
	ok := false
	if family == c.u.arith {
		switch kind {
		case 2, 3, 4, 5:
			if len(a) != 2 {
				c.err = ast.ErrASTDomain
				return
			}
			switch kind {
			case 2:
				h, ok = c.le(a[0], a[1])
			case 3:
				h, ok = c.le(a[1], a[0])
			case 4:
				return c.basic(ast.ASTOpNot, c.arith(2, a[1], a[0])), rewriter.Rewrite2
			case 5:
				return c.basic(ast.ASTOpNot, c.arith(2, a[0], a[1])), rewriter.Rewrite2
			}
		case 6, 7, 9:
			if len(a) == 0 {
				c.err = ast.ErrASTDomain
				return
			}
			h = a[0]
			ok = true
			for i := 1; i < len(a) && ok; i++ {
				var next ast.ASTHandle
				next, ok = c.binary(kind, h, a[i])
				if ok {
					h = next
				}
			}
		case 8:
			if len(a) != 1 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.negate(a[0])
		}
	} else if family == m.BasicFamilyID() {
		switch kind {
		case ast.ASTOpEq:
			if len(a) != 2 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.equal(a[0], a[1])
		case ast.ASTOpIte:
			if len(a) != 3 {
				c.err = ast.ErrASTDomain
				return
			}
			h, ok = c.ite(a[0], a[1], a[2])
		}
	} else {
		lt, err := m.RefGet(c.u.posLT)
		if err != nil {
			c.err = err
			return
		}
		le, err := m.RefGet(c.u.posLE)
		if err != nil {
			c.err = err
			return
		}
		if decl == lt || decl == le {
			line := int32(398)
			if decl == lt {
				line = 394
			}
			if c.err = bv2RealSourceAssert(line, "num_args == 2", len(a) == 2); c.err != nil {
				return
			}
			if len(a) != 2 {
				c.err = ast.ErrASTDomain
				return
			}
			if decl == lt {
				h, ok = c.approxLE(a[1], a[0], false, true)
			} else {
				h, ok = c.approxLE(a[0], a[1], true, false)
			}
		}
	}
	if ok {
		status = rewriter.RewriteDone
	}
	return
}
