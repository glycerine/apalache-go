package arith

// Source: src/tactic/arith/bv2int_rewriter.cpp:24-93;
// src/tactic/arith/bv2int_rewriter.h:29-51.

import (
	"math/big"
	"math/bits"
	"reflect"
	"sort"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BV2IntContext owns side conditions and the trail of generated exponents.
// Power2 keys are borrowed from the input goals, as in the source obj_map;
// keep those expressions alive while using the map. Do not copy this owner.
type BV2IntContext struct {
	self                       *BV2IntContext
	manager                    *ast.ASTManager
	context                    *uparams.ParamsContext
	trace                      *diagnostic.TraceTags
	maxSize                    uint32
	side, trail                []ast.ASTHandle
	power2                     map[ast.ASTHandle]ast.ASTHandle
	header                     uint64
	resetting, closing, closed bool
}

// NewBV2IntContext reads max_bv_size once. The source retains no params_ref.
func NewBV2IntContext(m *ast.ASTManager, c *uparams.ParamsContext, p *uparams.ParamsRef) (*BV2IntContext, error) {
	if m == nil || c == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	key := "max_bv_size"
	max, err := c.GetUIntName(p, &key, ^uint32(0))
	if err != nil {
		return nil, err
	}
	if _, err = m.RegisterArithmeticPlugin(); err != nil {
		return nil, err
	}
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(BV2IntContext{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &BV2IntContext{manager: m, context: c, maxSize: max, header: n, power2: make(map[ast.ASTHandle]ast.ASTHandle)}
	r.self = r
	return r, nil
}
func (r *BV2IntContext) owner() error {
	if r == nil || r.self != r || r.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (r *BV2IntContext) open() error {
	if err := r.owner(); err != nil {
		return err
	}
	if r.resetting || r.closing {
		return ast.ErrASTPending
	}
	return nil
}
func (r *BV2IntContext) MaxNumBits() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.maxSize, nil
}
func (r *BV2IntContext) AddSideCondition(e ast.ASTHandle) error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.manager.IncRef(e); err != nil {
		return err
	}
	r.side = append(r.side, e)
	return nil
}

// SideConditions returns a detached slice of borrowed expression handles.
func (r *BV2IntContext) SideConditions() ([]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	return append([]ast.ASTHandle(nil), r.side...), nil
}
func (r *BV2IntContext) IsPower2(e ast.ASTHandle) (ast.ASTHandle, bool, error) {
	if err := r.open(); err != nil {
		return ast.ASTHandle{}, false, err
	}
	h, ok := r.power2[e]
	return h, ok, nil
}

// Power2 returns a detached map of borrowed expressions for the retained NLA
// substitution consumer. Iteration order is unspecified, as in obj_map.
func (r *BV2IntContext) Power2() (map[ast.ASTHandle]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	result := make(map[ast.ASTHandle]ast.ASTHandle, len(r.power2))
	for k, v := range r.power2 {
		result[k] = v
	}
	return result, nil
}

func (r *BV2IntContext) basic(e ast.ASTHandle, op int32) (bool, error) {
	k, err := r.manager.GetKind(e)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := r.manager.GetFamilyID(e)
	if err != nil || f != r.manager.BasicFamilyID() {
		return false, err
	}
	kind, err := r.manager.GetDeclKind(e)
	return kind == op, err
}

// CollectPower2 recognizes exactly a disjunction of equalities to consecutive
// powers 1,2,4,..., in either equality orientation and any disjunct order.
// Repeated calls replace map entries but retain every old exponent in the trail.
func (r *BV2IntContext) CollectPower2(g *tactic.Goal) error {
	if err := r.open(); err != nil {
		return err
	}
	if g == nil {
		return ast.ErrASTHandle
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	for j := uint32(0); j < count; j++ {
		f, err := g.Form(j)
		if err != nil {
			return err
		}
		or, err := r.basic(f, ast.ASTOpOr)
		if err != nil {
			return err
		}
		if !or {
			continue
		}
		n, err := r.manager.GetNumArgs(f)
		if err != nil {
			return err
		}
		var v ast.ASTHandle
		bounds := make([]big.Rat, 0, n)
		ok := true
		for i := uint32(0); i < n; i++ {
			e, err := r.manager.GetArg(f, i)
			if err != nil {
				return err
			}
			eq, err := r.basic(e, ast.ASTOpEq)
			if err != nil {
				return err
			}
			if !eq {
				ok = false
				break
			}
			x, err := r.manager.GetArg(e, 0)
			if err != nil {
				return err
			}
			y, err := r.manager.GetArg(e, 1)
			if err != nil {
				return err
			}
			text, integer, found, err := r.manager.ArithmeticNumeralText(y)
			if err != nil {
				return err
			}
			if !(found && integer && (v.IsNull() || x == v)) {
				text, integer, found, err = r.manager.ArithmeticNumeralText(x)
				if err != nil {
					return err
				}
				if !(found && integer && (v.IsNull() || y == v)) {
					ok = false
					break
				}
				x = y
			}
			var value big.Rat
			if _, valid := value.SetString(text); !valid {
				return ast.ErrASTDomain
			}
			v = x
			bounds = append(bounds, value)
		}
		if !ok || v.IsNull() {
			continue
		}
		if err = bv2IntSourceAssert(68, "!bounds.empty()", len(bounds) > 0); err != nil {
			return err
		}
		sort.Slice(bounds, func(i, j int) bool { return bounds[i].Cmp(&bounds[j]) < 0 })
		var power big.Rat
		power.SetInt64(1)
		two := big.NewRat(2, 1)
		for i := range bounds {
			if power.Cmp(&bounds[i]) != 0 {
				ok = false
				break
			}
			power.Mul(&power, two)
		}
		if !ok {
			continue
		}
		width := uint32(bits.Len32(uint32(len(bounds))))
		if width == 0 {
			continue
		}
		s, err := r.manager.BitVectorSort(width)
		if err != nil {
			return err
		}
		logx, err := r.manager.MkFreshConst("log2_v", s)
		if err != nil {
			return err
		}
		// As in the source, construct zero_extend even when its amount is zero.
		logx, err = r.manager.MkBitVectorZeroExtend(uint32(len(bounds))-width, logx)
		if err != nil {
			return err
		}
		if err = r.manager.IncRef(logx); err != nil {
			return err
		}
		r.trail = append(r.trail, logx)
		if err = r.tracePower(v, logx); err != nil {
			return err
		}
		r.power2[v] = logx
	}
	return nil
}

func (r *BV2IntContext) release(values *[]ast.ASTHandle) error {
	for i, h := range *values {
		if h.IsNull() {
			continue
		}
		consumed, err := r.manager.DecRefConsumed(h)
		if consumed {
			(*values)[i] = ast.ASTHandle{}
		}
		if err != nil {
			return err
		}
	}
	*values = nil
	return nil
}

// Reset releases side conditions, then generated exponents, then the borrowed
// map. Consumed decrements are remembered so a stopped cleanup can be retried.
func (r *BV2IntContext) Reset() error {
	if err := r.owner(); err != nil {
		return err
	}
	if r.closing {
		return ast.ErrASTPending
	}
	r.resetting = true
	if err := r.release(&r.side); err != nil {
		return err
	}
	if err := r.release(&r.trail); err != nil {
		return err
	}
	clear(r.power2)
	r.resetting = false
	return nil
}
func (r *BV2IntContext) Close() error {
	if r != nil && r.self == r && r.closed {
		return nil
	}
	if err := r.owner(); err != nil {
		return err
	}
	r.closing = true
	// Implicit destruction reverses member order, unlike explicit reset.
	if err := r.release(&r.trail); err != nil {
		return err
	}
	clear(r.power2)
	if err := r.release(&r.side); err != nil {
		return err
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
