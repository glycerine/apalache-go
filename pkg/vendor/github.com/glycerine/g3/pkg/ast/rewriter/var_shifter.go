package rewriter

// Source: src/ast/rewriter/rewriter.cpp:222-416;
// src/ast/rewriter/rewriter.h:123-191.

import (
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
)

// Concrete forward/inverse source specializations share only their original
// var_shifter_core traversal. The base is constructed inline and the complete
// cold derived header is charged once; hot frames remain pointer-free.

type variableShifter struct {
	self                         *variableShifter
	core                         rewriterCore
	inverse                      bool
	bound, shift1, shift2, shift uint32
	header                       uint64
	closed                       bool
}

func newVarShifter(m *ast.ASTManager) (*variableShifter, error) { return newVariableShifter(m, false) }
func newInvVarShifter(m *ast.ASTManager) (*variableShifter, error) {
	return newVariableShifter(m, true)
}
func newVariableShifter(m *ast.ASTManager, inverse bool) (*variableShifter, error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if _, e := m.ProofsEnabled(); e != nil {
		return nil, e
	}
	bytes := uint64(reflect.TypeOf(variableShifter{}).Size())
	if e := m.BudgetField.Reserve(bytes); e != nil {
		return nil, e
	}
	s := &variableShifter{header: bytes}
	return s, s.init(m, inverse)
}
func (s *variableShifter) init(m *ast.ASTManager, inverse bool) error {
	s.self = s
	s.inverse = inverse
	return s.core.init(m, false)
}
func (s *variableShifter) open() error {
	if s == nil || s.self != s || s.closed {
		return ast.ErrASTHandle
	}
	return s.core.open()
}
func (s *variableShifter) Shift(t ast.ASTHandle, bound, shift1, shift2 uint32, out ast.ASTRef) (err error) {
	if e := s.open(); e != nil {
		return e
	}
	if s.inverse {
		return ast.ErrASTDomain
	}
	defer func() {
		if err != nil {
			s.core.cause = err
		}
	}()
	ground, e := s.core.manager.IsGround(t)
	if e != nil {
		return e
	}
	if ground {
		return s.core.manager.SetRef(out, t)
	}
	if e = s.core.resetCache(); e != nil {
		return e
	}
	s.bound = bound
	s.shift1 = shift1
	s.shift2 = shift2
	return s.mainLoop(t, out)
}
func (s *variableShifter) ShiftBy(t ast.ASTHandle, shift uint32, out ast.ASTRef) error {
	return s.Shift(t, 0, shift, 0, out)
}
func (s *variableShifter) InverseShift(t ast.ASTHandle, shift uint32, out ast.ASTRef) (err error) {
	if e := s.open(); e != nil {
		return e
	}
	if !s.inverse {
		return ast.ErrASTDomain
	}
	defer func() {
		if err != nil {
			s.core.cause = err
		}
	}()
	ground, e := s.core.manager.IsGround(t)
	if e != nil {
		return e
	}
	if ground {
		return s.core.manager.SetRef(out, t)
	}
	if e = s.core.resetCache(); e != nil {
		return e
	}
	s.shift = shift
	return s.mainLoop(t, out)
}
func (s *variableShifter) processVar(v ast.ASTHandle) error {
	c := &s.core
	m := c.manager
	index, e := m.GetVarIndex(v)
	if e != nil {
		return e
	}
	if index < c.numQVars {
		return appendRewriterRoot(m, &c.results, v)
	}
	if s.inverse {
		if ast.ASTSourceDebug && index < c.numQVars+s.shift {
			return ast.ErrASTDomain
		}
		index -= s.shift
	} else {
		freeIndex := index - c.numQVars
		if freeIndex >= s.bound {
			index += s.shift1
		} else {
			index += s.shift2
		}
	}
	sort, e := m.GetSort(v)
	if e != nil {
		return e
	}
	newVar, e := m.MkVar(index, sort)
	if e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, newVar); e != nil {
		return e
	}
	return c.setNewChild(v)
}
func (s *variableShifter) visit(t ast.ASTHandle) (bool, error) {
	c := &s.core
	m := c.manager
	ground, e := m.IsGround(t)
	if e != nil {
		return false, e
	}
	if ground {
		return true, appendRewriterRoot(m, &c.results, t)
	}
	cache, e := c.mustCache(t)
	if e != nil {
		return false, e
	}
	if cache {
		r, e := c.getCached(t)
		if e != nil {
			return false, e
		}
		if !r.IsNull() {
			if e = appendRewriterRoot(m, &c.results, r); e != nil {
				return false, e
			}
			return true, c.setNewChildIfDifferent(t, r)
		}
	}
	kind, e := m.GetKind(t)
	if e != nil {
		return false, e
	}
	switch kind {
	case ast.ASTApp, ast.ASTQuantifier:
		// The inherited push_frame(t,c) source overload takes c as its state argument
		// and recomputes must_cache. That unused 0/1 state is preserved here.
		state := uint32(0)
		if cache {
			state = 1
		}
		return false, c.pushFrame(t, state)
	case ast.ASTVar:
		return true, s.processVar(t)
	default:
		return false, ast.ErrASTDomain
	}
}
func (c *rewriterCore) resultArguments(offset, count uint32) (ast.AppArguments, error) {
	if uint64(offset) > uint64(len(c.results)) || uint64(count) > uint64(len(c.results))-uint64(offset) {
		return ast.AppArguments{}, ast.ErrASTDomain
	}
	return ast.SliceArguments(c.results[offset : offset+count]), nil
}

func (s *variableShifter) rebuildApp(t ast.ASTHandle, fr rewriterFrame, n uint32) (ast.ASTHandle, error) {
	c := &s.core
	m := c.manager
	decl, e := m.GetDecl(t)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	args, e := c.resultArguments(fr.spos, n)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.RewriterMakeApp(decl, args)
}

func (s *variableShifter) processApp() error {
	c := &s.core
	m := c.manager
	frameIndex := len(c.frames) - 1
	t := c.frames[frameIndex].curr
	n, e := m.GetNumArgs(t)
	if e != nil {
		return e
	}
	for c.frames[frameIndex].index() < n {
		i := c.frames[frameIndex].index()
		arg, e := m.GetArg(t, i)
		if e != nil {
			return e
		}
		c.frames[frameIndex].setIndex(i + 1)
		done, e := s.visit(arg)
		if e != nil {
			return e
		}
		if !done {
			return nil
		}
	}
	fr := c.frames[frameIndex]
	size := uint32(len(c.results))
	if size != fr.spos+n {
		return ast.ErrASTDomain
	}
	newTerm := t
	if fr.newChild() {
		newTerm, e = s.rebuildApp(t, fr, n)
		if e != nil {
			return e
		}
	}
	if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, newTerm); e != nil {
		return e
	}
	c.frames = c.frames[:frameIndex]
	if e = c.setNewChildIfDifferent(t, newTerm); e != nil {
		return e
	}
	if fr.cacheResult() {
		return c.cacheResult(t, newTerm)
	}
	return nil
}
func (s *variableShifter) quantifierChild(q ast.ASTHandle, i, patterns uint32) (ast.ASTHandle, error) {
	m := s.core.manager
	if i == 0 {
		return m.GetQuantifierBody(q)
	}
	if i <= patterns {
		return m.GetPattern(q, i-1)
	}
	return m.GetNoPattern(q, i-patterns-1)
}
func (s *variableShifter) rebuildQuantifier(q ast.ASTHandle, fr rewriterFrame, patterns, noPatterns uint32) (ast.ASTHandle, error) {
	c := &s.core
	m := c.manager
	body, e := c.resultAt(fr.spos)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	pats, e := c.resultArguments(fr.spos+1, patterns)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	noPats, e := c.resultArguments(fr.spos+1+patterns, noPatterns)
	if e != nil {
		return ast.ASTHandle{}, e
	}
	return m.UpdateQuantifierArguments(q, body, pats, noPats)
}
func (s *variableShifter) processQuantifier() error {
	c := &s.core
	m := c.manager
	frameIndex := len(c.frames) - 1
	q := c.frames[frameIndex].curr
	if c.frames[frameIndex].index() == 0 {
		if e := c.beginScope(); e != nil {
			return e
		}
		n, e := m.GetNumDeclarations(q)
		if e != nil {
			return e
		}
		c.numQVars += n
		body, e := m.GetQuantifierBody(q)
		if e != nil {
			return e
		}
		c.root = body
	}
	patterns, e := m.GetNumPatterns(q)
	if e != nil {
		return e
	}
	noPatterns, e := m.GetNumNoPatterns(q)
	if e != nil {
		return e
	}
	n := 1 + patterns + noPatterns
	for c.frames[frameIndex].index() < n {
		i := c.frames[frameIndex].index()
		child, e := s.quantifierChild(q, i, patterns)
		if e != nil {
			return e
		}
		c.frames[frameIndex].setIndex(i + 1)
		done, e := s.visit(child)
		if e != nil {
			return e
		}
		if !done {
			return nil
		}
	}
	fr := c.frames[frameIndex]
	size := uint32(len(c.results))
	if size != fr.spos+n {
		return ast.ErrASTDomain
	}
	newTerm := q
	if fr.newChild() {
		newTerm, e = s.rebuildQuantifier(q, fr, patterns, noPatterns)
		if e != nil {
			return e
		}
	}
	if e = shrinkRewriterRoots(m, &c.results, fr.spos); e != nil {
		return e
	}
	if e = appendRewriterRoot(m, &c.results, newTerm); e != nil {
		return e
	}
	c.frames = c.frames[:frameIndex]
	if e = c.setNewChildIfDifferent(q, newTerm); e != nil {
		return e
	}
	if e = c.endScope(); e != nil {
		return e
	}
	if fr.cacheResult() {
		return c.cacheResult(q, newTerm)
	}
	return nil
}
func (s *variableShifter) publishResult(out ast.ASTRef) error {
	c := &s.core
	m := c.manager
	n := uint32(len(c.results))
	if n != 1 {
		return ast.ErrASTDomain
	}
	h, e := c.resultAt(n - 1)
	if e != nil {
		return e
	}
	if e = m.SetRef(out, h); e != nil {
		return e
	}
	return c.popResult()
}
func (s *variableShifter) mainLoop(t ast.ASTHandle, out ast.ASTRef) error {
	c := &s.core
	m := c.manager
	n := uint32(len(c.results))
	if len(c.cacheStack) == 0 || c.cache != c.cacheStack[0] || len(c.frames) != 0 || n != 0 {
		return ast.ErrASTDomain
	}
	c.root = t
	done, e := s.visit(t)
	if e != nil {
		return e
	}
	if done {
		return s.publishResult(out)
	}
	for len(c.frames) != 0 {
		fr := c.frames[len(c.frames)-1]
		t := fr.curr
		if fr.index() == 0 && fr.cacheResult() {
			r, e := c.getCached(t)
			if e != nil {
				return e
			}
			if !r.IsNull() {
				if e = appendRewriterRoot(m, &c.results, r); e != nil {
					return e
				}
				c.frames = c.frames[:len(c.frames)-1]
				if e = c.setNewChildIfDifferent(t, r); e != nil {
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
			e = s.processApp()
		case ast.ASTQuantifier:
			e = s.processQuantifier()
		default:
			e = ast.ErrASTDomain
		}
		if e != nil {
			return e
		}
	}
	return s.publishResult(out)
}
func (s *variableShifter) Close() error {
	if s == nil || s.self != s {
		return ast.ErrASTHandle
	}
	if s.closed {
		return nil
	}
	if e := s.core.Close(); e != nil {
		return e
	}
	s.core.manager.BudgetField.Release(s.header)
	s.header = 0
	s.closed = true
	return nil
}
