package qe

// Source: src/qe/qsat.cpp kernel.

import (
	"errors"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// This concrete solver member is private until the full QSAT search exists.
type qsatKernel struct {
	self     *qsatKernel
	manager  *ast.ASTManager
	config   Config
	params   uparams.ParamsRef
	current  *smtcore.SMTSolver
	retired  []*smtcore.SMTSolver
	failures []memory.ConstructionFailure
	bytes    uint64
	closed   bool
}
type qsatKernelFailure struct {
	owner          *qsatKernel
	cause, cleanup error
	constructor    bool
}

func (e *qsatKernelFailure) Error() string        { return e.Unwrap().Error() }
func (e *qsatKernelFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *qsatKernelFailure) OriginalCause() error { return e.cause }
func (e *qsatKernelFailure) CleanupCause() error  { return e.cleanup }
func (e *qsatKernelFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.manager.NumeralService().RuntimeOwner()
}
func (e *qsatKernelFailure) Cleanup() error {
	if e.constructor {
		return e.owner.Close()
	}
	return e.owner.drain()
}

func newQSATKernel(m *ast.ASTManager, config Config) (result *qsatKernel, err error) {
	if m == nil || config.Context == nil || config.Globals == nil || config.Globals.Context != config.Context || config.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err = config.Context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if _, err = config.Cooperation.Enabled(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(qsatKernel{}).Size())
	if err = m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	k := &qsatKernel{manager: m, config: config, params: config.Context.EmptyValue(), bytes: n}
	k.self = k
	defer func() {
		if err != nil {
			k.retain(err)
			if cleanup := k.Close(); cleanup != nil {
				result = k
				err = &qsatKernelFailure{owner: k, cause: err, cleanup: cleanup, constructor: true}
			}
		}
	}()
	// Source initializes the solver from empty params before setting its
	// private overrides and updating the existing solver.
	k.current, err = k.create()
	if err != nil {
		return nil, err
	}
	name := "model"
	if err = config.Context.SetBoolName(&k.params, &name, true); err != nil {
		return nil, err
	}
	name = "relevancy_lvl"
	if err = config.Context.SetUIntName(&k.params, &name, 0); err != nil {
		return nil, err
	}
	name = "case_split_strategy"
	if err = config.Context.SetUIntName(&k.params, &name, 2); err != nil {
		return nil, err
	} // CS_ACTIVITY_WITH_CACHE
	if err = k.current.UpdateParams(&k.params); err != nil {
		return nil, err
	}
	// The native facade explicitly gates publication of generated models.
	if err = k.current.SetProduceModels(true); err != nil {
		return nil, err
	}
	return k, nil
}
func (k *qsatKernel) create() (*smtcore.SMTSolver, error) {
	c := k.config
	return smtcore.NewSMTSolver(k.manager, &k.params, "", smtcore.SMTSolverConfig{Params: c.Context, Globals: c.Globals, ModelEvaluator: model.ModelEvaluatorConfig{Context: c.Context, Globals: c.Globals, Memory: k.manager.NumeralService().RuntimeOwner(), Cooperation: c.Cooperation, Trace: c.Trace}})
}
func (k *qsatKernel) open() error {
	if k == nil || k.self != k {
		return ast.ErrASTHandle
	}
	if k.closed {
		return arena.ErrClosed
	}
	_, err := k.manager.ProofsEnabled()
	return err
}
func (k *qsatKernel) retain(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		k.failures = append(k.failures, failure)
	}
}
func (k *qsatKernel) drain() error {
	for len(k.failures) > 0 {
		i := len(k.failures) - 1
		if err := k.failures[i].Cleanup(); err != nil {
			return err
		}
		k.failures = k.failures[:i]
	}
	for len(k.retired) > 0 {
		i := len(k.retired) - 1
		if err := k.retired[i].Close(); err != nil {
			k.retain(err)
			return err
		}
		k.retired = k.retired[:i]
	}
	return nil
}
func (k *qsatKernel) finish(err *error) {
	k.retain(*err)
	if cleanup := k.drain(); cleanup != nil {
		*err = &qsatKernelFailure{owner: k, cause: *err, cleanup: cleanup}
	}
}
func (k *qsatKernel) Close() error {
	if k == nil || k.self != k {
		return ast.ErrASTHandle
	}
	if k.closed {
		return nil
	}
	if err := k.drain(); err != nil {
		return err
	}
	if k.current != nil {
		if err := k.current.Close(); err != nil {
			k.retain(err)
			return err
		}
		k.current = nil
	}
	if err := k.config.Context.Del(&k.params); err != nil {
		k.retain(err)
		return err
	}
	k.manager.Budget().Release(k.bytes)
	k.bytes = 0
	k.closed = true
	return nil
}
func (k *qsatKernel) solver() (*smtcore.SMTSolver, error) {
	if err := k.open(); err != nil {
		return nil, err
	}
	return k.current, nil
}
func (k *qsatKernel) reset() (err error) {
	if err = k.open(); err != nil {
		return err
	}
	if err = k.drain(); err != nil {
		return err
	}
	defer k.finish(&err)
	next, err := k.create()
	if err != nil {
		return err
	}
	// Retain a partially configured new owner without replacing the existing
	// solver until its model-publication setup has succeeded.
	if err = next.SetProduceModels(true); err != nil {
		k.retired = append(k.retired, next)
		return err
	}
	k.retired = append(k.retired, k.current)
	k.current = next
	return nil
}
func (k *qsatKernel) assertExpression(h ast.ASTHandle) (err error) {
	if err = k.open(); err != nil {
		return err
	}
	if err = k.drain(); err != nil {
		return err
	}
	defer k.finish(&err)
	return k.current.Assert(h)
}
func (k *qsatKernel) getCore(out *[]ast.ASTRef) (err error) {
	if err = k.open(); err != nil {
		return err
	}
	if out == nil {
		return ast.ErrASTHandle
	}
	if err = k.drain(); err != nil {
		return err
	}
	defer k.finish(&err)
	for len(*out) > 0 {
		i := len(*out) - 1
		if (*out)[i] != (ast.ASTRef{}) {
			if err = k.manager.DelRef(&(*out)[i]); err != nil {
				return err
			}
		}
		*out = (*out)[:i]
	}
	values, err := k.current.UnsatCore()
	if err != nil {
		return err
	}
	for _, h := range values {
		r, e := k.manager.NewRef(h)
		if e != nil {
			return e
		}
		*out = append(*out, r)
	}
	tags := k.config.Trace
	if tags == nil {
		tags = diagnostic.BorrowProcessTraceTags()
	}
	err = tags.EmitConditional("qe", "get_core", "src/qe/qsat.cpp", 562, func() bool { return true }, func(w io.Writer) error {
		w = traceBodyWriter{w}
		_, _ = io.WriteString(w, "core: ")
		if e := writeExpressionVector(k.manager, k.config, w, *out); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		if e := k.current.Display(w, nil); e != nil {
			return e
		}
		_, _ = io.WriteString(w, "\n")
		return nil
	})
	if tags.IsStreamFailure(err) {
		return nil
	}
	return err
}
