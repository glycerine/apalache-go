package ackermannization

// Source: src/ackermannization/lackr.cpp/.h, solver-backed eager/lazy paths.

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/solver"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

type acquiredLackrModel struct {
	model   *model.ModelCore
	cleanup bool
}

// Lackr owns one reduction/solve invocation. The UF-free backend and statistics
// are borrowed. Constructor parameters are copied; UpdateParams changes only
// eager/lazy selection, as in the source. A nonnil partial owner returned with
// an error must be closed.
type Lackr struct {
	encodingState
	self        *Lackr
	backend     solver.Solver
	encoder     eagerEncoder
	checker     *LazyModelChecker
	model       tactic.ModelRef
	candidate   tactic.ModelRef
	acquired    []acquiredLackrModel
	paramsReady bool
	header      uint64
	closed      bool
}

func NewLackr(m *ast.ASTManager, p *uparams.ParamsRef, stats *Stats, formulas []ast.ASTHandle, backend solver.Solver, services model.ModelEvaluatorConfig) (*Lackr, error) {
	c, g := services.Context, services.Globals
	if m == nil || stats == nil || c == nil || g == nil || g.Context != c || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(Lackr{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &Lackr{encodingState: encodingState{manager: m, context: c, globals: g, services: services, stats: stats}, backend: backend, header: n}
	r.self = r
	r.encoder.owner = &r.encodingState
	r.encoder.groups = make(termGroups)
	var err error
	r.params, err = c.NewValue()
	r.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&r.params, p)
	}
	if err == nil {
		err = r.encoder.constructForms(formulas)
	}
	r.retainFailure(err)
	return r, err
}
func (r *Lackr) retainFailure(err error) {
	var failure memory.ConstructionFailure
	if errors.As(err, &failure) {
		r.encoder.failures = append(r.encoder.failures, failure)
	}
}
func (r *Lackr) open() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return arena.ErrClosed
	}
	if !r.paramsReady {
		return ast.ErrASTPending
	}
	return nil
}
func (r *Lackr) releaseAcquired() error {
	for len(r.acquired) > 0 {
		i := len(r.acquired) - 1
		x := &r.acquired[i]
		if x.cleanup {
			if err := x.model.Close(); err != nil {
				return err
			}
		} else {
			consumed, err := x.model.DecRefConsumed()
			if consumed && err != nil {
				x.cleanup = true
			}
			if err != nil {
				return err
			}
		}
		r.acquired = r.acquired[:i]
	}
	return nil
}
func (r *Lackr) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if r.checker != nil {
		if err := r.checker.Close(); err != nil {
			return err
		}
		r.checker = nil
	}
	if err := r.candidate.Close(); err != nil {
		return err
	}
	if err := r.model.Close(); err != nil {
		return err
	}
	if err := r.releaseAcquired(); err != nil {
		return err
	}
	if err := r.encoder.Close(); err != nil {
		return err
	}
	if r.paramsReady {
		if err := r.context.Del(&r.params); err != nil {
			return err
		}
		r.paramsReady = false
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
func (r *Lackr) UpdateParams(p *uparams.ParamsRef) error {
	if err := r.open(); err != nil {
		return err
	}
	if p == nil {
		empty := r.context.EmptyValue()
		p = &empty
	}
	eager, err := r.readEager(p)
	if err == nil {
		r.encoder.eager = eager
	}
	return err
}
func (r *Lackr) BindTrace(tags *diagnostic.TraceTags) error {
	if err := r.open(); err != nil {
		return err
	}
	r.trace = tags
	return nil
}
func (r *Lackr) checkpoint() error {
	canceled, err := r.manager.Canceled()
	if err != nil {
		return err
	}
	if canceled {
		return tactic.NewTacticException(tactic.TacticCanceledMessage)
	}
	return r.services.Cooperation.Cooperate("lackr")
}
func (r *Lackr) acquire(out *tactic.ModelRef) error {
	md, err := r.backend.AcquireModel()
	if md != nil {
		r.acquired = append(r.acquired, acquiredLackrModel{model: md})
	}
	if err != nil {
		return err
	}
	if err = out.Set(md); err != nil {
		return err
	}
	return r.releaseAcquired()
}
func (r *Lackr) Solve() (status lbool.LBool, err error) {
	defer func() { r.retainFailure(err) }()
	if err = r.open(); err != nil {
		return lbool.Undef, err
	}
	if r.backend == nil {
		return lbool.Undef, ast.ErrASTHandle
	}
	initialized, err := r.encoder.initialize()
	if err != nil || !initialized {
		return lbool.Undef, err
	}
	if r.encoder.eager {
		status, err = r.eager()
	} else {
		status, err = r.lazy()
	}
	if err != nil || status != lbool.True {
		return status, err
	}
	if err = r.acquire(&r.model); err != nil {
		return status, err
	}
	err = r.encoder.emit("operator()", 59, func(w io.Writer) error {
		md, err := r.model.Get()
		if err != nil {
			return err
		}
		if _, err = io.WriteString(w, "abstr_model(\n"); err != nil {
			return err
		}
		if err = md.WriteSMT2(w, r.context, r.globals, 2); err != nil {
			return err
		}
		_, err = io.WriteString(w, ")\n")
		return err
	})
	return status, err
}
func (r *Lackr) Info() (*Info, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.encoder.info == nil {
		return nil, ast.ErrASTPending
	}
	if err := r.encoder.info.IncRef(); err != nil {
		return nil, err
	}
	return r.encoder.info, nil
}
func (r *Lackr) AcquireModel() (*model.ModelCore, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	md, err := r.model.Get()
	if err != nil || md == nil {
		return md, err
	}
	return md, md.IncRef()
}
func (r *Lackr) pushAbstraction() error {
	for _, f := range r.encoder.abstracted {
		if err := r.backend.Assert(f); err != nil {
			return err
		}
	}
	return nil
}
func (r *Lackr) eager() (lbool.LBool, error) {
	if err := r.pushAbstraction(); err != nil {
		return lbool.Undef, err
	}
	if err := r.encoder.emit("eager", 207, func(w io.Writer) error { _, e := io.WriteString(w, "run sat 0\n"); return e }); err != nil {
		return lbool.Undef, err
	}
	status, err := r.backend.CheckSat(nil)
	if err != nil || status == lbool.False {
		return status, err
	}
	if err = r.encoder.encode(); err != nil {
		return lbool.Undef, err
	}
	all, err := r.manager.MkAndArgs(r.encoder.lemmas)
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.manager.SetRef(r.encoder.out, all); err != nil {
		return lbool.Undef, err
	}
	if err = r.encoder.simplifier.Rewrite(all, r.encoder.simplified); err != nil {
		return lbool.Undef, err
	}
	all, err = r.manager.RefGet(r.encoder.simplified)
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.backend.Assert(all); err != nil {
		return lbool.Undef, err
	}
	if err = r.encoder.emit("eager", 215, func(w io.Writer) error { _, e := io.WriteString(w, "run sat all\n"); return e }); err != nil {
		return lbool.Undef, err
	}
	return r.backend.CheckSat(nil)
}
func (r *Lackr) lazy() (lbool.LBool, error) {
	var err error
	r.checker, err = NewLazyModelChecker(r.manager, r.encoder.info, r.services)
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.checker.BindTrace(r.trace); err != nil {
		return lbool.Undef, err
	}
	if err = r.pushAbstraction(); err != nil {
		return lbool.Undef, err
	}
	head := 0
	for {
		r.stats.Iterations++
		if err = r.checkpoint(); err != nil {
			return lbool.Undef, err
		}
		if err = r.encoder.emit("lazy", 227, func(w io.Writer) error { _, e := fmt.Fprintf(w, "lazy check: %d\n", r.stats.Iterations); return e }); err != nil {
			return lbool.Undef, err
		}
		status, err := r.backend.CheckSat(nil)
		if err != nil || status != lbool.True {
			return status, err
		}
		if err = r.acquire(&r.candidate); err != nil {
			return lbool.Undef, err
		}
		md, err := r.candidate.Get()
		if err != nil {
			return lbool.Undef, err
		}
		ok, err := r.checker.Check(md)
		if err != nil {
			return lbool.Undef, err
		}
		if ok {
			return lbool.True, nil
		}
		conflicts, err := r.checker.Conflicts()
		if err != nil {
			return lbool.Undef, err
		}
		for _, pair := range conflicts {
			if err = r.encoder.ackermann(pair.First, pair.Second); err != nil {
				return lbool.Undef, err
			}
		}
		for head < len(r.encoder.lemmas) {
			lemma := r.encoder.lemmas[head]
			head++ // Source post-increments before assert_expr.
			if err = r.backend.Assert(lemma); err != nil {
				return lbool.Undef, err
			}
		}
	}
}
