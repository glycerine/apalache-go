package model

// Source: src/model/model_evaluator.cpp:41-203,233-302,502-579;
// src/model/model_evaluator.h:30-64; src/model/model_evaluator_params.pyg.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"

	"github.com/glycerine/g3/pkg/util/cooperate"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	"github.com/glycerine/g3/pkg/util/messages"
	uparams "github.com/glycerine/g3/pkg/util/params"
)

var ErrModelEvaluatorTheory = errors.New("g3: model evaluator has no reducer for the theory")

// ModelEvaluatorConfig supplies the source's borrowed process services. Memory
// is the original raw allocation accounting owner; typed runtime-wide allocator
// wiring remains a separate port obligation.
type ModelEvaluatorConfig struct {
	Context     *uparams.ParamsContext
	Globals     *uparams.GlobalParamsService
	Params      *uparams.ParamsRef
	Memory      *umemory.Runtime
	Cooperation *cooperate.Cooperation
	Trace       *diagnostic.TraceTags
}

// ModelEvaluator runs the original no-proof traversal and evaluator_cfg.
type ModelEvaluator struct {
	self          *ModelEvaluator
	model         *ModelCore
	services      ModelEvaluatorConfig
	cfg           evaluatorConfiguration
	traversal     *rewriter.Traversal
	header        uint64
	impGeneration uint64
	impLive       bool
	ready, closed bool
}
type evaluatorConfiguration struct {
	expansion arrayExpansion
	equality  arrayEquality
	rewriter.DefaultConfiguration
	owner                               *ModelEvaluator
	boolean                             *rewriter.BoolRewriter
	arithmetic                          *rewriter.ArithRewriter
	bitVector                           *rewriter.BVRewriter
	array                               *rewriter.ArrayRewriter
	datatype                            *rewriter.DatatypeRewriter
	pseudoBoolean                       *rewriter.PseudoBooleanRewriter
	fpa                                 *rewriter.FPARewriter
	sequence                            *rewriter.SequenceRewriter
	arithmeticFamily, bitVectorFamily   ast.FamilyID
	arrayFamily, datatypeFamily         ast.FamilyID
	pseudoBooleanFamily, sequenceFamily ast.FamilyID
	substitution                        *rewriter.VarSubstitution
	params                              uparams.ModuleParamsOwner
	pending                             *FunctionInterpretation
	maxMemory                           uint64
	maxSteps                            uint32
	completion, cache, arrayEqualities  bool
}

func NewModelEvaluator(md *ModelCore, s ModelEvaluatorConfig) (*ModelEvaluator, error) {
	if md == nil || s.Context == nil || s.Globals == nil || s.Memory == nil || s.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if e := md.open(); e != nil {
		return nil, e
	}
	if s.Context.SymbolsField != md.manager.Symbols || s.Globals.Context != s.Context {
		return nil, uparams.ErrParamsOwner
	}
	astNumerals, paramsNumerals := md.manager.NumeralService(), s.Context.Numerals()
	if astNumerals == nil || paramsNumerals == nil || astNumerals.RuntimeOwner() == nil || paramsNumerals.RuntimeOwner() == nil {
		return nil, uparams.ErrParamsOwner
	}
	process, astProcess, paramsProcess := s.Memory.ProcessOwner(), astNumerals.RuntimeOwner().ProcessOwner(), paramsNumerals.RuntimeOwner().ProcessOwner()
	if (process != nil || astProcess != nil || paramsProcess != nil) && (process != astProcess || process != paramsProcess) {
		return nil, uparams.ErrParamsOwner
	}
	if _, e := s.Memory.AllocationSize(); e != nil {
		return nil, e
	}
	if _, e := s.Cooperation.Enabled(); e != nil {
		return nil, e
	}
	n := uint64(reflect.TypeOf(ModelEvaluator{}).Size())
	if e := md.manager.Budget().Reserve(n); e != nil {
		return nil, e
	}
	ev := &ModelEvaluator{model: md, services: s, header: n}
	ev.self = ev
	ev.services.Params = nil
	ev.impLive = true
	ev.traceImpAllocated(523)
	return ev, ev.init(s.Params)
}
func (ev *ModelEvaluator) init(p *uparams.ParamsRef) error {
	ev.cfg = evaluatorConfiguration{owner: ev}
	ev.cfg.params = uparams.ModuleParamsOwner{Budget: ev.services.Globals.Config.Budget}
	ev.cfg.params.Self = &ev.cfg.params
	if e := ev.cfg.params.Bind(ev.services.Context, ev.services.Globals); e != nil {
		return e
	}
	var e error
	ev.traversal, e = rewriter.NewTraversal(ev.model.manager, &ev.cfg)
	if e != nil {
		return e
	}
	if e = ev.traversal.EnableModelBooleanShortCircuit(); e != nil {
		return e
	}
	ev.cfg.boolean, e = rewriter.NewBoolRewriter(ev.model.manager, ev.services.Context, ev.services.Globals, nil)
	if e != nil {
		return e
	}
	ev.cfg.arithmetic, e = rewriter.NewArithRewriter(ev.model.manager, ev.services.Context, ev.services.Globals, p)
	if e != nil {
		return e
	}
	legacy, e := ev.services.Globals.Manager()
	if e != nil {
		return e
	}
	ev.cfg.bitVector, e = rewriter.NewBVRewriter(ev.model.manager, ev.services.Context, legacy, p)
	if e != nil {
		return e
	}
	ev.cfg.array, e = rewriter.NewArrayRewriter(ev.model.manager, ev.services.Context, ev.services.Globals, p)
	if e != nil {
		return e
	}
	if _, e = ev.model.manager.RegisterDatatypePlugin(); e != nil {
		return e
	}
	ev.cfg.datatype, e = rewriter.NewDatatypeRewriter(ev.model.manager)
	if e != nil {
		return e
	}
	ev.cfg.pseudoBoolean, e = rewriter.NewPseudoBooleanRewriter(ev.model.manager)
	if e != nil {
		return e
	}
	ev.cfg.sequence, e = rewriter.NewSequenceRewriter(ev.model.manager)
	if e != nil {
		return e
	}
	ev.cfg.arithmeticFamily, e = ev.model.manager.ArithmeticFamilyID()
	if e != nil {
		return e
	}
	ev.cfg.bitVectorFamily, e = ev.model.manager.BitVectorUtilityFamilyID()
	if e != nil {
		return e
	}
	ev.cfg.arrayFamily, e = ev.cfg.array.FamilyID()
	if e != nil {
		return e
	}
	ev.cfg.datatypeFamily, e = ev.model.manager.DatatypeFamilyID()
	if e != nil {
		return e
	}
	ev.cfg.pseudoBooleanFamily, e = ev.model.manager.RegisterPseudoBooleanPlugin()
	if e != nil {
		return e
	}
	ev.cfg.sequenceFamily, e = ev.model.manager.SequenceFamilyID()
	if e != nil {
		return e
	}
	if e = ev.cfg.boolean.SetFlat(true); e != nil {
		return e
	}
	if e = ev.cfg.bitVector.SetMkBVToNumeral(true); e != nil {
		return e
	}
	if e = ev.cfg.array.SetExpandSelectStore(true); e != nil {
		return e
	}
	if e = ev.cfg.array.SetExpandSelectIte(true); e != nil {
		return e
	}
	if e = ev.cfg.updateParams(p); e != nil {
		return e
	}
	if e = ev.traversal.SetCancelCheck(false); e != nil {
		return e
	}
	ev.ready = true
	return nil
}
func (ev *ModelEvaluator) open() error {
	if ev == nil || ev.self != ev {
		return ast.ErrASTHandle
	}
	if ev.closed {
		return arena.ErrClosed
	}
	if !ev.ready || ev.cfg.pending != nil || ev.cfg.substitution != nil || ev.cfg.expansion.active || ev.cfg.equality.active {
		return uparams.ErrParamsPrecondition
	}
	return ev.model.open()
}

// Manager returns the borrowed manager used by this evaluator.
func (ev *ModelEvaluator) Manager() (*ast.ASTManager, error) {
	if err := ev.open(); err != nil {
		return nil, err
	}
	return ev.model.manager, nil
}
func (ev *ModelEvaluator) Evaluate(t ast.ASTHandle, out ast.ASTRef) error {
	if e := ev.open(); e != nil {
		return e
	}
	if e := ev.traceEvaluate(t); e != nil {
		return e
	}
	if e := ev.traversal.Rewrite(t, out); e != nil {
		return e
	}
	return ev.cfg.expandValue(out)
}

// EvaluateValue is the source value-returning overload. Its entry TRACE runs
// before constructing the local result reference; Evaluate then emits the
// output-reference overload's second entry record.
func (ev *ModelEvaluator) EvaluateValue(t ast.ASTHandle) (ast.ASTRef, error) {
	if e := ev.open(); e != nil {
		return ast.ASTRef{}, e
	}
	if e := ev.traceEvaluateAt(t, 572); e != nil {
		return ast.ASTRef{}, e
	}
	result, e := ev.model.manager.NewRef(ast.ASTHandle{})
	if e != nil {
		return ast.ASTRef{}, e
	}
	if e = ev.Evaluate(t, result); e != nil {
		return ast.ASTRef{}, errors.Join(e, ev.model.manager.DelRef(&result))
	}
	return result, nil
}
func (ev *ModelEvaluator) UpdtParams(p *uparams.ParamsRef) error {
	if e := ev.open(); e != nil {
		return e
	}
	return ev.cfg.updateParams(p)
}
func (ev *ModelEvaluator) Reset(p *uparams.ParamsRef) error {
	if e := ev.open(); e != nil {
		return e
	}
	if e := ev.traversal.Reset(); e != nil {
		return e
	}
	return ev.cfg.updateParams(p)
}
func (ev *ModelEvaluator) SetModelCompletion(enabled bool) error {
	if e := ev.open(); e != nil {
		return e
	}
	ev.cfg.completion = enabled
	return nil
}
func (ev *ModelEvaluator) SetExpandArrayEqualities(enabled bool) error {
	if e := ev.open(); e != nil {
		return e
	}
	ev.cfg.arrayEqualities = enabled
	return nil
}
func (ev *ModelEvaluator) NumSteps() (uint32, error) {
	if e := ev.open(); e != nil {
		return 0, e
	}
	return ev.traversal.NumSteps()
}
func (ev *ModelEvaluator) closeImp() error {
	ev.ready = false
	if e := ev.cfg.equality.close(ev.model.manager); e != nil {
		return e
	}
	if e := ev.cfg.expansion.close(ev.model.manager); e != nil {
		return e
	}
	if ev.cfg.substitution != nil {
		if e := ev.cfg.substitution.Close(); e != nil {
			return e
		}
		ev.cfg.substitution = nil
	}
	if ev.cfg.pending != nil {
		if e := ev.cfg.pending.Close(); e != nil {
			return e
		}
		ev.cfg.pending = nil
	}
	if e := ev.cfg.params.CloseHelpers(); e != nil {
		return e
	}
	ev.cfg.sequence = nil
	if ev.cfg.fpa != nil {
		if e := ev.cfg.fpa.Close(); e != nil {
			return e
		}
		ev.cfg.fpa = nil
	}
	if ev.cfg.pseudoBoolean != nil {
		if e := ev.cfg.pseudoBoolean.Close(); e != nil {
			return e
		}
		ev.cfg.pseudoBoolean = nil
	}
	ev.cfg.datatype = nil
	if ev.cfg.array != nil {
		if e := ev.cfg.array.Close(); e != nil {
			return e
		}
		ev.cfg.array = nil
	}
	if ev.cfg.bitVector != nil {
		if e := ev.cfg.bitVector.Close(); e != nil {
			return e
		}
		ev.cfg.bitVector = nil
	}
	if ev.cfg.arithmetic != nil {
		if e := ev.cfg.arithmetic.Close(); e != nil {
			return e
		}
		ev.cfg.arithmetic = nil
	}
	if ev.cfg.boolean != nil {
		if e := ev.cfg.boolean.Close(); e != nil {
			return e
		}
		ev.cfg.boolean = nil
	}
	if ev.traversal != nil {
		if e := ev.traversal.Close(); e != nil {
			return e
		}
		ev.traversal = nil
	}
	return nil
}
func (ev *ModelEvaluator) Cleanup(p *uparams.ParamsRef) error {
	if e := ev.open(); e != nil {
		return e
	}
	if e := ev.closeImp(); e != nil {
		return e
	}
	ev.traceImpDeallocated(556)
	ev.impLive = false
	ev.impGeneration++
	ev.impLive = true
	ev.traceImpAllocated(557)
	return ev.init(p)
}
func (ev *ModelEvaluator) Close() error {
	if ev == nil || ev.self != ev {
		return ast.ErrASTHandle
	}
	if ev.closed {
		return nil
	}
	if e := ev.closeImp(); e != nil {
		return e
	}
	ev.traceImpDeallocated(531)
	ev.impLive = false
	ev.model.manager.Budget().Release(ev.header)
	ev.header = 0
	ev.closed = true
	return nil
}
func (c *evaluatorConfiguration) updateParams(local *uparams.ParamsRef) (err error) {
	o := &c.params
	if o.Running || o.DestructorCause != nil {
		return uparams.ErrParamsPrecondition
	}
	if local == nil {
		empty, e := o.Context.NewValue()
		if e != nil {
			return e
		}
		local = &empty
		defer func() { err = errors.Join(err, o.Context.Del(&empty)) }()
	}
	if e := o.Context.Check(local); e != nil {
		return e
	}
	o.Running = true
	defer func() { o.Running = false }()
	f, e := o.BeginHelper(local, "model_evaluator")
	if f != nil {
		defer func() { err = o.FinishHelper(f, err) }()
	}
	if e != nil {
		return e
	}
	u, e := o.FrameUInt(f, "max_memory", ^uint32(0))
	if e != nil {
		return e
	}
	c.maxMemory = umemory.MegabytesToBytes(u)
	u, e = o.FrameUInt(f, "max_steps", ^uint32(0))
	if e != nil {
		return e
	}
	c.maxSteps = u
	b, e := o.FrameBool(f, "completion", false)
	if e != nil {
		return e
	}
	c.completion = b
	b, e = o.FrameBool(f, "cache", true)
	if e != nil {
		return e
	}
	c.cache = b
	b, e = o.FrameBool(f, "array_equalities", true)
	if e != nil {
		return e
	}
	c.arrayEqualities = b
	return nil
}
func (c *evaluatorConfiguration) CacheResults() bool { return c.cache }
func (c *evaluatorConfiguration) MaxStepsExceeded(n uint32) (bool, error) {
	if e := c.owner.services.Cooperation.Cooperate("model evaluator"); e != nil {
		return false, e
	}
	size, e := c.owner.services.Memory.AllocationSize()
	if e != nil {
		return false, e
	}
	if size > c.maxMemory {
		return false, errors.New(messages.MaxMemory)
	}
	return n > c.maxSteps, nil
}
func (c *evaluatorConfiguration) evaluate(d ast.ASTHandle, args ast.AppArguments, out ast.ASTRef) (bool, error) {
	fi, e := c.owner.model.FunctionInterpretation(d)
	if e != nil || fi == nil {
		return false, e
	}
	if fi.NumEntries() == 0 {
		return false, nil
	}
	if args.Count() != fi.Arity() {
		return false, ast.ErrASTDomain
	}
	m := c.owner.model.manager
	for i := uint32(0); i < args.Count(); i++ {
		h, e := args.At(i)
		if e != nil {
			return false, e
		}
		value, e := m.IsValue(h)
		if e != nil || !value {
			return false, e
		}
	}
	entry, e := fi.FindEntryArguments(args)
	if e != nil || entry == nil {
		return false, e
	}
	result, e := entry.Result()
	if e != nil {
		return false, e
	}
	return true, m.SetRef(out, result)
}
func (c *evaluatorConfiguration) ReduceApp(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	md := c.owner.model
	m := md.manager
	if e := m.ResetRef(proof); e != nil {
		return rewriter.RewriteFailed, e
	}
	fid, e := m.GetFamilyID(d)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	fpaFamily, e := m.ExistingFloatingPointFamilyID()
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	consideredUninterpreted := false
	if fid != ast.NullFamilyID {
		consideredUninterpreted, e = m.IsConsideredUninterpreted(d)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	if args.Count() == 0 && (fid == ast.NullFamilyID || consideredUninterpreted) {
		value, e := md.ConstantInterpretation(d)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if !value.IsNull() {
			return rewriter.RewriteDone, m.SetRef(out, value)
		}
		if c.completion {
			s, e := m.GetRange(d)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			value, e = md.GetSomeValue(s)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			if e = md.RegisterConstant(d, value); e != nil {
				return rewriter.RewriteFailed, e
			}
			return rewriter.RewriteDone, m.SetRef(out, value)
		}
		return rewriter.RewriteFailed, nil
	}
	if fid == 0 {
		op, e := m.GetDeclKind(d)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if op == ast.ASTOpEq {
			if args.Count() != 2 {
				return rewriter.RewriteFailed, ast.ErrASTDomain
			}
			arg, e := args.At(0)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			sort, e := m.GetSort(arg)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			sortFamily, e := m.GetFamilyID(sort)
			if e != nil {
				return rewriter.RewriteFailed, e
			}
			// The source consults the sort's theory equality reducer first.
			if sortFamily == c.arithmeticFamily {
				right, e := args.At(1)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				status, e := c.arithmetic.MkEqCore(arg, right, out)
				if e != nil || status != rewriter.RewriteFailed {
					return status, e
				}
			} else if sortFamily == c.bitVectorFamily {
				right, e := args.At(1)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				status, e := c.bitVector.RewriteEquality(arg, right, out)
				if e != nil || status != rewriter.RewriteFailed {
					return status, e
				}
			} else if sortFamily == c.datatypeFamily {
				right, e := args.At(1)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				status, e := c.datatype.RewriteEquality(arg, right, out)
				if e != nil || status != rewriter.RewriteFailed {
					return status, e
				}
			} else if sortFamily == c.arrayFamily {
				right, e := args.At(1)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				status, e := c.arrayEq(arg, right, out)
				if e != nil || status != rewriter.RewriteFailed {
					return status, e
				}
			} else if fpaFamily != ast.NullFamilyID && sortFamily == fpaFamily {
				right, e := args.At(1)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				rw, e := c.fpaRewriter()
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				status, e := rw.MkEqCore(arg, right, out)
				if e != nil || status != rewriter.RewriteFailed {
					return status, e
				}
			} else if sortFamily == c.sequenceFamily {
				right, e := args.At(1)
				if e != nil {
					return rewriter.RewriteFailed, e
				}
				status, e := c.sequence.RewriteEquality(arg, right, out)
				if e != nil || status != rewriter.RewriteFailed {
					return status, e
				}
			} else if sortFamily > 4 {
				return rewriter.RewriteFailed, ErrModelEvaluatorTheory
			}
		}
		status, e := c.boolean.MkAppCore(d, args, out)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		final, e := c.evaluateReducedApp(status, out)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if status == rewriter.RewriteDone && final == rewriter.Rewrite1 {
			return final, nil
		}
		if e = c.traceReduceStatus(final, out); e != nil {
			return rewriter.RewriteFailed, e
		}
		return final, nil
	}
	if fid == c.arithmeticFamily {
		status, e := c.arithmetic.MkAppCore(d, args, out)
		return c.finishTheoryReduction(status, e, d, args, out, proof)
	}
	if fid == c.bitVectorFamily {
		status, e := c.bitVector.RewriteApp(d, args, out)
		return c.finishTheoryReduction(status, e, d, args, out, proof)
	}
	if fid == c.arrayFamily {
		status, e := c.array.MkAppCore(d, args, out)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if status == rewriter.RewriteFailed {
			return c.evaluatePartialTheoryWithTrace(d, args, out, proof)
		}
		final, e := c.evaluateReducedApp(status, out)
		if e != nil || status == rewriter.RewriteDone && final == rewriter.Rewrite1 {
			return final, e
		}
		if e = c.traceReduceStatus(final, out); e != nil {
			return rewriter.RewriteFailed, e
		}
		return final, nil
	}
	if fid == c.datatypeFamily {
		status, e := c.datatype.RewriteApp(d, args, out)
		return c.finishTheoryReduction(status, e, d, args, out, proof)
	}
	if fid == c.pseudoBooleanFamily {
		status, e := c.pseudoBoolean.RewriteApp(d, args, out)
		return c.finishTheoryReduction(status, e, d, args, out, proof)
	}
	if fpaFamily != ast.NullFamilyID && fid == fpaFamily {
		rw, e := c.fpaRewriter()
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		status, e := rw.MkAppCore(d, args, out)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if status == rewriter.RewriteFailed {
			return c.evaluatePartialTheoryWithTrace(d, args, out, proof)
		}
		final, e := c.evaluateReducedApp(status, out)
		if e != nil || status == rewriter.RewriteDone && final == rewriter.Rewrite1 {
			return final, e
		}
		if e = c.traceReduceStatus(final, out); e != nil {
			return rewriter.RewriteFailed, e
		}
		return final, nil
	}
	if fid == c.sequenceFamily {
		status, e := c.sequence.RewriteApp(d, args, out)
		return c.finishTheoryReduction(status, e, d, args, out, proof)
	}
	if fid == 1 && args.Count() == 1 {
		value, e := args.At(0)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = m.SetRef(out, value); e != nil {
			return rewriter.RewriteFailed, e
		}
		final, e := c.evaluateReducedApp(rewriter.RewriteDone, out)
		if e != nil || final == rewriter.Rewrite1 {
			return final, e
		}
		if e = c.traceReduceStatus(final, out); e != nil {
			return rewriter.RewriteFailed, e
		}
		return final, nil
	}
	// The original arithmetic/BV reducers leave their unspecified symbols at
	// BR_FAILED, then invoke the existing partial-theory interpretation path.
	if consideredUninterpreted {
		return c.evaluatePartialTheoryWithTrace(d, args, out, proof)
	}
	// Source arithmetic/BV numeral reducers return BR_FAILED, allowing the
	// original traversal to retain values reached through a theory macro.
	numeral, e := m.IsNumeralDeclaration(d)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if numeral {
		return c.evaluatePartialTheoryWithTrace(d, args, out, proof)
	}
	if fid > 4 {
		return rewriter.RewriteFailed, ErrModelEvaluatorTheory
	}
	found, e := c.evaluate(d, args, out)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if found {
		if e = c.traceReduceApp(d, args, out); e != nil {
			return rewriter.RewriteFailed, e
		}
		return rewriter.Rewrite1, nil
	}
	return rewriter.RewriteFailed, nil
}

func (c *evaluatorConfiguration) finishTheoryReduction(status rewriter.RewriteStatus, err error, d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if status == rewriter.RewriteFailed {
		return status, nil
	}
	final, err := c.evaluateReducedApp(status, out)
	if err != nil || status == rewriter.RewriteDone && final == rewriter.Rewrite1 {
		return final, err
	}
	if err = c.traceReduceStatus(final, out); err != nil {
		return rewriter.RewriteFailed, err
	}
	return final, nil
}

func (c *evaluatorConfiguration) fpaRewriter() (*rewriter.FPARewriter, error) {
	if c.fpa != nil {
		return c.fpa, nil
	}
	rw, err := rewriter.NewFPARewriter(c.owner.model.manager)
	if err != nil {
		return nil, err
	}
	c.fpa = rw
	return rw, nil
}

// Source: src/model/model_evaluator.cpp:194-202.
func (c *evaluatorConfiguration) evaluateReducedApp(status rewriter.RewriteStatus, out ast.ASTRef) (rewriter.RewriteStatus, error) {
	if status != rewriter.RewriteDone {
		return status, nil
	}
	m := c.owner.model.manager
	value, e := m.RefGet(out)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	kind, e := m.GetKind(value)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if kind != ast.ASTApp {
		return status, nil
	}
	d, e := m.GetDecl(value)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	args, e := m.AppArgumentView(value)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	found, e := c.evaluate(d, args, out)
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if found {
		return rewriter.Rewrite1, nil
	}
	return status, nil
}

// Source: src/model/model_evaluator.cpp:254-276.
func (c *evaluatorConfiguration) evaluatePartialTheoryWithTrace(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	status, err := c.evaluatePartialTheory(d, args, out, proof)
	if err != nil {
		return status, err
	}
	if err = c.traceReduceStatus(status, out); err != nil {
		return rewriter.RewriteFailed, err
	}
	return status, nil
}

// Source: src/model/model_evaluator.cpp:254-276.
func (c *evaluatorConfiguration) evaluatePartialTheory(d ast.ASTHandle, args ast.AppArguments, out, proof ast.ASTRef) (status rewriter.RewriteStatus, err error) {
	m := c.owner.model.manager
	if err = m.ResetRef(out); err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = m.ResetRef(proof); err != nil {
		return rewriter.RewriteFailed, err
	}
	fi, e := c.owner.model.FunctionInterpretation(d)
	if e != nil || fi == nil {
		return rewriter.RewriteFailed, e
	}
	partial, e := fi.IsPartial()
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if partial {
		sort, e := m.GetRange(d)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		value, e := m.GetSomeValue(sort)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if e = fi.SetElse(value); e != nil {
			return rewriter.RewriteFailed, e
		}
	}
	c.substitution, e = rewriter.NewVarSubstitution(m, false)
	defer func() {
		if c.substitution != nil {
			e := c.substitution.Close()
			err = errors.Join(err, e)
			if e == nil {
				c.substitution = nil
			}
		}
	}()
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	term, e := fi.Interpretation()
	if e != nil {
		return rewriter.RewriteFailed, e
	}
	if e = c.substitution.SubstituteArguments(term, args, out); e != nil {
		return rewriter.RewriteFailed, e
	}
	return rewriter.RewriteFull, nil
}
func (c *evaluatorConfiguration) GetMacro(d ast.ASTHandle) (ast.ASTHandle, ast.ASTHandle, ast.ASTHandle, bool, error) {
	md := c.owner.model
	m := md.manager
	fi, e := md.FunctionInterpretation(d)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if fi != nil {
		if e := c.traceGetMacro(d, 228); e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		partial, e := fi.IsPartial()
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if partial {
			if !c.completion {
				return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
			}
			s, e := m.GetRange(d)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			v, e := md.GetSomeValue(s)
			if e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
			if e = fi.SetElse(v); e != nil {
				return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
			}
		}
		def, e := fi.Interpretation()
		return def, ast.ASTHandle{}, ast.ASTHandle{}, e == nil, e
	}
	if !c.completion {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
	}
	fid, e := m.GetFamilyID(d)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if fid != ast.NullFamilyID {
		considered, e := m.IsConsideredUninterpreted(d)
		if e != nil {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
		}
		if !considered {
			return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, nil
		}
	}
	if e := c.traceGetMacro(d, 247); e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	s, e := m.GetRange(d)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	v, e := md.GetSomeValue(s)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	arity, e := m.GetArity(d)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	c.pending, e = NewFunctionInterpretation(m, arity)
	if e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	c.pending.traceEvaluatorFunctionAllocated(c.owner.impTraceTags())
	if e = c.pending.SetElse(v); e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	if e = md.RegisterFunction(d, c.pending); e != nil {
		return ast.ASTHandle{}, ast.ASTHandle{}, ast.ASTHandle{}, false, e
	}
	c.pending = nil
	return v, ast.ASTHandle{}, ast.ASTHandle{}, true, nil
}
