package aig

// Source: src/tactic/aig/aig_tactic.cpp:65-113.

import (
	"errors"
	"math"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

// Pass applies the source AIG tactic's two goal-rewriting branches.
type Pass struct {
	DefaultGateEncoding bool
	PerAssertion        bool
	maxMemory           uint64
	cooperation         *cooperate.Cooperation
	paramsSymbols       *usymbol.Manager
	manager             *Manager
	pendingRefs         []*Ref
}

func NewPass() *Pass {
	return &Pass{DefaultGateEncoding: true, PerAssertion: true, maxMemory: math.MaxUint64}
}

// UpdateParams follows aig_tactic::updt_params, including the UINT_MAX
// sentinel used by megabytes_to_bytes on the selected 64-bit profile.
func (p *Pass) UpdateParams(c *uparams.ParamsContext, value *uparams.ParamsRef) error {
	if p == nil || c == nil || c.SymbolsField == nil {
		return ErrOwner
	}
	memoryName, encodingName, perName := "max_memory", "aig_default_gate_encoding", "aig_per_assertion"
	mb, err := c.GetUIntName(value, &memoryName, math.MaxUint32)
	if err != nil {
		return err
	}
	encoding, err := c.GetBoolName(value, &encodingName, true)
	if err != nil {
		return err
	}
	per, err := c.GetBoolName(value, &perName, true)
	if err != nil {
		return err
	}
	limit := umemory.MegabytesToBytes(mb)
	p.maxMemory, p.DefaultGateEncoding, p.PerAssertion, p.paramsSymbols = limit, encoding, per, c.SymbolsField
	return nil
}

// CollectParamDescrs follows aig_tactic::collect_param_descrs in source order.
func (p *Pass) CollectParamDescrs(d *uparams.ParamDescrs) error {
	if p == nil || d == nil {
		return ErrOwner
	}
	if err := uparams.InsertMaxMemory(d); err != nil {
		return err
	}
	description := "(default: true) process one assertion at a time."
	return d.InsertName("aig_per_assertion", uparams.ParamBool, &description, nil, nil)
}

// BindCooperation borrows the process cooperation owner used at AIG checkpoints.
func (p *Pass) BindCooperation(owner *cooperate.Cooperation) error {
	if p == nil || owner == nil {
		return ErrOwner
	}
	if _, err := owner.Enabled(); err != nil {
		return err
	}
	p.cooperation = owner
	return nil
}

func (p *Pass) closeManager(m *Manager) error {
	if err := m.Close(); err != nil {
		p.manager = m
		return err
	}
	m.traceTacticDeallocated()
	if p.manager == m {
		p.manager = nil
	}
	return nil
}

func (p *Pass) closeRef(r *Ref) error {
	err := r.Close()
	if err != nil && !r.closed {
		p.pendingRefs = append(p.pendingRefs, r)
	}
	return err
}

// Close retries a stopped application destructor. Successful applications
// have no retained manager or references, matching source cleanup's no-op.
func (p *Pass) Close() error {
	if p == nil {
		return ErrOwner
	}
	for len(p.pendingRefs) > 0 {
		if err := p.pendingRefs[0].Close(); err != nil {
			return err
		}
		p.pendingRefs = p.pendingRefs[1:]
	}
	if p.manager != nil {
		return p.closeManager(p.manager)
	}
	return nil
}

func assertGoalWellSorted(g *tactic.Goal, line int32) error {
	debug := diagnostic.BorrowProcessDebugContext()
	enabled, err := diagnostic.SourceAssertionsEnabled(debug)
	if err != nil || !enabled {
		return err
	}
	good, err := g.IsWellSorted()
	if err != nil || good {
		return err
	}
	return diagnostic.DebugAssertionFailure(debug, "src/tactic/aig/aig_tactic.cpp", line, "g->is_well_sorted()")
}

func (p *Pass) rewriteOne(m *Manager, g *tactic.Goal, i uint32) (err error) {
	form, err := g.Form(i)
	if err != nil {
		return err
	}
	r, err := m.MkAIG(form)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.closeRef(r)) }()
	shared, err := m.MaxSharing(r)
	if err != nil {
		return err
	}
	closeErr := p.closeRef(r)
	r = shared
	if closeErr != nil {
		return closeErr
	}
	newForm, err := m.ToFormula(r)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.releaseAST(&newForm)) }()
	h, err := m.ast.RefGet(newForm)
	if err != nil {
		return err
	}
	dep, err := g.Dependency(i)
	if err != nil {
		return err
	}
	return g.Update(i, h, ast.ASTHandle{}, dep)
}

func (p *Pass) rewriteGoal(m *Manager, g *tactic.Goal) (err error) {
	if p.PerAssertion {
		for i := uint32(0); ; i++ {
			size, e := g.Size()
			if e != nil || i >= size {
				return e
			}
			if e = p.rewriteOne(m, g, i); e != nil {
				return e
			}
		}
	}
	cores, err := g.UnsatCoresEnabled()
	if err != nil {
		return err
	}
	if cores {
		return tactic.NewTacticException("aig does not support unsat core production")
	}
	r, err := m.MkAIGGoal(g)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, p.closeRef(r)) }()
	if err = g.Reset(); err != nil {
		return err
	}
	shared, err := m.MaxSharing(r)
	if err != nil {
		return err
	}
	closeErr := p.closeRef(r)
	r = shared
	if closeErr != nil {
		return closeErr
	}
	return m.ToGoal(r, g)
}

func (p *Pass) apply(g *tactic.Goal) (err error) {
	if p == nil || g == nil {
		return ErrOwner
	}
	if err = g.CheckOwner(); err != nil {
		return err
	}
	if p.paramsSymbols != nil && p.paramsSymbols != g.Manager().Symbols {
		return ErrOwner
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return err
	}
	if proofs {
		return tactic.NewTacticException("aig does not support proof production")
	}
	if err = p.Close(); err != nil {
		return err
	}
	if err = assertGoalWellSorted(g, 69); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("aig", g)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	m, err := newManagerWithGateEncoding(g.Manager(), p.DefaultGateEncoding, true)
	if err != nil {
		return err
	}
	if err = m.SetMaxMemory(p.maxMemory); err != nil {
		return errors.Join(err, p.closeManager(m))
	}
	if p.cooperation != nil {
		if err = m.BindCooperation(p.cooperation); err != nil {
			return errors.Join(err, p.closeManager(m))
		}
	}
	if err = p.rewriteGoal(m, g); err != nil {
		return errors.Join(err, p.closeManager(m))
	}
	if err = assertGoalWellSorted(g, 90); err != nil {
		return errors.Join(err, p.closeManager(m))
	}
	if err = p.closeManager(m); err != nil {
		return err
	}
	return nil
}

// Apply rewrites the goal and returns the independent source goal reference.
func (p *Pass) Apply(g *tactic.Goal) (*tactic.Goal, error) {
	if err := p.apply(g); err != nil {
		return nil, err
	}
	if err := g.IncDepth(); err != nil {
		return nil, err
	}
	if err := g.IncRef(); err != nil {
		return nil, err
	}
	return g, nil
}
