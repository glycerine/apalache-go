package smtlogics

// Source: src/tactic/smtlogics/qflia_tactic.cpp/.h.

import (
	"errors"
	"io"
	"os"

	"github.com/glycerine/g3/pkg/ast"
	sattactic "github.com/glycerine/g3/pkg/sat/tactic"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/aig"
	"github.com/glycerine/g3/pkg/tactic/arith"
	"github.com/glycerine/g3/pkg/tactic/bv"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// qfliaConstruction retains concrete tactic/probe owners until the complete
// composition owns them. Composition stops on the first construction failure.
type qfliaConstruction struct {
	c          *uparams.ParamsContext
	m          *ast.ASTManager
	owners     []tactic.Tactic
	probes     []tactic.ProbeEvaluator
	parameters []*uparams.ParamsRef
	pending    []io.Closer
	failures   []memory.ConstructionFailure
	err        error
}

type QFLIAConstructionFailure struct {
	owner          *qfliaConstruction
	cause, cleanup error
}

func (e *QFLIAConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *QFLIAConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QFLIAConstructionFailure) OriginalCause() error { return e.cause }
func (e *QFLIAConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *QFLIAConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.m.NumeralService().RuntimeOwner()
}
func (e *QFLIAConstructionFailure) Cleanup() error { return e.owner.close() }

func (b *qfliaConstruction) keep(t tactic.Tactic, err error) tactic.Tactic {
	if err != nil {
		b.err = errors.Join(b.err, err)
		var failure memory.ConstructionFailure
		if errors.As(err, &failure) {
			b.failures = append(b.failures, failure)
		}
		return nil
	}
	if err = t.IncRef(); err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	b.owners = append(b.owners, t)
	return t
}
func (b *qfliaConstruction) probe(p tactic.ProbeEvaluator, err error) tactic.ProbeEvaluator {
	if err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	if err = p.IncRef(); err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	b.probes = append(b.probes, p)
	return p
}
func (b *qfliaConstruction) then(ts ...tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewAndThenTactic(ts...))
}
func (b *qfliaConstruction) clean(t tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewCleanTactic(t))
}
func (b *qfliaConstruction) otherwise(ts ...tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewOrElseTactic(ts...))
}
func (b *qfliaConstruction) using(t tactic.Tactic, p *uparams.ParamsRef) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewUsingParamsTactic(b.c, t, p))
}
func (b *qfliaConstruction) annotate(name string, t tactic.Tactic) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewAnnotateTactic(name, t))
}
func (b *qfliaConstruction) timed(t tactic.Tactic, ms uint32) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	return b.keep(tactic.NewTryForTactic(t, ms))
}
func (b *qfliaConstruction) guard(p tactic.ProbeEvaluator, positive bool) tactic.Tactic {
	if b.err != nil {
		return nil
	}
	if positive {
		return b.keep(tactic.NewFailIfNotTactic(p))
	}
	return b.keep(tactic.NewFailIfTactic(p))
}
func (b *qfliaConstruction) and(p, q tactic.ProbeEvaluator) tactic.ProbeEvaluator {
	if b.err != nil {
		return nil
	}
	return b.probe(tactic.NewBinaryProbe(b.m.Budget(), tactic.ProbeAnd, p, q))
}
func (b *qfliaConstruction) not(p tactic.ProbeEvaluator) tactic.ProbeEvaluator {
	if b.err != nil {
		return nil
	}
	return b.probe(tactic.NewNotProbe(b.m.Budget(), p))
}
func (b *qfliaConstruction) params() *uparams.ParamsRef {
	p, err := b.c.NewValue()
	if err != nil {
		b.err = errors.Join(b.err, err)
		return nil
	}
	b.parameters = append(b.parameters, &p)
	return &p
}
func (b *qfliaConstruction) boolean(p *uparams.ParamsRef, key string, value bool) {
	if b.err == nil {
		b.err = b.c.SetBoolName(p, &key, value)
	}
}
func (b *qfliaConstruction) uint(p *uparams.ParamsRef, key string, value uint32) {
	if b.err == nil {
		b.err = b.c.SetUIntName(p, &key, value)
	}
}
func (b *qfliaConstruction) rational(p *uparams.ParamsRef, key string, value int64) {
	if b.err != nil {
		return
	}
	n, err := b.c.Numerals().NewValue()
	if err != nil {
		b.err = err
		return
	}
	err = b.c.Numerals().SetInt64(&n, value)
	if err == nil {
		err = b.c.SetRatName(p, &key, &n)
	}
	b.err = errors.Join(err, b.c.Numerals().Del(&n))
}
func (b *qfliaConstruction) close() error {
	var err error
	for len(b.failures) != 0 {
		i := len(b.failures) - 1
		if e := b.failures[i].Cleanup(); e != nil {
			return e
		}
		b.failures = b.failures[:i]
	}
	for len(b.pending) != 0 {
		i := len(b.pending) - 1
		if e := b.pending[i].Close(); e != nil {
			return e
		}
		b.pending = b.pending[:i]
	}
	for len(b.owners) != 0 {
		i := len(b.owners) - 1
		owner := b.owners[i]
		b.owners = b.owners[:i]
		if e := owner.DecRef(); e != nil {
			err = errors.Join(err, e)
			if closer, ok := owner.(io.Closer); ok {
				b.pending = append(b.pending, closer)
			}
		}
	}
	for len(b.probes) != 0 {
		i := len(b.probes) - 1
		err = errors.Join(err, b.probes[i].DecRef())
		b.probes = b.probes[:i]
	}
	for len(b.parameters) != 0 {
		i := len(b.parameters) - 1
		if e := b.c.Del(b.parameters[i]); e != nil {
			return errors.Join(err, e)
		}
		b.parameters = b.parameters[:i]
	}
	return err
}

func NewQFLIATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return NewQFLIATacticWithOutput(m, p, config, os.Stdout)
}

// NewIsQuasiPBProbe preserves the source conjunction and its short circuit:
// every arithmetic constant must be bounded before quasi_pb examines bounds.
func NewIsQuasiPBProbe(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService) (result *tactic.Probe, err error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	b := qfliaConstruction{c: c, m: m}
	defer func() { err = errors.Join(err, b.close()) }()
	unbounded := b.probe(arith.NewIsUnboundedProbe(m.Budget(), c, g))
	bounded := b.not(unbounded)
	quasi := b.probe(arith.NewQuasiPBProbe(m.Budget(), c, g))
	if b.err != nil {
		return nil, b.err
	}
	return tactic.NewBinaryProbe(m.Budget(), tactic.ProbeAnd, bounded, quasi)
}

// NewQFLIATacticWithOutput borrows the current command output for optional
// native SAT DIMACS output, just as the QF_IDL strategy does.
func NewQFLIATacticWithOutput(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, output io.Writer) (result tactic.Tactic, err error) {
	c, g := config.Params, config.Globals
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err = c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	b := qfliaConstruction{c: c, m: m}
	defer func() {
		if cleanup := b.close(); cleanup != nil {
			err = &QFLIAConstructionFailure{owner: &b, cause: err, cleanup: cleanup}
		}
		if err != nil && result != nil {
			if e := result.IncRef(); e != nil {
				err = errors.Join(err, e)
			} else {
				err = errors.Join(err, result.DecRef())
			}
			result = nil
		}
	}()
	main, pull, ctx, lhs := b.params(), b.params(), b.params(), b.params()
	bits, pb, extra, quasi := b.params(), b.params(), b.params(), b.params()
	for _, key := range []string{"elim_and", "som", "blast_distinct"} {
		b.boolean(main, key, true)
	}
	b.uint(main, "blast_distinct_threshold", 128)
	b.boolean(pull, "pull_cheap_ite", true)
	b.boolean(pull, "push_ite_arith", false)
	b.boolean(pull, "local_ctx", true)
	b.uint(pull, "local_ctx_limit", 10000000)
	b.uint(ctx, "max_depth", 30)
	b.uint(ctx, "max_steps", 5000000)
	b.boolean(lhs, "arith_lhs", true)
	b.boolean(bits, "flat", false)
	b.boolean(bits, "som", false)
	policy, e := m.Symbols.Intern("dyn_psm")
	if e != nil {
		b.err = errors.Join(b.err, e)
	}
	if b.err == nil {
		key := "gc"
		b.err = c.SetSymbolName(bits, &key, policy)
	}
	b.uint(pb, "pb2bv_all_clauses_limit", 8)
	b.boolean(extra, "ite_extra", true)
	b.uint(quasi, "lia2pb_max_bits", 64)
	if b.err != nil {
		return nil, b.err
	}

	simplify := func() tactic.Tactic { return b.clean(b.keep(core.NewSimplifyTactic(m, c, g, nil))) }
	propagate := func() tactic.Tactic { return b.keep(core.NewPropagateValuesTactic(m, c, g, nil)) }
	solve := func() tactic.Tactic {
		return b.clean(b.keep(core.NewSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator)))
	}
	unbounded := func() tactic.ProbeEvaluator { return b.probe(arith.NewIsUnboundedProbe(m.Budget(), c, g)) }
	ilp := func() tactic.ProbeEvaluator {
		return b.probe(arith.NewArithmeticFragmentProbe(m.Budget(), c, g, arith.FragmentILP))
	}
	proofs := func() tactic.Tactic {
		return b.guard(b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeProduceProofs, 0)), false)
	}
	cores := func() tactic.Tactic {
		return b.guard(b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeProduceUnsatCores, 0)), false)
	}
	decided := func() tactic.Tactic { return b.keep(tactic.NewFailIfUndecidedTactic(), nil) }
	noCut := func(seed uint32, relevancy bool) tactic.Tactic {
		settings := b.params()
		b.uint(settings, "arith.branch_cut_ratio", 10000000)
		b.uint(settings, "random_seed", seed)
		name := "no-cut-smt-tactic"
		if !relevancy {
			b.uint(settings, "relevancy", 0)
			name = "no-cut-relevancy-tactic"
		}
		return b.annotate(name, b.using(b.keep(smtcore.NewSMTTacticUsing(m, nil, "", false, config)), settings))
	}
	bv2sat := func() tactic.Tactic {
		return b.using(b.then(simplify(), propagate(), solve(),
			b.clean(b.keep(bv.NewMaxBVSharingTactic(m, c, nil, config.ModelEvaluator.Cooperation))),
			b.clean(b.keep(bv.NewBitBlasterTactic(m, c, g, nil, config.ModelEvaluator.Cooperation))),
			b.clean(b.keep(aig.NewTactic(c, nil, config.ModelEvaluator.Cooperation))),
			b.clean(b.keep(sattactic.NewSATTactic(c, nil, config.ModelEvaluator, output)))), bits)
	}
	pb2bv := func() tactic.Tactic {
		return b.using(b.clean(b.keep(arith.NewPB2BVTactic(m, c, g, nil, config.ModelEvaluator))), pb)
	}
	qfbv := func() tactic.Tactic {
		return b.guard(b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeIsQFBitVector, 0)), true)
	}
	ineq := func() tactic.Tactic { return b.keep(arith.MakePropagateIneqsTactic(m, c, nil)) }
	lia2sat := func() tactic.Tactic {
		return b.annotate("lia2sat-tactic", b.then(b.guard(unbounded(), false), proofs(), cores(), ineq(),
			b.clean(b.keep(arith.NewNormalizeBoundsTactic(m, c, g, nil, config.ModelEvaluator))),
			b.clean(b.keep(arith.NewLIA2PBTactic(m, c, g, nil, config.ModelEvaluator))), pb2bv(), qfbv(), b.using(bv2sat(), extra)))
	}
	addBounds := func(lower, upper int64) tactic.Tactic {
		settings := b.params()
		b.rational(settings, "add_bound_lower", lower)
		b.rational(settings, "add_bound_upper", upper)
		return b.using(b.clean(b.keep(arith.NewAddBoundsTactic(m, c, g, nil))), settings)
	}
	initial := b.then(simplify(), propagate(), b.using(b.clean(b.keep(core.NewContextSimplifyTactic(m, c, g, nil, config.ModelEvaluator))), ctx), b.using(simplify(), pull))
	preamble := b.then(initial, solve(), b.clean(b.keep(core.NewEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator))), b.using(simplify(), lhs))
	modelFinder := b.annotate("ilp-model-finder-tactic", b.then(b.guard(b.and(ilp(), unbounded()), true), proofs(), cores(), ineq(),
		b.otherwise(b.timed(noCut(100, true), 2000), b.then(addBounds(-16, 15), b.timed(lia2sat(), 5000)),
			b.timed(noCut(200, true), 5000), b.then(addBounds(-32, 31), b.timed(lia2sat(), 10000))), decided()))
	count := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeNumExprs, 0))
	limit := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeConstant, 80000))
	var large tactic.ProbeEvaluator
	if b.err == nil {
		large = b.probe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeGreaterEqual, count, limit))
	}
	pseudo := b.annotate("pb-tactic", b.then(b.guard(b.probe(arith.NewIsPBProbe(m.Budget(), c, g)), true), proofs(), cores(),
		b.otherwise(b.then(b.guard(large, false), b.guard(ilp(), true), decided()), b.then(pb2bv(), qfbv(), b.using(bv2sat(), extra)))))
	quasiProbe := b.probe(NewIsQuasiPBProbe(m, c, g))
	quasiRoute := b.then(b.guard(quasiProbe, true), b.using(lia2sat(), quasi), decided())
	bounded := b.annotate("bounded-tactic", b.then(b.guard(unbounded(), false),
		b.otherwise(b.timed(noCut(100, true), 5000), b.timed(noCut(200, false), 5000), b.timed(noCut(300, true), 15000)), decided()))
	body := b.then(preamble, b.otherwise(modelFinder, pseudo, quasiRoute, bounded, b.keep(smtcore.NewSMTTactic(m, nil, config))))
	if b.err != nil {
		return nil, b.err
	}
	result, err = tactic.NewUsingParamsTactic(c, body, main)
	if err != nil {
		return nil, err
	}
	if p == nil {
		empty := c.EmptyValue()
		p = &empty
	}
	err = result.UpdateParams(p)
	return result, err
}
