package rewriter

// Source: src/ast/rewriter/rewriter_params.hpp:5-37.

import uparams "github.com/glycerine/g3/pkg/util/params"

// RewriterParams is the concrete generated parameter facade. It borrows the
// caller's local parameters and owns the constructor-time rewriter snapshot.
type RewriterParams struct {
	self              *RewriterParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewRewriterParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*RewriterParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &RewriterParams{context: c}
	p.self = p
	if local == nil {
		p.empty = c.EmptyValue()
		local = &p.empty
	}
	if e := c.Check(local); e != nil {
		return nil, e
	}
	p.local = local
	var e error
	p.global, e = g.SourceGetModuleName("rewriter")
	if e != nil {
		return nil, e
	}
	return p, nil
}

func (p *RewriterParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *RewriterParams) boolean(key string, d bool) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, d)
}

func (p *RewriterParams) unsigned(key string, d uint32) (uint32, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, d)
}

func (p *RewriterParams) MaxMemory() (uint32, error)    { return p.unsigned("max_memory", ^uint32(0)) }
func (p *RewriterParams) MaxSteps() (uint32, error)     { return p.unsigned("max_steps", ^uint32(0)) }
func (p *RewriterParams) Flat() (bool, error)           { return p.boolean("flat", true) }
func (p *RewriterParams) SumOfMonomials() (bool, error) { return p.boolean("som", false) }
func (p *RewriterParams) SumOfMonomialsBlowup() (uint32, error) {
	return p.unsigned("som_blowup", ^uint32(0))
}
func (p *RewriterParams) HoistMultiplication() (bool, error) {
	return p.boolean("hoist_mul", false)
}
func (p *RewriterParams) HoistCoefficients() (bool, error) {
	return p.boolean("hoist_cmul", false)
}
func (p *RewriterParams) PushITEArith() (bool, error) { return p.boolean("push_ite_arith", false) }
func (p *RewriterParams) PushITEBV() (bool, error)    { return p.boolean("push_ite_bv", false) }
func (p *RewriterParams) PullCheapITE() (bool, error) { return p.boolean("pull_cheap_ite", false) }
func (p *RewriterParams) BVIneqConsistencyTestMax() (uint32, error) {
	return p.unsigned("bv_ineq_consistency_test_max", 0)
}
func (p *RewriterParams) CacheAll() (bool, error) { return p.boolean("cache_all", false) }
func (p *RewriterParams) RewritePatterns() (bool, error) {
	return p.boolean("rewrite_patterns", false)
}
func (p *RewriterParams) IgnorePatternsOnGroundQBody() (bool, error) {
	return p.boolean("ignore_patterns_on_ground_qbody", true)
}

func (p *RewriterParams) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	if p.destructorStarted {
		if e := p.context.CompleteDecrement(&p.global, p.decrement); e != nil {
			return e
		}
	} else {
		p.destructorStarted = true
		if e := p.context.DelTracked(&p.global, &p.decrement); e != nil {
			return e
		}
	}
	p.closed = true
	return nil
}
