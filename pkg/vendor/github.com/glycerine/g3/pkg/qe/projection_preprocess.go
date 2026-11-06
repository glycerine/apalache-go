package qe

// Source: src/qe/qe_mbp.cpp common mbp::impl preprocessing stages.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// This is the concrete preprocessing member of ModelBasedProjection.
type projectionPreprocessor struct {
	self        *projectionPreprocessor
	manager     *ast.ASTManager
	config      Config
	rewriter    *rewriter.TheoryRewriter
	boolVisited map[ast.ASTHandle]bool
	pending     []*projectionPreprocessCall
	bytes       uint64
	closed      bool
}
type projectionPreprocessCall struct {
	owner        *projectionPreprocessor
	refs         []ast.ASTRef
	evaluator    *model.ModelEvaluator
	replacements []*rewriter.ExprSafeReplace
	contains     []*ast.ContainsApp
	marks        []*ast.ASTExprMark
	failures     []memory.ConstructionFailure
}
type projectionPreprocessFailure struct {
	owner          *projectionPreprocessor
	cause, cleanup error
	constructor    bool
}

func (e *projectionPreprocessFailure) Error() string        { return e.Unwrap().Error() }
func (e *projectionPreprocessFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *projectionPreprocessFailure) OriginalCause() error { return e.cause }
func (e *projectionPreprocessFailure) CleanupCause() error  { return e.cleanup }
func (e *projectionPreprocessFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *projectionPreprocessFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func newProjectionPreprocessor(m *ast.ASTManager, config Config) (result *projectionPreprocessor, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(projectionPreprocessor{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	p := &projectionPreprocessor{manager: m, config: config, bytes: bytes, boolVisited: make(map[ast.ASTHandle]bool)}
	p.self = p
	defer func() {
		if err != nil {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				p.pending = append(p.pending, &projectionPreprocessCall{owner: p, failures: []memory.ConstructionFailure{failure}})
			}
			if cleanup := p.Close(); cleanup != nil {
				result = p
				err = &projectionPreprocessFailure{owner: p, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	p.rewriter, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (p *projectionPreprocessor) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *projectionPreprocessor) begin() (*projectionPreprocessCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &projectionPreprocessCall{owner: p}
	p.pending = append(p.pending, c)
	return c, nil
}
func (c *projectionPreprocessCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *projectionPreprocessCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.contains) != 0 {
		i := len(c.contains) - 1
		if err := c.contains[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.contains = c.contains[:i]
	}
	for len(c.replacements) != 0 {
		i := len(c.replacements) - 1
		if err := c.replacements[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.replacements = c.replacements[:i]
	}
	for len(c.marks) != 0 {
		i := len(c.marks) - 1
		if err := c.marks[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.marks = c.marks[:i]
	}
	if c.evaluator != nil {
		if err := c.evaluator.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.evaluator = nil
	}
	for len(c.refs) != 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[i]); err != nil {
				c.retain(err)
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	return nil
}
func (p *projectionPreprocessor) drain() error {
	for len(p.pending) != 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *projectionPreprocessor) finish(c *projectionPreprocessCall, err *error) {
	c.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &projectionPreprocessFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *projectionPreprocessor) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	if p.rewriter != nil {
		if err := p.rewriter.Close(); err != nil {
			return err
		}
		p.rewriter = nil
	}
	clear(p.boolVisited)
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (c *projectionPreprocessCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *projectionPreprocessCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = c.ref(h)
	return h, err
}
func (c *projectionPreprocessCall) replacement() (*rewriter.ExprSafeReplace, error) {
	r, err := rewriter.NewExprSafeReplace(c.owner.manager)
	if r != nil {
		c.replacements = append(c.replacements, r)
	}
	return r, err
}
func (c *projectionPreprocessCall) startEvaluator(md *model.ModelCore) error {
	p := c.owner
	if md == nil || md.Manager() != p.manager {
		return ast.ErrASTHandle
	}
	var err error
	c.evaluator, err = model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	return err
}
func (c *projectionPreprocessCall) eval(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(r)
}
func (p *projectionPreprocessor) boolean(h ast.ASTHandle) (bool, error) {
	s, err := p.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := p.manager.MkBoolSort()
	return s == b, err
}
func (p *projectionPreprocessor) appendRef(forms *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := p.manager.NewRef(h)
	if err == nil {
		*forms = append(*forms, r)
	}
	return err
}
func (p *projectionPreprocessor) push(forms *[]ast.ASTRef, h ast.ASTHandle) error {
	if p.manager.IsTrue(h) {
		return nil
	}
	return p.appendRef(forms, h)
}
func (p *projectionPreprocessor) shrink(forms *[]ast.ASTRef, n int) error {
	for len(*forms) > n {
		i := len(*forms) - 1
		if (*forms)[i] != (ast.ASTRef{}) {
			if err := p.manager.DelRef(&(*forms)[i]); err != nil {
				return err
			}
		}
		*forms = (*forms)[:i]
	}
	return nil
}
func (p *projectionPreprocessor) erase(forms *[]ast.ASTRef, index *int) error {
	last := len(*forms) - 1
	h, err := p.manager.RefGet((*forms)[last])
	if err != nil {
		return err
	}
	if err = p.manager.SetRef((*forms)[*index], h); err != nil {
		return err
	}
	if err = p.shrink(forms, last); err != nil {
		return err
	}
	*index--
	return nil
}
func (p *projectionPreprocessor) emit(function string, line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", function, "src/qe/qe_mbp.cpp", line, func() bool { return true }, func(w io.Writer) error { return body(traceBodyWriter{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (p *projectionPreprocessor) displayForms(w io.Writer, forms []ast.ASTRef) error {
	return writeExpressionVector(p.manager, p.config, w, forms)
}

type projectionApplication struct {
	family ast.FamilyID
	op     int32
	args   []ast.ASTHandle
}

func (p *projectionPreprocessor) application(h ast.ASTHandle) (projectionApplication, error) {
	m := p.manager
	a := projectionApplication{family: ast.NullFamilyID, op: -1}
	kind, err := m.GetKind(h)
	if err != nil || kind != ast.ASTApp {
		return a, err
	}
	a.family, err = m.GetFamilyID(h)
	if err != nil {
		return a, err
	}
	a.op, err = m.GetDeclKind(h)
	if err != nil {
		return a, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return a, err
	}
	a.args = make([]ast.ASTHandle, n)
	for i := range a.args {
		a.args[i], err = m.GetArg(h, uint32(i))
		if err != nil {
			return a, err
		}
	}
	return a, nil
}
func (a projectionApplication) basic(op int32) bool { return a.family == 0 && a.op == op }

func (p *projectionPreprocessor) solveEqualities(vars, forms *[]ast.ASTRef) (changed bool, err error) {
	c, err := p.begin()
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if vars == nil || forms == nil {
		return false, ast.ErrASTHandle
	}
	if len(*vars) == 0 {
		return false, nil
	}
	m := p.manager
	selected := make(map[ast.ASTHandle]bool)
	removed := make(map[ast.ASTHandle]bool)
	for _, r := range *vars {
		h, e := m.RefGet(r)
		if e != nil {
			return false, e
		}
		if _, e = m.GetDecl(h); e != nil {
			return false, e
		}
		selected[h] = true
	}
	for i := 0; i < len(*forms); i++ {
		h, e := m.RefGet((*forms)[i])
		if e != nil {
			return changed, e
		}
		a, e := p.application(h)
		if e != nil {
			return changed, e
		}
		if !a.basic(ast.ASTOpEq) || len(a.args) != 2 {
			continue
		}
		left, right := a.args[0], a.args[1]
		if selected[right] {
			left, right = right, left
		}
		if !selected[left] {
			continue
		}
		cont, e := ast.NewContainsApp(m, left)
		if cont != nil {
			c.contains = append(c.contains, cont)
		}
		if e != nil {
			return changed, e
		}
		occurs, e := cont.Check(right)
		if e != nil {
			return changed, e
		}
		if occurs {
			continue
		}
		// Keep the erased equality's children live through substitution setup.
		if _, e = c.ref(h); e != nil {
			return changed, e
		}
		changed = true
		if e = p.erase(forms, &i); e != nil {
			return changed, e
		}
		sub, e := c.replacement()
		if e != nil {
			return changed, e
		}
		if e = sub.Insert(left, right); e != nil {
			return changed, e
		}
		removed[left] = true
		tmp, e := c.ref(ast.ASTHandle{})
		if e != nil {
			return changed, e
		}
		for _, r := range *forms {
			v, e := m.RefGet(r)
			if e != nil {
				return changed, e
			}
			if e = sub.Apply(v, tmp); e != nil {
				return changed, e
			}
			v, e = m.RefGet(tmp)
			if e != nil {
				return changed, e
			}
			if e = p.rewriter.Rewrite(v, tmp); e != nil {
				return changed, e
			}
			v, e = m.RefGet(tmp)
			if e != nil {
				return changed, e
			}
			if e = m.SetRef(r, v); e != nil {
				return changed, e
			}
		}
	}
	if changed {
		j := 0
		for i, r := range *vars {
			h, e := m.RefGet(r)
			if e != nil {
				return changed, e
			}
			if !removed[h] {
				if i != j {
					if e = m.SetRef((*vars)[j], h); e != nil {
						return changed, e
					}
				}
				j++
			}
		}
		if err = p.shrink(vars, j); err != nil {
			return changed, err
		}
	}
	return changed, nil
}

func (p *projectionPreprocessor) filterVariables(vars, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if vars == nil || forms == nil {
		return ast.ErrASTHandle
	}
	mark, err := ast.NewASTExprMark(p.manager)
	if mark != nil {
		c.marks = append(c.marks, mark)
	}
	if err != nil {
		return err
	}
	for _, r := range *forms {
		h, e := p.manager.RefGet(r)
		if e != nil {
			return e
		}
		if _, e = p.manager.GetNumExprsWithExprMark(h, mark); e != nil {
			return e
		}
	}
	j := 0
	for i, r := range *vars {
		h, e := p.manager.RefGet(r)
		if e != nil {
			return e
		}
		yes, e := mark.IsMarked(h)
		if e != nil {
			return e
		}
		if yes {
			if i != j {
				if e = p.manager.SetRef((*vars)[j], h); e != nil {
					return e
				}
			}
			j++
		}
	}
	return p.shrink(vars, j)
}

func (p *projectionPreprocessor) projectBooleans(md *model.ModelCore, vars, forms *[]ast.ASTRef) (err error) {
	c, err := p.begin()
	if err != nil {
		return err
	}
	defer p.finish(c, &err)
	if vars == nil || forms == nil {
		return ast.ErrASTHandle
	}
	if err = c.startEvaluator(md); err != nil {
		return err
	}
	sub, err := c.replacement()
	if err != nil {
		return err
	}
	m := p.manager
	j := 0
	for i, r := range *vars {
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		yes, e := p.boolean(h)
		if e != nil {
			return e
		}
		if yes {
			v, e := c.eval(h)
			if e != nil {
				return e
			}
			if e = sub.Insert(h, v); e != nil {
				return e
			}
		} else {
			if j != i {
				if e = m.SetRef((*vars)[j], h); e != nil {
					return e
				}
			}
			j++
		}
	}
	if j == len(*vars) {
		return nil
	}
	if err = p.shrink(vars, j); err != nil {
		return err
	}
	j = 0
	for i, r := range *forms {
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		out, e := c.ref(ast.ASTHandle{})
		if e != nil {
			return e
		}
		if e = sub.Apply(h, out); e != nil {
			return e
		}
		v, e := m.RefGet(out)
		if e != nil {
			return e
		}
		if e = p.rewriter.Rewrite(v, out); e != nil {
			return e
		}
		v, e = m.RefGet(out)
		if e != nil {
			return e
		}
		if !m.IsTrue(v) {
			if e = p.emit("project_bools", 285, func(w io.Writer) error {
				if e := writeExpression(m, p.config, w, h); e != nil {
					return e
				}
				_, _ = io.WriteString(w, " -> ")
				if e := writeExpression(m, p.config, w, v); e != nil {
					return e
				}
				_, _ = io.WriteString(w, "\n")
				return nil
			}); e != nil {
				return e
			}
			if e = m.SetRef((*forms)[i], v); e != nil {
				return e
			}
			if j != i {
				if e = m.SetRef((*forms)[j], v); e != nil {
					return e
				}
			}
			j++
		}
	}
	return p.shrink(forms, j)
}
