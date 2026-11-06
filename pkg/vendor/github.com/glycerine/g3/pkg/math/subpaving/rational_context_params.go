package subpaving

// Source: src/math/subpaving/subpaving_t_def.h:419-438,473-513,
// concrete config_mpq named parameter construction and update.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// NewRationalContextFromParams performs the source constructor's one ordered
// update pass. A refused pass returns its partial owner for explicit Close.
func NewRationalContextFromParams(m *numeral.RationalManager, limit *control.Reslimit, pc *params.ParamsContext, p *params.ParamsRef) (*RationalContext, error) {
	c, err := newRationalContextOwner(m, limit)
	if err != nil {
		return nil, err
	}
	if err := c.UpdateParamsRef(pc, p); err != nil {
		return c, err
	}
	return c, nil
}

// UpdateParamsRef reads and applies each source option before reading the next.
func (c *RationalContext) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
	if err := c.open(); err != nil {
		return err
	}
	if pc == nil || p == nil {
		return arena.ErrInvalidSlot
	}
	if err := pc.Check(p); err != nil {
		return err
	}
	epsilon := "epsilon"
	v, err := pc.GetUIntName(p, &epsilon, 20)
	if err != nil {
		return err
	}
	if err := c.updateEpsilon(v); err != nil {
		return err
	}
	maxBound := "max_bound"
	v, err = pc.GetUIntName(p, &maxBound, 10)
	if err != nil {
		return err
	}
	if err = c.updateMaxBound(v); err != nil {
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
	return c.UpdateRootPrecision(v)
}

func CollectRationalParamDescrs(d *params.ParamDescrs) error {
	return CollectHardwareParamDescrs(d)
}
