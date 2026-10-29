package rewriter

// Source: src/ast/rewriter/poly_rewriter.h:95-123;
// src/ast/rewriter/poly_rewriter_def.h:28-45;
// src/ast/rewriter/poly_rewriter_params.pyg;
// src/ast/rewriter/arith_rewriter_params.pyg.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// BVPolynomialConfiguration owns generated parameter snapshots, not the full
// polynomial rewriter's persistent expression map or current-sort storage.
type BVPolynomialConfiguration struct {
	self            *BVPolynomialConfiguration
	context         *uparams.ParamsContext
	globals         *uparams.GlobalParams
	state           BVPolynomialConfigurationState
	bitVector       BVRewriterConfigurationState
	snapshots       [4]uparams.ParamsRef // poly, arith, optional outer default params_ref, BV
	placed          [4]bool
	decrements      [4]uparams.ParamsDecrementReceipt
	header          uint64
	passes          uint64
	stopped, closed bool
	cause           error
}

type BVPolynomialConfigurationState struct {
	Flat, SumOfMonomials, HoistMultiplication, HoistCoefficients, SortSums, ASTOrder bool
	MonomialLimit                                                                    uint32
}

// On a reached failure the returned owner retains the actual placed prefix for
// Close. A caller-supplied local params_ref is borrowed only during each update.
func NewBVPolynomialConfiguration(c *uparams.ParamsContext, g *uparams.GlobalParams, local *uparams.ParamsRef) (*BVPolynomialConfiguration, error) {
	if c == nil || g == nil || g.ContextField != c {
		return nil, uparams.ErrParamsOwner
	}
	var defaultLocal uparams.ParamsRef
	if local != nil {
		if e := c.Check(local); e != nil {
			return nil, e
		}
	} else {
		v, e := c.NewValue()
		if e != nil {
			return nil, e
		}
		defaultLocal = v
	}
	bytes := uint64(reflect.TypeOf(BVPolynomialConfiguration{}).Size())
	if e := c.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	p := &BVPolynomialConfiguration{context: c, globals: g, header: bytes}
	p.self = p
	if local == nil {
		p.snapshots[2] = defaultLocal
		p.placed[2] = true
		local = &p.snapshots[2]
	}
	if e := p.Update(local); e != nil {
		return p, e
	}
	if ast.ASTSourceDebug && ((p.state.SumOfMonomials && !p.state.Flat) || (p.state.SumOfMonomials && p.state.HoistMultiplication)) {
		return p, p.fail(ast.ErrASTDomain)
	}
	if e := p.Update(local); e != nil {
		return p, e
	}
	if e := p.end(2); e != nil {
		return p, p.fail(e)
	}
	return p, nil
}
func (p *BVPolynomialConfiguration) open() error {
	if p == nil || p.self != p || p.closed {
		return uparams.ErrParamsOwner
	}
	if p.stopped {
		return p.cause
	}
	return nil
}
func (p *BVPolynomialConfiguration) fail(e error) error {
	p.stopped = true
	if p.cause == nil {
		p.cause = e
	}
	return e
}
func (p *BVPolynomialConfiguration) end(i int) error {
	if !p.placed[i] {
		return nil
	}
	var e error
	if p.decrements[i].Invocation() != 0 {
		e = p.context.CompleteDecrement(&p.snapshots[i], p.decrements[i])
	} else {
		e = p.context.DelTracked(&p.snapshots[i], &p.decrements[i])
	}
	if e == nil {
		p.placed[i] = false
		p.decrements[i] = uparams.ParamsDecrementReceipt{}
	}
	return e
}
func (p *BVPolynomialConfiguration) boolean(local *uparams.ParamsRef, key string, def bool, dst *bool) error {
	v, e := p.context.GetBoolFallbackName(local, &key, &p.snapshots[0], def)
	if e == nil {
		*dst = v
	}
	return e
}
func (p *BVPolynomialConfiguration) Update(local *uparams.ParamsRef) (err error) {
	if e := p.open(); e != nil {
		return e
	}
	if e := p.context.Check(local); e != nil {
		return e
	}
	defer func() {
		if err != nil {
			p.fail(err)
		}
	}()
	p.snapshots[0], err = p.globals.GetModuleName("rewriter")
	if err != nil {
		return err
	}
	p.placed[0] = true
	if err = p.boolean(local, "flat", true, &p.state.Flat); err != nil {
		return err
	}
	if err = p.boolean(local, "som", false, &p.state.SumOfMonomials); err != nil {
		return err
	}
	if err = p.boolean(local, "hoist_mul", false, &p.state.HoistMultiplication); err != nil {
		return err
	}
	if err = p.boolean(local, "hoist_cmul", false, &p.state.HoistCoefficients); err != nil {
		return err
	}
	key := "som_blowup"
	v, e := p.context.GetUIntFallbackName(local, &key, &p.snapshots[0], ^uint32(0))
	if e != nil {
		return e
	}
	p.state.MonomialLimit = v
	if !p.state.Flat {
		p.state.SumOfMonomials = false
	}
	if p.state.SumOfMonomials {
		p.state.HoistMultiplication = false
	}
	p.snapshots[1], err = p.globals.GetModuleName("rewriter")
	if err != nil {
		return err
	}
	p.placed[1] = true
	key = "arith_ineq_lhs"
	lhs, e := p.context.GetBoolFallbackName(local, &key, &p.snapshots[1], false)
	if e != nil {
		return e
	}
	p.state.ASTOrder = !lhs
	// Reverse generated helper destruction, after all assignments.
	if err = p.end(1); err != nil {
		return err
	}
	if err = p.end(0); err != nil {
		return err
	}
	p.passes++
	return nil
}
func (p *BVPolynomialConfiguration) State() (BVPolynomialConfigurationState, error) {
	if e := p.open(); e != nil {
		return BVPolynomialConfigurationState{}, e
	}
	return p.state, nil
}
func (p *BVPolynomialConfiguration) SetFlat(v bool) error {
	if e := p.open(); e != nil {
		return e
	}
	p.state.Flat = v
	return nil
}
func (p *BVPolynomialConfiguration) SetSortSums(v bool) error {
	if e := p.open(); e != nil {
		return e
	}
	p.state.SortSums = v
	return nil
}
func (p *BVPolynomialConfiguration) AdditionOptions() (bool, BVPolynomialAdditionOptions, error) {
	s, e := p.State()
	return s.Flat, BVPolynomialAdditionOptions{SortSums: s.SortSums, ASTOrder: s.ASTOrder, HoistCoefficients: s.HoistCoefficients, HoistMultiplication: s.HoistMultiplication}, e
}
func (p *BVPolynomialConfiguration) MultiplicationOptions() (bool, BVPolynomialMultiplicationOptions, error) {
	s, e := p.State()
	return s.Flat, BVPolynomialMultiplicationOptions{ASTOrder: s.ASTOrder, HoistCoefficients: s.HoistCoefficients, SumOfMonomials: s.SumOfMonomials, MonomialLimit: s.MonomialLimit}, e
}
func BVPolynomialParameterDescriptions(d *uparams.ParamDescrs) error {
	return catalogCollectPolyRewriterParams(d)
}
func (p *BVPolynomialConfiguration) Close() error {
	if p == nil || p.self != p {
		return uparams.ErrParamsOwner
	}
	if p.closed {
		return nil
	}
	for _, i := range [4]int{3, 1, 0, 2} {
		if e := p.end(i); e != nil {
			return e
		}
	}
	p.context.BudgetField.Release(p.header)
	p.header = 0
	p.closed = true
	return nil
}
