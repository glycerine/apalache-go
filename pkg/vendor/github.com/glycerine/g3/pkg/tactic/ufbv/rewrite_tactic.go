package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter_tactic.cpp:26-113.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var ErrRewriterUnsatCores = errors.New("g3: ufbv-rewriter does not support unsat cores")

type rewriteTacticImp struct {
	manager *ast.ASTManager
	boolean *rewriter.BoolRewriter
	header  uint64
}

// RewriterTactic owns the configured Boolean reducer used by the concrete
// UFBV formula processor. Its AST and parameter services are borrowed.
type RewriterTactic struct {
	self           *RewriterTactic
	manager        *ast.ASTManager
	context        *uparams.ParamsContext
	globals        *uparams.GlobalParamsService
	imp            *rewriteTacticImp
	retired        []*rewriteTacticImp
	trace          *diagnostic.TraceTags
	header         uint64
	allocationLine int
	closed         bool
}

// NewRewriterTactic applies the source's fixed elim_and=true setting to its
// internal Boolean reducer. Other tactic parameters do not change this pass.
func NewRewriterTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*RewriterTactic, error) {
	return newRewriterTactic(m, c, g, 121)
}

func newRewriterTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, sourceLine int) (*RewriterTactic, error) {
	if m == nil || c == nil || g == nil || c.SymbolsField != m.Symbols || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	n := uint64(reflect.TypeOf(RewriterTactic{}).Size())
	if e := m.Budget().Reserve(n); e != nil {
		return nil, e
	}
	t := &RewriterTactic{manager: m, context: c, globals: g, header: n, allocationLine: sourceLine}
	t.self = t
	t.traceOuterAllocated()
	imp, e := t.newImp(81)
	t.imp = imp
	return t, e
}

// Translate constructs the source virtual tactic for another AST manager.
// The caller provides the destination manager's parameter services.
func (t *RewriterTactic) Translate(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (*RewriterTactic, error) {
	if err := t.check(); err != nil {
		return nil, err
	}
	return newRewriterTactic(m, c, g, 85)
}

func (t *RewriterTactic) check() error {
	if t == nil || t.self != t || t.closed || t.manager == nil || t.imp == nil || t.imp.boolean == nil {
		return ast.ErrASTHandle
	}
	_, e := t.manager.ProofsEnabled()
	return e
}

// Apply follows the source's goal reset, assertion and depth transition. It
// returns the same goal with an independent reference, matching the source
// goal_ref_buffer push. The caller releases the result with DecRef.
func (t *RewriterTactic) Apply(goal *tactic.Goal) (_ *tactic.Goal, result error) {
	if e := t.check(); e != nil {
		return nil, e
	}
	if goal == nil || goal.Manager() != t.manager {
		return nil, ast.ErrASTHandle
	}
	if e := goal.CheckOwner(); e != nil {
		return nil, e
	}
	if e := assertRewriterGoalWellSorted(goal, 39); e != nil {
		return nil, e
	}
	report, e := tactic.NewTacticReport("ufbv-rewriter", goal)
	if e != nil {
		return nil, e
	}
	defer func() { result = errors.Join(result, report.Close()) }()
	cores, e := goal.UnsatCoresEnabled()
	if e != nil {
		return nil, e
	}
	if cores {
		return nil, ErrRewriterUnsatCores
	}
	proofs, e := goal.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	n, e := goal.Size()
	if e != nil {
		return nil, e
	}
	forms := make([]ast.ASTHandle, n)
	formPins := 0
	defer func() {
		// The source forms ref_vector releases its entries after the younger
		// proof and rewritten-formula owners, in insertion order.
		for i := 0; i < formPins; i++ {
			result = errors.Join(result, t.manager.DecRef(forms[i]))
		}
	}()
	var proofNodes []ast.ASTHandle
	if proofs {
		proofNodes = make([]ast.ASTHandle, n)
	}
	proofPins := 0
	defer func() {
		for i := 0; i < proofPins; i++ {
			result = errors.Join(result, t.manager.DecRef(proofNodes[i]))
		}
	}()
	for i := range forms {
		forms[i], e = goal.Form(uint32(i))
		if e != nil {
			return nil, e
		}
		if e = t.manager.IncRef(forms[i]); e != nil {
			return nil, e
		}
		formPins++
		if proofs {
			proofNodes[i], e = goal.Proof(uint32(i))
			if e != nil {
				return nil, e
			}
			if e = t.manager.IncRef(proofNodes[i]); e != nil {
				return nil, e
			}
			proofPins++
		}
	}
	processor, e := newFormulaProcessor(t.manager, t.imp.boolean)
	if processor != nil {
		processor.trace = t.trace
	}
	if e != nil {
		if processor != nil {
			result = errors.Join(result, processor.close())
		}
		return nil, errors.Join(e, result)
	}
	output, e := processor.run(forms)
	if e != nil {
		return nil, errors.Join(e, processor.close())
	}
	defer func() {
		for _, h := range output {
			result = errors.Join(result, t.manager.DecRef(h))
		}
	}()
	if e := processor.close(); e != nil {
		return nil, e
	}
	if e := goal.Reset(); e != nil {
		return nil, e
	}
	for i, h := range output {
		if proofs {
			if e := goal.AssertWithProof(h, proofNodes[i], ast.ASTDependency{}); e != nil {
				return nil, e
			}
		} else if e := goal.Assert(h); e != nil {
			return nil, e
		}
	}
	if e := goal.IncDepth(); e != nil {
		return nil, e
	}
	if e := goal.IncRef(); e != nil {
		return nil, e
	}
	if e := t.tracePublishedGoal(goal); e != nil {
		return nil, errors.Join(e, goal.DecRef())
	}
	if e := assertRewriterGoalWellSorted(goal, 68); e != nil {
		return nil, errors.Join(e, goal.DecRef())
	}
	return goal, nil
}

func (t *RewriterTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	for len(t.retired) != 0 {
		if e := t.releaseImp(t.retired[0], 115); e != nil {
			return e
		}
		t.retired = t.retired[1:]
	}
	if t.imp != nil {
		if e := t.releaseImp(t.imp, 89); e != nil {
			return e
		}
		t.imp = nil
	}
	t.traceOuterDeallocated()
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}

// Cleanup constructs the replacement before retiring the previous imp, as
// the source swap does. A stopped old-owner release remains reachable.
func (t *RewriterTactic) Cleanup() error {
	if e := t.check(); e != nil {
		return e
	}
	for len(t.retired) != 0 {
		if e := t.releaseImp(t.retired[0], 115); e != nil {
			return e
		}
		t.retired = t.retired[1:]
	}
	next, e := t.newImp(113)
	if e != nil {
		if next != nil {
			t.retired = append(t.retired, next)
			releaseErr := t.releaseImp(next, 115)
			if releaseErr == nil {
				t.retired = t.retired[:len(t.retired)-1]
			}
			return errors.Join(e, releaseErr)
		}
		return e
	}
	old := t.imp
	t.imp = next
	t.retired = append(t.retired, old)
	if e := t.releaseImp(old, 115); e != nil {
		return e
	}
	t.retired = t.retired[1:]
	return nil
}
