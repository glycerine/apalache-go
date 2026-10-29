package portfolio

// Source: src/tactic/portfolio/bounded_int2bv_solver.cpp:141-323.

import (
	"errors"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	unumeral "github.com/glycerine/g3/pkg/util/numeral"
)

type boundedInt2BVCall struct {
	owner                     *BoundedInt2BVSolver
	proof, first, second      ast.ASTRef
	refs                      []ast.ASTRef
	sub                       *rewriter.ExprSafeReplace
	extension                 *tactic.ExtensionModelConverter
	filter                    *tactic.FilterModelConverter
	md                        tactic.ModelRef
	raw, output               *model.ModelCore
	rawCleanup, outputCleanup bool
	numerals                  []*unumeral.OwnedRational
}

func (c *boundedInt2BVCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := c.owner.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, ref)
	return h, nil
}
func (c *boundedInt2BVCall) close() error {
	m := c.owner.manager
	for _, q := range c.numerals {
		if q != nil {
			if err := c.retireNumber(q); err != nil {
				return err
			}
		}
	}
	if c.filter != nil {
		if err := c.filter.Close(); err != nil {
			return err
		}
		c.filter = nil
	}
	if c.extension != nil {
		if err := c.extension.Close(); err != nil {
			return err
		}
		c.extension = nil
	}
	if err := c.md.Close(); err != nil {
		return err
	}
	if err := enum2BVReleaseModel(&c.raw, &c.rawCleanup); err != nil {
		return err
	}
	if err := enum2BVReleaseModel(&c.output, &c.outputCleanup); err != nil {
		return err
	}
	for _, ref := range []*ast.ASTRef{&c.second, &c.first, &c.proof} {
		if *ref != (ast.ASTRef{}) {
			if err := m.DelRef(ref); err != nil {
				return err
			}
		}
	}
	for i := len(c.refs) - 1; i >= 0; i-- {
		if err := m.DelRef(&c.refs[i]); err != nil {
			return err
		}
	}
	if c.sub != nil {
		if err := c.sub.Close(); err != nil {
			return err
		}
		c.sub = nil
	}
	return nil
}
func (s *BoundedInt2BVSolver) finishCall(c *boundedInt2BVCall, err *error) {
	if cleanup := c.close(); cleanup != nil {
		s.pending = c
		*err = errors.Join(*err, cleanup)
	}
}
func (s *BoundedInt2BVSolver) appendAssertion(h ast.ASTHandle) error {
	ref, err := s.manager.NewRef(h)
	if err != nil {
		return err
	}
	s.assertions = append(s.assertions, ref)
	return nil
}
func (s *BoundedInt2BVSolver) releaseDecls(decls *[]ast.ASTHandle, limit uint32) error {
	for i := int(limit); i < len(*decls); i++ {
		if (*decls)[i].IsNull() {
			continue
		}
		consumed, err := s.manager.DecRefConsumed((*decls)[i])
		if consumed {
			(*decls)[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
		if !consumed {
			return ast.ErrASTPending
		}
	}
	*decls = (*decls)[:limit]
	return nil
}
func (s *BoundedInt2BVSolver) clearConsequences() error {
	for i := range s.consequences {
		if err := s.manager.DelRef(&s.consequences[i]); err != nil {
			return err
		}
	}
	s.consequences = nil
	return nil
}

// The source starts at one bit and then doubles bound while bound<=k.
// This intentionally uses one more bit than the minimum positive width.
func boundedInt2BVNumBits(k *big.Rat) (uint32, error) {
	if k == nil || k.Sign() < 0 || !k.IsInt() {
		return 0, ast.ErrASTDomain
	}
	return uint32(k.Num().BitLen()) + 1, nil
}
func (c *boundedInt2BVCall) definition(decl ast.ASTHandle) (ast.ASTHandle, error) {
	s := c.owner
	m := s.manager
	h, err := c.hold(m.MkApp(decl, nil))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	h, err = c.hold(m.MkBitVectorToInt(h))
	if err != nil {
		return ast.ASTHandle{}, err
	}
	offset := s.offsets[decl]
	if offset.Sign() != 0 {
		n, err := c.hold(m.ArithmeticNumeralRat(&offset, true))
		if err != nil {
			return ast.ASTHandle{}, err
		}
		h, err = c.hold(m.MkArithmeticAdd(h, n))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return h, nil
}
func (s *BoundedInt2BVSolver) accumulateSub(c *boundedInt2BVCall) error {
	m := s.manager
	for _, bm := range s.bounds {
		vars, err := bm.Variables()
		if err != nil {
			return err
		}
		for _, e := range vars {
			lo, strictLo, hasLo, err := bm.Lower(e)
			if err != nil {
				return err
			}
			hi, strictHi, hasHi, err := bm.Upper(e)
			if err != nil {
				return err
			}
			if !hasLo || !hasHi || strictLo || strictHi || lo.Cmp(&hi) > 0 {
				continue
			}
			f, yes, err := enum2BVConstant(m, e)
			if err != nil {
				return err
			}
			if !yes {
				return ast.ErrASTDomain
			}
			bv, found := s.int2bv[f]
			if !found {
				var n big.Rat
				n.Sub(&hi, &lo)
				n.Add(&n, big.NewRat(1, 1))
				bits, err := boundedInt2BVNumBits(&n)
				if err != nil {
					return err
				}
				sort, err := m.BitVectorSort(bits)
				if err != nil {
					return err
				}
				b, err := c.hold(m.MkFreshConst("b", sort))
				if err != nil {
					return err
				}
				bv, err = m.GetDecl(b)
				if err != nil {
					return err
				}
				s.int2bv[f] = bv
				s.bv2int[bv] = f
				var copy big.Rat
				copy.Set(&lo)
				s.offsets[bv] = copy
				if err = m.IncRef(bv); err != nil {
					return err
				}
				s.bvDecls = append(s.bvDecls, bv)
				if err = m.IncRef(f); err != nil {
					return err
				}
				s.intDecls = append(s.intDecls, f)
				// Source only adds the range cap when the offset is nonzero.
				power := new(big.Int).Sub(n.Num(), big.NewInt(1))
				power.And(power, n.Num())
				if lo.Sign() != 0 && power.Sign() != 0 {
					var max big.Rat
					max.Sub(&n, big.NewRat(1, 1))
					q, err := c.numeral()
					if err != nil {
						return err
					}
					err = m.NumeralService().SetString(q, max.RatString())
					var cap ast.ASTHandle
					if err == nil {
						cap, err = m.BitVectorNumeral(q, bits)
					}
					if err == nil {
						cap, err = c.hold(cap, nil)
					}
					err = errors.Join(err, c.retireNumber(q))
					if err != nil {
						return err
					}
					guard, err := c.hold(m.MkBitVectorULE(b, cap))
					if err != nil {
						return err
					}
					if err = s.appendAssertion(guard); err != nil {
						return err
					}
				}
			}
			t, err := c.definition(bv)
			if err != nil {
				return err
			}
			if err = c.sub.Insert(e, t); err != nil {
				return err
			}
		}
	}
	return nil
}
func (s *BoundedInt2BVSolver) flushAssertions() (err error) {
	if len(s.assertions) == 0 {
		return nil
	}
	c := &boundedInt2BVCall{owner: s}
	defer s.finishCall(c, &err)
	bm := s.bounds[len(s.bounds)-1]
	for _, ref := range s.assertions {
		h, e := s.manager.RefGet(ref)
		if e != nil {
			return e
		}
		if e = bm.Collect(h, ast.ASTDependency{}); e != nil {
			return e
		}
	}
	c.sub, err = rewriter.NewExprSafeReplace(s.manager)
	if err != nil {
		return err
	}
	if err = s.accumulateSub(c); err != nil {
		return err
	}
	c.proof, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.first, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.second, err = s.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	empty, err := c.sub.Empty()
	if err != nil {
		return err
	}
	for _, ref := range s.assertions {
		h, e := s.manager.RefGet(ref)
		if e != nil {
			return e
		}
		if !empty {
			if e = c.sub.Apply(h, c.first); e != nil {
				return e
			}
			h, e = s.manager.RefGet(c.first)
			if e != nil {
				return e
			}
			if e = s.rw.RewriteWithProof(h, c.second, c.proof); e != nil {
				return e
			}
			canceled, e := s.manager.Canceled()
			if e != nil {
				return e
			}
			if canceled {
				return s.rw.Reset()
			}
			h, e = s.manager.RefGet(c.second)
			if e != nil {
				return e
			}
		}
		if e = s.backend.Assert(h); e != nil {
			return e
		}
	}
	if err = s.clearAssertions(); err != nil {
		return err
	}
	return s.rw.Reset()
}
func (s *BoundedInt2BVSolver) AcquireModel() (_ *model.ModelCore, err error) {
	if err = s.open(); err != nil {
		return nil, err
	}
	c := &boundedInt2BVCall{owner: s}
	fail := func(cause error) (*model.ModelCore, error) { s.finishCall(c, &cause); return nil, cause }
	c.raw, err = s.backend.AcquireModel()
	if err != nil {
		return fail(err)
	}
	if c.raw == nil {
		return nil, nil
	}
	if err = c.md.Set(c.raw); err != nil {
		return fail(err)
	}
	if err = enum2BVReleaseModel(&c.raw, &c.rawCleanup); err != nil {
		return fail(err)
	}
	c.extension, err = tactic.NewExtensionModel(s.manager, s.services)
	if err != nil {
		return fail(err)
	}
	for f, bv := range s.int2bv {
		value, e := c.definition(bv)
		if e != nil {
			return fail(e)
		}
		if err = c.extension.Insert(f, value); err != nil {
			return fail(err)
		}
	}
	if err = c.extension.ApplyModel(&c.md, 0); err != nil {
		return fail(err)
	}
	if err = c.extension.Close(); err != nil {
		return fail(err)
	}
	c.extension = nil
	if len(s.bvDecls) > 0 {
		c.filter, err = tactic.NewFilterModel(s.manager)
		if err != nil {
			return fail(err)
		}
		for _, bv := range s.bvDecls {
			if err = c.filter.Insert(bv); err != nil {
				return fail(err)
			}
		}
		if err = c.filter.ApplyModel(&c.md, 0); err != nil {
			return fail(err)
		}
	}
	c.output, err = c.md.Get()
	if err != nil {
		return fail(err)
	}
	if c.output != nil {
		if err = c.output.IncRef(); err != nil {
			c.output = nil
			return fail(err)
		}
	}
	result := c.output
	c.output = nil
	if err = c.close(); err != nil {
		c.output = result
		s.pending = c
		return nil, err
	}
	return result, nil
}
func (s *BoundedInt2BVSolver) consequencesCore(a, v []ast.ASTHandle) (status lbool.LBool, out []ast.ASTHandle, err error) {
	if err = s.flushAssertions(); err != nil {
		return lbool.Undef, nil, err
	}
	c := &boundedInt2BVCall{owner: s}
	defer s.finishCall(c, &err)
	vars := make([]ast.ASTHandle, 0, len(v))
	for _, h := range v {
		d, yes, e := enum2BVConstant(s.manager, h)
		if e != nil {
			return lbool.Undef, nil, e
		}
		if bv, ok := s.int2bv[d]; yes && ok {
			h, e = c.hold(s.manager.MkApp(bv, nil))
			if e != nil {
				return lbool.Undef, nil, e
			}
		} else {
			h, e = c.hold(h, nil)
			if e != nil {
				return lbool.Undef, nil, e
			}
		}
		vars = append(vars, h)
	}
	status, result, err := s.backend.GetConsequences(a, vars)
	for _, h := range result {
		ref, e := s.manager.NewRef(h)
		if e != nil {
			return status, nil, errors.Join(err, e)
		}
		s.consequences = append(s.consequences, ref)
	}
	if err != nil {
		return status, result, err
	}
	for i, h := range result {
		next, e := s.projectConsequence(c, h)
		if e != nil {
			return status, result, e
		}
		if e = s.manager.SetRef(s.consequences[i], next); e != nil {
			return status, result, e
		}
		out = append(out, next)
	}
	return status, out, nil
}
func (s *BoundedInt2BVSolver) projectConsequence(c *boundedInt2BVCall, h ast.ASTHandle) (ast.ASTHandle, error) {
	m := s.manager
	yes, err := enum2BVOperation(m, h, ast.ASTOpImplies)
	if err != nil || !yes {
		return h, err
	}
	premise, err := m.GetArg(h, 0)
	if err != nil {
		return h, err
	}
	head, err := m.GetArg(h, 1)
	if err != nil {
		return h, err
	}
	yes, err = enum2BVOperation(m, head, ast.ASTOpEq)
	if err != nil || !yes {
		return h, err
	}
	u, err := m.GetArg(head, 0)
	if err != nil {
		return h, err
	}
	v, err := m.GetArg(head, 1)
	if err != nil {
		return h, err
	}
	bv, yes, err := enum2BVConstant(m, u)
	if err != nil || !yes {
		return h, err
	}
	f, found := s.bv2int[bv]
	if !found {
		return h, nil
	}
	q, err := c.numeral()
	if err != nil {
		return h, err
	}
	var width uint32
	yes, err = m.BitVectorNumeralValue(v, q, &width)
	var text string
	if err == nil && yes {
		text, err = m.NumeralService().String(q)
	}
	err = errors.Join(err, c.retireNumber(q))
	if err != nil || !yes {
		return h, err
	}
	n, ok := new(big.Rat).SetString(text)
	if !ok {
		return h, ast.ErrASTDomain
	}
	offset := s.offsets[bv]
	n.Add(n, &offset)
	original, err := c.hold(m.MkApp(f, nil))
	if err != nil {
		return h, err
	}
	value, err := c.hold(m.ArithmeticNumeralRat(n, true))
	if err != nil {
		return h, err
	}
	eq, err := c.hold(m.MkEq(original, value))
	if err != nil {
		return h, err
	}
	return c.hold(m.MkImplies(premise, eq))
}

func (c *boundedInt2BVCall) numeral() (*unumeral.OwnedRational, error) {
	q, err := c.owner.manager.NumeralService().NewValue()
	if err != nil {
		return nil, err
	}
	c.numerals = append(c.numerals, &q)
	return &q, nil
}
func (c *boundedInt2BVCall) retireNumber(q *unumeral.OwnedRational) error {
	if err := c.owner.manager.NumeralService().Del(q); err != nil {
		return err
	}
	for i, v := range c.numerals {
		if v == q {
			c.numerals[i] = nil
			break
		}
	}
	return nil
}
