package arith

// Source: src/tactic/arith/normalize_bounds_tactic.cpp/.h.

import (
	"errors"
	"math/big"
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

type NormalizeBoundsTactic struct {
	self     *NormalizeBoundsTactic
	manager  *ast.ASTManager
	context  *uparams.ParamsContext
	globals  *uparams.GlobalParamsService
	services model.ModelEvaluatorConfig
	// Source outer m_params is the constructor snapshot. updt_params only
	// changes the current implementation; cleanup/translate use this snapshot.
	params                           uparams.ParamsRef
	bounds                           *BoundManager
	rewriter                         *rewriter.TheoryRewriter
	retired                          []*NormalizeBoundsTactic
	trace                            *diagnostic.TraceTags
	ownerBytes                       uint64
	refs                             uint32
	integerOnly, paramsReady, closed bool
}

var _ tactic.Tactic = (*NormalizeBoundsTactic)(nil)

func NewNormalizeBoundsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (*NormalizeBoundsTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c || services.Context != c || services.Globals != g || services.Memory == nil || services.Cooperation == nil {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(NormalizeBoundsTactic{}).Size())
	if err := m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &NormalizeBoundsTactic{manager: m, context: c, globals: g, services: services, ownerBytes: bytes}
	t.self = t
	var err error
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.bounds, err = NewBoundManager(m, c, g)
	}
	if err == nil {
		t.rewriter, err = rewriter.NewTheoryRewriter(m, c, g, &t.params)
	}
	if err == nil {
		err = t.updatePolicy(&t.params)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeNormalizeBoundsTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef, services model.ModelEvaluatorConfig) (tactic.Tactic, error) {
	child, err := NewNormalizeBoundsTactic(m, c, g, p, services)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *NormalizeBoundsTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed || t.bounds == nil || t.rewriter == nil {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *NormalizeBoundsTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *NormalizeBoundsTactic) DecRef() error {
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
func (t *NormalizeBoundsTactic) Close() error {
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
	if t.bounds != nil {
		if err := t.bounds.Close(); err != nil {
			return err
		}
		t.bounds = nil
	}
	if err := t.drainRetired(); err != nil {
		return err
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
func (t *NormalizeBoundsTactic) Cleanup() (err error) {
	if err = t.open(); err != nil {
		return err
	}
	if err = t.drainRetired(); err != nil {
		return err
	}
	fresh, err := NewNormalizeBoundsTactic(t.manager, t.context, t.globals, &t.params, t.services)
	if err != nil {
		return err
	}
	// Source publishes the fresh implementation before destroying the old.
	t.rewriter, fresh.rewriter = fresh.rewriter, t.rewriter
	t.bounds, fresh.bounds = fresh.bounds, t.bounds
	t.integerOnly, fresh.integerOnly = fresh.integerOnly, t.integerOnly
	t.bounds.trace = t.trace
	if err = fresh.Close(); err != nil {
		t.retired = append(t.retired, fresh)
	}
	return err
}
func (t *NormalizeBoundsTactic) drainRetired() error {
	for len(t.retired) > 0 {
		if err := t.retired[0].Close(); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *NormalizeBoundsTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewNormalizeBoundsTactic(m, t.context, t.globals, &t.params, t.services)
	if err == nil {
		other.trace = t.trace
		other.bounds.trace = t.trace
	}
	return other, err
}
func (t *NormalizeBoundsTactic) updatePolicy(p *uparams.ParamsRef) error {
	if err := t.rewriter.UpdateParams(p); err != nil {
		return err
	}
	key := "norm_int_only"
	var err error
	t.integerOnly, err = t.context.GetBoolName(p, &key, true)
	return err
}
func (t *NormalizeBoundsTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	return t.updatePolicy(p)
}
func (t *NormalizeBoundsTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := uparams.InsertProduceModels(out); err != nil {
		return err
	}
	description := "(default: true) normalize only the bounds of integer constants."
	return out.InsertName("norm_int_only", uparams.ParamBool, &description, nil, nil)
}
func (t *NormalizeBoundsTactic) CollectStatistics() map[string]float64  { return nil }
func (t *NormalizeBoundsTactic) ResetStatistics() error                 { return t.open() }
func (t *NormalizeBoundsTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *NormalizeBoundsTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *NormalizeBoundsTactic) Reset() error                           { return t.Cleanup() }
func (t *NormalizeBoundsTactic) target(variable ast.ASTHandle) (value big.Rat, found bool, err error) {
	constant, err := t.bounds.constant(variable)
	if err != nil || !constant {
		return value, false, err
	}
	if t.integerOnly {
		sort, e := t.manager.GetSort(variable)
		if e != nil {
			return value, false, e
		}
		family, e := t.manager.GetFamilyID(sort)
		if e != nil {
			return value, false, e
		}
		kind, e := t.manager.GetDeclKind(sort)
		if e != nil {
			return value, false, e
		}
		if family != t.bounds.family || kind != 1 {
			return value, false, nil
		}
	}
	value, _, found, err = t.bounds.Lower(variable)
	return value, found && value.Sign() != 0, err
}
func (t *NormalizeBoundsTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
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
	if err = result.Prepare(t.manager); err != nil {
		return err
	}
	report, err := tactic.NewTacticReport("normalize-bounds", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = t.bounds.CollectGoal(goal); err != nil {
		return err
	}
	found := false
	for _, variable := range t.bounds.variables {
		if err = t.traceLower(variable); err != nil {
			return err
		}
		_, ok, e := t.target(variable)
		if e != nil {
			return e
		}
		if ok {
			found = true
			break
		}
	}
	if !found {
		return result.AddGoal(goal)
	}
	if err = t.normalize(goal, result); err != nil {
		return err
	}
	if err = t.traceNormalized(goal); err != nil {
		return err
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	return result.AddGoal(goal)
}
func (t *NormalizeBoundsTactic) normalize(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
	m := t.manager
	models, err := goal.ModelsEnabled()
	if err != nil {
		return err
	}
	proofs, err := goal.ProofsEnabled()
	if err != nil {
		return err
	}
	var extension *tactic.ExtensionModelConverter
	var filter *tactic.FilterModelConverter
	if models {
		extension, err = tactic.NewExtensionModel(m, t.services)
		if err != nil {
			return err
		}
		if err = extension.IncRef(); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, extension.DecRef()) }()
		f, e := tactic.NewFilterModel(m)
		if e != nil {
			return e
		}
		if err = f.IncRef(); err != nil {
			return err
		}
		defer func() { err = errors.Join(err, f.DecRef()) }()
		filter = f
		if err = f.BindTrace(t.trace, t.globals); err != nil {
			return err
		}
		converter, e := tactic.ConcatModels(m.Budget(), f, extension)
		if e != nil {
			return e
		}
		if err = result.SetModelConverter(converter); err != nil {
			return errors.Join(err, converter.Close())
		}
	}
	sub, err := m.NewExprSubstitution()
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, m.DelExprSubstitution(&sub)) }()
	var count uint32
	for _, variable := range t.bounds.variables {
		value, found, e := t.target(variable)
		if e != nil {
			return e
		}
		if !found {
			continue
		}
		count++
		sort, e := m.GetSort(variable)
		if e != nil {
			return e
		}
		kind, e := m.GetDeclKind(sort)
		if e != nil {
			return e
		}
		fresh, e := m.MkFreshConstNullable(nil, sort)
		if e != nil {
			return e
		}
		// Protect fresh/numeral temporaries until the substitution takes over.
		ref, e := m.NewRef(fresh)
		if e != nil {
			return e
		}
		definition, e := func() (ast.ASTHandle, error) {
			number, e := m.ArithmeticNumeralRat(&value, kind == 1)
			if e != nil {
				return ast.ASTHandle{}, e
			}
			return m.MkFamilyApp(t.bounds.family, 6, nil, []ast.ASTHandle{fresh, number})
		}()
		if e == nil {
			e = m.ExprSubstitutionInsert(sub, variable, definition, ast.ASTHandle{}, ast.ASTDependency{})
		}
		if e == nil && models {
			decl, x := m.GetDecl(variable)
			e = x
			if e == nil {
				e = extension.Insert(decl, definition)
			}
			if e == nil {
				decl, e = m.GetDecl(fresh)
				if e == nil {
					e = filter.Insert(decl)
				}
			}
		}
		e = errors.Join(e, m.DelRef(&ref))
		if e != nil {
			return e
		}
	}
	if err = tactic.ReportTacticProgress(":normalized-bounds", count); err != nil {
		return err
	}
	if err = t.rewriter.SetSubstitution(&sub); err != nil {
		return err
	}
	defer func() { err = errors.Join(err, t.rewriter.SetSubstitution(nil)) }()
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
	return nil
}
