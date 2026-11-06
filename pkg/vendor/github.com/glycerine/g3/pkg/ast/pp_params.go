package ast

// Source: src/ast/pp_params.hpp:5-51.

import uparams "github.com/glycerine/g3/pkg/util/params"

// PPParams is the generated pretty-printer parameter facade. It borrows the
// caller's local parameters and owns the constructor-time pp module snapshot.
type PPParams struct {
	self              *PPParams
	context           *uparams.ParamsContext
	local             *uparams.ParamsRef
	empty             uparams.ParamsRef
	global            uparams.ParamsRef
	decrement         uparams.ParamsDecrementReceipt
	destructorStarted bool
	closed            bool
}

func NewPPParams(c *uparams.ParamsContext, g *uparams.GlobalParamsService, local *uparams.ParamsRef) (*PPParams, error) {
	if c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	p := &PPParams{context: c}
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
	p.global, e = g.SourceGetModuleName("pp")
	if e != nil {
		return nil, e
	}
	return p, nil
}

func (p *PPParams) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	return p.context.Check(p.local)
}

func (p *PPParams) unsigned(key string, d uint32) (uint32, error) {
	if e := p.open(); e != nil {
		return 0, e
	}
	return p.context.GetUIntFallbackName(p.local, &key, &p.global, d)
}

func (p *PPParams) boolean(key string, d bool) (bool, error) {
	if e := p.open(); e != nil {
		return false, e
	}
	return p.context.GetBoolFallbackName(p.local, &key, &p.global, d)
}

func (p *PPParams) MaxIndent() (uint32, error)    { return p.unsigned("max_indent", ^uint32(0)) }
func (p *PPParams) MaxNumLines() (uint32, error)  { return p.unsigned("max_num_lines", ^uint32(0)) }
func (p *PPParams) MaxWidth() (uint32, error)     { return p.unsigned("max_width", 80) }
func (p *PPParams) MaxRibbon() (uint32, error)    { return p.unsigned("max_ribbon", 80) }
func (p *PPParams) MaxDepth() (uint32, error)     { return p.unsigned("max_depth", 5) }
func (p *PPParams) MinAliasSize() (uint32, error) { return p.unsigned("min_alias_size", 10) }
func (p *PPParams) Decimal() (bool, error)        { return p.boolean("decimal", false) }
func (p *PPParams) DecimalPrecision() (uint32, error) {
	return p.unsigned("decimal_precision", 10)
}
func (p *PPParams) BVLiterals() (bool, error)     { return p.boolean("bv_literals", true) }
func (p *PPParams) FPRealLiterals() (bool, error) { return p.boolean("fp_real_literals", false) }
func (p *PPParams) BVNeg() (bool, error)          { return p.boolean("bv_neg", false) }
func (p *PPParams) FlatAssoc() (bool, error)      { return p.boolean("flat_assoc", true) }
func (p *PPParams) FixedIndent() (bool, error)    { return p.boolean("fixed_indent", false) }
func (p *PPParams) SingleLine() (bool, error)     { return p.boolean("single_line", false) }
func (p *PPParams) Bounded() (bool, error)        { return p.boolean("bounded", false) }
func (p *PPParams) PrettyProof() (bool, error)    { return p.boolean("pretty_proof", false) }
func (p *PPParams) SimplifyImplies() (bool, error) {
	return p.boolean("simplify_implies", true)
}

func (p *PPParams) Close() error {
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
