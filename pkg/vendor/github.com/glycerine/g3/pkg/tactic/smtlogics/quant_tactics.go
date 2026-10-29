package smtlogics

// Source: src/tactic/smtlogics/quant_tactics.cpp retained preprocessing and
// mk_ufnia_tactic/mk_uflra_tactic/mk_auflia_tactic/mk_auflira_tactic/mk_aufnira_tactic.

import (
	"errors"
	"io"

	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/qe"
	"github.com/glycerine/g3/pkg/smtcore"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/tactic/core"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	"github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

// QuantifiedConstructionFailure retains actual children stopped during factory
// cleanup. The caller retries cleanup after resolving the reported failure.
type QuantifiedConstructionFailure struct {
	owner          *qfliaConstruction
	cause, cleanup error
}

func (e *QuantifiedConstructionFailure) Error() string        { return e.Unwrap().Error() }
func (e *QuantifiedConstructionFailure) Unwrap() error        { return errors.Join(e.cause, e.cleanup) }
func (e *QuantifiedConstructionFailure) OriginalCause() error { return e.cause }
func (e *QuantifiedConstructionFailure) CleanupCause() error  { return e.cleanup }
func (e *QuantifiedConstructionFailure) RuntimeOwner() *memory.Runtime {
	return e.owner.m.NumeralService().RuntimeOwner()
}
func (e *QuantifiedConstructionFailure) Cleanup() error { return e.owner.close() }

// The enclosing trace stream records failed insertions. Source ostream failure
// does not skip later parameter lookups in this particular trace body.
type quantTraceBodyWriter struct{ out io.Writer }

func (w quantTraceBodyWriter) Write(p []byte) (int, error) {
	_, _ = w.out.Write(p)
	return len(p), nil
}

func NewUFLRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return newQuantifiedArithmeticTactic(m, p, config, quantStandard)
}

func NewUFNIATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return newQuantifiedArithmeticTactic(m, p, config, quantUFNIA)
}

func NewAUFLIATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return newQuantifiedArithmeticTactic(m, p, config, quantAUFLIA)
}

func NewAUFLIRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return newQuantifiedArithmeticTactic(m, p, config, quantStandard)
}

func NewAUFNIRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return newQuantifiedArithmeticTactic(m, p, config, quantStandard)
}

// NewQuantifierFreeLIRATactic is the quantifier-free retained branch of
// mk_lira_tactic: run the shared quantifier preprocessor, QE-lite, then SMT.
// The general ALL dispatcher guards this specialization with !has-quantifier.
func NewQuantifierFreeLIRATactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig) (tactic.Tactic, error) {
	return newQuantifiedArithmeticTactic(m, p, config, quantQFLIRA)
}

type quantifiedStrategy uint8

const (
	quantStandard quantifiedStrategy = iota
	quantAUFLIA
	quantUFNIA
	quantQFLIRA
)

// AUFLIA disables Gaussian elimination and tries qi.cost=0 only when the
// preprocessed goal has at most 128 expressions. UFNIA also disables Gaussian
// elimination and runs QE-lite before SMT. The remaining factories use the
// source pattern-guarded solve-eqs and a single SMT stage.
func newQuantifiedArithmeticTactic(m *ast.ASTManager, p *uparams.ParamsRef, config smtcore.SMTSolverConfig, strategy quantifiedStrategy) (result tactic.Tactic, err error) {
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
			err = &QuantifiedConstructionFailure{owner: &b, cause: err, cleanup: cleanup}
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
	var qi *uparams.ParamsRef
	if strategy == quantAUFLIA {
		qi = b.params()
		key, value := "qi.cost", "0"
		if b.err == nil {
			b.err = c.SetStringName(qi, &key, &value)
		}
		if b.err != nil {
			return nil, b.err
		}
		tags := config.ModelEvaluator.Trace
		if tags == nil {
			tags = diagnostic.BorrowProcessTraceTags()
		}
		err = tags.EmitConditional("qi_cost", "mk_auflia_tactic", "src/tactic/smtlogics/quant_tactics.cpp", 82, func() bool { return true }, func(out io.Writer) error {
			if e := c.DisplayTo(quantTraceBodyWriter{out}, qi); e != nil {
				return e
			}
			_, _ = io.WriteString(out, "\n")
			fallback := "<null>"
			value, _, e := c.GetStringName(qi, &key, &fallback)
			if e != nil {
				return e
			}
			_, _ = io.WriteString(out, value+"\n")
			return nil
		})
		if tags.IsStreamFailure(err) {
			err = nil
		}
		if err != nil {
			return nil, err
		}
	}
	pull := b.params()
	b.boolean(pull, "pull_cheap_ite", true)
	b.boolean(pull, "local_ctx", true)
	b.uint(pull, "local_ctx_limit", 10000000)
	ctx := b.params()
	b.uint(ctx, "max_depth", 30)
	b.uint(ctx, "max_steps", 5000000)
	if b.err != nil {
		return nil, b.err
	}
	var solve tactic.Tactic
	if strategy == quantAUFLIA || strategy == quantUFNIA {
		solve = b.keep(tactic.NewSkipTactic(), nil)
	} else {
		pattern := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeHasPattern, 0))
		noPattern := b.not(pattern)
		child := b.keep(core.MakeSolveEquationsTactic(m, c, g, nil, config.ModelEvaluator))
		if b.err == nil {
			solve = b.keep(tactic.NewWhenTactic(noPattern, child))
		}
	}
	pre := b.then(
		b.keep(core.MakeSimplifyTactic(m, c, g, nil)),
		b.keep(core.NewPropagateValuesTactic(m, c, g, nil)),
		b.using(b.keep(core.MakeContextSimplifyTactic(m, c, g, nil, config.ModelEvaluator)), ctx),
		b.using(b.keep(core.MakeSimplifyTactic(m, c, g, nil)), pull),
		solve,
		b.keep(core.MakeEliminateUnconstrainedTactic(m, c, g, nil, config.ModelEvaluator)),
		b.keep(core.MakeSimplifyTactic(m, c, g, nil)))
	var backend tactic.Tactic
	var elimination tactic.Tactic
	if strategy == quantUFNIA || strategy == quantQFLIRA {
		elimination = b.keep(qe.NewLiteTactic(m, p, qe.Config{Context: c, Globals: g, Cooperation: config.ModelEvaluator.Cooperation, Trace: config.ModelEvaluator.Trace}))
	}
	if strategy == quantAUFLIA {
		count := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeNumExprs, 0))
		limit := b.probe(tactic.NewProbe(m.Budget(), tactic.ProbeConstant, 128))
		var large tactic.ProbeEvaluator
		if b.err == nil {
			large = b.probe(tactic.NewBinaryProbe(m.Budget(), tactic.ProbeGreater, count, limit))
		}
		fast := b.then(b.guard(large, false), b.using(b.keep(smtcore.NewSMTTactic(m, nil, config)), qi), b.keep(tactic.NewFailIfUndecidedTactic(), nil))
		backend = b.otherwise(fast, b.keep(smtcore.NewSMTTactic(m, nil, config)))
	} else {
		backend = b.keep(smtcore.NewSMTTactic(m, nil, config))
	}
	if b.err != nil {
		return nil, b.err
	}
	if strategy == quantUFNIA || strategy == quantQFLIRA {
		result, err = tactic.NewAndThenTactic(pre, elimination, backend)
	} else {
		result, err = tactic.NewAndThenTactic(pre, backend)
	}
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
