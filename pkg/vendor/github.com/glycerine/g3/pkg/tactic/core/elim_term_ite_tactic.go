package core

// Source: src/tactic/core/elim_term_ite_tactic.cpp.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/normal_forms"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type eliminateTermITEConfig struct {
	rewriter.DefaultConfiguration
	owner         *EliminateTermITETactic
	definedNames  *normal_forms.DefinedNames
	goal          *tactic.Goal
	filter        *tactic.FilterModelConverter
	produceModels bool
}

func (c *eliminateTermITEConfig) MaxStepsExceeded(uint32) (bool, error) {
	if err := c.owner.services.Cooperation.Cooperate("elim term ite"); err != nil {
		return false, err
	}
	n, err := c.owner.services.Memory.AllocationSize()
	if err != nil {
		return false, err
	}
	if n > c.owner.maxMemory {
		return false, tactic.NewTacticException(tactic.TacticMaxMemoryMessage)
	}
	return false, nil
}

func (c *eliminateTermITEConfig) ReduceApp(decl ast.ASTHandle, arguments ast.AppArguments, out, proof ast.ASTRef) (rewriter.RewriteStatus, error) {
	m := c.owner.manager
	family, err := m.GetFamilyID(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	op, err := m.GetDeclKind(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if family != m.BasicFamilyID() || op != ast.ASTOpIte {
		return rewriter.RewriteFailed, nil
	}
	rangeSort, err := m.GetRange(decl)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	boolSort, err := m.MkBoolSort()
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if rangeSort == boolSort {
		return rewriter.RewriteFailed, nil
	}
	args := make([]ast.ASTHandle, arguments.Count())
	for i := range args {
		args[i], err = arguments.At(uint32(i))
		if err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	term, err := m.MkApp(decl, args)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	termRef, err := m.NewRef(term)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	defer func() { _ = m.DelRef(&termRef) }()
	definition, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	defer func() { _ = m.DelRef(&definition) }()
	definitionProof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	defer func() { _ = m.DelRef(&definitionProof) }()
	name, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	defer func() { _ = m.DelRef(&name) }()
	fresh, err := c.definedNames.MkName(term, definition, definitionProof, name, proof)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	nameHandle, err := m.RefGet(name)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = m.SetRef(out, nameHandle); err != nil {
		return rewriter.RewriteFailed, err
	}
	if !fresh {
		return rewriter.RewriteDone, nil
	}
	def, err := m.RefGet(definition)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	defProof, err := m.RefGet(definitionProof)
	if err != nil {
		return rewriter.RewriteFailed, err
	}
	if err = c.goal.AssertWithProof(def, defProof, ast.ASTDependency{}); err != nil {
		return rewriter.RewriteFailed, err
	}
	c.owner.numFresh++
	if c.produceModels {
		if c.filter == nil {
			c.filter, err = tactic.NewFilterModel(m)
			if err != nil {
				return rewriter.RewriteFailed, err
			}
			if err = c.filter.IncRef(); err != nil {
				return rewriter.RewriteFailed, errors.Join(err, c.filter.Close())
			}
		}
		freshDecl, e := m.GetDecl(nameHandle)
		if e != nil {
			return rewriter.RewriteFailed, e
		}
		if err = c.filter.Insert(freshDecl); err != nil {
			return rewriter.RewriteFailed, err
		}
	}
	return rewriter.RewriteDone, nil
}

type EliminateTermITETactic struct {
	self       *EliminateTermITETactic
	manager    *ast.ASTManager
	context    *uparams.ParamsContext
	globals    *uparams.GlobalParamsService
	services   model.ModelEvaluatorConfig
	params     uparams.ParamsRef
	config     *eliminateTermITEConfig
	traversal  *rewriter.Traversal
	maxMemory  uint64
	numFresh   uint32
	refs       uint32
	ownerBytes uint64
	closed     bool
}

var _ tactic.Tactic = (*EliminateTermITETactic)(nil)

func NewEliminateTermITETactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*EliminateTermITETactic, error) {
	if m == nil || context == nil || globals == nil || globals.Context != context || services.Context != context || services.Globals != globals || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := context.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	params, err := context.NewValue()
	if err != nil {
		return nil, err
	}
	amount := uint64(reflect.TypeOf(EliminateTermITETactic{}).Size())
	if err = m.Budget().Reserve(amount); err != nil {
		return nil, errors.Join(err, context.Del(&params))
	}
	t := &EliminateTermITETactic{manager: m, context: context, globals: globals, services: services, params: params, ownerBytes: amount}
	t.self = t
	if p != nil {
		err = context.Assign(&t.params, p)
	}
	if err == nil {
		err = t.readPolicy(&t.params)
	}
	if err == nil {
		err = t.initialize()
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}

func MakeEliminateTermITETactic(m *ast.ASTManager, context *uparams.ParamsContext, globals *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewEliminateTermITETactic(m, context, globals, p, services)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return clean, nil
}

func (t *EliminateTermITETactic) initialize() error {
	names, err := normal_forms.NewDefinedNamesWithPrefix(t.manager, nil)
	if err != nil {
		return err
	}
	t.config = &eliminateTermITEConfig{owner: t, definedNames: names}
	return nil
}

func (t *EliminateTermITETactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.config == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *EliminateTermITETactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *EliminateTermITETactic) DecRef() error {
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

func (t *EliminateTermITETactic) readPolicy(p *uparams.ParamsRef) error {
	name := "max_memory"
	megabytes, err := t.context.GetUIntName(p, &name, ^uint32(0))
	t.maxMemory = umemory.MegabytesToBytes(megabytes)
	return err
}

func (t *EliminateTermITETactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.readPolicy(p)
}

func (t *EliminateTermITETactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertMaxMemory(out); err != nil {
		return err
	}
	if err := uparams.InsertMaxSteps(out); err != nil {
		return err
	}
	description := "(default: 128) maximum number of arguments (per application) that will be considered by the greedy (quadratic) heuristic."
	return out.InsertName("max_args", uparams.ParamUInt, &description, nil, nil)
}

func (t *EliminateTermITETactic) CollectStatistics() map[string]float64 {
	if t == nil || t.self != t || t.closed || t.numFresh == 0 {
		return nil
	}
	return map[string]float64{"elim-term-ite-consts": float64(t.numFresh)}
}

func (t *EliminateTermITETactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.numFresh = 0
	return nil
}

func (t *EliminateTermITETactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *EliminateTermITETactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *EliminateTermITETactic) Reset() error                           { return t.Cleanup() }

func (t *EliminateTermITETactic) releaseInvocation() (err error) {
	if t.traversal != nil {
		err = errors.Join(err, t.traversal.Close())
		t.traversal = nil
	}
	if t.config != nil {
		if t.config.filter != nil {
			err = errors.Join(err, t.config.filter.DecRef())
			t.config.filter = nil
		}
		t.config.goal = nil
		t.config.produceModels = false
		err = errors.Join(err, t.config.definedNames.Reset())
	}
	return err
}

func (t *EliminateTermITETactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	return t.releaseInvocation()
}

func (t *EliminateTermITETactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewEliminateTermITETactic(m, t.context, t.globals, &t.params, t.services)
}

func (t *EliminateTermITETactic) Close() (err error) {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	err = errors.Join(err, t.releaseInvocation())
	if t.config != nil {
		err = errors.Join(err, t.config.definedNames.Close())
		t.config = nil
	}
	err = errors.Join(err, t.context.Del(&t.params))
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return err
}

func (t *EliminateTermITETactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err = t.releaseInvocation(); err != nil {
		return err
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("elim-term-ite", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	t.config.goal = goal
	t.config.produceModels = models
	if proofs {
		t.traversal, err = rewriter.NewProofTraversal(t.manager, t.config)
	} else {
		t.traversal, err = rewriter.NewTraversal(t.manager, t.config)
	}
	if err != nil {
		return err
	}
	t.numFresh = 0
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		current, e := goal.Form(i)
		if e != nil {
			return e
		}
		newCurrent, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			return e
		}
		newProof, e := t.manager.NewRef(ast.ASTHandle{})
		if e != nil {
			_ = t.manager.DelRef(&newCurrent)
			return e
		}
		if proofs {
			e = t.traversal.RewriteWithProof(current, newCurrent, newProof)
		} else {
			e = t.traversal.Rewrite(current, newCurrent)
		}
		if e == nil {
			rewritten, x := t.manager.RefGet(newCurrent)
			if x != nil {
				e = x
			} else {
				proof := ast.ASTHandle{}
				if proofs {
					oldProof, x := goal.Proof(i)
					if x != nil {
						e = x
					} else {
						rewriteProof, x := t.manager.RefGet(newProof)
						if x != nil {
							e = x
						} else {
							proof, e = t.manager.MkProofModusPonens(oldProof, rewriteProof)
						}
					}
				}
				if e == nil {
					dependency, x := goal.Dependency(i)
					if x != nil {
						e = x
					} else {
						e = goal.Update(i, rewritten, proof, dependency)
					}
				}
			}
		}
		e = errors.Join(e, t.manager.DelRef(&newProof), t.manager.DelRef(&newCurrent))
		if e != nil {
			return e
		}
	}
	if t.config.filter != nil {
		if err = result.SetModelConverter(t.config.filter); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	return tactic.ReportTacticProgress(":elim-term-ite-consts", t.numFresh)
}
