package smtcore

// Source: src/smt/tactic/unit_subsumption_tactic.cpp.
import (
	"errors"
	"reflect"

	"github.com/glycerine/g3/pkg/arena"
	"github.com/glycerine/g3/pkg/ast"
	"github.com/glycerine/g3/pkg/tactic"
	uparams "github.com/glycerine/g3/pkg/util/params"
	usymbol "github.com/glycerine/g3/pkg/util/symbol"
)

type UnitSubsumptionTactic struct {
	self         *UnitSubsumptionTactic
	manager      *ast.ASTManager
	config       SMTSolverConfig
	params       uparams.ParamsRef
	paramsReady  bool
	solver       *SMTSolver
	solverBroken bool
	proxies      []ast.ASTRef
	refs         uint32
	bytes        uint64
	closed       bool
}

func NewUnitSubsumptionTactic(m *ast.ASTManager, p *uparams.ParamsRef, config SMTSolverConfig) (*UnitSubsumptionTactic, error) {
	if m == nil || config.Params == nil {
		return nil, ast.ErrASTHandle
	}
	if err := config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		return nil, err
	}
	n := uint64(reflect.TypeOf(UnitSubsumptionTactic{}).Size())
	if err := m.Budget().Reserve(n); err != nil {
		return nil, err
	}
	t := &UnitSubsumptionTactic{manager: m, config: config, bytes: n}
	t.self = t
	var err error
	t.params, err = config.Params.NewValue()
	t.paramsReady = err == nil
	if err == nil && p != nil {
		err = config.Params.Assign(&t.params, p)
	}
	if err == nil {
		t.solver, err = NewSMTSolver(m, p, "", config)
	}
	if err != nil {
		return nil, errors.Join(err, t.Close())
	}
	return t, nil
}
func (t *UnitSubsumptionTactic) open() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return arena.ErrClosed
	}
	return nil
}
func (t *UnitSubsumptionTactic) IncRef() error {
	if e := t.open(); e != nil {
		return e
	}
	t.refs++
	return nil
}
func (t *UnitSubsumptionTactic) DecRef() error {
	if e := t.open(); e != nil {
		return e
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
func (t *UnitSubsumptionTactic) Close() error {
	if t == nil || t.self != t {
		return ast.ErrASTHandle
	}
	if t.closed {
		return nil
	}
	for len(t.proxies) > 0 {
		i := len(t.proxies) - 1
		if e := t.manager.DelRef(&t.proxies[i]); e != nil {
			return e
		}
		t.proxies = t.proxies[:i]
	}
	if t.solver != nil {
		if e := t.solver.Close(); e != nil {
			return e
		}
		t.solver = nil
	}
	if t.paramsReady {
		if e := t.config.Params.Del(&t.params); e != nil {
			return e
		}
		t.paramsReady = false
	}
	t.manager.Budget().Release(t.bytes)
	t.bytes = 0
	t.closed = true
	return nil
}
func (t *UnitSubsumptionTactic) proxy(i int) (ast.ASTHandle, error) {
	if i == len(t.proxies) {
		b, e := t.manager.MkBoolSort()
		if e != nil {
			return ast.ASTHandle{}, e
		}
		h, e := t.manager.MkFreshConst("#clause", b)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		r, e := t.manager.NewRef(h)
		if e != nil {
			return ast.ASTHandle{}, e
		}
		t.proxies = append(t.proxies, r)
	}
	return t.manager.RefGet(t.proxies[i])
}
func (t *UnitSubsumptionTactic) Apply(g *tactic.Goal, out *tactic.TacticResult) (resultErr error) {
	if err := t.open(); err != nil {
		return err
	}
	if g == nil || out == nil || g.Manager() != t.manager {
		return ast.ErrASTHandle
	}
	if t.solverBroken {
		if err := t.rebuildSolver(); err != nil {
			return err
		}
	}
	defer func() {
		if resultErr != nil {
			t.solverBroken = true
		}
	}()
	if err := out.Prepare(t.manager); err != nil {
		return err
	}
	count, err := g.Size()
	if err != nil {
		return err
	}
	base, err := t.solver.ScopeLevel()
	if err != nil {
		return err
	}
	defer func() {
		level, e := t.solver.ScopeLevel()
		if e != nil {
			resultErr = errors.Join(resultErr, e)
			return
		}
		if level > base {
			resultErr = errors.Join(resultErr, t.solver.Pop(level-base))
		}
	}()
	if err = t.solver.Push(); err != nil {
		return err
	}
	clauses := make([]ast.ASTHandle, count)
	for i := uint32(0); i < count; i++ {
		clauses[i], err = t.proxy(int(i))
		if err != nil {
			return err
		}
		form, e := g.Form(i)
		if e != nil {
			return e
		}
		eq, e := t.manager.MkIff(clauses[i], form)
		if e != nil {
			return e
		}
		if e = t.solver.Assert(eq); e != nil {
			return e
		}
	}
	if err = t.solver.Push(); err != nil {
		return err
	}
	deleted := make([]bool, count)
	for i := uint32(0); i < count; i++ {
		if err = t.solver.Push(); err != nil {
			return err
		}
		for j, clause := range clauses {
			expr := clause
			if uint32(j) == i {
				expr, err = t.manager.MkNot(clause)
			} else if deleted[j] {
				continue
			}
			if err == nil {
				err = t.solver.Assert(expr)
			}
			if err != nil {
				return err
			}
		}
		if err = t.solver.Push(); err != nil {
			return err
		}
		deleted[i] = t.solver.kernel.assertedFormulasInconsistent() || t.solver.kernel.internalizer.Inconsistent()
		if err = t.solver.Pop(2); err != nil {
			return err
		}
		if deleted[i] {
			if err = t.traceRemoval(i); err != nil {
				return err
			}
		}
	}
	truth, err := t.manager.MkTrue()
	if err != nil {
		return err
	}
	for i, drop := range deleted {
		if drop {
			if err = g.Update(uint32(i), truth, ast.ASTHandle{}, ast.ASTDependency{}); err != nil {
				return err
			}
		}
	}
	if err = g.ElimTrue(); err != nil {
		return err
	}
	if err = out.AddGoal(g); err != nil {
		return err
	}
	if err = t.solver.Pop(2); err != nil {
		return err
	}
	return t.traceResult(g)
}

// A failed source push can leave solver_na2as and its core at different
// scope levels. Rebuild the private context before reusing the tactic while
// preserving the constructor's solver parameters, which UpdateParams does not
// apply to the existing context.
func (t *UnitSubsumptionTactic) rebuildSolver() error {
	old := t.solver
	next, err := NewSMTSolver(t.manager, &old.params, "", t.config)
	if err != nil {
		return err
	}
	if err = old.Close(); err != nil {
		return errors.Join(err, next.Close())
	}
	t.solver = next
	t.solverBroken = false
	return nil
}
func (t *UnitSubsumptionTactic) UpdateParams(p *uparams.ParamsRef) error {
	if e := t.open(); e != nil {
		return e
	}
	return t.config.Params.Assign(&t.params, p)
}
func (t *UnitSubsumptionTactic) CollectParamDescrs(*uparams.ParamDescrs) error { return t.open() }
func (*UnitSubsumptionTactic) CollectStatistics() map[string]float64           { return nil }
func (t *UnitSubsumptionTactic) ResetStatistics() error                        { return t.open() }
func (t *UnitSubsumptionTactic) Cleanup() error                                { return t.open() }
func (t *UnitSubsumptionTactic) Reset() error                                  { return t.open() }
func (t *UnitSubsumptionTactic) SetLogic(usymbol.Symbol) error                 { return t.open() }
func (t *UnitSubsumptionTactic) SetProgressCallback(func(uint32)) error        { return t.open() }
func (t *UnitSubsumptionTactic) Translate(m *ast.ASTManager) (tactic.Tactic, error) {
	if e := t.open(); e != nil {
		return nil, e
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if err := t.config.Params.CheckServices(m.Symbols, m.NumeralService()); err != nil {
		if t.config.TargetConfig == nil {
			return nil, err
		}
		config, err := t.config.TargetConfig(m)
		if err != nil {
			return nil, err
		}
		return t.TranslateWithConfig(m, config)
	}
	return NewUnitSubsumptionTactic(m, &t.params, t.config)
}

// TranslateWithConfig rebuilds the source tactic using the target services.
func (t *UnitSubsumptionTactic) TranslateWithConfig(m *ast.ASTManager, config SMTSolverConfig) (result tactic.Tactic, resultErr error) {
	if err := t.open(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ast.ErrASTHandle
	}
	if config.Params == nil {
		return nil, uparams.ErrParamsOwner
	}
	params, err := t.config.Params.TranslateTo(&t.params, config.Params)
	if err != nil {
		return nil, err
	}
	var next *UnitSubsumptionTactic
	defer func() {
		resultErr = errors.Join(resultErr, config.Params.Del(&params))
		if resultErr != nil && next != nil {
			resultErr = errors.Join(resultErr, next.Close())
			result = nil
		}
	}()
	next, err = NewUnitSubsumptionTactic(m, &params, config)
	if err != nil {
		return nil, err
	}
	return next, nil
}
