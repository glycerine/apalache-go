package subpaving

// Source: src/math/subpaving/subpaving.cpp:54 and
// src/math/subpaving/subpaving_t_def.h:473-501, concrete context_hwf.

import (
	"math"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/util/control"
	"github.com/glycerine/g3/pkg/util/numeral"
	"github.com/glycerine/g3/pkg/util/params"
)

// NewHardwareContextFromParams follows the source context constructor's
// single updt_params pass. A failed pass returns its partial context for the
// caller's explicit owning Close.
func NewHardwareContextFromParams(m *numeral.HardwareFloatNumeralManager, limit *control.Reslimit, pc *params.ParamsContext, p *params.ParamsRef) (*HardwareContext, error) {
	if m == nil || limit == nil {
		return nil, arena.ErrInvalidSlot
	}
	c := &HardwareContext{numbers: m, limit: limit}
	c.self = c
	if err := c.UpdateParamsRef(pc, p); err != nil {
		return c, err
	}
	return c, nil
}

// UpdateParamsRef reads each retained parameter immediately before applying
// its source-ordered mutation. A later getter failure leaves the earlier
// context fields changed, as context_t::updt_params does.
func (c *HardwareContext) UpdateParamsRef(pc *params.ParamsContext, p *params.ParamsRef) error {
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
	if err = c.updateEpsilon(v); err != nil {
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
	c.updateMaxMemory(v)
	rootPrecision := "nth_root_precision"
	v, err = pc.GetUIntName(p, &rootPrecision, 8192)
	if err != nil {
		return err
	}
	return c.updateRootPrecision(v)
}

// CollectHardwareParamDescrs follows context_t<config_hwf>::collect_param_descrs.
// InsertName retains completed entries if a later insertion is refused.
func CollectHardwareParamDescrs(d *params.ParamDescrs) error {
	if d == nil {
		return arena.ErrInvalidSlot
	}
	for _, entry := range []struct{ name, description string }{
		{"max_nodes", "(default: 8192) maximum number of nodes in the subpaving tree."},
		{"max_depth", "(default: 128) maximum depth of the subpaving tree."},
		{"epsilon", "(default: 20) value k s.t. a new lower (upper) bound for x is propagated only new-lower(x) > lower(k) + 1/k * max(min(upper(x) - lower(x), |lower|), 1) (new-upper(x) < upper(x) - 1/k * max(min(upper(x) - lower(x), |lower|), 1)). If k = 0, then this restriction is ignored."},
		{"max_bound", "(default 10) value k s.t. a new upper (lower) bound for x is propagated only if upper(x) > -10^k or lower(x) = -oo (lower(x) < 10^k or upper(x) = oo)"},
		{"nth_root_precision", "(default 8192) value k s.t. 1/k is the precision for computing the nth root in the subpaving module."},
	} {
		if err := d.InsertName(entry.name, params.ParamUInt, &entry.description, nil, nil); err != nil {
			return err
		}
	}
	return nil
}
