package qe

// Source: src/qe/qsat.cpp/.h qmax, retained by opt_context::maximize_quantified.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/math/simplex"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/statistics"
)

// QMax owns the quantified-optimization search used by the retained optimizer.
// As in the source caller, construct a fresh instance for each optimization.
type QMax struct {
	self     *QMax
	manager  *ast.ASTManager
	search   *qsatSearch
	failures []memory.ConstructionFailure
	bytes    uint64
	closed   bool
}
type QMaxFailure struct {
	owner          *QMax
	cause, cleanup error
}

func (e *QMaxFailure) Error() string        { return e.Unwrap().Error() }
func (e *QMaxFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QMaxFailure) OriginalCause() error { return e.cause }
func (e *QMaxFailure) CleanupCause() error  { return e.cleanup }
func (e *QMaxFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *QMaxFailure) Cleanup() error { return e.owner.Close() }
func NewQMax(m *ast.ASTManager, _ *uparams.ParamsRef, config Config) (result *QMax, err error) {
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	n := uint64(reflect.TypeOf(QMax{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	q := &QMax{manager: m, bytes: n}
	q.self = q
	defer func() {
		if err != nil {
			var failure memory.ConstructionFailure
			if errors.As(err, &failure) {
				q.failures = append(q.failures, failure)
			}
			if cleanup := q.Close(); cleanup != nil {
				result = q
				err = &QMaxFailure{owner: q, cause: err, cleanup: cleanup}
			}
		}
	}()
	q.search, err = newQSATSearch(m, qsatMaximize, config)
	if err != nil {
		return nil, err
	}
	return q, nil
}
func (q *QMax) open() error {
	if q == nil || q.self != q {
		return ast.ErrASTHandle
	}
	if q.closed || q.search == nil {
		return arena.ErrClosed
	}
	return q.search.open()
}
func (q *QMax) Maximize(forms []ast.ASTRef, objective ast.ASTHandle, value *simplex.Optimum, md *tactic.ModelRef) (lbool.LBool, error) {
	if err := q.open(); err != nil {
		return lbool.Undef, err
	}
	return q.search.maximize(forms, objective, md, value)
}
func (q *QMax) CollectStatistics(out *statistics.Statistics) error {
	if err := q.open(); err != nil {
		return err
	}
	return q.search.collectStatistics(out)
}
func (q *QMax) Close() error {
	if q == nil || q.self != q {
		return ast.ErrASTHandle
	}
	if q.closed {
		return nil
	}
	for len(q.failures) > 0 {
		i := len(q.failures) - 1
		if err := q.failures[i].Cleanup(); err != nil {
			return err
		}
		q.failures = q.failures[:i]
	}
	if q.search != nil {
		if err := q.search.Close(); err != nil {
			return err
		}
		q.search = nil
	}
	q.manager.Budget().Release(q.bytes)
	q.bytes = 0
	q.closed = true
	return nil
}
