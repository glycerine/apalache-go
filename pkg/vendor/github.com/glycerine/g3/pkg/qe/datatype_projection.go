package qe

// Source: src/qe/qe_datatypes.cpp/.h datatype_project_plugin.

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

// DatatypeProjection is the concrete model-guided constructor projector.
// The manager/config are borrowed. Model interpretations and completed writes
// to the caller's formula/variable vectors remain owned by those callers.
type DatatypeProjection struct {
	self     *DatatypeProjection
	manager  *ast.ASTManager
	config   Config
	family   ast.FamilyID
	value    ast.ASTRef
	variable *ast.ContainsApp
	pending  []*datatypeProjectionCall
	bytes    uint64
	closed   bool
}
type datatypeProjectionCall struct {
	owner        *DatatypeProjection
	refs         []ast.ASTRef
	evaluator    *model.ModelEvaluator
	replacements []*rewriter.ExprSafeReplace
	rewriters    []*rewriter.TheoryRewriter
	failures     []memory.ConstructionFailure
}
type DatatypeProjectionFailure struct {
	owner          *DatatypeProjection
	cause, cleanup error
	constructor    bool
}

func (e *DatatypeProjectionFailure) Error() string        { return e.Unwrap().Error() }
func (e *DatatypeProjectionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *DatatypeProjectionFailure) OriginalCause() error { return e.cause }
func (e *DatatypeProjectionFailure) CleanupCause() error  { return e.cleanup }
func (e *DatatypeProjectionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *DatatypeProjectionFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func NewDatatypeProjection(m *ast.ASTManager, config Config) (result *DatatypeProjection, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	family, err := m.DatatypeFamilyID()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(DatatypeProjection{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	p := &DatatypeProjection{manager: m, config: config, family: family, bytes: bytes}
	p.self = p
	defer func() {
		if err != nil {
			if cleanup := p.Close(); cleanup != nil {
				result = p
				err = &DatatypeProjectionFailure{owner: p, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	p.value, err = m.NewRef(ast.ASTHandle{})
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (p *DatatypeProjection) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *DatatypeProjection) FamilyID() (ast.FamilyID, error) {
	if err := p.open(); err != nil {
		return ast.NullFamilyID, err
	}
	return p.family, nil
}
func (p *DatatypeProjection) begin() (*datatypeProjectionCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &datatypeProjectionCall{owner: p}
	p.pending = append(p.pending, c)
	return c, nil
}
func (c *datatypeProjectionCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *datatypeProjectionCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.rewriters) != 0 {
		i := len(c.rewriters) - 1
		if err := c.rewriters[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.rewriters = c.rewriters[:i]
	}
	for len(c.replacements) != 0 {
		i := len(c.replacements) - 1
		if err := c.replacements[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.replacements = c.replacements[:i]
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
func (p *DatatypeProjection) drain() error {
	for len(p.pending) != 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *DatatypeProjection) finish(c *datatypeProjectionCall, err *error) {
	c.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &DatatypeProjectionFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *DatatypeProjection) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	if p.variable != nil {
		if err := p.variable.Close(); err != nil {
			return err
		}
		p.variable = nil
	}
	if p.value != (ast.ASTRef{}) {
		if err := p.manager.DelRef(&p.value); err != nil {
			return err
		}
	}
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (c *datatypeProjectionCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *datatypeProjectionCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = c.ref(h)
	return h, err
}
func (c *datatypeProjectionCall) startEvaluator(md *model.ModelCore) error {
	p := c.owner
	if md == nil || md.Manager() != p.manager {
		return ast.ErrASTHandle
	}
	var err error
	c.evaluator, err = model.NewModelEvaluator(md, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	return err
}
func (c *datatypeProjectionCall) eval(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(r)
}
func (p *DatatypeProjection) appendRef(forms *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := p.manager.NewRef(h)
	if err == nil {
		*forms = append(*forms, r)
	}
	return err
}
func (p *DatatypeProjection) erase(forms *[]ast.ASTRef, index *int) error {
	last := len(*forms) - 1
	h, err := p.manager.RefGet((*forms)[last])
	if err != nil {
		return err
	}
	if err = p.manager.SetRef((*forms)[*index], h); err != nil {
		return err
	}
	if err = p.manager.DelRef(&(*forms)[last]); err != nil {
		return err
	}
	*forms = (*forms)[:last]
	*index--
	return nil
}
func (p *DatatypeProjection) emit(function string, line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", function, "src/qe/qe_datatypes.cpp", line, func() bool { return true }, func(w io.Writer) error { return body(traceBodyWriter{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (p *DatatypeProjection) datatype(h ast.ASTHandle) (bool, error) {
	s, err := p.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	f, err := p.manager.GetFamilyID(s)
	return f == p.family, err
}

func (p *DatatypeProjection) ProjectVariable(md *model.ModelCore, variable ast.ASTHandle, additional, forms *[]ast.ASTRef) (success bool, err error) {
	c, err := p.begin()
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if additional == nil || forms == nil {
		return false, ast.ErrASTHandle
	}
	if _, err = p.manager.GetDecl(variable); err != nil {
		return false, err
	}
	yes, err := p.datatype(variable)
	if err != nil {
		return false, err
	}
	if !yes {
		return false, ast.ErrASTDomain
	}
	if _, err = c.ref(variable); err != nil {
		return false, err
	}
	if err = c.startEvaluator(md); err != nil {
		return false, err
	}
	value, err := c.eval(variable)
	if err != nil {
		return false, err
	}
	if _, err = p.manager.GetDecl(value); err != nil {
		return false, err
	}
	if err = p.emit("operator()", 48, func(w io.Writer) error {
		if e := writeExpression(p.manager, p.config, w, variable); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " := ")
		if e := writeExpression(p.manager, p.config, w, value); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return false, err
	}
	if err = p.manager.SetRef(p.value, value); err != nil {
		return false, err
	}
	_, constructor, err := p.manager.DatatypeConstructorApp(value)
	if err != nil {
		return false, err
	}
	if !constructor {
		return true, nil
	}
	if p.variable != nil {
		if err = p.variable.Close(); err != nil {
			return false, err
		}
		p.variable = nil
	}
	p.variable, err = ast.NewContainsApp(p.manager, variable)
	if err != nil {
		return false, err
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return false, err
	}
	recursive, err := p.manager.DatatypeIsRecursive(sort)
	if err != nil {
		return false, err
	}
	if recursive {
		err = c.projectRecursive(md, additional, forms)
	} else {
		err = c.projectNonrecursive(md, additional, forms)
	}
	return err == nil, err
}
func (c *datatypeProjectionCall) projectNonrecursive(md *model.ModelCore, additional, forms *[]ast.ASTRef) error {
	p := c.owner
	m := p.manager
	value, err := m.RefGet(p.value)
	if err != nil {
		return err
	}
	decl, yes, err := m.DatatypeConstructorApp(value)
	if err != nil {
		return err
	}
	if !yes {
		return ast.ErrASTDomain
	}
	accessors, err := m.DatatypeConstructorAccessors(decl)
	if err != nil {
		return err
	}
	args := make([]ast.ASTHandle, 0, len(accessors))
	for i, a := range accessors {
		name, e := m.GetASTName(a)
		if e != nil {
			return e
		}
		prefix, e := m.Symbols.String(name)
		if e != nil {
			return e
		}
		sort, e := m.GetRange(a)
		if e != nil {
			return e
		}
		arg, e := c.hold(m.MkFreshConst(prefix, sort))
		if e != nil {
			return e
		}
		d, e := m.GetDecl(arg)
		if e != nil {
			return e
		}
		v, e := m.GetArg(value, uint32(i))
		if e != nil {
			return e
		}
		if e = md.RegisterConstant(d, v); e != nil {
			return e
		}
		args = append(args, arg)
	}
	rhs, err := c.hold(m.MkApp(decl, args))
	if err != nil {
		return err
	}
	target, err := p.variable.Target()
	if err != nil {
		return err
	}
	if err = p.emit("project_nonrec", 85, func(w io.Writer) error {
		if e := writeExpression(m, p.config, w, target); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " |-> ")
		if e := writeExpression(m, p.config, w, rhs); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	// Source deliberately does not append the fresh field constants to vars.
	return c.reduce(rhs, forms)
}
func (c *datatypeProjectionCall) reduce(rhs ast.ASTHandle, forms *[]ast.ASTRef) error {
	p := c.owner
	m := p.manager
	target, err := p.variable.Target()
	if err != nil {
		return err
	}
	sub, err := rewriter.NewExprSafeReplace(m)
	if sub != nil {
		c.replacements = append(c.replacements, sub)
	}
	if err != nil {
		return err
	}
	rw, err := rewriter.NewTheoryRewriter(m, p.config.Context, p.config.Globals, nil)
	if rw != nil {
		c.rewriters = append(c.rewriters, rw)
	}
	if err != nil {
		return err
	}
	if err = sub.Insert(target, rhs); err != nil {
		return err
	}
	if err = p.emit("reduce", 113, func(w io.Writer) error {
		if e := writeExpression(m, p.config, w, target); e != nil {
			return e
		}
		_, _ = io.WriteString(w, " = ")
		if e := writeExpression(m, p.config, w, rhs); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		if e := writeExpressionVector(m, p.config, w, *forms); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return err
	}
	out, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return err
	}
	for _, r := range *forms {
		h, e := m.RefGet(r)
		if e != nil {
			return e
		}
		if e = sub.Apply(h, out); e != nil {
			return e
		}
		h, e = m.RefGet(out)
		if e != nil {
			return e
		}
		if e = rw.Rewrite(h, out); e != nil {
			return e
		}
		h, e = m.RefGet(out)
		if e != nil {
			return e
		}
		if e = m.SetRef(r, h); e != nil {
			return e
		}
	}
	return nil
}
