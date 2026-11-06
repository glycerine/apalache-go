package ufbv

// Source: src/tactic/ufbv/ufbv_rewriter.cpp:652-749.

import (
	"errors"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/diagnostic"
)

// formulaProcessor owns the concrete indexes and work queues for one UFBV
// formula batch. The configured Boolean reducer is borrowed from its caller.
type formulaProcessor struct {
	manager *ast.ASTManager
	trace   *diagnostic.TraceTags
	index   *rewriteRuleIndex
	back    *rewriteBackIndex
	loop    *rewriteTraversal
	bool    *rewriter.BoolRewriter
	ran     bool
	closed  bool
}

func newFormulaProcessor(m *ast.ASTManager, boolean *rewriter.BoolRewriter) (*formulaProcessor, error) {
	if m == nil || boolean == nil {
		return nil, ast.ErrASTHandle
	}
	r := &formulaProcessor{manager: m, bool: boolean}
	var e error
	r.index, e = newRewriteRuleIndex(m)
	if e != nil {
		return r, e
	}
	r.back, e = newRewriteBackIndex(m)
	if e != nil {
		return r, e
	}
	r.loop, e = newRewriteTraversal(m)
	return r, e
}

func (r *formulaProcessor) check() error {
	if r == nil || r.closed || r.manager == nil || r.bool == nil || r.index == nil || r.back == nil || r.loop == nil {
		return ast.ErrASTHandle
	}
	if e := r.index.check(); e != nil {
		return e
	}
	if e := r.back.check(); e != nil {
		return e
	}
	return r.loop.check()
}

// run follows the source LIFO main todo loop, then publishes the processed
// formulas and surviving indexed demodulators as independently owned refs.
func (r *formulaProcessor) run(formulas []ast.ASTHandle) (output []ast.ASTHandle, result error) {
	if e := r.check(); e != nil {
		return nil, e
	}
	if r.ran {
		return nil, ast.ErrASTDomain
	}
	r.ran = true
	r.index.trace = r.formulaTraceTags()
	m := r.manager
	defer func() {
		if result != nil {
			for _, h := range output {
				result = errors.Join(result, m.DecRef(h))
			}
			output = nil
		}
	}()
	publish := func(h ast.ASTHandle) error {
		if e := m.IncRef(h); e != nil {
			return e
		}
		output = append(output, h)
		return nil
	}
	proofs, e := m.ProofsEnabled()
	if e != nil {
		return nil, e
	}
	if proofs {
		if e := diagnostic.BorrowProcessWarningService().Warning("PRE_DEMODULATOR=true is not supported when proofs are enabled."); e != nil {
			return nil, e
		}
		for _, h := range formulas {
			if e := publish(h); e != nil {
				return output, e
			}
		}
		return output, nil
	}
	if e := r.traceFormulas(668, "before demodulator:\n", formulas); e != nil {
		return nil, e
	}
	for _, h := range formulas {
		if e := m.IncRef(h); e != nil {
			return output, e
		}
		r.back.todo = append(r.back.todo, h)
	}
	// Source reserve(max_var_id) only prepares vector capacity. The concrete
	// matcher's native maps allocate on demand and need no preflight.
	for len(r.back.todo) != 0 {
		if e := r.processLast(); e != nil {
			return output, e
		}
	}
	for h := range r.back.processed {
		if e := publish(h); e != nil {
			return output, e
		}
		if e := r.traceFormulas(736, "", []ast.ASTHandle{h}); e != nil {
			return output, e
		}
	}
	for _, qs := range r.index.byFunc {
		for _, q := range qs.values {
			if e := publish(q); e != nil {
				return output, e
			}
			if e := r.traceFormulas(748, "", []ast.ASTHandle{q}); e != nil {
				return output, e
			}
		}
	}
	if e := r.traceFormulas(753, "after demodulator:\n", output); e != nil {
		return output, e
	}
	return output, nil
}

func (r *formulaProcessor) processLast() (result error) {
	m := r.manager
	last := len(r.back.todo) - 1
	h := r.back.todo[last]
	cur, e := m.NewRef(h)
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, m.DelRef(&cur)) }()
	r.back.todo[last] = ast.ASTHandle{}
	r.back.todo = r.back.todo[:last]
	consumed, e := m.DecRefConsumed(h)
	if !consumed {
		r.back.owned = append(r.back.owned, h)
	}
	if e != nil {
		return e
	}
	if !consumed {
		return ast.ErrASTPending
	}
	np, e := r.loop.rewrite(r.index, r.bool, h)
	if e != nil {
		return e
	}
	defer func() { result = errors.Join(result, m.DelRef(&np)) }()
	rewritten, e := m.RefGet(np)
	if e != nil {
		return e
	}
	if e := r.assertRewrittenFixedPoint(rewritten); e != nil {
		return e
	}
	c := demodulatorClassifier{manager: m, trace: r.formulaTraceTags()}
	large, small, demodulator, e := c.classify(rewritten)
	if e != nil {
		return e
	}
	if !demodulator {
		return r.back.addProcessed(rewritten)
	}
	defer func() {
		result = errors.Join(result, m.DelRef(&small), m.DelRef(&large))
	}()
	lhs, e := m.RefGet(large)
	if e != nil {
		return e
	}
	rhs, e := m.RefGet(small)
	if e != nil {
		return e
	}
	if e := r.traceFoundDemodulator(lhs, rhs); e != nil {
		return e
	}
	f, e := r.demodulatorFunction(lhs)
	if e != nil {
		return e
	}
	if e := r.back.rescheduleProcessed(f); e != nil {
		return e
	}
	if e := r.back.rescheduleDemodulators(r.index, f, lhs); e != nil {
		return e
	}
	if e := r.index.insert(rewritten, lhs, rhs); e != nil {
		return e
	}
	return r.back.addDemodulator(r.index, rewritten)
}

func (r *formulaProcessor) close() error {
	if r == nil || r.closed {
		return nil
	}
	if r.loop != nil {
		if e := r.loop.close(); e != nil {
			return e
		}
	}
	if r.back != nil {
		if e := r.back.close(); e != nil {
			return e
		}
	}
	if r.index != nil {
		if e := r.index.close(); e != nil {
			return e
		}
	}
	r.closed = true
	return nil
}
