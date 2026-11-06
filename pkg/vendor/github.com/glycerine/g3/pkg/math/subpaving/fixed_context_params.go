package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:416-435,476-505,
// concrete config_mpfx.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// NewFixedContextFromParams runs the source constructor's named update pass.
// A stopped pass returns its partial owner for explicit Close.
func NewFixedContextFromParams(m *numeral.FixedPointManager, limit *control.Reslimit, pc *params.ParamsContext, p *params.ParamsRef) (*FixedContext, error) {
	c, err := newFixedContextOwner(m, limit, nil)
	if err != nil {
		return nil, err
	}
	if err := c.UpdateParamsRef(pc, p); err != nil {
		return c, err
	}
	if err := c.newRoot(); err != nil {
		return c, err
	}
	return c, nil
}

// UpdateParamsRef applies each source mutation before reading the next name.
// Numeric values therefore retain their completed prefix on a later refusal.
func (c *FixedContext) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if c == nil || c.self != c || c.numbers == nil {
		return arena.ErrInvalidSlot
	}
	if c.closed {
		return arena.ErrClosed
	}
	if pc == nil || p == nil {
		return arena.ErrInvalidSlot
	}
	if err := pc.Check(p); err != nil {
		return err
	}
	m := c.numbers
	epsilon := "epsilon"
	v, err := pc.GetUIntName(p, &epsilon, 20)
	if err != nil {
		return err
	}
	if v != 0 {
		if err := m.SetInt64(c.epsilon, int64(int32(v))); err != nil {
			return err
		}
		if err := m.Inv(c.epsilon); err != nil {
			return err
		}
		c.zeroEpsilon = false
	} else {
		if err := m.Reset(c.epsilon); err != nil {
			return err
		}
		c.zeroEpsilon = true
	}
	maxBound := "max_bound"
	v, err = pc.GetUIntName(p, &maxBound, 10)
	if err != nil {
		return err
	}
	if err := m.SetInt64(c.maxBound, 10); err != nil {
		return err
	}
	if err := m.Power(c.maxBound, v, c.maxBound); err != nil {
		return err
	}
	if err := m.Copy(c.minusMaxBound, c.maxBound); err != nil {
		return err
	}
	if err := m.Neg(c.minusMaxBound); err != nil {
		return err
	}
	maxDepth := "max_depth"
	v, err = pc.GetUIntName(p, &maxDepth, 128)
	if err != nil {
		return err
	}
	c.maxDepth = v
	maxNodes := "max_nodes"
	v, err = pc.GetUIntName(p, &maxNodes, 8192)
	if err != nil {
		return err
	}
	c.maxNodes = v
	maxMemory := "max_memory"
	v, err = pc.GetUIntName(p, &maxMemory, math.MaxUint32)
	if err != nil {
		return err
	}
	c.maxMemory = memory.MegabytesToBytes(v)
	precision := "nth_root_precision"
	v, err = pc.GetUIntName(p, &precision, 8192)
	if err != nil {
		return err
	}
	if v == 0 {
		v = 1
	}
	if err := m.SetInt64(c.rootPrecision, int64(int32(v))); err != nil {
		return err
	}
	return m.Inv(c.rootPrecision)
}

func CollectFixedParamDescrs(d *params.ParamDescrs) error {
	return CollectHardwareParamDescrs(d)
}
