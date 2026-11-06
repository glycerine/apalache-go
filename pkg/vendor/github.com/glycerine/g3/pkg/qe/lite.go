package qe

// Source: src/qe/qe_lite.cpp qe_lite::impl and qe_lite owning overloads.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// Lite combines the source equality, arithmetic and optional array phases.
// Config and manager are borrowed; constructor parameters are copied by DER.
type Lite struct {
	self             *Lite
	manager          *ast.ASTManager
	config           Config
	der              *EqualityDER
	fm               *FourierMotzkin
	array            *ArrayDER
	dummy            *VariableTest
	rewriter         *rewriter.TheoryRewriter
	traversal        *rewriter.Traversal
	traversalConfig  liteConfiguration
	pending          []*liteCall
	active           *liteCall
	useArray, closed bool
	bytes            uint64
}

type liteCall struct {
	owner         *Lite
	refs          []ast.ASTRef
	vectors       []*[]ast.ASTRef
	selectors     []*VariableTest
	booleans      []*rewriter.BoolRewriter
	abstractors   []*ast.ExprAbstractor
	substitutions []*rewriter.VarSubstitution
	failures      []memory.ConstructionFailure
}

func NewLite(m *ast.ASTManager, p *uparams.ParamsRef, useArray bool, config Config) (result *Lite, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(Lite{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	l := &Lite{manager: m, config: config, useArray: useArray, bytes: bytes}
	l.self = l
	l.traversalConfig.owner = l
	defer func() {
		if err != nil {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				l.pending = append(l.pending, &liteCall{owner: l, failures: []memory.ConstructionFailure{failure}})
			}
			if cleanup := l.Close(); cleanup != nil {
				result = l
				err = &LiteFailure{owner: l, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	l.der, err = NewEqualityDER(m, p, config)
	if err != nil {
		return nil, err
	}
	// Source phase selectors are assigned immediately before use. The empty
	// native selector satisfies phase constructor owner checks until then.
	l.dummy, err = NewVariableSetTest(m, nil, true)
	if err != nil {
		return nil, err
	}
	l.fm, err = NewFourierMotzkin(m, l.dummy, config)
	if err != nil {
		return nil, err
	}
	l.array, err = NewArrayDER(m, l.dummy, config)
	if err != nil {
		return nil, err
	}
	l.traversal, err = rewriter.NewTraversal(m, &l.traversalConfig)
	if err != nil {
		return nil, err
	}
	l.rewriter, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		return nil, err
	}
	return l, nil
}
func (l *Lite) check() error {
	if l == nil || l.self != l || l.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (l *Lite) begin() (*liteCall, error) {
	if err := l.check(); err != nil {
		return nil, err
	}
	if l.active != nil {
		return nil, ast.ErrASTDomain
	}
	if err := l.drain(); err != nil {
		return nil, err
	}
	c := &liteCall{owner: l}
	l.pending = append(l.pending, c)
	return c, nil
}
func (c *liteCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *liteCall) hold(h ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	_, err = c.ref(h)
	return h, err
}
func (c *liteCall) vector(hs []ast.ASTHandle) (*[]ast.ASTRef, error) {
	v := new([]ast.ASTRef)
	c.vectors = append(c.vectors, v)
	for _, h := range hs {
		ref, err := c.owner.manager.NewRef(h)
		if err != nil {
			return nil, err
		}
		*v = append(*v, ref)
	}
	return v, nil
}
func (c *liteCall) boolean() (*rewriter.BoolRewriter, error) {
	l := c.owner
	r, err := rewriter.NewBoolRewriter(l.manager, l.config.Context, l.config.Globals, nil)
	if r != nil {
		c.booleans = append(c.booleans, r)
	}
	return r, err
}
func (c *liteCall) close() error {
	for len(c.failures) != 0 {
		i := len(c.failures) - 1
		if err := c.failures[i].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:i]
	}
	for len(c.substitutions) != 0 {
		i := len(c.substitutions) - 1
		if err := c.substitutions[i].Close(); err != nil {
			return err
		}
		c.substitutions = c.substitutions[:i]
	}
	for len(c.abstractors) != 0 {
		i := len(c.abstractors) - 1
		if err := c.abstractors[i].Close(); err != nil {
			return err
		}
		c.abstractors = c.abstractors[:i]
	}
	for len(c.booleans) != 0 {
		i := len(c.booleans) - 1
		if err := c.booleans[i].Close(); err != nil {
			return err
		}
		c.booleans = c.booleans[:i]
	}
	for len(c.selectors) != 0 {
		i := len(c.selectors) - 1
		if err := c.selectors[i].Close(); err != nil {
			return err
		}
		c.selectors = c.selectors[:i]
	}
	for len(c.vectors) != 0 {
		i := len(c.vectors) - 1
		if err := truncateLiteVector(c.owner.manager, c.vectors[i], 0); err != nil {
			return err
		}
		c.vectors = c.vectors[:i]
	}
	for len(c.refs) != 0 {
		i := len(c.refs) - 1
		if c.refs[i] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[i]); err != nil {
				return err
			}
		}
		c.refs = c.refs[:i]
	}
	return nil
}
func (l *Lite) drain() error {
	for len(l.pending) != 0 {
		i := len(l.pending) - 1
		if err := l.pending[i].close(); err != nil {
			return err
		}
		l.pending = l.pending[:i]
	}
	return nil
}

type LiteFailure struct {
	owner          *Lite
	cause, cleanup error
	constructor    bool
}

var _ memory.ConstructionFailure = (*LiteFailure)(nil)

func (e *LiteFailure) Error() string        { return e.Unwrap().Error() }
func (e *LiteFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *LiteFailure) OriginalCause() error { return e.cause }
func (e *LiteFailure) CleanupCause() error  { return e.cleanup }
func (e *LiteFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *LiteFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}
func (l *Lite) finish(c *liteCall, err *error) {
	var failure memory.ConstructionFailure
	if errors.As(*err, &failure) {
		c.failures = append(c.failures, failure)
	}
	if cleanup := l.drain(); cleanup != nil {
		*err = &LiteFailure{owner: l, cause: *err, cleanup: cleanup}
	}
}
func (l *Lite) Close() error {
	if l == nil || l.self != l {
		return ast.ErrASTHandle
	}
	if l.closed {
		return nil
	}
	if l.active != nil {
		return ast.ErrASTDomain
	}
	if err := l.drain(); err != nil {
		return err
	}
	if l.rewriter != nil {
		if err := l.rewriter.Close(); err != nil {
			return err
		}
		l.rewriter = nil
	}
	if l.traversal != nil {
		if err := l.traversal.Close(); err != nil {
			return err
		}
		l.traversal = nil
	}
	if l.array != nil {
		if err := l.array.Close(); err != nil {
			return err
		}
		l.array = nil
	}
	if l.fm != nil {
		if err := l.fm.Close(); err != nil {
			return err
		}
		l.fm = nil
	}
	if l.der != nil {
		if err := l.der.Close(); err != nil {
			return err
		}
		l.der = nil
	}
	if l.dummy != nil {
		if err := l.dummy.Close(); err != nil {
			return err
		}
		l.dummy = nil
	}
	l.manager.Budget().Release(l.bytes)
	l.bytes = 0
	l.closed = true
	return nil
}
