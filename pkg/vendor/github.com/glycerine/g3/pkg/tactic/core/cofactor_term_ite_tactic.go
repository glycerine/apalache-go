package core

// Source: src/tactic/core/cofactor_term_ite_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	umemory "github.com/glycerine/g3/pkg/util/memory"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type CofactorTermITETactic struct {
	self               *CofactorTermITETactic
	manager            *ast.ASTManager
	context            *uparams.ParamsContext
	globals            *uparams.GlobalParamsService
	services           model.ModelEvaluatorConfig
	params             uparams.ParamsRef
	theory             *rewriter.TheoryRewriter
	maxMemory          uint64
	cofactorEqualities bool
	refs               uint32
	header             uint64
	paramsReady        bool
	closed             bool
}

var _ tactic.Tactic = (*CofactorTermITETactic)(nil)

func NewCofactorTermITETactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (_ *CofactorTermITETactic, resultErr error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(CofactorTermITETactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &CofactorTermITETactic{manager: m, context: c, globals: g, services: services, header: n}
	t.self = t
	defer func() {
		if resultErr != nil {
			resultErr = errors.Join(resultErr, t.Close())
		}
	}()
	var err error
	t.params, err = c.NewValue()
	if err != nil {
		return nil, err
	}
	t.paramsReady = true
	if p != nil {
		if err = c.Assign(&t.params, p); err != nil {
			return nil, err
		}
	}
	if err = t.readPolicy(&t.params); err != nil {
		return nil, err
	}
	t.theory, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func MakeCofactorTermITETactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewCofactorTermITETactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	clean, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return clean, nil
}

func (t *CofactorTermITETactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || !t.paramsReady || t.theory == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}

func (t *CofactorTermITETactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}

func (t *CofactorTermITETactic) DecRef() error {
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

func (t *CofactorTermITETactic) readPolicy(p *uparams.ParamsRef) error {
	name := "max_memory"
	megabytes, err := t.context.GetUIntName(p, &name, ^uint32(0))
	if err != nil {
		return err
	}
	t.maxMemory = umemory.MegabytesToBytes(megabytes)
	name = "cofactor_equalities"
	t.cofactorEqualities, err = t.context.GetBoolName(p, &name, true)
	return err
}

func (t *CofactorTermITETactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if p == nil {
		empty := t.context.EmptyValue()
		p = &empty
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	if err := t.readPolicy(&t.params); err != nil {
		return err
	}
	return t.theory.UpdateParams(&t.params)
}

func (t *CofactorTermITETactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	text := "(default: true) use equalities to rewrite bodies of ite-expressions. This is potentially expensive."
	return out.InsertName("cofactor_equalities", uparams.ParamBool, &text, nil, nil)
}

func (t *CofactorTermITETactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	return NewCofactorTermITETactic(m, t.context, t.globals, &t.params, t.services)
}

func (t *CofactorTermITETactic) Cleanup() (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if err = t.theory.Close(); err != nil {
		return err
	}
	t.theory, err = rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	return err
}

func (t *CofactorTermITETactic) Reset() error                           { return t.Cleanup() }
func (t *CofactorTermITETactic) CollectStatistics() map[string]float64  { return nil }
func (t *CofactorTermITETactic) ResetStatistics() error                 { return t.open() }
func (t *CofactorTermITETactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *CofactorTermITETactic) SetProgressCallback(func(uint32)) error { return t.open() }

func (t *CofactorTermITETactic) Apply(goal *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if goal == nil || out == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if err := tactic.FailIfProofGeneration("cofactor-term-ite", goal); err != nil {
		return err
	}
	if err := tactic.FailIfUnsatCoreGeneration("cofactor-term-ite", goal); err != nil {
		return err
	}
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("cofactor-term-ite", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { resultErr = errors.Join(resultErr, report.Close()) }()
	}
	r := newCofactorTermITERewriter(t)
	defer func() { resultErr = errors.Join(resultErr, r.Close()) }()
	size, err := goal.Size()
	if err != nil {
		return err
	}
	for i := uint32(0); i < size; i++ {
		inconsistent, e := goal.Inconsistent()
		if e != nil {
			return e
		}
		if inconsistent {
			break
		}
		formula, e := goal.Form(i)
		if e != nil {
			return e
		}
		formula, e = r.rewrite(formula)
		if e != nil {
			return e
		}
		dependency, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if e = goal.Update(i, formula, ast.ASTHandle{}, dependency); e != nil {
			return e
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return out.AddGoal(goal)
}

func (t *CofactorTermITETactic) Close() (result error) {
	if t == nil || t.self != t || t.closed {
		return nil
	}
	if t.theory != nil {
		result = errors.Join(result, t.theory.Close())
		t.theory = nil
	}
	if t.paramsReady {
		result = errors.Join(result, t.context.Del(&t.params))
		t.paramsReady = false
	}
	if result != nil {
		return result
	}
	t.manager.Budget().Release(t.header)
	t.header = 0
	t.closed = true
	return nil
}
