package qe

// Source: src/qe/qe_lite.cpp ar::der (lines 829-973).

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/memory"
)

// ArrayDER implements the array pass of QE-lite. Apply mutates caller-owned
// conjunction references in source order; an error preserves completed writes.
// The manager, selection and cooperation service outlive this owner.
type ArrayDER struct {
	self      *ArrayDER
	manager   *ast.ASTManager
	selection *VariableTest
	config    Config
	array     ast.FamilyID
	visited   *ast.ASTExprMark
	pending   []*arrayDERCall
	bytes     uint64
	closed    bool
}

type arrayDERCall struct {
	owner    *ArrayDER
	replace  *rewriter.ExprSafeReplace
	out      ast.ASTRef
	failures []memory.ConstructionFailure
}

func (c *arrayDERCall) close() error {
	for len(c.failures) != 0 {
		last := len(c.failures) - 1
		if err := c.failures[last].Cleanup(); err != nil {
			return err
		}
		c.failures = c.failures[:last]
	}
	if c.replace != nil {
		if err := c.replace.Close(); err != nil {
			return err
		}
		c.replace = nil
	}
	if c.out != (ast.ASTRef{}) {
		if err := c.owner.manager.DelRef(&c.out); err != nil {
			return err
		}
	}
	return nil
}

// ArrayDERFailure retains concrete lexical owners if destruction is stopped.
type ArrayDERFailure struct {
	owner          *ArrayDER
	cause, cleanup error
}

var _ memory.ConstructionFailure = (*ArrayDERFailure)(nil)

func (e *ArrayDERFailure) Error() string        { return e.Unwrap().Error() }
func (e *ArrayDERFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *ArrayDERFailure) OriginalCause() error { return e.cause }
func (e *ArrayDERFailure) CleanupCause() error  { return e.cleanup }
func (e *ArrayDERFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *ArrayDERFailure) Cleanup() error { return e.owner.drain() }

func NewArrayDER(m *ast.ASTManager, selection *VariableTest, config Config) (*ArrayDER, error) {
	if m == nil || config.Cooperation == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || selection == nil {
		return nil, ast.ErrASTHandle
	}
	if err := selection.check(); err != nil {
		return nil, err
	}
	if selection.manager != m {
		return nil, ast.ErrASTDomain
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
	bytes := uint64(reflect.TypeOf(ArrayDER{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	d := &ArrayDER{manager: m, selection: selection, config: config, array: family, bytes: bytes}
	d.self = d
	d.visited, err = ast.NewASTExprMark(m)
	if err != nil {
		return nil, errors.Join(err, d.Close())
	}
	return d, nil
}

func (d *ArrayDER) check() error {
	if d == nil || d.self != d || d.closed {
		return ast.ErrASTHandle
	}
	return nil
}

func (d *ArrayDER) SetVariableTest(selection *VariableTest) error {
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

func (d *ArrayDER) drain() error {
	for len(d.pending) != 0 {
		last := len(d.pending) - 1
		if err := d.pending[last].close(); err != nil {
			return err
		}
		d.pending = d.pending[:last]
	}
	return nil
}

func (d *ArrayDER) operation(expression ast.ASTHandle, family ast.FamilyID, op int32) (bool, error) {
	kind, err := d.manager.GetKind(expression)
	if err != nil || kind != ast.ASTApp {
		return false, err
	}
	actual, err := d.manager.GetFamilyID(expression)
	if err != nil || actual != family {
		return false, err
	}
	operation, err := d.manager.GetDeclKind(expression)
	return operation == op, err
}

func (d *ArrayDER) mark(expression ast.ASTHandle) error {
	_, err := d.manager.GetNumExprsWithExprMark(expression, d.visited)
	return err
}

func (d *ArrayDER) solveSelect(conjunctions []ast.ASTRef, index int, selected, value ast.ASTHandle) (matched bool, err error) {
	isSelect, err := d.operation(selected, d.array, ast.ASTArrayOpSelect)
	if err != nil || !isSelect {
		return false, err
	}
	array, err := d.manager.GetArg(selected, 0)
	if err != nil {
		return false, err
	}
	variable, err := d.selection.IsVariable(array)
	if err != nil || !variable {
		return false, err
	}
	if err = d.visited.Reset(); err != nil {
		return false, err
	}
	n, err := d.manager.GetNumArgs(selected)
	if err != nil {
		return false, err
	}
	args := []ast.ASTHandle{array}
	for i := uint32(1); i < n; i++ {
		h, e := d.manager.GetArg(selected, i)
		if e != nil {
			return false, e
		}
		if e = d.mark(h); e != nil {
			return false, e
		}
		args = append(args, h)
	}
	if err = d.mark(value); err != nil {
		return false, err
	}
	marked, err := d.visited.IsMarked(array)
	if err != nil {
		return false, err
	}
	if marked {
		return false, nil
	}
	args = append(args, value)
	store, err := d.manager.MkArrayAppArguments(d.array, ast.ASTArrayOpStore, ast.SliceArguments(args))
	if err != nil {
		return false, err
	}
	call := &arrayDERCall{owner: d}
	d.pending = append(d.pending, call)
	defer func() {
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			call.failures = append(call.failures, failure)
		}
		if cleanup := d.drain(); cleanup != nil {
			err = &ArrayDERFailure{owner: d, cause: err, cleanup: cleanup}
		}
	}()
	call.replace, err = rewriter.NewExprSafeReplace(d.manager)
	if err != nil {
		return false, err
	}
	if err = call.replace.Insert(array, store); err != nil {
		return false, err
	}
	call.out, err = d.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return false, err
	}
	if err = d.traceSelect(selected, value); err != nil {
		return false, err
	}
	for j, conjunction := range conjunctions {
		if j == index {
			yes, e := d.manager.MkTrue()
			if e != nil {
				return false, e
			}
			if e = d.manager.SetRef(conjunction, yes); e != nil {
				return false, e
			}
		} else {
			h, e := d.manager.RefGet(conjunction)
			if e != nil {
				return false, e
			}
			if e = call.replace.Apply(h, call.out); e != nil {
				return false, e
			}
			h, e = d.manager.RefGet(call.out)
			if e != nil {
				return false, e
			}
			if e = d.manager.SetRef(conjunction, h); e != nil {
				return false, e
			}
		}
	}
	return true, nil
}

func (d *ArrayDER) solveEquality(conjunctions []ast.ASTRef, index int, expression ast.ASTHandle) error {
	isEq, err := d.operation(expression, d.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil || !isEq {
		return err
	}
	left, err := d.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	right, err := d.manager.GetArg(expression, 1)
	if err != nil {
		return err
	}
	matched, err := d.solveSelect(conjunctions, index, left, right)
	if err != nil || matched {
		return err
	}
	_, err = d.solveSelect(conjunctions, index, right, left)
	return err
}

func (d *ArrayDER) solveDisequality(conjunctions []ast.ASTRef, index int, expression ast.ASTHandle) error {
	isNot, err := d.operation(expression, d.manager.BasicFamilyID(), ast.ASTOpNot)
	if err != nil || !isNot {
		return err
	}
	inner, err := d.manager.GetArg(expression, 0)
	if err != nil {
		return err
	}
	isEq, err := d.operation(inner, d.manager.BasicFamilyID(), ast.ASTOpEq)
	if err != nil || !isEq {
		return err
	}
	left, err := d.manager.GetArg(inner, 0)
	if err != nil {
		return err
	}
	right, err := d.manager.GetArg(inner, 1)
	if err != nil {
		return err
	}
	for _, h := range []ast.ASTHandle{left, right} {
		selected, e := d.operation(h, d.array, ast.ASTArrayOpSelect)
		if e != nil || !selected {
			return e
		}
	}
	n, err := d.manager.GetNumArgs(left)
	if err != nil {
		return err
	}
	other, err := d.manager.GetNumArgs(right)
	if err != nil || n != other {
		return err
	}
	a, err := d.manager.GetArg(left, 0)
	if err != nil {
		return err
	}
	b, err := d.manager.GetArg(right, 0)
	if err != nil {
		return err
	}
	if err = d.visited.Reset(); err != nil {
		return err
	}
	for j, ref := range conjunctions {
		if j == index {
			continue
		}
		h, e := d.manager.RefGet(ref)
		if e != nil {
			return e
		}
		if e = d.mark(h); e != nil {
			return e
		}
	}
	if err = d.mark(a); err != nil {
		return err
	}
	if err = d.mark(b); err != nil {
		return err
	}
	for j := uint32(1); j < n; j++ {
		x, e := d.manager.GetArg(left, j)
		if e != nil {
			return e
		}
		y, e := d.manager.GetArg(right, j)
		if e != nil {
			return e
		}
		variable, e := d.selection.IsVariable(x)
		if e != nil {
			return e
		}
		if !variable || x != y {
			return nil
		}
		marked, e := d.visited.IsMarked(x)
		if e != nil {
			return e
		}
		if marked {
			return nil
		}
	}
	eq, err := d.manager.MkEq(a, b)
	if err != nil {
		return err
	}
	negated, err := d.manager.MkNot(eq)
	if err != nil {
		return err
	}
	return d.manager.SetRef(conjunctions[index], negated)
}

func (d *ArrayDER) Apply(conjunctions []ast.ASTRef) error {
	if err := d.check(); err != nil {
		return err
	}
	if err := d.selection.check(); err != nil {
		return err
	}
	if err := d.drain(); err != nil {
		return err
	}
	for i, ref := range conjunctions {
		// The source calls cooperation before testing manager cancellation.
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
		h, err := d.manager.RefGet(ref)
		if err != nil {
			return err
		}
		if err = d.solveEquality(conjunctions, i, h); err != nil {
			return err
		}
		h, err = d.manager.RefGet(ref)
		if err != nil {
			return err
		}
		if err = d.solveDisequality(conjunctions, i, h); err != nil {
			return err
		}
	}
	return nil
}

func (d *ArrayDER) Close() error {
	if d == nil || d.self != d {
		return ast.ErrASTHandle
	}
	if d.closed {
		return nil
	}
	if err := d.drain(); err != nil {
		return err
	}
	if d.visited != nil {
		if err := d.visited.Close(); err != nil {
			return err
		}
		d.visited = nil
	}
	d.manager.Budget().Release(d.bytes)
	d.bytes = 0
	d.closed = true
	return nil
}
