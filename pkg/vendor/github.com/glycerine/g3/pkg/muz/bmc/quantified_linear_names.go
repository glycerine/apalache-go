package bmc

// Source: src/muz/bmc/dl_bmc_engine.cpp, qlinear indexed symbols.

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/muz/base"
	"github.com/glycerine/g3/pkg/util/memory"
)

// quantifiedLinear owns the temporary expressions of one compilation.
type quantifiedLinear struct {
	manager  *ast.ASTManager
	width    uint32
	index    ast.ASTHandle
	variable ast.ASTHandle
	one      ast.ASTHandle
	refs     []ast.ASTRef
}

type quantifiedLinearCleanupFailure struct {
	cause      error
	cleanupErr error
	reported   error
	reportText string
	owner      *quantifiedLinear
	results    []ast.ASTRef
	nested     memory.ConstructionFailure
}

func (f *quantifiedLinearCleanupFailure) Error() string {
	return f.reportText
}
func (f *quantifiedLinearCleanupFailure) Unwrap() error {
	return f.reported
}

func (f *quantifiedLinearCleanupFailure) captureReport() {
	f.reported = errors.Join(f.cause, f.cleanupErr)
	f.reportText = f.reported.Error()
}

func (f *quantifiedLinearCleanupFailure) closeOwner() error {
	if f.nested != nil {
		if err := f.nested.Cleanup(); err != nil {
			return err
		}
		f.nested = nil
	}
	return f.owner.close()
}

func (f *quantifiedLinearCleanupFailure) Cleanup() error {
	f.cleanupErr = f.closeOwner()
	if f.cleanupErr == nil {
		f.cleanupErr = closeQuantifiedResults(f.owner.manager, f.results)
	}
	return f.cleanupErr
}
func (f *quantifiedLinearCleanupFailure) RuntimeOwner() *memory.Runtime {
	return f.owner.manager.NumeralService().RuntimeOwner()
}
func (f *quantifiedLinearCleanupFailure) OriginalCause() error { return f.cause }
func (f *quantifiedLinearCleanupFailure) CleanupCause() error  { return f.cleanupErr }

func newQuantifiedLinear(m *ast.ASTManager, width uint32) (q *quantifiedLinear, err error) {
	if m == nil || width == 0 {
		return nil, ast.ErrASTDomain
	}
	q = &quantifiedLinear{manager: m, width: width}
	defer func() {
		if err != nil {
			failure := &quantifiedLinearCleanupFailure{cause: err, owner: q}
			errors.As(err, &failure.nested)
			if cleanup := failure.closeOwner(); cleanup != nil {
				failure.cleanupErr = cleanup
				failure.captureReport()
				err = failure
			}
			q = nil
		}
	}()
	if _, err = m.RegisterBitVectorPlugin(); err != nil {
		return q, err
	}
	q.index, err = q.hold(m.BitVectorSort(width))
	if err != nil {
		return q, err
	}
	q.variable, err = q.hold(m.MkVar(0, q.index))
	if err != nil {
		return q, err
	}
	q.one, err = q.hold(m.BitVectorNumeralRat(big.NewRat(1, 1), width))
	return q, err
}

func (q *quantifiedLinear) close() (err error) {
	for i := len(q.refs) - 1; i >= 0; i-- {
		if q.refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err = q.manager.DelRef(&q.refs[i]); err != nil {
			return err
		}
	}
	q.refs = nil
	return err
}

func (q *quantifiedLinear) dispose() error {
	if err := q.close(); err != nil {
		failure := &quantifiedLinearCleanupFailure{cause: err, cleanupErr: err, owner: q}
		failure.captureReport()
		return failure
	}
	return nil
}

func closeQuantifiedResults(manager *ast.ASTManager, refs []ast.ASTRef) error {
	for i := len(refs) - 1; i >= 0; i-- {
		if refs[i] == (ast.ASTRef{}) {
			continue
		}
		if err := manager.DelRef(&refs[i]); err != nil {
			return err
		}
	}
	return nil
}

// disposeResults publishes refs only when q cleanup and the originating
// operation both succeed. On either failure the concrete retry owner keeps q
// and every result reference reachable until cleanup completes.
func (q *quantifiedLinear) disposeResults(refs []ast.ASTRef, cause error) error {
	failure := &quantifiedLinearCleanupFailure{cause: cause, owner: q, results: refs}
	if cause != nil {
		errors.As(cause, &failure.nested)
	}
	cleanup := failure.closeOwner()
	if cause == nil && cleanup == nil {
		return nil
	}
	if cause == nil {
		// Preserve the originating cleanup refusal after a later successful
		// retry so Error and Unwrap remain stable diagnostics.
		cause = cleanup
	}
	failure.cause = cause
	failure.cleanupErr = cleanup
	if cleanup == nil {
		failure.cleanupErr = closeQuantifiedResults(q.manager, refs)
	}
	if failure.cleanupErr != nil {
		failure.captureReport()
		return failure
	}
	return cause
}

func (q *quantifiedLinear) hold(value ast.ASTHandle, err error) (ast.ASTHandle, error) {
	if err != nil {
		return ast.ASTHandle{}, err
	}
	ref, err := q.manager.NewRef(value)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	q.refs = append(q.refs, ref)
	return value, nil
}

func (q *quantifiedLinear) declaration(predicate ast.ASTHandle, suffix string, rangeSort ast.ASTHandle) (ast.ASTHandle, error) {
	name, err := q.manager.GetASTName(predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	text, err := q.manager.Symbols.Display(name)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	symbol, err := q.manager.Symbols.Intern(text + suffix)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkFuncDecl(symbol, []ast.ASTHandle{q.index}, rangeSort, ast.ASTFuncDeclInfo{}))
}

func (q *quantifiedLinear) predicate(predicate ast.ASTHandle, at ast.ASTHandle) (ast.ASTHandle, error) {
	rangeSort, err := q.manager.GetRange(predicate)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := q.declaration(predicate, "#", rangeSort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{at}))
}

func (q *quantifiedLinear) rule(predicate ast.ASTHandle, index uint32) (ast.ASTHandle, error) {
	boolean, err := q.manager.MkBoolSort()
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := q.declaration(predicate, fmt.Sprintf("#%d", index), boolean)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{q.variable}))
}

func (q *quantifiedLinear) argument(predicate ast.ASTHandle, column uint32, current bool) (ast.ASTHandle, error) {
	sort, err := q.manager.GetDomain(predicate, column)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	decl, err := q.declaration(predicate, fmt.Sprintf("#%d", column), sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	index := q.variable
	if !current {
		index, err = q.hold(q.manager.MkBitVectorSub(index, q.one))
		if err != nil {
			return ast.ASTHandle{}, err
		}
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{index}))
}

func (q *quantifiedLinear) local(predicate, sort ast.ASTHandle, rule, index uint32) (ast.ASTHandle, error) {
	decl, err := q.declaration(predicate, fmt.Sprintf("#%d_%d", rule, index), sort)
	if err != nil {
		return ast.ASTHandle{}, err
	}
	return q.hold(q.manager.MkApp(decl, []ast.ASTHandle{q.variable}))
}

func (q *quantifiedLinear) bindings(rule *base.Rule, predicate ast.ASTHandle, ruleIndex uint32) ([]ast.ASTHandle, error) {
	sorts, err := rule.VariableSorts()
	if err != nil {
		return nil, err
	}
	values := make([]ast.ASTHandle, len(sorts))
	bind := func(app, decl ast.ASTHandle, current bool) error {
		arity, e := q.manager.GetArity(decl)
		if e != nil {
			return e
		}
		for i := uint32(0); i < arity; i++ {
			arg, e := q.manager.GetArg(app, i)
			if e != nil {
				return e
			}
			kind, e := q.manager.GetKind(arg)
			if e != nil {
				return e
			}
			if kind != ast.ASTVar {
				continue
			}
			index, e := q.manager.GetVarIndex(arg)
			if e != nil || int(index) >= len(values) {
				return errors.Join(e, ast.ErrASTDomain)
			}
			if values[index].IsNull() {
				values[index], e = q.argument(decl, i, current)
				if e != nil {
					return e
				}
			}
		}
		return nil
	}
	head, err := rule.Head()
	if err != nil {
		return nil, err
	}
	if err = bind(head, predicate, true); err != nil {
		return nil, err
	}
	count, err := rule.UninterpretedTailSize()
	if err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		tail, _, e := rule.Tail(i)
		if e != nil {
			return nil, e
		}
		decl, e := rule.TailDeclaration(i)
		if e != nil {
			return nil, e
		}
		if e = bind(tail, decl, false); e != nil {
			return nil, e
		}
	}
	var next uint32
	for i, sort := range sorts {
		if sort.IsNull() || !values[i].IsNull() {
			continue
		}
		values[i], err = q.local(predicate, sort, ruleIndex, next)
		if err != nil {
			return nil, err
		}
		next++
	}
	return values, nil
}
