package qe

// Source: src/qe/qe_lite.cpp eq::der.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/numeral"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// EqualityDER owns reusable substitution and theory-rewriting state. Its
// selector and Config services are borrowed. Parameter p is copied solely for
// source unused-variable elimination; the source theory rewriter uses defaults.
type EqualityDER struct {
	self         *EqualityDER
	manager      *ast.ASTManager
	config       Config
	selection    *VariableTest
	params       uparams.ParamsRef
	paramsPlaced bool
	substitution *rewriter.VarSubstitution
	rewriter     *rewriter.TheoryRewriter
	arithmetic   ast.FamilyID
	pending      []*equalityDERCall
	bytes        uint64
	closed       bool
}

type equalityDefinition struct {
	variable, term ast.ASTHandle
	position       int
}
type equalitySolution struct{ variable, term ast.ASTHandle }
type equalityDERCall struct {
	owner       *EqualityDER
	refs        []ast.ASTRef
	numbers     []*numeral.OwnedRational
	booleans    []*rewriter.BoolRewriter
	selectors   []*VariableTest
	failures    []memory.ConstructionFailure
	definitions []equalityDefinition
	positions   []int
	order       []uint32
	bytes       uint64
}

func (c *equalityDERCall) hold(h ast.ASTHandle) (ast.ASTHandle, error) {
	r, err := c.owner.manager.NewRef(h)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	c.refs = append(c.refs, r)
	return h, nil
}
func (c *equalityDERCall) ref(h ast.ASTHandle) (ast.ASTRef, error) {
	r, err := c.owner.manager.NewRef(h)
	if err == nil {
		c.refs = append(c.refs, r)
	}
	return r, err
}
func (c *equalityDERCall) rational() (*numeral.OwnedRational, error) {
	n := new(numeral.OwnedRational)
	var err error
	*n, err = c.owner.manager.NumeralService().NewValue()
	if err != nil {
		return nil, err
	}
	c.numbers = append(c.numbers, n)
	return n, nil
}
func (c *equalityDERCall) boolean() (*rewriter.BoolRewriter, error) {
	d := c.owner
	r, err := rewriter.NewBoolRewriter(d.manager, d.config.Context, d.config.Globals, nil)
	if r != nil {
		c.booleans = append(c.booleans, r)
	}
	return r, err
}
func (c *equalityDERCall) close() error {
	for len(c.failures) != 0 {
		last := len(c.failures) - 1
		if err := c.failures[last].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:last]
	}
	for len(c.booleans) != 0 {
		last := len(c.booleans) - 1
		if err := c.booleans[last].Close(); err != nil {
			return err
		}
		c.booleans = c.booleans[:last]
	}
	for len(c.selectors) != 0 {
		last := len(c.selectors) - 1
		if err := c.selectors[last].Close(); err != nil {
			return err
		}
		c.selectors = c.selectors[:last]
	}
	for len(c.numbers) != 0 {
		last := len(c.numbers) - 1
		n := c.numbers[last]
		if *n != (numeral.OwnedRational{}) {
			if err := c.owner.manager.NumeralService().Del(n); err != nil {
				return err
			}
		}
		c.numbers = c.numbers[:last]
	}
	for len(c.refs) != 0 {
		last := len(c.refs) - 1
		if c.refs[last] != (ast.ASTRef{}) {
			if err := c.owner.manager.DelRef(&c.refs[last]); err != nil {
				return err
			}
		}
		c.refs = c.refs[:last]
	}
	c.owner.manager.Budget().Release(c.bytes)
	c.bytes = 0
	c.definitions = nil
	c.positions = nil
	c.order = nil
	return nil
}

// EqualityDERFailure retains the actual stopped constructor or call owner.
type EqualityDERFailure struct {
	owner          *EqualityDER
	cause, cleanup error
	constructor    bool
}

var _ memory.ConstructionFailure = (*EqualityDERFailure)(nil)

func (e *EqualityDERFailure) Error() string        { return e.Unwrap().Error() }
func (e *EqualityDERFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *EqualityDERFailure) OriginalCause() error { return e.cause }
func (e *EqualityDERFailure) CleanupCause() error  { return e.cleanup }
func (e *EqualityDERFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *EqualityDERFailure) Cleanup() error {
	if e.owner.closed {
		return nil
	}
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func NewEqualityDER(m *ast.ASTManager, p *uparams.ParamsRef, config Config) (result *EqualityDER, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(EqualityDER{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	d := &EqualityDER{manager: m, config: config, bytes: bytes}
	d.self = d
	defer func() {
		if err != nil {
			if cleanup := d.Close(); cleanup != nil {
				result = d
				err = &EqualityDERFailure{owner: d, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	d.params, err = config.Context.NewValue()
	if err != nil {
		return nil, err
	}
	d.paramsPlaced = true
	if p != nil {
		if err = config.Context.Assign(&d.params, p); err != nil {
			return nil, err
		}
	}
	d.substitution, err = rewriter.NewVarSubstitution(m, true)
	if err != nil {
		return nil, err
	}
	d.rewriter, err = rewriter.NewTheoryRewriter(m, config.Context, config.Globals, nil)
	if err != nil {
		return nil, err
	}
	d.arithmetic, err = m.ArithmeticFamilyID()
	if err != nil {
		return nil, err
	}
	return d, nil
}
func (d *EqualityDER) check() error {
	if d == nil || d.self != d || d.closed {
		return ast.ErrASTHandle
	}
	return nil
}
func (d *EqualityDER) SetVariableTest(selection *VariableTest) error {
	if err := d.check(); err != nil {
		return err
	}
	if err := selection.check(); err != nil {
		return err
	}
	if selection.manager != d.manager {
		return ast.ErrASTDomain
	}
	d.selection = selection
	return nil
}
func (d *EqualityDER) drain() error {
	for len(d.pending) != 0 {
		last := len(d.pending) - 1
		if err := d.pending[last].close(); err != nil {
			return err
		}
		d.pending = d.pending[:last]
	}
	return nil
}
func (d *EqualityDER) finish(c *equalityDERCall, err *error) {
	var failure memory.ConstructionFailure
	if errors.As(*err, &failure) {
		c.failures = append(c.failures, failure)
	}
	if cleanup := d.drain(); cleanup != nil {
		*err = &EqualityDERFailure{owner: d, cause: *err, cleanup: cleanup}
	}
}
func (d *EqualityDER) checkpoint() error {
	if err := d.config.Cooperation.Cooperate("der"); err != nil {
		return err
	}
	canceled, err := d.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(d.manager.ResourceLimit().CancelMessage())
	}
	return nil
}
func (d *EqualityDER) operation(h ast.ASTHandle, family ast.FamilyID, operation int32) (bool, error) {
	k, err := d.manager.GetKind(h)
	if err != nil || k != ast.ASTApp {
		return false, err
	}
	f, err := d.manager.GetFamilyID(h)
	if err != nil || f != family {
		return false, err
	}
	op, err := d.manager.GetDeclKind(h)
	return op == operation, err
}
func (d *EqualityDER) children(h ast.ASTHandle) ([]ast.ASTHandle, error) {
	n, err := d.manager.GetNumArgs(h)
	if err != nil {
		return nil, err
	}
	result := make([]ast.ASTHandle, n)
	for i := range result {
		result[i], err = d.manager.GetArg(h, uint32(i))
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
func (d *EqualityDER) booleanSort(h ast.ASTHandle) (bool, error) {
	s, err := d.manager.GetSort(h)
	if err != nil {
		return false, err
	}
	b, err := d.manager.MkBoolSort()
	return s == b, err
}
func (d *EqualityDER) emit(tag, function string, line int, body func(io.Writer) error) error {
	tags := d.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err := tags.EmitConditional(tag, function, "src/qe/qe_lite.cpp", line, func() bool { return true }, func(out io.Writer) error { return body(traceBodyWriter{out}) })
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
func (d *EqualityDER) traceExpression(function string, line int, h ast.ASTHandle) error {
	return d.emit("qe_lite", function, line, func(out io.Writer) error {
		if err := writeExpression(d.manager, d.config, out, h); err != nil {
			return err
		}
		_, _ = io.WriteString(out, "\n")
		return nil
	})
}
func (d *EqualityDER) Close() error {
	if d == nil || d.self != d {
		return ast.ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if err := d.drain(); err != nil {
		return err
	}
	if d.rewriter != nil {
		if err := d.rewriter.Close(); err != nil {
			return err
		}
		d.rewriter = nil
	}
	if d.substitution != nil {
		if err := d.substitution.Close(); err != nil {
			return err
		}
		d.substitution = nil
	}
	if d.paramsPlaced {
		if err := d.config.Context.Del(&d.params); err != nil {
			return err
		}
		d.paramsPlaced = false
	}
	d.manager.Budget().Release(d.bytes)
	d.bytes = 0
	d.closed = true
	return nil
}
