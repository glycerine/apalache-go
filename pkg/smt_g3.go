package apalache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/cmd_context"
	_ "github.com/glycerine/g3/pkg/engine"
	"github.com/glycerine/g3/pkg/util/lbool"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type SMTStatus uint8

type smtQueryTimeoutKey struct{}

// WithSMTQueryTimeout configures a fresh timeout for every SMT query made
// with the returned context. A non-positive duration disables a timeout
// inherited through this mechanism.
func WithSMTQueryTimeout(ctx context.Context, timeout time.Duration) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, smtQueryTimeoutKey{}, timeout)
}

func smtQueryContext(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	timeout, _ := ctx.Value(smtQueryTimeoutKey{}).(time.Duration)
	if timeout <= 0 {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, timeout)
}

const (
	SMTUnknown SMTStatus = iota
	SMTUnsatisfiable
	SMTSatisfiable
)

type SMTSort uint8

const (
	SMTBool SMTSort = iota
	SMTInt
	SMTUninterpreted
	SMTArray
)

// SMTTerm is an opaque solver term. Terms are owned by the context that
// created them and cannot be mixed across contexts.
type SMTTerm struct {
	owner            *G3SolverContext
	handle           ast.ASTHandle
	sort             SMTSort
	sortID           string
	arrayIndexSort   SMTSort
	arrayIndexSortID string
	arrayValueSort   SMTSort
	arrayValueSortID string
}

func (term SMTTerm) Sort() SMTSort { return term.sort }

// G3SolverContext is the production pure-Go solver boundary. Its default logic
// is the quantifier-free integer/Boolean/array fragment used by the active
// encoding; explicit-logic contexts also expose the quantified adapter path.
type G3SolverContext struct {
	context *cmd_context.Context
	manager *ast.ASTManager
	closed  bool
}

func NewG3SolverContext() (*G3SolverContext, error) {
	return NewG3SolverContextWithLogic("QF_AUFNIA")
}

// NewG3SolverContextWithLogic creates a solver boundary for an explicit
// SMT-LIB logic. The production ground encoding uses QF_AUFNIA; the explicit
// constructor keeps array and quantified adapter paths testable without
// silently broadening that production contract.
func NewG3SolverContextWithLogic(logic string) (*G3SolverContext, error) {
	if logic == "" {
		return nil, fmt.Errorf("SMT logic cannot be empty")
	}
	command, err := cmd_context.NewStandaloneContext()
	if err != nil {
		return nil, err
	}
	fail := func(err error) (*G3SolverContext, error) {
		return nil, errors.Join(err, command.Close())
	}
	if err = command.SetLogic(logic); err != nil {
		return fail(err)
	}
	manager, err := command.Manager()
	if err != nil {
		return fail(err)
	}
	return &G3SolverContext{context: command, manager: manager}, nil
}

func (solver *G3SolverContext) lookupSort(sort SMTSort, sortID string) (ast.ASTHandle, error) {
	if err := solver.validate(); err != nil {
		return ast.ASTHandle{}, err
	}
	name := sortID
	if sort == SMTBool {
		name = "Bool"
	} else if sort == SMTInt {
		name = "Int"
	} else if sort != SMTUninterpreted || name == "" {
		return ast.ASTHandle{}, fmt.Errorf("unsupported scalar SMT sort %d", sort)
	}
	return solver.context.LookupSort(name)
}

func parseG3ValueOutput(output string) (string, error) {
	const prefix = "(($value "
	text := strings.TrimSpace(output)
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, "))") {
		return "", fmt.Errorf("unexpected g3 get-value output %q", text)
	}
	return strings.TrimSpace(text[len(prefix) : len(text)-2]), nil
}

// EvaluateText evaluates a term in the most recent satisfiable model and
// returns its SMT-LIB value representation.
func (solver *G3SolverContext) EvaluateText(term SMTTerm) (string, error) {
	if err := solver.validateTerm(term, term.sort); err != nil {
		return "", err
	}
	var output bytes.Buffer
	if err := solver.context.WriteValuesWithText(&output, []ast.ASTHandle{term.handle}, []string{"$value"}); err != nil {
		return "", err
	}
	return parseG3ValueOutput(output.String())
}

func (solver *G3SolverContext) EvaluateBool(term SMTTerm) (bool, error) {
	if err := solver.validateTerm(term, SMTBool); err != nil {
		return false, err
	}
	text, err := solver.EvaluateText(term)
	if err != nil {
		return false, err
	}
	switch text {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("expected a Boolean model value, found %q", text)
	}
}

func (solver *G3SolverContext) EvaluateBigInt(term SMTTerm) (*big.Int, error) {
	if err := solver.validateTerm(term, SMTInt); err != nil {
		return nil, err
	}
	text, err := solver.EvaluateText(term)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(text, "(- ") && strings.HasSuffix(text, ")") {
		text = "-" + strings.TrimSpace(text[3:len(text)-1])
	}
	value, ok := new(big.Int).SetString(text, 10)
	if !ok {
		return nil, fmt.Errorf("expected an integer model value, found %q", text)
	}
	return value, nil
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

func (solver *G3SolverContext) validateSameSort(left, right SMTTerm) error {
	if left.sort != right.sort || left.sortID != right.sortID {
		return fmt.Errorf("cannot combine SMT terms of different sorts")
	}
	if err := solver.validateTerm(left, left.sort); err != nil {
		return err
	}
	return solver.validateTerm(right, right.sort)
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
	return SMTTerm{owner: solver, handle: handle, sort: sort, sortID: sortName}, err
}

// DeclareUninterpreted declares a constant in a named uninterpreted sort,
// creating the sort the first time it is used.
func (solver *G3SolverContext) DeclareUninterpreted(name, sortName string) (SMTTerm, error) {
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	if sortName == "" {
		return SMTTerm{}, fmt.Errorf("uninterpreted SMT sort name cannot be empty")
	}
	rangeSort, err := solver.context.LookupSort(sortName)
	if err != nil {
		rangeSort, err = solver.context.DeclareSort(sortName)
		if err != nil {
			return SMTTerm{}, err
		}
	}
	if _, err = solver.context.DeclareFunction(name, nil, rangeSort); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.context.MakeApp(name, nil, nil, ast.ASTHandle{})
	return SMTTerm{owner: solver, handle: handle, sort: SMTUninterpreted, sortID: sortName}, err
}

// DeclareArray declares a one-dimensional SMT array over scalar sorts.
func (solver *G3SolverContext) DeclareArray(name string, indexSort, valueSort SMTSort) (SMTTerm, error) {
	indexHandle, err := solver.lookupSort(indexSort, "")
	if err != nil {
		return SMTTerm{}, fmt.Errorf("array index sort: %w", err)
	}
	valueHandle, err := solver.lookupSort(valueSort, "")
	if err != nil {
		return SMTTerm{}, fmt.Errorf("array value sort: %w", err)
	}
	arrayHandle, err := solver.manager.MkArraySort(ast.SliceArguments([]ast.ASTHandle{indexHandle}), valueHandle)
	if err != nil {
		return SMTTerm{}, err
	}
	if _, err = solver.context.DeclareFunction(name, nil, arrayHandle); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.context.MakeApp(name, nil, nil, ast.ASTHandle{})
	indexSortID := scalarSortID(indexSort)
	valueSortID := scalarSortID(valueSort)
	return SMTTerm{
		owner: solver, handle: handle, sort: SMTArray,
		sortID:         fmt.Sprintf("(Array %s %s)", indexSortID, valueSortID),
		arrayIndexSort: indexSort, arrayIndexSortID: indexSortID,
		arrayValueSort: valueSort, arrayValueSortID: valueSortID,
	}, err
}

func scalarSortID(sort SMTSort) string {
	if sort == SMTBool {
		return "Bool"
	}
	if sort == SMTInt {
		return "Int"
	}
	return ""
}

// Bound creates a de Bruijn variable for quantified formulas.
func (solver *G3SolverContext) Bound(index uint32, sort SMTSort) (SMTTerm, error) {
	sortHandle, err := solver.lookupSort(sort, "")
	if err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkVar(index, sortHandle)
	return SMTTerm{owner: solver, handle: handle, sort: sort, sortID: scalarSortID(sort)}, err
}

func (solver *G3SolverContext) quantify(forall bool, name string, bound, body SMTTerm) (SMTTerm, error) {
	if name == "" {
		return SMTTerm{}, fmt.Errorf("quantified variable name cannot be empty")
	}
	if err := solver.validateTerm(bound, bound.sort); err != nil {
		return SMTTerm{}, err
	}
	kind, err := solver.manager.GetKind(bound.handle)
	if err != nil {
		return SMTTerm{}, err
	}
	if kind != ast.ASTVar {
		return SMTTerm{}, fmt.Errorf("quantified binder must be a bound SMT variable")
	}
	if bound.sort != SMTBool && bound.sort != SMTInt {
		return SMTTerm{}, fmt.Errorf("unsupported quantified SMT sort %d", bound.sort)
	}
	if err := solver.validateTerm(body, SMTBool); err != nil {
		return SMTTerm{}, err
	}
	sortHandle, err := solver.lookupSort(bound.sort, bound.sortID)
	if err != nil {
		return SMTTerm{}, err
	}
	symbol, err := solver.manager.Symbols.Intern(name)
	if err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkQuantifier(ast.ASTQuantifierSpec{
		Forall: forall, Sorts: []ast.ASTHandle{sortHandle}, Names: []usymbol.Symbol{symbol}, Body: body.handle,
	})
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) Forall(name string, bound, body SMTTerm) (SMTTerm, error) {
	return solver.quantify(true, name, bound, body)
}

func (solver *G3SolverContext) Exists(name string, bound, body SMTTerm) (SMTTerm, error) {
	return solver.quantify(false, name, bound, body)
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
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) Int(value int64) (SMTTerm, error) {
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.ArithmeticIntegerNumeralText(strconv.FormatInt(value, 10))
	return SMTTerm{owner: solver, handle: handle, sort: SMTInt, sortID: "Int"}, err
}

func (solver *G3SolverContext) BigInt(value *big.Int) (SMTTerm, error) {
	if value == nil {
		return SMTTerm{}, fmt.Errorf("SMT integer cannot be nil")
	}
	if err := solver.validate(); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.ArithmeticIntegerNumeralText(value.String())
	return SMTTerm{owner: solver, handle: handle, sort: SMTInt, sortID: "Int"}, err
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
	sortID := "Bool"
	if sort == SMTInt {
		sortID = "Int"
	}
	return SMTTerm{owner: solver, handle: handle, sort: sort, sortID: sortID}, err
}

func (solver *G3SolverContext) Not(term SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(term, SMTBool); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkNot(term.handle)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
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
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) Or(terms ...SMTTerm) (SMTTerm, error) {
	handles := make([]ast.ASTHandle, len(terms))
	for i, term := range terms {
		if err := solver.validateTerm(term, SMTBool); err != nil {
			return SMTTerm{}, fmt.Errorf("argument %d: %w", i, err)
		}
		handles[i] = term.handle
	}
	handle, err := solver.manager.MkOrArray(handles)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) Equal(left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateSameSort(left, right); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkEq(left.handle, right.handle)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) Distinct(terms ...SMTTerm) (SMTTerm, error) {
	if len(terms) < 2 {
		return solver.Bool(true)
	}
	handles := make([]ast.ASTHandle, len(terms))
	for index, term := range terms {
		if err := solver.validateSameSort(terms[0], term); err != nil {
			return SMTTerm{}, fmt.Errorf("argument %d: %w", index, err)
		}
		handles[index] = term.handle
	}
	handle, err := solver.manager.MkDistinct(handles)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) Implies(left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(left, SMTBool); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, SMTBool); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkImplies(left.handle, right.handle)
	return SMTTerm{owner: solver, handle: handle, sort: SMTBool, sortID: "Bool"}, err
}

func (solver *G3SolverContext) IfThenElse(condition, thenTerm, elseTerm SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(condition, SMTBool); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateSameSort(thenTerm, elseTerm); err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkIte(condition.handle, thenTerm.handle, elseTerm.handle)
	result := thenTerm
	result.handle = handle
	return result, err
}

func (solver *G3SolverContext) Select(array, index SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(array, SMTArray); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(index, array.arrayIndexSort); err != nil {
		return SMTTerm{}, err
	}
	if index.sortID != array.arrayIndexSortID {
		return SMTTerm{}, fmt.Errorf("array index has sort %s, expected %s", index.sortID, array.arrayIndexSortID)
	}
	family, err := solver.manager.ArrayFamilyID()
	if err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkFamilyApp(family, ast.ASTArrayOpSelect, nil, []ast.ASTHandle{array.handle, index.handle})
	return SMTTerm{owner: solver, handle: handle, sort: array.arrayValueSort, sortID: array.arrayValueSortID}, err
}

func (solver *G3SolverContext) Store(array, index, value SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(array, SMTArray); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(index, array.arrayIndexSort); err != nil {
		return SMTTerm{}, err
	}
	if index.sortID != array.arrayIndexSortID {
		return SMTTerm{}, fmt.Errorf("array index has sort %s, expected %s", index.sortID, array.arrayIndexSortID)
	}
	if err := solver.validateTerm(value, array.arrayValueSort); err != nil {
		return SMTTerm{}, err
	}
	if value.sortID != array.arrayValueSortID {
		return SMTTerm{}, fmt.Errorf("array value has sort %s, expected %s", value.sortID, array.arrayValueSortID)
	}
	family, err := solver.manager.ArrayFamilyID()
	if err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkFamilyApp(family, ast.ASTArrayOpStore, nil, []ast.ASTHandle{array.handle, index.handle, value.handle})
	result := array
	result.handle = handle
	return result, err
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

func (solver *G3SolverContext) arithmeticBinary(name string, left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(left, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	return solver.application(name, SMTInt, left, right)
}

func (solver *G3SolverContext) Subtract(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticBinary("-", left, right)
}

func (solver *G3SolverContext) Multiply(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticBinary("*", left, right)
}

func (solver *G3SolverContext) Divide(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticBinary("div", left, right)
}

func (solver *G3SolverContext) Modulo(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticBinary("mod", left, right)
}

func (solver *G3SolverContext) Power(left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(left, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	family, err := solver.manager.ArithmeticFamilyID()
	if err != nil {
		return SMTTerm{}, err
	}
	handle, err := solver.manager.MkFamilyApp(family, 18, nil, []ast.ASTHandle{left.handle, right.handle})
	return SMTTerm{owner: solver, handle: handle, sort: SMTInt, sortID: "Int"}, err
}

func (solver *G3SolverContext) Negate(term SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(term, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	return solver.application("-", SMTInt, term)
}

func (solver *G3SolverContext) arithmeticCompare(name string, left, right SMTTerm) (SMTTerm, error) {
	if err := solver.validateTerm(left, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	if err := solver.validateTerm(right, SMTInt); err != nil {
		return SMTTerm{}, err
	}
	return solver.application(name, SMTBool, left, right)
}

func (solver *G3SolverContext) GreaterEqual(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticCompare(">=", left, right)
}

func (solver *G3SolverContext) Greater(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticCompare(">", left, right)
}

func (solver *G3SolverContext) LessEqual(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticCompare("<=", left, right)
}

func (solver *G3SolverContext) Less(left, right SMTTerm) (SMTTerm, error) {
	return solver.arithmeticCompare("<", left, right)
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
	ctx, cancel := smtQueryContext(ctx)
	defer cancel()
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
