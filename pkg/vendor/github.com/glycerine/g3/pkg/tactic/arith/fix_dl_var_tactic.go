package arith

// Source: src/tactic/arith/fix_dl_var_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/ast/rewriter"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type FixDLVarTactic struct {
	self                *FixDLVarTactic
	manager             *ast.ASTManager
	context             *uparams.ParamsContext
	globals             *uparams.GlobalParamsService
	services            model.ModelEvaluatorConfig
	params              uparams.ParamsRef
	family              ast.FamilyID
	rewriter            *rewriter.TheoryRewriter
	trace               *diagnostic.TraceTags
	ownerBytes          uint64
	refs                uint32
	paramsReady, closed bool
}

var _ tactic.Tactic = (*FixDLVarTactic)(nil)

func NewFixDLVarTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*FixDLVarTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(FixDLVarTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &FixDLVarTactic{manager: m, context: c, globals: g, services: services, ownerBytes: bytes}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.family, err = m.RegisterArithmeticPlugin()
	}
	if err == nil {
		t.rewriter, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeFixDLVarTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewFixDLVarTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *FixDLVarTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.rewriter == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *FixDLVarTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *FixDLVarTactic) DecRef() error {
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
func (t *FixDLVarTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if t.rewriter != nil {
		if err := t.rewriter.Close(); err != nil {
			return err
		}
		t.rewriter = nil
	}
	if t.paramsReady {
		if err := t.context.Del(&t.params); err != nil {
			return err
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.ownerBytes)
	t.ownerBytes = 0
	t.closed = true
	return nil
}
func (t *FixDLVarTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	fresh, err := rewriter.NewTheoryRewriter(t.manager, t.context, t.globals, &t.params)
	if err != nil {
		if fresh != nil {
			err = errors.Join(err, fresh.Close())
		}
		return err
	}
	if err = t.rewriter.Close(); err != nil {
		return errors.Join(err, fresh.Close())
	}
	t.rewriter = fresh
	return nil
}
func (t *FixDLVarTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	translated, err := NewFixDLVarTactic(m, t.context, t.globals, &t.params, t.services)
	if err == nil {
		translated.trace = t.trace
	}
	return translated, err
}
func (t *FixDLVarTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	return t.rewriter.UpdateParams(p)
}
func (t *FixDLVarTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	return rewriter.TheoryRewriterParameterDescriptions(out)
}
func (t *FixDLVarTactic) CollectStatistics() map[string]float64  { return nil }
func (t *FixDLVarTactic) ResetStatistics() error                 { return t.open() }
func (t *FixDLVarTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *FixDLVarTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *FixDLVarTactic) Reset() error                           { return t.Cleanup() }

func (t *FixDLVarTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	defer func() {
		if rewriter.IsRewriterException(err) {
			err = tactic.NewTacticException(err.Error())
		}
	}()
	if err = t.open(); err != nil {
		return err
	}
	if goal == nil || result == nil || goal.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if ast.ASTSourceDebug {
		if ok, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("fix-dl-var", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	variable, err := t.selectDLVariable(goal)
	if err != nil {
		return err
	}
	if !variable.IsNull() {
		if err = t.fixDLVariable(goal, result, variable); err != nil {
			return err
		}
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if err = t.traceDLGoal(goal); err != nil {
		return err
	}
	if ast.ASTSourceDebug {
		if ok, e := goal.IsWellSorted(); e != nil {
			return e
		} else if !ok {
			return ast.ErrASTDomain
		}
	}
	return nil
}
func (t *FixDLVarTactic) fixDLVariable(goal *tactic.Goal, result *tactic.TacticResult, variable ast.ASTHandle) (err error) {
	m := t.manager
	if err = t.verboseDLChoice(variable); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("fix-dl-var", goal)
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
	sort, err := m.GetSort(variable)
	if err != nil {
		return err
	}
	sortKind, err := m.GetDeclKind(sort)
	if err != nil {
		return err
	}
	var zero ast.ASTHandle
	if sortKind == 1 {
		zero, err = m.ArithmeticIntegerNumeralText("0")
	} else {
		zero, err = m.ArithmeticRealNumeralText("0")
	}
	if err != nil {
		return err
	}
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	if err = m.ExprSubstitutionInsert(sub, variable, zero, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
		return err
	}
	if err = t.rewriter.SetSubstitution(&sub); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.rewriter.SetSubstitution(nil)) }()
	if models {
		converter, e := tactic.NewExtensionModel(m, t.services)
		if e != nil {
			return e
		}
		if e = converter.IncRef(); e != nil {
			return e
		}
		defer func() { err = errors.Join(err, converter.DecRef()) }()
		decl, e := m.GetDecl(variable)
		if e != nil {
			return e
		}
		if err = converter.Insert(decl, zero); err != nil {
			return err
		}
		if err = result.SetModelConverter(converter); err != nil {
			return err
		}
	}
	out, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&out)) }()
	proof, err := m.NewRef(ast.ASTHandle{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelRef(&proof)) }()
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
		term, e := goal.Form(i)
		if e != nil {
			return e
		}
		if proofs {
			err = t.rewriter.RewriteWithProof(term, out, proof)
		} else {
			err = t.rewriter.Rewrite(term, out)
		}
		if err != nil {
			return err
		}
		value, e := m.RefGet(out)
		if e != nil {
			return e
		}
		pr, e := m.RefGet(proof)
		if e != nil {
			return e
		}
		if proofs {
			old, e := goal.Proof(i)
			if e != nil {
				return e
			}
			pr, e = m.MkProofModusPonens(old, pr)
			if e != nil {
				return e
			}
			if err = m.SetRef(proof, pr); err != nil {
				return err
			}
		}
		dep, e := goal.Dependency(i)
		if e != nil {
			return e
		}
		if err = goal.Update(i, value, pr, dep); err != nil {
			return err
		}
	}
	return goal.IncDepth()
}
