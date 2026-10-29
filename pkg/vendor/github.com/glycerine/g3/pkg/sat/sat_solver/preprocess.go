package sat_solver

// Source: src/sat/sat_solver/inc_sat_solver.cpp init_preprocess/internalize_goal.

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	bb "github.com/glycerine/g3/pkg/ast/rewriter/bit_blaster"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/sat"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/lbool"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	"github.com/glycerine/g3/pkg/util/z3exception"
)

// IncrementalEncoder owns the persistent bit-vector/atom state and preprocessing
// portion of inc_sat_solver. It borrows the actual SAT core; assertions, solving,
// model reconstruction and the solver facade are separate source operations.
// Internal parameters are copied. A nonnil owner returned with an error must
// be closed. The SAT core and all borrowed evaluator services outlive this owner.
type IncrementalEncoder struct {
	self        *IncrementalEncoder
	manager     *ast.ASTManager
	backend     *sat.Solver
	services    model.ModelEvaluatorConfig
	params      uparams.ParamsRef
	paramsReady bool
	atoms       *ast.Expr2Var
	blaster     *bb.BitBlasterRewriter
	pipeline    *incrementalPipeline
	retired     []*incrementalPipeline
	translation *sattactic.GoalTranslator
	last        tactic.TacticResult
	scopes      uint32
	header      uint64
	closed      bool
}
type incrementalPipeline struct {
	owner      *IncrementalEncoder
	root       tactic.Tactic
	owners     []tactic.Tactic
	pending    []io.Closer
	failures   []memory.ConstructionFailure
	simplifier uparams.ParamsRef
	paramsLive bool
	err        error
}

func NewIncrementalEncoder(m *ast.ASTManager, backend *sat.Solver, internalParams *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*IncrementalEncoder, error) {
	c, g := services.Context, services.Globals
	if m == nil || backend == nil || c == nil || g == nil || g.Context != c || services.Memory == nil || services.Cooperation == nil {
		return nil, ast.ErrASTHandle
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if err := backend.CheckOwner(); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(IncrementalEncoder{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	r := &IncrementalEncoder{manager: m, backend: backend, services: services, header: n}
	r.self = r
	var err error
	r.params, err = c.NewValue()
	r.paramsReady = err == nil
	if err == nil && internalParams != nil {
		err = c.Assign(&r.params, internalParams)
	}
	if err == nil {
		r.atoms, err = ast.NewExpr2Var(m)
	}
	if err == nil {
		err = r.initPreprocess()
	}
	return r, err
}
func (r *IncrementalEncoder) open() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return arena.ErrClosed
	}
	if !r.paramsReady || r.atoms == nil {
		return ast.ErrASTPending
	}
	return r.backend.CheckOwner()
}
func (r *IncrementalEncoder) drainRetired() error {
	for len(r.retired) > 0 {
		i := len(r.retired) - 1
		if err := r.retired[i].close(); err != nil {
			return err
		}
		r.retired = r.retired[:i]
	}
	return nil
}
func (r *IncrementalEncoder) Close() error {
	if r == nil || r.self != r {
		return ast.ErrASTHandle
	}
	if r.closed {
		return nil
	}
	if err := r.last.Close(); err != nil {
		return err
	}
	if r.translation != nil {
		if err := r.translation.Close(); err != nil {
			return err
		}
		r.translation = nil
	}
	if err := r.drainRetired(); err != nil {
		return err
	}
	if r.pipeline != nil {
		if err := r.pipeline.close(); err != nil {
			return err
		}
		r.pipeline = nil
	}
	if r.blaster != nil {
		if err := r.blaster.Close(); err != nil {
			return err
		}
		r.blaster = nil
	}
	if r.atoms != nil {
		if err := r.atoms.Close(); err != nil {
			return err
		}
		r.atoms = nil
	}
	if r.paramsReady {
		if err := r.services.Context.Del(&r.params); err != nil {
			return err
		}
		r.paramsReady = false
	}
	r.manager.Budget().Release(r.header)
	r.header = 0
	r.closed = true
	return nil
}
func (p *incrementalPipeline) keep(t tactic.Tactic, err error) tactic.Tactic {
	if err != nil {
		p.err = errors.Join(p.err, err)
		if closer, ok := t.(io.Closer); ok && !reflect.ValueOf(closer).IsNil() {
			p.pending = append(p.pending, closer)
		}
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			p.failures = append(p.failures, failure)
		}
		return nil
	}
	if err = t.IncRef(); err != nil {
		p.err = errors.Join(p.err, err)
		if closer, ok := t.(io.Closer); ok {
			p.pending = append(p.pending, closer)
		}
		return nil
	}
	p.owners = append(p.owners, t)
	return t
}
func (p *incrementalPipeline) clean(t tactic.Tactic) tactic.Tactic {
	if p.err != nil {
		return nil
	}
	return p.keep(tactic.NewCleanTactic(t))
}
func (p *incrementalPipeline) simplify() tactic.Tactic {
	r := p.clean(p.keep(core.NewSimplifyTactic(p.owner.manager, p.owner.services.Context, p.owner.services.Globals, nil)))
	if p.err != nil {
		return nil
	}
	return p.keep(tactic.NewUsingParamsTactic(p.owner.services.Context, r, &p.simplifier))
}
func (p *incrementalPipeline) close() error {
	for len(p.failures) > 0 {
		i := len(p.failures) - 1
		if err := p.failures[i].Cleanup(); err != nil {
			return err
		}
		p.failures = p.failures[:i]
	}
	for len(p.pending) > 0 {
		i := len(p.pending) - 1
		if err := p.pending[i].Close(); err != nil {
			return err
		}
		p.pending = p.pending[:i]
	}
	var result error
	for len(p.owners) > 0 {
		i := len(p.owners) - 1
		owner := p.owners[i]
		p.owners = p.owners[:i]
		if err := owner.DecRef(); err != nil {
			result = errors.Join(result, err)
			if closer, ok := owner.(io.Closer); ok {
				p.pending = append(p.pending, closer)
			}
		}
	}
	if result != nil {
		return result
	}
	if p.paramsLive {
		if err := p.owner.services.Context.Del(&p.simplifier); err != nil {
			return err
		}
		p.paramsLive = false
	}
	p.root = nil
	return nil
}
func (r *IncrementalEncoder) initPreprocess() error {
	if err := r.drainRetired(); err != nil {
		return err
	}
	if r.pipeline != nil {
		if err := r.pipeline.root.Reset(); err != nil {
			return err
		}
	}
	if r.blaster == nil {
		var err error
		r.blaster, err = bb.NewBitBlasterRewriter(r.manager, nil)
		if err != nil {
			return err
		}
		if err = r.blaster.BindCooperation(r.services.Cooperation); err != nil {
			return err
		}
	}
	p := &incrementalPipeline{owner: r}
	r.retired = append(r.retired, p)
	c := r.services.Context
	var err error
	p.simplifier, err = c.NewValue()
	p.paramsLive = err == nil
	if err == nil {
		err = c.Assign(&p.simplifier, &r.params)
	}
	if err != nil {
		return err
	}
	for _, option := range []struct {
		name  string
		value bool
	}{{"som", true}, {"pull_cheap_ite", true}, {"push_ite_bv", false}, {"local_ctx", true}, {"flat", true}, {"hoist_mul", false}, {"elim_and", true}, {"blast_distinct", true}} {
		if err = c.SetBoolName(&p.simplifier, &option.name, option.value); err != nil {
			return err
		}
	}
	name := "local_ctx_limit"
	if err = c.SetUIntName(&p.simplifier, &name, 10000000); err != nil {
		return err
	}
	card := p.keep(arith.MakeCard2BVTactic(r.manager, c, r.services.Globals, &r.params))
	first := p.simplify()
	sharing := p.clean(p.keep(bv.NewMaxBVSharingTactic(r.manager, c, &r.params, r.services.Cooperation)))
	blast := p.keep(bv.MakeBitBlasterTacticWithRewriter(r.manager, r.blaster, c, r.services.Globals, nil, r.services.Cooperation))
	last := p.simplify()
	if p.err != nil {
		return p.err
	}
	p.root = p.keep(tactic.NewAndThenTactic(card, first, sharing, blast, last))
	if p.err != nil {
		return p.err
	}
	old := r.pipeline
	r.pipeline = p
	r.retired[len(r.retired)-1] = old
	if old == nil {
		r.retired = r.retired[:len(r.retired)-1]
	}
	if err = r.drainRetired(); err != nil {
		return err
	}
	for {
		n, err := r.blaster.NumScopes()
		if err != nil {
			return err
		}
		if n >= r.scopes {
			break
		}
		if err = r.blaster.Push(); err != nil {
			return err
		}
	}
	return r.pipeline.root.Reset()
}

// Compile mutates the temporary goal through source preprocessing and appends
// its clauses to the borrowed SAT core. The latest result/dependency handles
// remain borrowed until the next compile or Close. A tactic exception drops
// the preprocessing/bit-blaster state as in source internalize_goal.
func (r *IncrementalEncoder) Compile(g *tactic.Goal) (lbool.LBool, error) {
	if err := r.open(); err != nil {
		return lbool.Undef, err
	}
	if g == nil || g.Manager() != r.manager {
		return lbool.Undef, ast.ErrASTHandle
	}
	if err := r.last.Prepare(r.manager); err != nil {
		return lbool.Undef, err
	}
	if err := r.initPreprocess(); err != nil {
		return lbool.Undef, err
	}
	models, err := g.ModelsEnabled()
	if err != nil {
		return lbool.Undef, err
	}
	if ast.ASTSourceDebug && !models {
		return lbool.Undef, ast.ErrASTDomain
	}
	proofs, err := g.ProofsEnabled()
	if err != nil {
		return lbool.Undef, err
	}
	if proofs {
		return lbool.Undef, z3exception.NewDefaultException("generation of proof objects is not supported in this mode")
	}
	if err = incrementalTrace(r.services, "internalize_goal", 399, func(out io.Writer) error { return g.Display(out) }); err != nil {
		return lbool.Undef, err
	}
	if err = r.pipeline.root.Apply(g, &r.last); err != nil {
		var exception tactic.TacticException
		if errors.As(err, &exception) {
			if e := incrementalVerbose(0, "exception in tactic "+exception.Message()+"\n"); e != nil {
				return lbool.Undef, e
			}
			if e := incrementalTrace(r.services, "internalize_goal", 405, func(out io.Writer) error {
				_, e := io.WriteString(out, "exception: "+exception.Message()+"\n")
				return e
			}); e != nil {
				return lbool.Undef, e
			}
			r.retired = append(r.retired, r.pipeline)
			r.pipeline = nil
			if cleanup := r.drainRetired(); cleanup != nil {
				return lbool.Undef, errors.Join(err, cleanup)
			}
			if cleanup := r.blaster.Close(); cleanup != nil {
				return lbool.Undef, errors.Join(err, cleanup)
			}
			r.blaster = nil
			return lbool.Undef, nil
		}
		return lbool.Undef, err
	}
	if len(r.last.Goals) != 1 {
		if err = incrementalVerbose(0, fmt.Sprintf("size of subgoals is not 1, it is: %d\n", len(r.last.Goals))); err != nil {
			return lbool.Undef, err
		}
		return lbool.Undef, nil
	}
	if err = incrementalTrace(r.services, "internalize_goal", 416, func(out io.Writer) error { return r.last.Goals[0].DisplayWithDependencies(out) }); err != nil {
		return lbool.Undef, err
	}
	if r.translation != nil {
		if err = r.translation.Close(); err != nil {
			return lbool.Undef, err
		}
		r.translation = nil
	}
	r.translation, err = sattactic.NewGoalTranslatorWithAtomMap(r.manager, r.backend, r.atoms, true)
	if err != nil {
		return lbool.Undef, err
	}
	if err = r.translation.BindCooperation(r.services.Cooperation); err != nil {
		return lbool.Undef, err
	}
	if err = r.translation.CompileGoal(r.last.Goals[0]); err != nil {
		return lbool.Undef, err
	}
	interpreted, err := r.translation.HasInterpretedAtoms()
	if err != nil || interpreted {
		return lbool.Undef, err
	}
	return lbool.True, nil
}
func (r *IncrementalEncoder) Push() error {
	if err := r.open(); err != nil {
		return err
	}
	if err := r.backend.UserPush(); err != nil {
		return err
	}
	r.scopes++
	if r.blaster != nil {
		if err := r.blaster.Push(); err != nil {
			return err
		}
	}
	return r.atoms.Push()
}
func (r *IncrementalEncoder) Pop(n uint32) error {
	if err := r.open(); err != nil {
		return err
	}
	if n > r.scopes {
		n = r.scopes
	}
	if r.blaster != nil {
		if err := r.blaster.Pop(n); err != nil {
			return err
		}
	}
	if err := r.atoms.Pop(n); err != nil {
		return err
	}
	if err := r.backend.UserPop(n); err != nil {
		return err
	}
	r.scopes -= n
	return nil
}

// BitVectorBindings returns the persistent source const2bits map, with handles
// borrowed until the corresponding scope is popped or the encoder is closed.
func (r *IncrementalEncoder) BitVectorBindings() (map[ast.ASTHandle]ast.ASTHandle, error) {
	if err := r.open(); err != nil {
		return nil, err
	}
	if r.blaster == nil {
		return nil, nil
	}
	return r.blaster.Const2Bits()
}
func (r *IncrementalEncoder) ScopeLevel() (uint32, error) {
	if err := r.open(); err != nil {
		return 0, err
	}
	return r.scopes, nil
}
