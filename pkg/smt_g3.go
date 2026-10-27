package apalache

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/lbool"
)

type SMTStatus uint8

const (
	SMTUnknown SMTStatus = iota
	SMTUnsatisfiable
	SMTSatisfiable
)

type SMTSort uint8

const (
	SMTBool SMTSort = iota
	SMTInt
)

// SMTTerm is an opaque solver term. Terms are owned by the context that
// created them and cannot be mixed across contexts.
type SMTTerm struct {
	owner  *G3SolverContext
	handle ast.ASTHandle
	sort   SMTSort
}

func (term SMTTerm) Sort() SMTSort { return term.sort }

// G3SolverContext is the production pure-Go solver boundary. The initial
// surface deliberately covers the quantifier-free integer/Boolean fragment
// needed by the first symbolic slices and grows with the rewriter.
type G3SolverContext struct {
	context *cmd_context.Context
	manager *ast.ASTManager
	closed  bool
}

func NewG3SolverContext() (*G3SolverContext, error) {
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*G3SolverContext, error) {
		return nil, errors.Join(err, command.Close())
	}
	if err = command.SetLogic("QF_LIA"); err != nil {
		return fail(err)
	}
	manager, err := command.Manager()
	if err != nil {
		return fail(err)
	}
	return &G3SolverContext{context: command, manager: manager}, nil
}

func (solver *G3SolverContext) validate() error {
	if solver == nil || solver.closed || solver.context == nil || solver.manager == nil {
		return fmt.Errorf("closed g3 solver context")
	}
	return nil
}

func (solver *G3SolverContext) validateTerm(term SMTTerm, sort SMTSort) error {
	if err := solver.validate(); err != nil {
		return err
	}
	if term.owner != solver || term.handle.IsNull() {
		return fmt.Errorf("SMT term belongs to a different solver context")
	}
	if term.sort != sort {
		return fmt.Errorf("SMT term has sort %d, expected %d", term.sort, sort)
	}
	return nil
}

func (solver *G3SolverContext) Declare(name string, sort SMTSort) (SMTTerm, error) {
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	sortName := "Bool"
	if sort == SMTInt {
		sortName = "Int"
	} else if sort != SMTBool {
		return SMTTerm{}, fmt.Errorf("unsupported SMT sort %d", sort)
	}
	rangeSort, err := solver.context.LookupSort(sortName)
	if err != nil {
		return SMTTerm{}, err
	}
	if _, err = solver.context.DeclareFunction(name, nil, rangeSort); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.context.MakeApp(name, nil, nil, ast.ASTHandle{})
	return SMTTerm{owner: solver, handle: handle, sort: sort}, err
}

func (solver *G3SolverContext) Bool(value bool) (SMTTerm, error) {
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	var handle ast.ASTHandle
	var err error
	if value {
		handle, err = solver.manager.MkTrue()
	} else {
		handle, err = solver.manager.MkFalse()
	}
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool}, err
}

func (solver *G3SolverContext) Int(value int64) (SMTTerm, error) {
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.ArithmeticIntegerNumeralText(strconv.FormatInt(value, 10))
	return SMTTerm{owner: solver, handle: handle, sort: SMTInt}, err
}

func (solver *G3SolverContext) application(name string, sort SMTSort, arguments ...SMTTerm) (SMTTerm, error) {
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	handles := make([]ast.ASTHandle, len(arguments))
	for i, argument := range arguments {
		if argument.owner != solver || argument.handle.IsNull() {
			return SMTTerm{}, fmt.Errorf("SMT argument %d belongs to a different solver context", i)
		}
		handles[i] = argument.handle
	}
	handle, err := solver.context.MakeApp(name, handles, nil, ast.ASTHandle{})
	return SMTTerm{owner: solver, handle: handle, sort: sort}, err
}

func (solver *G3SolverContext) Not(term SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(term, SMTBool); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkNot(term.handle)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool}, err
}

func (solver *G3SolverContext) And(terms ...SMTTerm) (SMTTerm, error) {
	handles := make([]ast.ASTHandle, len(terms))
	for i, term := range terms {
		if err := solver.validateTerm(term, SMTBool); err != nil {
			return SMTTerm{}, fmt.Errorf("argument %d: %w", i, err)
		}
		handles[i] = term.handle
	}
	handle, err := solver.manager.MkAndArray(handles)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool}, err
}

func (solver *G3SolverContext) Equal(left, right SMTTerm) (SMTTerm, error) {
	if left.sort != right.sort {
		return SMTTerm{}, fmt.Errorf("cannot compare SMT terms of different sorts")
	}
	if err := solver.validateTerm(left, left.sort); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, right.sort); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkEq(left.handle, right.handle)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool}, err
}

func (solver *G3SolverContext) Add(left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(left, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	return solver.application("+", SMTInt, left, right)
}

func (solver *G3SolverContext) GreaterEqual(left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(left, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	return solver.application(">=", SMTBool, left, right)
}

func (solver *G3SolverContext) Assert(formula SMTTerm) error {
	if err := solver.validateTerm(formula, SMTBool); err != nil {
		return err
	}
	return solver.context.Assert(formula.handle, "")
}

func (solver *G3SolverContext) Push() error {
	if err := solver.validate(); err != nil {
		return err
	}
	return solver.context.Push(1)
}

func (solver *G3SolverContext) Pop() error {
	if err := solver.validate(); err != nil {
		return err
	}
	return solver.context.Pop(1)
}

func (solver *G3SolverContext) Check(ctx context.Context) (SMTStatus, error) {
	if err := solver.validate(); err != nil {
		return SMTUnknown, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return SMTUnknown, err
	}
	finished := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			solver.manager.ResourceLimit().Cancel()
		case <-finished:
		}
	}()
	status, err := solver.context.CheckSat(nil)
	close(finished)
	<-watchDone
	solver.manager.ResourceLimit().ResetCancel()
	if contextErr := ctx.Err(); contextErr != nil {
		return SMTUnknown, contextErr
	}
	if err != nil {
		return SMTUnknown, err
	}
	switch status {
	case lbool.True:
		return SMTSatisfiable, nil
	case lbool.False:
		return SMTUnsatisfiable, nil
	default:
		return SMTUnknown, nil
	}
}

func (solver *G3SolverContext) Close() error {
	if solver == nil || solver.closed {
		return nil
	}
	solver.closed = true
	return solver.context.Close()
}
