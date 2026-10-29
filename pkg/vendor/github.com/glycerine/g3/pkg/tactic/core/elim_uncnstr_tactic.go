package core

// Source: src/tactic/core/elim_uncnstr_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type EliminateUnconstrainedTactic struct {
	self                                      *EliminateUnconstrainedTactic
	manager                                   *ast.ASTManager
	context                                   *uparams.ParamsContext
	globals                                   *uparams.GlobalParamsService
	services                                  model.ModelEvaluatorConfig
	params                                    uparams.ParamsRef
	arithmetic, bitVector, array, datatype    ast.FamilyID
	variables                                 map[ast.ASTHandle]bool
	orderedVariables                          []ast.ASTHandle
	trace                                     *diagnostic.TraceTags
	configuration                             *elimUnconstrainedRewriteConfig
	rewriter                                  *rewriter.Traversal
	converter                                 *tactic.ExtensionModelConverter
	maxMemory, ownerBytes, configurationBytes uint64
	maxSteps, eliminated, refs                uint32
	closed                                    bool
}

var _ tactic.Tactic = (*EliminateUnconstrainedTactic)(nil)

func NewEliminateUnconstrainedTactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*EliminateUnconstrainedTactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	if err := m.RegisterDeclarationPlugins(); err != nil {
		return nil, err
	}
	params, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(EliminateUnconstrainedTactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&params))
	}
	t := &EliminateUnconstrainedTactic{manager: m, context: context, globals: globals, services: services, params: params, ownerBytes: amount, variables: make(map[ast.ASTHandle]bool)}
	t.self = t
	if p != nil {
		err = context.Assign(&t.params, p)
	}
	if err == nil {
		err = t.readPolicy(&t.params)
	}
	if err == nil {
		t.arithmetic, err = m.RegisterArithmeticPlugin()
	}
	if err == nil {
		t.bitVector, err = m.RegisterBitVectorPlugin()
	}
	if err == nil {
		t.array, err = m.RegisterArrayPlugin()
	}
	if err == nil {
		t.datatype, err = m.RegisterDatatypePlugin()
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeEliminateUnconstrainedTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewEliminateUnconstrainedTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *EliminateUnconstrainedTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *EliminateUnconstrainedTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *EliminateUnconstrainedTactic) DecRef() error {
	if err := t.open(); err != nil {
		return err
	}
	if t.refs == 0 {
		return arena.ErrBounds
	}
	t.refs--
	if t.refs == 0 {
		return t.Close()
	}
	return nil
}
func (t *EliminateUnconstrainedTactic) readPolicy(p *uparams.ParamsRef) error {
	key := "max_memory"
	mb, err := t.context.GetUIntName(p, &key, ^uint32(0))
	if err != nil {
		return err
	}
	t.maxMemory = umemory.MegabytesToBytes(mb)
	key = "max_steps"
	t.maxSteps, err = t.context.GetUIntName(p, &key, ^uint32(0))
	return err
}
func (t *EliminateUnconstrainedTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(p)
}
func (t *EliminateUnconstrainedTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	return uparams.InsertMaxSteps(out)
}
func (t *EliminateUnconstrainedTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.self != t || t.closed || t.eliminated == 0 {
		return nil
	}
	return map[string]float64{"eliminated applications": float64(t.eliminated)}
}
func (t *EliminateUnconstrainedTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.eliminated = 0
	return nil
}
func (t *EliminateUnconstrainedTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *EliminateUnconstrainedTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *EliminateUnconstrainedTactic) Reset() error                           { return t.Cleanup() }
func (t *EliminateUnconstrainedTactic) releaseImplementation() error {
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
	}
	if t.configuration != nil {
		if err := t.configuration.owned.Close(); err != nil {
			return err
		}
		t.configuration = nil
		t.manager.Budget().Release(t.configurationBytes)
		t.configurationBytes = 0
	}
	if t.converter != nil {
		if err := t.converter.DecRef(); err != nil {
			return err
		}
		t.converter = nil
	}
	clear(t.variables)
	t.orderedVariables = nil
	return nil
}
func (t *EliminateUnconstrainedTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.releaseImplementation(); err != nil {
		return err
	}
	return t.readPolicy(&t.params)
}
func (t *EliminateUnconstrainedTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	translated, err := NewEliminateUnconstrainedTactic(m, t.context, t.globals, &t.params, t.services)
	if err != nil {
		return nil, err
	}
	translated.trace = t.trace
	return translated, nil
}
func (t *EliminateUnconstrainedTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.releaseImplementation(); err != nil {
		return err
	}
	if err := t.context.Del(&t.params); err != nil {
		return err
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *EliminateUnconstrainedTactic) collect(goal *tactic.Goal) error {
	variables, err := collectSingleOccurrenceVariables(goal)
	if err != nil {
		return err
	}
	clear(t.variables)
	t.orderedVariables = variables
	for _, v := range variables {
		t.variables[v] = true
	}
	return nil
}

func (t *EliminateUnconstrainedTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	if err = t.run(goal, result); err != nil {
		return err
	}
	return tactic.ReportTacticProgress(":num-elim-apps", t.eliminated)
}
func (t *EliminateUnconstrainedTactic) run(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	// A previous interrupted invocation may still own traversal frames. Retire
	// those owners before replacing the source scoped rewriter/converter.
	if err = t.releaseImplementation(); err != nil {
		return err
	}
	if err = t.traceGoal(goal); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("elim-uncnstr-vars", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.collect(goal); err != nil {
		return err
	}
	if len(t.variables) == 0 {
		return result.AddGoal(goal)
	}
	if err = t.traceVariables(); err != nil {
		return err
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	if models {
		t.converter, err = tactic.NewExtensionModel(t.manager, t.services)
		if err != nil {
			return err
		}
		if err = t.converter.IncRef(); err != nil {
			return err
		}
	}
	amount := uint64(reflect.TypeOf(elimUnconstrainedRewriteConfig{}).Size())
	if err = t.manager.Budget().Reserve(amount); err != nil {
		return err
	}
	t.configurationBytes = amount
	t.configuration = &elimUnconstrainedRewriteConfig{owner: t, proofs: proofs, converter: t.converter, cache: make(map[ast.ASTHandle]ast.ASTHandle), owned: solveEquationFrame{manager: t.manager}}
	if proofs {
		t.rewriter, err = rewriter.NewProofTraversal(t.manager, t.configuration)
	} else {
		t.rewriter, err = rewriter.NewTraversal(t.manager, t.configuration)
	}
	if err != nil {
		return err
	}
	out, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&out)) }()
	proof, err := t.manager.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.manager.DelRef(&proof)) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	modified := true
	var index uint32
	for {
		for ; index < size; index++ {
			f, e := goal.Form(index)
			if e != nil {
				return e
			}
			if proofs {
				err = t.rewriter.RewriteWithProof(f, out, proof)
			} else {
				err = t.rewriter.Rewrite(f, out)
			}
			if err != nil {
				return err
			}
			value, e := t.manager.RefGet(out)
			if e != nil {
				return e
			}
			if f == value {
				continue
			}
			modified = true
			pr, e := t.manager.RefGet(proof)
			if e != nil {
				return e
			}
			if proofs {
				old, e := goal.Proof(index)
				if e != nil {
					return e
				}
				pr, e = t.manager.MkProofModusPonens(old, pr)
				if e != nil {
					return e
				}
				if e = t.manager.SetRef(proof, pr); e != nil {
					return e
				}
			}
			dep, e := goal.Dependency(index)
			if e != nil {
				return e
			}
			if e = goal.Update(index, value, pr, dep); e != nil {
				return e
			}
		}
		if !modified {
			t.eliminated = uint32(len(t.configuration.fresh))
			if models && t.eliminated != 0 {
				if err = t.publishConverter(result); err != nil {
					return err
				}
			}
			if err = t.releaseImplementation(); err != nil {
				return err
			}
			if err = t.traceConverter(result.ModelConverter); err != nil {
				return err
			}
			if err = result.AddGoal(goal); err != nil {
				return err
			}
			return goal.IncDepth()
		}
		modified = false
		size, err = goal.Size()
		if err != nil {
			return err
		}
		if err = t.rewriter.Reset(); err != nil {
			return err
		}
		if err = t.collect(goal); err != nil {
			return err
		}
		if len(t.variables) == 0 {
			index = size
		} else {
			index = 0
		}
	}
}
func (t *EliminateUnconstrainedTactic) publishConverter(result *tactic.TacticResult) (err error) {
	filter, err := tactic.NewFilterModel(t.manager)
	if err != nil {
		return err
	}
	if err = filter.IncRef(); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, filter.DecRef()) }()
	if err = filter.BindTrace(t.trace, t.globals); err != nil {
		return err
	}
	for _, fresh := range t.configuration.fresh {
		d, e := t.manager.GetDecl(fresh)
		if e != nil {
			return e
		}
		if err = filter.Insert(d); err != nil {
			return err
		}
	}
	combined, err := tactic.ConcatModels(t.manager.Budget(), filter, t.converter)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(combined); err != nil {
		return errors.Join(err, combined.Close())
	}
	return nil
}
