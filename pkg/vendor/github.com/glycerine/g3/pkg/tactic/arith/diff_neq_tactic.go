package arith

// Source: src/tactic/arith/diff_neq_tactic.cpp/.h.

import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/model"
	"github.com/glycerine/g3/pkg/tactic"
	"github.com/glycerine/g3/pkg/util/diagnostic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type diffNeqDisequality struct {
	variable uint32
	offset   int32
}
type diffNeqVariable struct {
	term          ast.ASTHandle
	lower, upper  int32
	disequalities []diffNeqDisequality
}
type diffNeqImplementation struct {
	variables      []diffNeqVariable
	indices        map[ast.ASTHandle]uint32
	stack          []int32
	forbidden      []bool
	forbiddenBytes uint64
	maxK, maxNegK  int64
	conflicts      uint32
	released       int
}

type DiffNeqTactic struct {
	self                *DiffNeqTactic
	manager             *ast.ASTManager
	context             *uparams.ParamsContext
	globals             *uparams.GlobalParamsService
	params              uparams.ParamsRef
	family              ast.FamilyID
	current             diffNeqImplementation
	retired             []diffNeqImplementation
	trace               *diagnostic.TraceTags
	ownerBytes          uint64
	refs                uint32
	paramsReady, closed bool
}

var _ tactic.Tactic = (*DiffNeqTactic)(nil)

func NewDiffNeqTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (*DiffNeqTactic, error) {
	if m == nil || c == nil || g == nil || g.Context != c {
		return nil, uparams.ErrParamsOwner
	}
	if err := c.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	family, err := m.RegisterArithmeticPlugin()
	if err != nil {
		return nil, err
	}
	bytes := uint64(reflect.TypeOf(DiffNeqTactic{}).Size())
	if err = m.Budget().Reserve(bytes); err != nil {
		return nil, err
	}
	t := &DiffNeqTactic{manager: m, context: c, globals: g, family: family, ownerBytes: bytes}
	t.self = t
	t.params, err = c.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = c.Assign(&t.params, p)
	}
	if err == nil {
		t.current, err = t.buildDiffNeq()
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func MakeDiffNeqTactic(m *ast.ASTManager, c *uparams.ParamsContext, g *uparams.GlobalParamsService, p *uparams.ParamsRef) (tactic.Tactic, error) {
	child, err := NewDiffNeqTactic(m, c, g, p)
	if err != nil {
		return nil, err
	}
	wrapped, err := tactic.NewCleanTactic(child)
	if err != nil {
		return nil, errors.Join(err, child.Close())
	}
	return wrapped, nil
}
func (t *DiffNeqTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return t.context.Check(&t.params)
}
func (t *DiffNeqTactic) IncRef() error {
	if err := t.open(); err != nil {
		return err
	}
	t.refs++
	return nil
}
func (t *DiffNeqTactic) DecRef() error {
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
func (t *DiffNeqTactic) policy(p *uparams.ParamsRef) (max, negative int64, err error) {
	key := "diff_neq_max_k"
	n, err := t.context.GetUIntName(p, &key, 1024)
	if err != nil {
		return 0, 0, err
	}
	max = int64(n)
	negative = -max
	// Preserve the source order: its negative limit is computed before the
	// positive INT_MAX/2 clamp, including nondefault large parameter values.
	if max >= 1073741823 {
		max = 1073741823
	}
	return max, negative, nil
}
func (t *DiffNeqTactic) buildDiffNeq() (imp diffNeqImplementation, err error) {
	imp.maxK, imp.maxNegK, err = t.policy(&t.params)
	if err == nil {
		imp.indices = make(map[ast.ASTHandle]uint32)
	}
	return imp, err
}
func (t *DiffNeqTactic) releaseDiffNeq(imp *diffNeqImplementation) error {
	for imp.released < len(imp.variables) {
		consumed, err := t.manager.DecRefConsumed(imp.variables[imp.released].term)
		if consumed {
			imp.released++
		}
		if err != nil {
			return err
		}
	}
	imp.variables = nil
	imp.indices = nil
	imp.stack = nil
	imp.forbidden = nil
	t.manager.Budget().Release(imp.forbiddenBytes)
	imp.forbiddenBytes = 0
	imp.released = 0
	return nil
}
func (t *DiffNeqTactic) drainDiffNeq() error {
	for len(t.retired) > 0 {
		if err := t.releaseDiffNeq(&t.retired[0]); err != nil {
			return err
		}
		t.retired = t.retired[1:]
	}
	return nil
}
func (t *DiffNeqTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	if err := t.releaseDiffNeq(&t.current); err != nil {
		return err
	}
	if err := t.drainDiffNeq(); err != nil {
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
func (t *DiffNeqTactic) Cleanup() error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.drainDiffNeq(); err != nil {
		return err
	}
	fresh, err := t.buildDiffNeq()
	if err != nil {
		return err
	}
	fresh.conflicts = t.current.conflicts
	old := t.current
	t.current = fresh
	if err = t.releaseDiffNeq(&old); err != nil {
		t.retired = append(t.retired, old)
	}
	return err
}
func (t *DiffNeqTactic) UpdateParams(p *uparams.ParamsRef) error {
	if err := t.open(); err != nil {
		return err
	}
	if err := t.context.Assign(&t.params, p); err != nil {
		return err
	}
	max, negative, err := t.policy(p)
	if err == nil {
		t.current.maxK, t.current.maxNegK = max, negative
	}
	return err
}
func (t *DiffNeqTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	other, err := NewDiffNeqTactic(m, t.context, t.globals, &t.params)
	if err == nil {
		other.trace = t.trace
	}
	return other, err
}
func (t *DiffNeqTactic) CollectParamDescrs(out *uparams.ParamDescrs) error {
	if err := t.open(); err != nil {
		return err
	}
	description := "(default: 1024) maximum variable upper bound for diff neq solver."
	return out.InsertName("diff_neq_max_k", uparams.ParamUInt, &description, nil, nil)
}
func (t *DiffNeqTactic) CollectStatistics() map[string]float64 {
	if t == nil || t.self != t || t.closed {
		return nil
	}
	return map[string]float64{"conflicts": float64(t.current.conflicts)}
}
func (t *DiffNeqTactic) ResetStatistics() error {
	if err := t.open(); err != nil {
		return err
	}
	t.current.conflicts = 0
	return nil
}
func (t *DiffNeqTactic) SetLogic(usymbol.Symbol) error          { return t.open() }
func (t *DiffNeqTactic) SetProgressCallback(func(uint32)) error { return t.open() }
func (t *DiffNeqTactic) Reset() error                           { return t.Cleanup() }

func (t *DiffNeqTactic) Apply(goal *tactic.Goal, result *tactic.TacticResult) (err error) {
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
	report, err := tactic.NewTacticReport("diff-neq", goal)
	if err != nil {
		return err
	}
	if report != nil {
		defer func() { err = errors.Join(err, report.Close()) }()
	}
	if err = tactic.FailIfProofGeneration("diff-neq", goal); err != nil {
		return err
	}
	if err = tactic.FailIfUnsatCoreGeneration("diff-neq", goal); err != nil {
		return err
	}
	inconsistent, err := goal.Inconsistent()
	if err != nil {
		return err
	}
	if inconsistent {
		return result.AddGoal(goal)
	}
	if err = t.compileDiffNeq(goal); err != nil {
		return err
	}
	if err = t.traceCompiledDiffNeq(goal); err != nil {
		return err
	}
	sat, err := t.searchDiffNeq()
	if err != nil {
		return err
	}
	if err = tactic.ReportTacticProgress(":conflicts", t.current.conflicts); err != nil {
		return err
	}
	if sat {
		models, e := goal.ModelsEnabled()
		if e != nil {
			return e
		}
		if models {
			if err = t.publishDiffNeqModel(result); err != nil {
				return err
			}
		}
		if err = goal.Reset(); err != nil {
			return err
		}
	} else {
		f, e := t.manager.MkFalse()
		if e != nil {
			return e
		}
		if err = goal.Assert(f); err != nil {
			return err
		}
	}
	if err = goal.IncDepth(); err != nil {
		return err
	}
	if err = result.AddGoal(goal); err != nil {
		return err
	}
	if err = t.traceDiffNeqResult(goal); err != nil {
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
func (t *DiffNeqTactic) publishDiffNeqModel(result *tactic.TacticResult) (err error) {
	imp := &t.current
	if len(imp.stack) != len(imp.variables) {
		return ast.ErrASTDomain
	}
	md, err := model.NewModel(t.manager)
	if err != nil {
		return err
	}
	var held tactic.ModelRef
	if err = held.Set(&md.CoreField); err != nil {
		return errors.Join(err, md.Close())
	}
	defer func() { err = errors.Join(err, held.Close()) }()
	for i, v := range imp.variables {
		decl, e := t.manager.GetDecl(v.term)
		if e != nil {
			return e
		}
		number, e := t.manager.ArithmeticIntegerNumeralText(diffNeqIntText(imp.stack[i]))
		if e != nil {
			return e
		}
		if err = md.CoreField.RegisterConstant(decl, number); err != nil {
			return err
		}
	}
	converter, err := tactic.NewStoredModel(&md.CoreField)
	if err != nil {
		return err
	}
	if err = result.SetModelConverter(converter); err != nil {
		return errors.Join(err, converter.Close())
	}
	return nil
}
