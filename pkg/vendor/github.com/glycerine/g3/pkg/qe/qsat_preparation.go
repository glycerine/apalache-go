package qe

// Source: src/qe/qsat.cpp qsat::hoist/filter_vars/initialize_levels/get_vars.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
)

type qsatMode uint8

const (
	qsatQE qsatMode = iota
	qsatRecursiveQE
	qsatSAT
	qsatMaximize
)

// qsatPreparation is the actual prefix/predicate state of the pending QSAT
// search owner. It stays private until the search and tactic entry points exist.
type qsatPreparation struct {
	self        *qsatPreparation
	manager     *ast.ASTManager
	config      Config
	abstraction *PredicateAbstraction
	variables   [][]ast.ASTRef
	pending     []*qsatPreparationCall
	bytes       uint64
	closed      bool
}
type qsatPreparationCall struct {
	owner     *qsatPreparation
	variables []ast.ASTRef
	proof     ast.ASTRef
	labels    *rewriter.LabelRewriter
	hoister   *rewriter.QuantifierHoister
	results   []*rewriter.QuantifierHoistResult
	failures  []memory.ConstructionFailure
}
type qsatPreparationFailure struct {
	owner          *qsatPreparation
	cause, cleanup error
	constructor    bool
}

func (e *qsatPreparationFailure) Error() string        { return e.Unwrap().Error() }
func (e *qsatPreparationFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *qsatPreparationFailure) OriginalCause() error { return e.cause }
func (e *qsatPreparationFailure) CleanupCause() error  { return e.cleanup }
func (e *qsatPreparationFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *qsatPreparationFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func newQSATPreparation(m *ast.ASTManager, config Config) (result *qsatPreparation, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Cooperation == nil || config.Globals.Context != config.Context {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(qsatPreparation{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	p := &qsatPreparation{manager: m, config: config, bytes: n}
	p.self = p
	defer func() {
		if err != nil {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				p.pending = append(p.pending, &qsatPreparationCall{owner: p, failures: []memory.ConstructionFailure{failure}})
			}
			if cleanup := p.Close(); cleanup != nil {
				result = p
				err = &qsatPreparationFailure{owner: p, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	p.abstraction, err = NewPredicateAbstraction(m, config)
	if err != nil {
		return nil, err
	}
	return p, nil
}
func (p *qsatPreparation) open() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return arena.ErrClosed
	}
	_, err := p.manager.ProofsEnabled()
	return err
}
func (c *qsatPreparationCall) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		c.failures = append(c.failures, failure)
	}
}
func (c *qsatPreparationCall) close() error {
	for len(c.failures) > 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.results) > 0 {
		i := len(c.results) - 1
		if err := c.results[i].Close(); err != nil {
			c.retain(err)
			return err
		}
		c.results = c.results[:i]
	}
	if c.hoister != nil {
		if err := c.hoister.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.hoister = nil
	}
	if c.labels != nil {
		if err := c.labels.Close(); err != nil {
			c.retain(err)
			return err
		}
		c.labels = nil
	}
	if err := c.owner.clearRefs(&c.variables); err != nil {
		c.retain(err)
		return err
	}
	if c.proof != (ast.ASTRef{}) {
		if err := c.owner.manager.DelRef(&c.proof); err != nil {
			c.retain(err)
			return err
		}
	}
	return nil
}
func (p *qsatPreparation) clearRefs(v *[]ast.ASTRef) error {
	for len(*v) > 0 {
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
func (p *qsatPreparation) drain() error {
	for len(p.pending) > 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	return nil
}
func (p *qsatPreparation) finish(c *qsatPreparationCall, err *error) {
	c.retain(*err)
	if cleanup := p.drain(); cleanup != nil {
		*err = &qsatPreparationFailure{owner: p, cause: *err, cleanup: cleanup}
	}
}
func (p *qsatPreparation) reset() error {
	if err := p.open(); err != nil {
		return err
	}
	if err := p.drain(); err != nil {
		return err
	}
	if err := p.abstraction.Reset(); err != nil {
		return err
	}
	for len(p.variables) > 0 {
		i := len(p.variables) - 1
		if err := p.clearRefs(&p.variables[i]); err != nil {
			return err
		}
		p.variables = p.variables[:i]
	}
	return nil
}
func (p *qsatPreparation) Close() error {
	if p == nil || p.self != p {
		return ast.ErrASTHandle
	}
	if p.closed {
		return nil
	}
	if err := p.drain(); err != nil {
		return err
	}
	for len(p.variables) > 0 {
		i := len(p.variables) - 1
		if err := p.clearRefs(&p.variables[i]); err != nil {
			return err
		}
		p.variables = p.variables[:i]
	}
	if p.abstraction != nil {
		if err := p.abstraction.Close(); err != nil {
			return err
		}
		p.abstraction = nil
	}
	p.manager.Budget().Release(p.bytes)
	p.bytes = 0
	p.closed = true
	return nil
}
func (p *qsatPreparation) appendRefs(out *[]ast.ASTRef, values []ast.ASTRef) error {
	for _, r := range values {
		h, err := p.manager.RefGet(r)
		if err != nil {
			return err
		}
		v, err := p.manager.NewRef(h)
		if err != nil {
			return err
		}
		*out = append(*out, v)
	}
	return nil
}
func (p *qsatPreparation) appendBlock(values []ast.ASTRef) error {
	p.variables = append(p.variables, nil)
	return p.appendRefs(&p.variables[len(p.variables)-1], values)
}
func (p *qsatPreparation) filterVariables(values []ast.ASTRef) error {
	f, err := p.abstraction.FilterModelConverter()
	if err != nil {
		return err
	}
	for _, r := range values {
		h, err := p.manager.RefGet(r)
		if err != nil {
			return err
		}
		d, err := p.manager.GetDecl(h)
		if err != nil {
			return err
		}
		if err = f.Insert(d); err != nil {
			return err
		}
	}
	return nil
}
func (p *qsatPreparation) initializeLevels() error {
	for i, block := range p.variables {
		level := EmptyMaxLevel()
		if i%2 == 0 {
			level.Existential = uint32(i)
		} else {
			level.Universal = uint32(i)
		}
		for _, r := range block {
			h, err := p.manager.RefGet(r)
			if err != nil {
				return err
			}
			if err = p.abstraction.SetExpressionLevel(h, level); err != nil {
				return err
			}
		}
	}
	return nil
}
func (c *qsatPreparationCall) pull(forall bool, formula ast.ASTRef) error {
	p := c.owner
	h, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	r, err := c.hoister.PullQuantifier(forall, h, true, true)
	if r != nil {
		c.results = append(c.results, r)
	}
	if err != nil {
		return err
	}
	if err = p.appendRefs(&c.variables, r.Variables); err != nil {
		return err
	}
	h, err = p.manager.RefGet(r.Formula)
	if err != nil {
		return err
	}
	return p.manager.SetRef(formula, h)
}
func (p *qsatPreparation) hoist(mode qsatMode, formula ast.ASTRef) (err error) {
	if err = p.open(); err != nil {
		return err
	}
	if mode > qsatMaximize {
		return ast.ErrASTDomain
	}
	if err = p.drain(); err != nil {
		return err
	}
	c := &qsatPreparationCall{owner: p}
	p.pending = append(p.pending, c)
	defer p.finish(c, &err)
	c.proof, err = p.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	c.labels, err = rewriter.NewLabelRewriter(p.manager)
	if err != nil {
		return err
	}
	if err = c.labels.RemoveLabels(formula, c.proof); err != nil {
		return err
	}
	c.hoister, err = rewriter.NewQuantifierHoister(p.manager, p.config.Context, p.config.Globals, nil)
	if err != nil {
		return err
	}
	h, err := p.manager.RefGet(formula)
	if err != nil {
		return err
	}
	if err = p.abstraction.GetFreeVariables(h, &c.variables); err != nil {
		return err
	}
	if err = p.appendBlock(c.variables); err != nil {
		return err
	}
	if err = p.clearRefs(&c.variables); err != nil {
		return err
	}
	forall := mode != qsatSAT
	if err = c.pull(forall, formula); err != nil {
		return err
	}
	if forall {
		if err = p.appendBlock(c.variables); err != nil {
			return err
		}
	} else {
		if err = p.appendRefs(&p.variables[len(p.variables)-1], c.variables); err != nil {
			return err
		}
	}
	if err = p.filterVariables(c.variables); err != nil {
		return err
	}
	for {
		forall = !forall
		if err = p.clearRefs(&c.variables); err != nil {
			return err
		}
		if err = c.pull(forall, formula); err != nil {
			return err
		}
		if err = p.appendBlock(c.variables); err != nil {
			return err
		}
		if err = p.filterVariables(c.variables); err != nil {
			return err
		}
		if len(c.variables) == 0 {
			break
		}
	}
	if err = p.initializeLevels(); err != nil {
		return err
	}
	return p.abstraction.emit("hoist", 749, func(w io.Writer) error {
		h, e := p.manager.RefGet(formula)
		if e != nil {
			return e
		}
		if e = writeExpression(p.manager, p.config, w, h); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
}
func (p *qsatPreparation) getVariables(level uint32, out *[]ast.ASTRef) error {
	if err := p.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err := p.clearRefs(out); err != nil {
		return err
	}
	for i := uint64(level); i < uint64(len(p.variables)); i++ {
		if err := p.appendRefs(out, p.variables[i]); err != nil {
			return err
		}
	}
	return nil
}
