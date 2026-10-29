package qe

// Source: src/qe/qe_arrays.cpp/.h enabled array_project_plugin paths.

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

// ArrayProjection follows the retained source equality/select algorithm. The
// commented-out rw_cfg and its NOT_IMPLEMENTED_YET comparator are unreachable
// and are deliberately not represented by an invented Go rewrite adapter.
type ArrayProjection struct {
	self     *ArrayProjection
	manager  *ast.ASTManager
	config   Config
	family   ast.FamilyID
	variable *ast.ContainsApp
	pending  []*arrayProjectionCall
	bytes    uint64
	closed   bool
}
type arrayProjectionCall struct {
	owner        *ArrayProjection
	model        *model.ModelCore
	refs         []ast.ASTRef
	evaluator    *model.ModelEvaluator
	replacements []*rewriter.ExprSafeReplace
	failures     []memory.ConstructionFailure
}
type ArrayProjectionFailure struct {
	owner          *ArrayProjection
	cause, cleanup error
}

func (e *ArrayProjectionFailure) Error() string        { return e.Unwrap().Error() }
func (e *ArrayProjectionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ArrayProjectionFailure) OriginalCause() error { return e.cause }
func (e *ArrayProjectionFailure) CleanupCause() error  { return e.cleanup }
func (e *ArrayProjectionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *ArrayProjectionFailure) Cleanup() error { return e.owner.drain() }

func NewArrayProjection(m *ast.ASTManager, config Config) (*ArrayProjection, error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err := config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	family, err := m.ArrayFamilyID()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(ArrayProjection{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	p := &ArrayProjection{manager: m, config: config, family: family, bytes: bytes}
	p.self = p
	return p, nil
}
func (p *ArrayProjection) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (p *ArrayProjection) FamilyID() (ast.FamilyID, error) {
	if err := p.open(); err != nil {
		return ast.NullFamilyID, err
	}
	return p.family, nil
}
func (p *ArrayProjection) begin(md *model.ModelCore) (*arrayProjectionCall, error) {
	if err := p.open(); err != nil {
		return nil, err
	}
	if md == nil || md.Manager() != p.manager {
		return nil, ast.ErrASTHandle
	}
	if err := p.drain(); err != nil {
		return nil, err
	}
	c := &arrayProjectionCall{owner: p, model: md}
	p.pending = append(p.pending, c)
	return c, nil
}
func (c *arrayProjectionCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *arrayProjectionCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
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
func (p *ArrayProjection) drain() error {
	for len(p.pending) != 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *ArrayProjection) finish(c *arrayProjectionCall, err *error) {
	c.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &ArrayProjectionFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *ArrayProjection) Close() error {
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
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (c *arrayProjectionCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *arrayProjectionCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = c.ref(h)
	return h, err
}
func (c *arrayProjectionCall) startEvaluator() error {
	if c.evaluator != nil {
		return nil
	}
	p := c.owner
	var err error
	c.evaluator, err = model.NewModelEvaluator(c.model, model.ModelEvaluatorConfig{Context: p.config.Context, Globals: p.config.Globals, Memory: p.manager.NumeralService().RuntimeOwner(), Cooperation: p.config.Cooperation, Trace: p.config.Trace})
	return err
}
func (c *arrayProjectionCall) eval(h ast.ASTHandle) (ast.ASTHandle, error) {
	if err := c.startEvaluator(); err != nil {
		return ast.ASTHandle{}, err
	}
	r, err := c.ref(ast.ASTHandle{})
	if err != nil {
		return ast.ASTHandle{}, err
	}
	if err = c.evaluator.Evaluate(h, r); err != nil {
		return ast.ASTHandle{}, err
	}
	return c.owner.manager.RefGet(r)
}
func (c *arrayProjectionCall) replacement() (*rewriter.ExprSafeReplace, error) {
	r, err := rewriter.NewExprSafeReplace(c.owner.manager)
	if r != nil {
		c.replacements = append(c.replacements, r)
	}
	return r, err
}
func (p *ArrayProjection) appendRef(v *[]ast.ASTRef, h ast.ASTHandle) error {
	r, err := p.manager.NewRef(h)
	if err == nil {
		*v = append(*v, r)
	}
	return err
}
func (p *ArrayProjection) shrink(v *[]ast.ASTRef, n int) error {
	for len(*v) > n {
		i := len(*v) - 1
		if (*v)[i] != (ast.ASTRef{}) {
			if err := p.manager.DelRef(&(*v)[i]); err != nil {
				return err
			}
		}
		*v = (*v)[:i]
	}
	return nil
}
func (p *ArrayProjection) erase(v *[]ast.ASTRef, i *int) error {
	h, err := p.manager.RefGet((*v)[len(*v)-1])
	if err != nil {
		return err
	}
	if err = p.manager.SetRef((*v)[*i], h); err != nil {
		return err
	}
	if err = p.shrink(v, len(*v)-1); err != nil {
		return err
	}
	*i--
	return nil
}
func (p *ArrayProjection) operation(h ast.ASTHandle, family ast.FamilyID, op int32) ([]ast.ASTHandle, bool, error) {
	m := p.manager
	k, err := m.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return nil, false, err
	}
	fid, err := m.GetFamilyID(h)
	if err != nil || fid != family {
		return nil, false, err
	}
	kind, err := m.GetDeclKind(h)
	if err != nil || kind != op {
		return nil, false, err
	}
	n, err := m.GetNumArgs(h)
	if err != nil {
		return nil, false, err
	}
	args := make([]ast.ASTHandle, n)
	for i := range args {
		args[i], err = m.GetArg(h, uint32(i))
		if err != nil {
			return nil, false, err
		}
	}
	return args, true, nil
}
func (p *ArrayProjection) emit(line int, body func(io.Writer) error) error {
	tags := p.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional("qe", "operator()", "src/qe/qe_arrays.cpp", line, func() bool { return true }, func(w io.Writer) error { return body(traceBodyWriter{w}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (p *ArrayProjection) Solve(md *model.ModelCore, vars, lits *[]ast.ASTRef) (bool, error) {
	if err := p.open(); err != nil {
		return false, err
	}
	if md == nil || md.Manager() != p.manager || vars == nil || lits == nil {
		return false, ast.ErrASTHandle
	}
	return false, nil
}

func (p *ArrayProjection) ProjectVariable(md *model.ModelCore, variable ast.ASTHandle, additional, lits *[]ast.ASTRef) (success bool, err error) {
	c, err := p.begin(md)
	if err != nil {
		return false, err
	}
	defer p.finish(c, &err)
	if additional == nil || lits == nil {
		return false, ast.ErrASTHandle
	}
	if _, err = p.manager.GetDecl(variable); err != nil {
		return false, err
	}
	sort, err := p.manager.GetSort(variable)
	if err != nil {
		return false, err
	}
	family, err := p.manager.GetFamilyID(sort)
	if err != nil {
		return false, err
	}
	if family != p.family {
		return false, ast.ErrASTDomain
	}
	if err = p.emit(119, func(w io.Writer) error {
		if e := writeExpression(p.manager, p.config, w, variable); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return writeExpressionVector(p.manager, p.config, w, *lits)
	}); err != nil {
		return false, err
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
	if solved, e := c.solveEquality(additional, lits); e != nil {
		return false, e
	} else if solved {
		return true, nil
	}
	selects, ok, err := c.checkDisequalities(lits)
	if err != nil {
		return false, err
	}
	if !ok {
		if err = p.emit(135, func(w io.Writer) error {
			_, _ = io.WriteString(w, "Could not project ")
			if e := writeExpression(p.manager, p.config, w, variable); e != nil {
				return e
			}
			_, _ = io.WriteString(w, " for:\n")
			if e := writeExpressionVector(p.manager, p.config, w, *lits); e != nil {
				return e
			}
			_, _ = io.WriteString(w, "\n")
			return nil
		}); err != nil {
			return false, err
		}
		return false, nil
	}
	for i := 0; i < len(*lits); i++ {
		h, e := p.manager.RefGet((*lits)[i])
		if e != nil {
			return false, e
		}
		yes, e := p.disequality(h)
		if e != nil {
			return false, e
		}
		if yes {
			if e = p.erase(lits, &i); e != nil {
				return false, e
			}
		}
	}
	if err = c.ackermannize(selects, additional, lits); err != nil {
		return false, err
	}
	if err = p.emit(148, func(w io.Writer) error {
		for _, h := range selects {
			_, _ = io.WriteString(w, "\n")
			if e := writeExpression(p.manager, p.config, w, h); e != nil {
				return e
			}
		}
		_, _ = io.WriteString(w, "\n")
		if e := writeExpressionVector(p.manager, p.config, w, *lits); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	}); err != nil {
		return false, err
	}
	return true, nil
}
