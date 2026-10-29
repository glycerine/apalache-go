package rewriter

// Source: src/ast/rewriter/rewriter_def.h:23-750;
// src/ast/rewriter/rewriter.h:194-363.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/util/messages"
)

// The source specializations share the original iterative states, scopes,
// bounded revisit depths and borrowed binding vectors. The core proofGen bit
// selects the source's proof-generating bookkeeping without changing frames.
type rewriterEngine struct {
	self                     *rewriterEngine
	core                     rewriterCore
	cfg                      rewriterConfiguration
	numSteps                 uint32
	bindings                 []ast.ASTHandle
	patterns, noPatterns     []ast.ASTHandle
	shifter, invShifter      variableShifter
	r, pr, pr2               ast.ASTRef
	shifts                   []uint32
	header                   uint64
	ready, closed            bool
	modelBooleanShortCircuit bool
}

func newRewriterEngine(m *ast.ASTManager, cfg rewriterConfiguration) (*rewriterEngine, error) {
	if m == nil || cfg == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(rewriterEngine{}).Size())
	if e := m.BudgetField.Reserve(n); e != nil {
		return nil, e
	}
	r := &rewriterEngine{header: n}
	return r, r.init(m, cfg)
}
func (r *rewriterEngine) init(m *ast.ASTManager, cfg rewriterConfiguration) error {
	return r.initMode(m, cfg, false)
}
func (r *rewriterEngine) initMode(m *ast.ASTManager, cfg rewriterConfiguration, proofGen bool) error {
	r.self = r
	r.cfg = cfg
	if e := r.core.init(m, proofGen); e != nil {
		return e
	}
	if e := r.shifter.init(m, false); e != nil {
		return e
	}
	if e := r.invShifter.init(m, true); e != nil {
		return e
	}
	var e error
	r.r, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	r.pr, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	r.pr2, e = m.NewRef(ast.ASTHandle{})
	if e != nil {
		return e
	}
	r.ready = true
	return nil
}
func (r *rewriterEngine) open() error {
	if r == nil || r.self != r || r.closed || !r.ready {
		return ast.ErrASTHandle
	}
	return r.core.open()
}
func (r *rewriterEngine) pushBinding(h ast.ASTHandle, shift uint32) {
	r.bindings = append(r.bindings, h)
	r.shifts = append(r.shifts, shift)
}
func (r *rewriterEngine) SetBindings(values []ast.ASTHandle) error {
	return r.setBindings(values, false)
}
func (r *rewriterEngine) SetInvBindings(values []ast.ASTHandle) error {
	return r.setBindings(values, true)
}
func (r *rewriterEngine) setBindings(values []ast.ASTHandle, inverse bool) error {
	if e := r.open(); e != nil {
		return e
	}
	if uint64(len(values)) > uint64(^uint32(0)) {
		return ast.ErrASTDomain
	}
	return r.setBindingArguments(ast.SliceArguments(values), inverse)
}
func (r *rewriterEngine) setBindingArguments(values ast.AppArguments, inverse bool) error {
	if e := r.open(); e != nil {
		return e
	}
	if len(r.core.frames) != 0 {
		return ast.ErrASTDomain
	}
	if r.core.proofGen {
		return ast.ErrASTDomain
	}
	r.bindings = r.bindings[:0]
	r.shifts = r.shifts[:0]
	n := values.Count()
	for i := uint32(0); i < n; i++ {
		j := n - i - 1
		if inverse {
			j = i
		}
		h, e := values.At(j)
		if e != nil {
			return e
		}
		r.pushBinding(h, n)
	}
	return nil
}
func (r *rewriterEngine) Reset() error {
	if e := r.open(); e != nil {
		return e
	}
	if e := r.releasePatterns(); e != nil {
		return e
	}
	r.cfg.reset()
	if e := r.core.Reset(); e != nil {
		return e
	}
	r.bindings = r.bindings[:0]
	r.shifts = r.shifts[:0]
	if e := r.shifter.core.Reset(); e != nil {
		return e
	}
	return r.invShifter.core.Reset()
}
func (r *rewriterEngine) Cleanup() error {
	if e := r.open(); e != nil {
		return e
	}
	if e := r.releasePatterns(); e != nil {
		return e
	}
	r.cfg.cleanup()
	if e := r.core.Cleanup(); e != nil {
		return e
	}
	r.bindings = nil
	if e := r.shifter.core.Cleanup(); e != nil {
		return e
	}
	r.shifts = nil
	return r.invShifter.core.Cleanup()
}
func (r *rewriterEngine) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	m := r.core.manager
	if e := r.releasePatterns(); e != nil {
		return e
	}
	r.shifts = nil
	for _, ref := range []*ast.ASTRef{&r.pr2, &r.pr, &r.r} {
		if *ref != (ast.ASTRef{}) {
			if e := m.DelRef(ref); e != nil {
				return e
			}
		}
	}
	if r.invShifter.self != nil {
		if e := r.invShifter.Close(); e != nil {
			return e
		}
	}
	if r.shifter.self != nil {
		if e := r.shifter.Close(); e != nil {
			return e
		}
	}
	r.bindings = nil
	if e := r.core.Close(); e != nil {
		return e
	}
	m.BudgetField.Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
func (r *rewriterEngine) pushFrame(t ast.ASTHandle, cache bool, depth uint32) error {
	if r.cfg.maxFramesExceeded(uint32(len(r.core.frames))) {
		return newRewriterException(messages.MaxFrames)
	}
	return r.core.pushFrameCore(t, cache, 0, depth)
}
func (r *rewriterEngine) beginScope() error {
	if r.cfg.maxScopesExceeded(uint32(len(r.core.scopes))) {
		return newRewriterException(messages.MaxScopes)
	}
	return r.core.beginScope()
}
func (r *rewriterEngine) mustCache(t ast.ASTHandle) (bool, error) {
	c := &r.core
	if r.cfg.cacheAllResults() {
		if t == c.root {
			return false, nil
		}
		kind, e := c.manager.GetKind(t)
		if e != nil {
			return false, e
		}
		if kind == ast.ASTQuantifier {
			return true, nil
		}
		if kind != ast.ASTApp {
			return false, nil
		}
		n, e := c.manager.GetNumArgs(t)
		return n > 0, e
	}
	if r.cfg.cacheResults() {
		return c.mustCache(t)
	}
	return false, nil
}
func (r *rewriterEngine) cache(fr rewriterFrame, value ast.ASTHandle) error {
	if fr.cacheResult() {
		if r.core.proofGen {
			proof, e := r.coreProofBack()
			if e != nil {
				return e
			}
			return r.core.cacheResultProof(fr.curr, value, proof)
		}
		return r.core.cacheResult(fr.curr, value)
	}
	return nil
}
func (r *rewriterEngine) appendProof(h ast.ASTHandle) error {
	return appendRewriterRoot(r.core.manager, &r.core.proofResults, h)
}
func (r *rewriterEngine) coreProofBack() (ast.ASTHandle, error) {
	if len(r.core.proofResults) == 0 {
		return ast.ASTHandle{}, ast.ErrASTDomain
	}
	return r.core.proofResults[len(r.core.proofResults)-1], nil
}
func (r *rewriterEngine) popProof() error {
	if len(r.core.proofResults) == 0 {
		return ast.ErrASTDomain
	}
	return shrinkRewriterRoots(r.core.manager, &r.core.proofResults, uint32(len(r.core.proofResults)-1))
}
func (r *rewriterEngine) sameSort(old, value ast.ASTHandle) error {
	if !ast.ASTSourceDebug {
		return nil
	}
	m := r.core.manager
	a, e := m.GetSort(old)
	if e != nil {
		return e
	}
	b, e := m.GetSort(value)
	if e != nil {
		return e
	}
	if a != b {
		return ast.ErrASTDomain
	}
	return nil
}
func (r *rewriterEngine) visit(t ast.ASTHandle, depth uint32) (bool, error) {
	c := &r.core
	m := c.manager
	sub, subProof, found, e := r.cfg.getSubst(t)
	if e != nil {
		return false, e
	}
	if found {
		if e = r.sameSort(t, sub); e != nil {
			return false, e
		}
		if e = appendRewriterRoot(m, &c.results, sub); e != nil {
			return false, e
		}
		if c.proofGen {
			if e = r.appendProof(subProof); e != nil {
				return false, e
			}
		}
		return true, r.setNewChildIfDifferent(t, sub)
	}
	if depth == 0 {
		if e = appendRewriterRoot(m, &c.results, t); e != nil {
			return false, e
		}
		if c.proofGen {
			e = r.appendProof(ast.ASTHandle{})
		}
		return true, e
	}
	cache, e := r.mustCache(t)
	if e != nil {
		return false, e
	}
	if cache {
		h, e := c.getCached(t)
		if e != nil {
			return false, e
		}
		if !h.IsNull() {
			if e = r.sameSort(t, h); e != nil {
				return false, e
			}
			if e = appendRewriterRoot(m, &c.results, h); e != nil {
				return false, e
			}
			if c.proofGen {
				pr, e := c.getCachedProof(t)
				if e != nil {
					return false, e
				}
				if e = r.appendProof(pr); e != nil {
					return false, e
				}
			}
			return true, r.setNewChildIfDifferent(t, h)
		}
	}
	allowed, e := r.cfg.preVisit(t)
	if e != nil {
		return false, e
	}
	if !allowed {
		if e = appendRewriterRoot(m, &c.results, t); e != nil {
			return false, e
		}
		if c.proofGen {
			e = r.appendProof(ast.ASTHandle{})
		}
		return true, e
	}
	kind, e := m.GetKind(t)
	if e != nil {
		return false, e
	}
	switch kind {
	case ast.ASTVar:
		return true, r.processVar(t)
	case ast.ASTApp:
		n, e := m.GetNumArgs(t)
		if e != nil {
			return false, e
		}
		if n == 0 {
			return true, r.processConst(t)
		}
	case ast.ASTQuantifier:
	default:
		return false, ast.ErrASTDomain
	}
	if depth != 3 {
		depth--
	}
	return false, r.pushFrame(t, cache, depth)
}
func (r *rewriterEngine) processVar(t ast.ASTHandle) (err error) {
	c := &r.core
	m := c.manager
	done, e := r.cfg.reduceVar(t, r.r, r.pr)
	if e != nil {
		return e
	}
	if done {
		h, e := m.RefGet(r.r)
		if e != nil {
			return e
		}
		if e = appendRewriterRoot(m, &c.results, h); e != nil {
			return e
		}
		if c.proofGen {
			proof, e := m.RefGet(r.pr)
			if e != nil {
				return e
			}
			if e = r.appendProof(proof); e != nil {
				return e
			}
			if e = m.ResetRef(r.pr); e != nil {
				return e
			}
		}
		if e = r.sameSort(t, h); e != nil {
			return e
		}
		if e = r.setNewChild(t); e != nil {
			return e
		}
		return m.ResetRef(r.r)
	}
	if c.proofGen {
		if e = appendRewriterRoot(m, &c.results, t); e != nil {
			return e
		}
		return r.appendProof(ast.ASTHandle{})
	}
	index, e := m.GetVarIndex(t)
	if e != nil {
		return e
	}
	if index < uint32(len(r.bindings)) {
		i := uint32(len(r.bindings)) - index - 1
		h := r.bindings[i]
		if !h.IsNull() {
			if e = r.sameSort(t, h); e != nil {
				return e
			}
			ground, e := m.IsGround(h)
			if e != nil {
				return e
			}
			if !ground && r.shifts[i] != uint32(len(r.bindings)) {
				e = func() (err error) {
					tmp, e := m.NewRef(ast.ASTHandle{})
					if e != nil {
						return e
					}
					defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
					if e = r.shifter.ShiftBy(h, uint32(len(r.bindings))-r.shifts[i], tmp); e != nil {
						return e
					}
					value, e := m.RefGet(tmp)
					if e != nil {
						return e
					}
					return appendRewriterRoot(m, &c.results, value)
				}()
				if e != nil {
					return e
				}
				return r.setNewChild(t)
			}
			if e = appendRewriterRoot(m, &c.results, h); e != nil {
				return e
			}
			return r.setNewChild(t)
		}
	}
	return appendRewriterRoot(m, &c.results, t)
}
func (r *rewriterEngine) processConst(t ast.ASTHandle) error {
	c := &r.core
	m := c.manager
	d, e := m.GetDecl(t)
	if e != nil {
		return e
	}
	st, e := r.cfg.reduceApp(d, ast.AppArguments{}, r.r, r.pr)
	if e != nil {
		return e
	}
	if st == brDone {
		h, e := m.RefGet(r.r)
		if e != nil {
			return e
		}
		if e = r.sameSort(t, h); e != nil {
			return e
		}
		if e = appendRewriterRoot(m, &c.results, h); e != nil {
			return e
		}
		if c.proofGen {
			proof, e := m.RefGet(r.pr)
			if e != nil {
				return e
			}
			if proof.IsNull() {
				proof, e = m.MkProofRewrite(t, h)
				if e != nil {
					return e
				}
			}
			if e = r.appendProof(proof); e != nil {
				return e
			}
			if e = m.ResetRef(r.pr); e != nil {
				return e
			}
		}
		if e = m.ResetRef(r.r); e != nil {
			return e
		}
		return r.setNewChild(t)
	}
	if ast.ASTSourceDebug && st != brFailed {
		return ast.ErrASTDomain
	}
	if e = appendRewriterRoot(m, &c.results, t); e != nil {
		return e
	}
	if c.proofGen {
		return r.appendProof(ast.ASTHandle{})
	}
	return nil
}

// Complete one frame while preserving the source's explicit temporary-ref
// lifetime: delayed builtin and macro states retain m_r until its next use.
func (r *rewriterEngine) finish(fr rewriterFrame, unconditional, clear bool) error {
	c := &r.core
	m := c.manager
	h, e := m.RefGet(r.r)
	if e != nil {
		return e
	}
	if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, h); e != nil {
		return e
	}
	if e = r.cache(fr, h); e != nil {
		return e
	}
	c.frames = c.frames[:len(c.frames)-1]
	if unconditional {
		e = r.setNewChild(fr.curr)
	} else {
		e = r.setNewChildIfDifferent(fr.curr, h)
	}
	if e != nil {
		return e
	}
	if clear {
		return m.ResetRef(r.r)
	}
	return nil
}
func (r *rewriterEngine) rewritten(fr rewriterFrame, clear bool) error {
	c, m := &r.core, r.core.manager
	h, e := c.resultBack()
	if e != nil {
		return e
	}
	if e = m.SetRef(r.r, h); e != nil {
		return e
	}
	if e = c.popResult(); e != nil {
		return e
	}
	if e = c.popResult(); e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, h); e != nil {
		return e
	}
	if e = r.cache(fr, h); e != nil {
		return e
	}
	c.frames = c.frames[:len(c.frames)-1]
	if e = r.setNewChild(fr.curr); e != nil {
		return e
	}
	if clear {
		return m.ResetRef(r.r)
	}
	return nil
}
func (r *rewriterEngine) constantFold(frameIndex int) (bool, error) {
	c := &r.core
	m := c.manager
	fr := c.frames[frameIndex]
	if r.modelBooleanShortCircuit && fr.index() > 0 {
		helper := boolRewriter{manager: m}
		and, err := helper.isOperator(fr.curr, ast.ASTOpAnd)
		if err != nil {
			return false, err
		}
		or, err := helper.isOperator(fr.curr, ast.ASTOpOr)
		if err != nil {
			return false, err
		}
		if and || or {
			value, err := c.resultAt(fr.spos + fr.index() - 1)
			if err != nil {
				return false, err
			}
			decisive := and && m.IsFalse(value) || or && m.IsTrue(value)
			if decisive {
				if err := m.SetRef(r.r, value); err != nil {
					return false, err
				}
				return true, r.finish(fr, false, true)
			}
		}
	}
	if fr.index() != 1 {
		return false, nil
	}
	helper := boolRewriter{manager: m}
	ite, e := helper.isOperator(fr.curr, ast.ASTOpIte)
	if e != nil || !ite {
		return false, e
	}
	cond, e := c.resultAt(fr.spos)
	if e != nil {
		return false, e
	}
	truth, e := helper.isOperator(cond, ast.ASTOpTrue)
	if e != nil {
		return false, e
	}
	falsity, e := helper.isOperator(cond, ast.ASTOpFalse)
	if e != nil {
		return false, e
	}
	if !truth && !falsity {
		return false, nil
	}
	i := uint32(2)
	if truth {
		i = 1
	}
	arg, e := m.GetArg(fr.curr, i)
	if e != nil {
		return false, e
	}
	if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
		return false, e
	}
	if e = appendRewriterRoot(m, &c.results, arg); e != nil {
		return false, e
	}
	c.frames[frameIndex].setState(1)
	done, e := r.visit(arg, fr.maxDepth())
	if e != nil {
		return false, e
	}
	if done {
		if e = r.rewritten(fr, false); e != nil {
			return false, e
		}
	}
	return true, m.ResetRef(r.r)
}

func (r *rewriterEngine) processApp() (err error) {
	if r.core.proofGen {
		return r.processAppProof()
	}
	c := &r.core
	m := c.manager
	fi := len(c.frames) - 1
	fr := c.frames[fi]
	t := fr.curr
	n, e := m.GetNumArgs(t)
	if e != nil {
		return e
	}
	switch fr.state() {
	case 0:
		for c.frames[fi].index() < n {
			folded, e := r.constantFold(fi)
			if e != nil {
				return e
			}
			if folded {
				return nil
			}
			i := c.frames[fi].index()
			arg, e := m.GetArg(t, i)
			if e != nil {
				return e
			}
			c.frames[fi].setIndex(i + 1)
			done, e := r.visit(arg, fr.maxDepth())
			if e != nil {
				return e
			}
			if !done {
				return nil
			}
		}
		fr = c.frames[fi]
		d, e := m.GetDecl(t)
		if e != nil {
			return e
		}
		assoc, e := m.RewriterAssociative(d)
		if e != nil {
			return e
		}
		rc, e := m.GetRefCount(t)
		if e != nil {
			return e
		}
		if assoc && rc <= 1 && fi > 0 {
			prev := c.frames[fi-1]
			kind, e := m.GetKind(prev.curr)
			if e != nil {
				return e
			}
			if kind == ast.ASTApp && prev.state() == 0 {
				pd, e := m.GetDecl(prev.curr)
				if e != nil {
					return e
				}
				if pd == d {
					flat, e := r.cfg.flatAssoc(d)
					if e != nil {
						return e
					}
					if flat {
						c.frames = c.frames[:fi]
						return r.setNewChild(t)
					}
				}
			}
		}
		size := uint32(len(c.results))
		args, e := c.resultArguments(fr.spos, size-fr.spos)
		if e != nil {
			return e
		}
		st, e := r.cfg.reduceApp(d, args, r.r, r.pr2)
		if e != nil {
			return e
		}
		if st != brFailed {
			h, e := m.RefGet(r.r)
			if e != nil {
				return e
			}
			if e = r.sameSort(t, h); e != nil {
				return e
			}
			if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
				return e
			}
			if e = appendRewriterRoot(m, &c.results, h); e != nil {
				return e
			}
			if st == brDone {
				return r.finish(fr, true, true)
			}
			if st > brRewriteFull {
				return ast.ErrASTDomain
			}
			c.frames[fi].setState(1)
			depth := uint32(st)
			if depth != 3 {
				depth++
			}
			done, e := r.visit(h, depth)
			if e != nil {
				return e
			}
			if done {
				return r.rewritten(fr, true)
			}
			return m.ResetRef(r.r)
		}
		def, _, _, macro, e := r.cfg.getMacro(d)
		if e != nil {
			return e
		}
		if macro {
			if e = r.sameSort(t, def); e != nil {
				return e
			}
			ground, e := m.IsGround(def)
			if e != nil {
				return e
			}
			if ground {
				if e = m.SetRef(r.r, def); e != nil {
					return e
				}
			} else {
				if args.Count() != n {
					return ast.ErrASTDomain
				}
				c.frames[fi].setState(2)
				sz := uint32(len(r.bindings))
				for i := n; i > 0; {
					i--
					h, e := args.At(i)
					if e != nil {
						return e
					}
					r.pushBinding(h, sz)
				}
				if e = appendRewriterRoot(m, &c.results, def); e != nil {
					return e
				}
				if e = r.beginScope(); e != nil {
					return e
				}
				c.numQVars += n
				c.root = def
				return r.pushFrame(def, false, 3)
			}
		} else {
			h := t
			if fr.newChild() {
				h, e = m.RewriterMakeApp(d, args)
				if e != nil {
					return e
				}
			}
			if e = m.SetRef(r.r, h); e != nil {
				return e
			}
		}
		return r.finish(fr, false, true)
	case 1:
		return r.rewritten(fr, false)
	case 2:
		tmp, e := m.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		defer func() { err = errors.Join(err, m.DelRef(&tmp)) }()
		if n > uint32(len(r.bindings)) || n > uint32(len(r.shifts)) {
			return ast.ErrASTDomain
		}
		r.bindings = r.bindings[:uint32(len(r.bindings))-n]
		r.shifts = r.shifts[:uint32(len(r.shifts))-n]
		c.numQVars -= n
		if e = c.endScope(); e != nil {
			return e
		}
		h, e := c.resultBack()
		if e != nil {
			return e
		}
		if e = m.SetRef(r.r, h); e != nil {
			return e
		}
		ground, e := m.IsGround(h)
		if e != nil {
			return e
		}
		if !ground {
			if e = r.invShifter.InverseShift(h, n, tmp); e != nil {
				return e
			}
			h, e = m.RefGet(tmp)
			if e != nil {
				return e
			}
			if e = m.SetRef(r.r, h); e != nil {
				return e
			}
		}
		return r.finish(fr, true, false)
	default:
		return ast.ErrASTDomain // Source REWRITE_RULE is NOT_IMPLEMENTED_YET.
	}
}

func (r *rewriterEngine) processQuantifier() (err error) {
	if r.core.proofGen {
		return r.processQuantifierProof()
	}
	c := &r.core
	m := c.manager
	fi := len(c.frames) - 1
	fr := c.frames[fi]
	q := fr.curr
	nd, e := m.GetNumDeclarations(q)
	if e != nil {
		return e
	}
	np, e := m.GetNumPatterns(q)
	if e != nil {
		return e
	}
	nn, e := m.GetNumNoPatterns(q)
	if e != nil {
		return e
	}
	body, e := m.GetQuantifierBody(q)
	if e != nil {
		return e
	}
	if fr.index() == 0 {
		if e = r.beginScope(); e != nil {
			return e
		}
		c.root = body
		sz := uint32(len(r.bindings))
		for i := uint32(0); i < nd; i++ {
			r.pushBinding(ast.ASTHandle{}, sz)
		}
		c.numQVars += nd
	}
	children := uint32(1)
	if r.cfg.rewritePatterns() {
		children += np + nn
	}
	for c.frames[fi].index() < children {
		i := c.frames[fi].index()
		h := body
		if i > 0 {
			if i <= np {
				h, e = m.GetPattern(q, i-1)
			} else {
				h, e = m.GetNoPattern(q, i-1-np)
			}
			if e != nil {
				return e
			}
		}
		c.frames[fi].setIndex(i + 1)
		done, e := r.visit(h, fr.maxDepth())
		if e != nil {
			return e
		}
		if !done {
			return nil
		}
	}
	fr = c.frames[fi]
	if len(r.patterns) != 0 || len(r.noPatterns) != 0 {
		return ast.ErrASTPending
	}
	defer func() { err = errors.Join(err, r.releasePatterns()) }()
	for i := uint32(0); i < np; i++ {
		h, e := m.GetPattern(q, i)
		if e != nil {
			return e
		}
		if e = appendRewriterRoot(m, &r.patterns, h); e != nil {
			return e
		}
	}
	for i := uint32(0); i < nn; i++ {
		h, e := m.GetNoPattern(q, i)
		if e != nil {
			return e
		}
		if e = appendRewriterRoot(m, &r.noPatterns, h); e != nil {
			return e
		}
	}
	if r.cfg.rewritePatterns() {
		for i := uint32(0); i < np+nn; i++ {
			h, e := c.resultAt(fr.spos + 1 + i)
			if e != nil {
				return e
			}
			pattern, e := m.RewriterPattern(h)
			if e != nil {
				return e
			}
			if pattern {
				v, j := &r.patterns, i
				if i >= np {
					v, j = &r.noPatterns, i-np
				}
				if e = replaceRewriterRoot(m, v, j, h); e != nil {
					return e
				}
			}
		}
	}
	newBody, e := c.resultAt(fr.spos)
	if e != nil {
		return e
	}
	pa := ast.SliceArguments(r.patterns)
	na := ast.SliceArguments(r.noPatterns)
	done, e := r.cfg.reduceQuantifier(q, newBody, pa, na, r.r, r.pr)
	if e != nil {
		return e
	}
	if !done {
		h := q
		if fr.newChild() {
			h, e = m.UpdateQuantifierArguments(q, newBody, pa, na)
			if e != nil {
				return e
			}
		}
		if e = m.SetRef(r.r, h); e != nil {
			return e
		}
	}
	h, e := m.RefGet(r.r)
	if e != nil {
		return e
	}
	if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, h); e != nil {
		return e
	}
	if nd > uint32(len(r.bindings)) || nd > uint32(len(r.shifts)) {
		return ast.ErrASTDomain
	}
	r.bindings = r.bindings[:uint32(len(r.bindings))-nd]
	r.shifts = r.shifts[:uint32(len(r.shifts))-nd]
	if e = c.endScope(); e != nil {
		return e
	}
	if e = r.cache(fr, h); e != nil {
		return e
	}
	if e = m.ResetRef(r.r); e != nil {
		return e
	}
	c.frames = c.frames[:fi]
	// Source compares q with m_r after clearing m_r, so even reuse marks parent.
	return r.setNewChild(q)
}
func (r *rewriterEngine) checkCancel() error {
	if !r.core.cancelCheck {
		return nil
	}
	canceled, e := r.core.manager.Canceled()
	if e != nil {
		return e
	}
	if !canceled {
		return nil
	}
	message := r.core.manager.Limit.CancelMessage()
	if e = r.Reset(); e != nil {
		return e
	}
	return newRewriterException(message)
}
func (r *rewriterEngine) Rewrite(t ast.ASTHandle, out ast.ASTRef) error {
	return r.rewrite(t, out, ast.ASTRef{})
}
func (r *rewriterEngine) RewriteWithProof(t ast.ASTHandle, out, proof ast.ASTRef) error {
	if !r.core.proofGen {
		return ast.ErrASTDomain
	}
	return r.rewrite(t, out, proof)
}
func (r *rewriterEngine) rewrite(t ast.ASTHandle, out, proof ast.ASTRef) error {
	if e := r.open(); e != nil {
		return e
	}
	if e := r.checkCancel(); e != nil {
		return e
	}
	c := &r.core
	if len(c.frames) != 0 || c.cache != c.cacheStack[0] {
		return ast.ErrASTDomain
	}
	c.root = t
	c.numQVars = 0
	r.numSteps = 0
	done, e := r.visit(t, 3)
	if e != nil {
		return e
	}
	if done {
		return r.publish(out, proof)
	}
	return r.resume(out, proof)
}
func (r *rewriterEngine) Resume(out ast.ASTRef) error {
	return r.resume(out, ast.ASTRef{})
}
func (r *rewriterEngine) ResumeWithProof(out, proof ast.ASTRef) error {
	if !r.core.proofGen {
		return ast.ErrASTDomain
	}
	return r.resume(out, proof)
}
func (r *rewriterEngine) resume(out, proof ast.ASTRef) error {
	if e := r.open(); e != nil {
		return e
	}
	c := &r.core
	m := c.manager
	if len(c.frames) == 0 {
		return ast.ErrASTDomain
	}
	for len(c.frames) != 0 {
		if e := r.checkCancel(); e != nil {
			return e
		}
		r.numSteps++
		exceeded, err := r.cfg.maxStepsExceeded(r.numSteps)
		if err != nil {
			return err
		}
		if exceeded {
			return newRewriterException(messages.MaxSteps)
		}
		fr := c.frames[len(c.frames)-1]
		t := fr.curr
		if fr.state() == 0 && fr.index() == 0 && fr.cacheResult() {
			h, e := c.getCached(t)
			if e != nil {
				return e
			}
			if !h.IsNull() {
				if e = appendRewriterRoot(m, &c.results, h); e != nil {
					return e
				}
				if c.proofGen {
					pr, e := c.getCachedProof(t)
					if e != nil {
						return e
					}
					if e = r.appendProof(pr); e != nil {
						return e
					}
				}
				c.frames = c.frames[:len(c.frames)-1]
				if e = r.setNewChildIfDifferent(t, h); e != nil {
					return e
				}
				continue
			}
		}
		kind, e := m.GetKind(t)
		if e != nil {
			return e
		}
		switch kind {
		case ast.ASTApp:
			e = r.processApp()
		case ast.ASTQuantifier:
			e = r.processQuantifier()
		case ast.ASTVar:
			c.frames = c.frames[:len(c.frames)-1]
			e = r.processVar(t)
		default:
			e = ast.ErrASTDomain
		}
		if e != nil {
			return e
		}
	}
	return r.publish(out, proof)
}
func (r *rewriterEngine) publish(out, proof ast.ASTRef) error {
	m := r.core.manager
	h, e := r.core.resultBack()
	if e != nil {
		return e
	}
	if e = m.SetRef(out, h); e != nil {
		return e
	}
	if r.core.proofGen {
		pr, e := r.coreProofBack()
		if e != nil {
			return e
		}
		if pr.IsNull() {
			pr, e = m.MkProofReflexivity(r.core.root)
			if e != nil {
				return e
			}
		}
		if proof != (ast.ASTRef{}) {
			if e = m.SetRef(proof, pr); e != nil {
				return e
			}
		}
		if e = r.popProof(); e != nil {
			return e
		}
	}
	return r.core.popResult()
}

// rewriter_tpl overrides the base child assertion: builtin/macro frames may
// receive replacements that are not children of their original application.
func (r *rewriterEngine) setNewChild(old ast.ASTHandle) error {
	c := &r.core
	if e := c.open(); e != nil {
		return e
	}
	if len(c.frames) == 0 || c.frames[len(c.frames)-1].state() == 0 {
		return c.setNewChild(old)
	}
	c.frames[len(c.frames)-1].flags |= 2
	return nil
}
func (r *rewriterEngine) setNewChildIfDifferent(old, new ast.ASTHandle) error {
	if old == new {
		return r.core.open()
	}
	return r.setNewChild(old)
}

func (r *rewriterEngine) releasePatterns() error {
	m := r.core.manager
	if e := shrinkRewriterRoots(m, &r.noPatterns, 0); e != nil {
		return e
	}
	r.noPatterns = nil
	if e := shrinkRewriterRoots(m, &r.patterns, 0); e != nil {
		return e
	}
	r.patterns = nil
	return nil
}
